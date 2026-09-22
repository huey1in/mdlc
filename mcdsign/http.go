package mcdsign

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// SignHTTPRequest signs an existing HTTP request without sending it.
//
// Query optionally supplies parameters when the URL has no query string.
// The exact bytes in req.Body are used as the payload. Reading the body
// does not consume it for the caller or the HTTP transport.
func (s *Signer) SignHTTPRequest(req *http.Request, query map[string]string, options SignOptions) (Signature, error) {
	if req == nil {
		return Signature{}, fmt.Errorf("request is required")
	}
	if req.URL == nil {
		return Signature{}, fmt.Errorf("request URL is required")
	}
	if req.Header == nil {
		req.Header = make(http.Header)
	}

	body, err := readRequestBody(req)
	if err != nil {
		return Signature{}, fmt.Errorf("read request body: %w", err)
	}

	headers := make(map[string]string, len(req.Header))
	for name := range req.Header {
		headers[name] = req.Header.Get(name)
	}

	signature, err := s.Sign(&Request{
		Method:  req.Method,
		URL:     req.URL.String(),
		Body:    body,
		Query:   query,
		Headers: headers,
	}, options)
	if err != nil {
		return Signature{}, err
	}

	sv, _ := lookupHeader(headers, "sv")
	gatewayVersion, _ := lookupHeader(headers, "x-mcd-gw-v")
	req.Header.Set("sv", sv)
	req.Header.Set("x-mcd-gw-v", gatewayVersion)
	req.Header.Set("authorization", signature.Authorization)
	req.Header.Set("x-hmac-digest", signature.Digest)
	return signature, nil
}

func readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}

	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		defer body.Close()
		return io.ReadAll(body)
	}

	originalBody := req.Body
	body, err := io.ReadAll(originalBody)
	closeErr := originalBody.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return body, nil
}
