package mcdsign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	AuthScheme    = "hmac-auth-v1"
	HashAlgorithm = "hmac-sha256"
)

var DefaultSignedHeaders = []string{
	"ct",
	"language",
	"p",
	"sid",
	"sv",
	"token",
	"v",
	"x-mcd-gw-v",
}

type Credentials struct {
	AccessKey string
	SecretKey string
}

// Request contains the exact Android request bytes. Query optionally supplies
// query parameters when URL has no query; URL query is authoritative.
type Request struct {
	Method  string
	URL     string
	Body    []byte
	Query   map[string]string
	Headers map[string]string
}

type SignOptions struct {
	// Time is the client time before TimeOffset is applied. Zero means time.Now().
	Time time.Time
	// TimeOffset mirrors the client/server clock correction used by the app.
	TimeOffset time.Duration
	// SignedHeaders overrides the default list when non-empty.
	SignedHeaders []string
}

type Signature struct {
	Authorization    string
	Digest           string
	Date             string
	CanonicalRequest string
	Payload          string
	SignedHeaders    []string
}

type Signer struct {
	credentials   Credentials
	signedHeaders []string
}

func NewSigner(credentials Credentials) (*Signer, error) {
	if credentials.AccessKey == "" {
		return nil, errors.New("access key is required")
	}
	if credentials.SecretKey == "" {
		return nil, errors.New("secret key is required")
	}

	return &Signer{
		credentials:   credentials,
		signedHeaders: append([]string(nil), DefaultSignedHeaders...),
	}, nil
}

// Sign applies the Android HMAC v4 signature.
func (s *Signer) Sign(req *Request, options SignOptions) (Signature, error) {
	if req == nil {
		return Signature{}, errors.New("request is required")
	}
	if req.URL == "" {
		return Signature{}, errors.New("request URL is required")
	}

	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return Signature{}, fmt.Errorf("parse request URL: %w", err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return Signature{}, errors.New("request URL must be absolute")
	}

	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	req.Headers["sv"] = "v4"
	if value, exists := lookupHeader(req.Headers, "x-mcd-gw-v"); !exists || value == "" {
		req.Headers["x-mcd-gw-v"] = "1"
	}

	signedHeaders := options.SignedHeaders
	if len(signedHeaders) == 0 {
		signedHeaders = s.signedHeaders
	}
	signedHeaders = append([]string(nil), signedHeaders...)

	now := options.Time
	if now.IsZero() {
		now = time.Now()
	}
	date := now.Add(options.TimeOffset).UTC().Format(http.TimeFormat)
	method := strings.ToUpper(req.Method)
	payload := string(req.Body)
	if method == http.MethodGet {
		payload = ""
	}

	digest := hmacSHA256Base64(payload, s.credentials.SecretKey)
	canonical := buildCanonicalRequest(
		method,
		parsedURL,
		req.Query,
		req.Headers,
		s.credentials.AccessKey,
		date,
		signedHeaders,
	)
	authSignature := hmacSHA256Base64(canonical, s.credentials.SecretKey)
	authorization := strings.Join([]string{
		AuthScheme,
		s.credentials.AccessKey,
		authSignature,
		HashAlgorithm,
		date,
		strings.Join(signedHeaders, ";"),
	}, "#")

	req.Headers["authorization"] = authorization
	req.Headers["x-hmac-digest"] = digest

	return Signature{
		Authorization:    authorization,
		Digest:           digest,
		Date:             date,
		CanonicalRequest: canonical,
		Payload:          payload,
		SignedHeaders:    signedHeaders,
	}, nil
}

func buildCanonicalRequest(
	method string,
	parsedURL *url.URL,
	query map[string]string,
	headers map[string]string,
	accessKey string,
	date string,
	signedHeaders []string,
) string {
	path := strings.ReplaceAll(strings.ReplaceAll(parsedURL.Path, "{", ""), "}", "")
	if path == "" {
		path = "/"
	}

	canonicalQuery := canonicalURLQuery(parsedURL.RawQuery)
	if parsedURL.RawQuery == "" && len(query) > 0 {
		values := url.Values{}
		for key, value := range query {
			values.Set(key, value)
		}
		canonicalQuery = canonicalURLQuery(values.Encode())
	}

	var headerBlock strings.Builder
	for index, name := range signedHeaders {
		if index > 0 {
			headerBlock.WriteByte('\n')
		}
		headerBlock.WriteString(name)
		headerBlock.WriteByte(':')
		value, _ := lookupHeader(headers, name)
		headerBlock.WriteString(value)
	}

	return strings.Join([]string{
		method,
		path,
		canonicalQuery,
		accessKey,
		date,
		headerBlock.String(),
		"",
	}, "\n")
}

// Android sorts encoded query segments and restores these five escapes.
func canonicalURLQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	segments := strings.Split(rawQuery, "&")
	sort.Strings(segments)
	replacer := strings.NewReplacer("%28", "(", "%29", ")", "%27", "'", "%21", "!", "%7E", "~")
	for i, segment := range segments {
		segments[i] = replacer.Replace(segment)
	}
	return strings.Join(segments, "&")
}

func lookupHeader(headers map[string]string, name string) (string, bool) {
	if value, exists := headers[name]; exists {
		return value, true
	}
	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return value, true
		}
	}
	return "", false
}

func hmacSHA256Base64(message string, key string) string {
	hash := hmac.New(sha256.New, []byte(key))
	_, _ = hash.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}
