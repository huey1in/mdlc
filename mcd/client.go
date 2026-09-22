// Package mcd provides an unofficial Android API client. It performs no
// automatic retries, login, or credential persistence. Clients are immutable
// after construction and may be shared across goroutines.
package mcd

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"github.com/huey1in/mdlc/mcdsign"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Version = "0.1.0"
const AndroidVersion = "7.0.36.0"

var ErrNoData = errors.New("mcd: successful response contains no data")
var ErrVerificationRequired = errors.New("mcd: additional verification required")

// APIError reports a rejected HTTP or API response without exposing its body.
type APIError struct {
	StatusCode, Code int
	Message, TraceID string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("mcd: HTTP %d API %d: %s (trace %s)", e.StatusCode, e.Code, e.Message, e.TraceID)
}

// Config is copied by NewClient. Credentials are required unless Signer is set.
// The supplied HTTPClient is copied and redirects are disabled to avoid
// forwarding credentials. AESKey is required only for SMS/login operations.
type Config struct {
	BaseURL                                                                      string
	Credentials                                                                  mcdsign.Credentials
	Signer                                                                       *mcdsign.Signer
	HTTPClient                                                                   *http.Client
	AESKey                                                                       string
	AppVersion                                                                   string
	DeviceID, DeviceD, DeviceToken, Partner, Token, SID, MeddyID, TID, RoutingID string
}
type Client struct {
	config Config
	http   *http.Client
	signer *mcdsign.Signer
}
type Response struct {
	Success bool            `json:"success"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	TraceID string          `json:"traceId"`
	Data    json.RawMessage `json:"data"`
}

// Decode preserves unknown API fields in Data and distinguishes empty success.
func (r Response) Decode(dst any) error {
	if len(r.Data) == 0 || string(r.Data) == "null" {
		return ErrNoData
	}
	return json.Unmarshal(r.Data, dst)
}
func NewClient(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api2.mcd.cn"
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("mcd: invalid base URL; expected HTTP(S) origin")
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.AppVersion == "" {
		cfg.AppVersion = AndroidVersion
	}
	if cfg.AESKey != "" {
		if _, err := aes.NewCipher([]byte(cfg.AESKey)); err != nil {
			return nil, fmt.Errorf("mcd: AES key: %w", err)
		}
	}
	signer := cfg.Signer
	if signer == nil {
		signer, err = mcdsign.NewSigner(cfg.Credentials)
		if err != nil {
			return nil, err
		}
	}
	hc := http.Client{Timeout: 25 * time.Second}
	if cfg.HTTPClient != nil {
		hc = *cfg.HTTPClient
		if hc.Timeout == 0 {
			hc.Timeout = 25 * time.Second
		}
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if cfg.DeviceToken == "" {
		cfg.DeviceToken = cfg.DeviceID
	}
	return &Client{config: cfg, http: &hc, signer: signer}, nil
}

// Do signs a GET or POST against the configured origin. Headers are copied.
// On API rejection the response is returned together with *APIError.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, payload any, headers http.Header) (Response, error) {
	var result Response
	u, err := url.Parse(path)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(path, "/bff/") || u.RawQuery != "" || u.Fragment != "" {
		return result, errors.New("mcd: path must be a /bff/ path without query")
	}
	if method != http.MethodGet && method != http.MethodPost {
		return result, errors.New("mcd: unsupported method")
	}
	if method == http.MethodGet && payload != nil {
		return result, errors.New("mcd: GET payload is not supported")
	}
	var body []byte
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return result, err
		}
	}
	endpoint := c.config.BaseURL + u.EscapedPath()
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	cfg := c.config
	for k, v := range map[string]string{"ct": "102", "language": "cn", "v": cfg.AppVersion, "p": cfg.Partner, "token": cfg.Token, "sid": cfg.SID, "meddyid": cfg.MeddyID, "mcdToken": cfg.DeviceToken, "d": cfg.DeviceD, "tid": cfg.TID, "routingid": cfg.RoutingID, "user-agent": "mcdonald_Android/" + cfg.AppVersion + " (Android)"} {
		req.Header.Set(k, v)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range headers {
		req.Header[k] = append([]string(nil), vs...)
	}
	if _, err = c.signer.SignHTTPRequest(req, nil, mcdsign.SignOptions{}); err != nil {
		return result, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return result, fmt.Errorf("mcd: request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (16<<20)+1))
	if err != nil {
		return result, err
	}
	if len(raw) > 16<<20 {
		return result, errors.New("mcd: response exceeds 16 MiB")
	}
	decodeErr := json.Unmarshal(raw, &result)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, &APIError{resp.StatusCode, result.Code, result.Message, result.TraceID}
	}
	if decodeErr != nil {
		return result, fmt.Errorf("mcd: decode response: %w", decodeErr)
	}
	if !result.Success {
		return result, &APIError{resp.StatusCode, result.Code, result.Message, result.TraceID}
	}
	return result, nil
}

func business(from, scenario string) http.Header {
	h := http.Header{}
	h.Set("biz_from", from)
	h.Set("biz_scenario", scenario)
	return h
}

var numeric = regexp.MustCompile(`^[0-9]+$`)

func (c *Client) encrypt(value string) (string, error) {
	b, err := aes.NewCipher([]byte(c.config.AESKey))
	if err != nil {
		return "", fmt.Errorf("mcd: configure AESKey: %w", err)
	}
	raw := []byte(value)
	n := b.BlockSize() - len(raw)%b.BlockSize()
	raw = append(raw, bytes.Repeat([]byte{byte(n)}, n)...)
	for i := 0; i < len(raw); i += b.BlockSize() {
		b.Encrypt(raw[i:i+b.BlockSize()], raw[i:i+b.BlockSize()])
	}
	return hex.EncodeToString(raw), nil
}
func validatePhone(phone, region string) error {
	if !numeric.MatchString(phone) || len(phone) > 15 || !numeric.MatchString(region) || len(region) > 4 {
		return errors.New("mcd: invalid phone or region")
	}
	return nil
}

// SendSMS sends one real SMS request; callers decide whether to retry.
func (c *Client) SendSMS(ctx context.Context, phone, region, securityVerification string) (Response, error) {
	if err := validatePhone(phone, region); err != nil {
		return Response{}, err
	}
	tel, err := c.encrypt(phone)
	if err != nil {
		return Response{}, err
	}
	h := business("1011", "500")
	if securityVerification != "" {
		h.Set("X-Mcd-Sec-Verify", securityVerification)
	}
	return c.Do(ctx, "POST", "/bff/passport/verifyCode/sms/send", nil, map[string]any{"tel": tel, "regionCode": region, "type": 1}, h)
}

type Session struct {
	SID        string `json:"sid"`
	MeddyID    string `json:"meddyId"`
	DeviceID   string `json:"deviceId"`
	NeedVerify bool   `json:"needVerify"`
}

// LoginSMS does not change this client's session. Construct a new client with
// the returned SID for authenticated calls. It never persists credentials.
func (c *Client) LoginSMS(ctx context.Context, phone, region, code string) (Session, error) {
	var session Session
	if err := validatePhone(phone, region); err != nil {
		return session, err
	}
	if len(code) != 6 || !numeric.MatchString(code) || c.config.DeviceID == "" {
		return session, errors.New("mcd: six digit code and device ID required")
	}
	tel, err := c.encrypt(phone)
	if err != nil {
		return session, err
	}
	encryptedCode, err := c.encrypt(code)
	if err != nil {
		return session, err
	}
	r, err := c.Do(ctx, "POST", "/bff/passport/login/mobile", nil, map[string]any{"tel": tel, "code": encryptedCode, "regionCode": region, "deviceInfoId": c.config.DeviceID, "citicRegister": false, "secondPhoneFlag": false}, business("1004", "500"))
	if err != nil {
		return session, err
	}
	if err = r.Decode(&session); err != nil {
		return session, err
	}
	session.DeviceID = c.config.DeviceID
	if session.NeedVerify {
		return session, ErrVerificationRequired
	}
	if session.SID == "" {
		return session, ErrNoData
	}
	return session, nil
}
func (c *Client) NearbyStores(ctx context.Context, latitude, longitude float64) (Response, error) {
	if math.IsNaN(latitude) || math.IsNaN(longitude) || math.IsInf(latitude, 0) || math.IsInf(longitude, 0) || latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return Response{}, errors.New("mcd: invalid coordinates")
	}
	return c.Do(ctx, "GET", "/bff/store/stores/vicinity", url.Values{"showType": {"1"}, "latitude": {strconv.FormatFloat(latitude, 'f', -1, 64)}, "longitude": {strconv.FormatFloat(longitude, 'f', -1, 64)}}, nil, business("1005", "100"))
}
func (c *Client) ReservationTimes(ctx context.Context, store string) (Response, error) {
	if !numeric.MatchString(store) {
		return Response{}, errors.New("mcd: invalid store code")
	}
	return c.Do(ctx, "GET", "/bff/store/stores/"+store+"/menu/reservationTimeOption", url.Values{"beType": {"1"}}, nil, business("1004", "101"))
}

// MenuOptions selects an in-store menu. Date and Time must be supplied together
// and should come from ReservationTimes when the store is closed.
type MenuOptions struct{ StoreCode, DayPartCode, Date, Time string }

func (c *Client) Menu(ctx context.Context, o MenuOptions) (Response, error) {
	if !numeric.MatchString(o.StoreCode) || (o.DayPartCode != "" && !numeric.MatchString(o.DayPartCode)) {
		return Response{}, errors.New("mcd: invalid store or daypart")
	}
	if (o.Date == "") != (o.Time == "") {
		return Response{}, errors.New("mcd: date and time must be supplied together")
	}
	q := url.Values{"storeCode": {o.StoreCode}, "orderType": {"1"}, "beType": {"1"}}
	if o.DayPartCode != "" {
		q.Set("dayPartCode", o.DayPartCode)
	}
	if o.Date != "" {
		if _, err := time.Parse("2006-01-02", o.Date); err != nil {
			return Response{}, err
		}
		if _, err := time.Parse("15:04", o.Time); err != nil {
			return Response{}, err
		}
		q.Set("date", o.Date)
		q.Set("time", o.Time)
	}
	r, err := c.Do(ctx, "GET", "/bff/spc/menu", q, nil, business("1006", "101"))
	if err == nil {
		var data json.RawMessage
		err = r.Decode(&data)
	}
	return r, err
}
