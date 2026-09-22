package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"github.com/huey1in/mdlc/mcd"
	"github.com/huey1in/mdlc/mcdsign"
)

const (
	defaultBaseURL = "https://api2.mcd.cn"
	phoneAESKey    = "w8ZJ4wrUl7dDB1A7"
	apkVersion     = "7.0.36.0"
)

type config struct {
	baseURL   string
	phone     string
	code      string
	region    string
	sid       string
	meddyID   string
	deviceD   string
	deviceID  string
	partner   string
	token     string
	secVerify string
	mcdToken  string
	tid       string
	routingID string
	sendOnly  bool
	timeout   time.Duration
}

type client struct {
	config config
	http   *http.Client
	signer *mcdsign.Signer
}

type apiResponse struct {
	Success bool            `json:"success"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type sendPayload struct {
	Telephone string `json:"tel"`
	Type      int    `json:"type"`
	Region    string `json:"regionCode"`
}

type verifyPayload struct {
	Telephone       string `json:"tel"`
	Code            string `json:"code"`
	DeviceInfoID    string `json:"deviceInfoId"`
	Region          string `json:"regionCode"`
	CiticRegister   bool   `json:"citicRegister"`
	SecondPhoneFlag bool   `json:"secondPhoneFlag"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("create cookie jar: %w", err)
	}
	c := &client{
		config: cfg,
		http: &http.Client{
			Jar:           jar,
			Timeout:       cfg.timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		signer: mcdsign.NewAndroid7036Signer(),
	}

	ctx := context.Background()
	code := strings.TrimSpace(cfg.code)
	if code == "" {
		response, err := c.sendCode(ctx)
		if err != nil {
			return err
		}
		printResponse("send", response)
		if !response.Success {
			return fmt.Errorf("send SMS rejected: code=%d message=%s", response.Code, response.Message)
		}
		if cfg.sendOnly {
			return nil
		}

		fmt.Print("请输入收到的 6 位验证码: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read verification code: %w", err)
		}
		code = strings.TrimSpace(line)
	}
	if len(code) != 6 {
		return errors.New("verification code must contain 6 digits")
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return errors.New("verification code must contain 6 digits")
		}
	}

	response, err := c.verifyCode(ctx, code)
	if err != nil {
		return err
	}
	printResponse("verify", response)
	if !response.Success {
		return fmt.Errorf("login rejected: code=%d message=%s", response.Code, response.Message)
	}

	if err := validateLoginData(response.Data); err != nil {
		return err
	}
	fmt.Println("登录成功：响应中已取得 sid。")
	return nil
}

func validateLoginData(data json.RawMessage) error {
	var result struct {
		SID        string `json:"sid"`
		NeedVerify bool   `json:"needVerify"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("decode login data: %w", err)
	}
	if result.NeedVerify {
		return errors.New("服务器要求进一步安全验证，尚未确认登录完成")
	}
	if result.SID == "" {
		return errors.New("登录响应没有 sid，尚未取得凭证")
	}
	return nil
}

func parseConfig() (config, error) {
	var cfg config

	flag.StringVar(&cfg.baseURL, "base-url", defaultBaseURL, "API base URL")
	flag.StringVar(&cfg.phone, "phone", os.Getenv("MCD_PHONE"), "mobile telephone number")
	flag.StringVar(&cfg.code, "code", os.Getenv("MCD_SMS_CODE"), "existing 6-digit SMS code; skips sending")
	flag.StringVar(&cfg.region, "region", envOrDefault("MCD_REGION", "86"), "telephone country/region code")
	flag.StringVar(&cfg.sid, "sid", os.Getenv("MCD_SID"), "existing session ID; empty during a new SMS login")
	flag.StringVar(&cfg.meddyID, "meddy-id", os.Getenv("MCD_MEDDY_ID"), "optional meddyId request header")
	flag.StringVar(&cfg.deviceD, "device-d", os.Getenv("MCD_DEVICE_D"), "optional device risk-control d header")
	flag.StringVar(&cfg.deviceID, "device-id", os.Getenv("MCD_DEVICE_ID"), "APK deviceInfoId; required, reuse for send and login")
	flag.StringVar(&cfg.partner, "partner", os.Getenv("MCD_PARTNER"), "APK p header")
	flag.StringVar(&cfg.token, "token", os.Getenv("MCD_TOKEN"), "APK token header")
	flag.StringVar(&cfg.secVerify, "sec-verify", os.Getenv("MCD_SEC_VERIFY"), "completed security verification result")
	flag.StringVar(&cfg.mcdToken, "mcd-token", os.Getenv("MCD_DEVICE_TOKEN"), "APK mcdToken; defaults to device-id")
	flag.StringVar(&cfg.tid, "tid", os.Getenv("MCD_TID"), "APK tid header")
	flag.StringVar(&cfg.routingID, "routing-id", os.Getenv("MCD_ROUTING_ID"), "APK routingid header")
	flag.BoolVar(&cfg.sendOnly, "send-only", false, "send the SMS and exit without prompting")
	flag.DurationVar(&cfg.timeout, "timeout", 20*time.Second, "HTTP request timeout")
	flag.Parse()

	cfg.phone = strings.TrimSpace(cfg.phone)
	cfg.region = strings.TrimPrefix(strings.TrimSpace(cfg.region), "+")
	cfg.baseURL = strings.TrimRight(strings.TrimSpace(cfg.baseURL), "/")
	if cfg.phone == "" {
		return config{}, errors.New("phone is required; use -phone or MCD_PHONE")
	}
	if cfg.region == "" {
		return config{}, errors.New("region is required")
	}
	if cfg.baseURL == "" {
		return config{}, errors.New("base URL is required")
	}
	cfg.deviceID = strings.TrimSpace(cfg.deviceID)
	if cfg.deviceID == "" {
		return config{}, errors.New("APK device ID is required; use -device-id or MCD_DEVICE_ID")
	}
	if cfg.mcdToken == "" {
		cfg.mcdToken = cfg.deviceID
	}
	if cfg.timeout <= 0 {
		return config{}, errors.New("timeout must be positive")
	}
	return cfg, nil
}

func (c *client) sendCode(ctx context.Context) (apiResponse, error) {
	payload := sendPayload{
		Telephone: encryptHex(c.config.phone),
		Type:      1,
		Region:    c.config.region,
	}
	return c.post(ctx, "/bff/passport/verifyCode/sms/send", payload)
}

func (c *client) verifyCode(ctx context.Context, code string) (apiResponse, error) {
	payload := verifyPayload{Telephone: encryptHex(c.config.phone), Code: encryptHex(code), DeviceInfoID: c.config.deviceID, Region: c.config.region}
	return c.post(ctx, "/bff/passport/login/mobile", payload)
}

func (c *client) post(ctx context.Context, path string, payload any) (apiResponse, error) {
	cfg := c.config
	sdk, err := mcd.NewClient(mcd.Config{BaseURL: cfg.baseURL, Signer: c.signer, HTTPClient: c.http, DeviceID: cfg.deviceID, DeviceToken: cfg.mcdToken, DeviceD: cfg.deviceD, Partner: cfg.partner, Token: cfg.token, SID: cfg.sid, MeddyID: cfg.meddyID, TID: cfg.tid, RoutingID: cfg.routingID})
	if err != nil {
		return apiResponse{}, err
	}
	h := http.Header{}
	h.Set("biz_from", "1004")
	h.Set("biz_scenario", "500")
	if path == "/bff/passport/verifyCode/sms/send" {
		h.Set("biz_from", "1011")
		if cfg.secVerify != "" {
			h.Set("X-Mcd-Sec-Verify", cfg.secVerify)
		}
	}
	r, err := sdk.Do(ctx, "POST", path, nil, payload, h)
	return apiResponse{Success: r.Success, Code: r.Code, Message: r.Message, Data: r.Data}, err
}

func encryptHex(plaintext string) string {
	block, err := aes.NewCipher([]byte(phoneAESKey))
	if err != nil {
		panic(err)
	}
	padding := block.BlockSize() - len([]byte(plaintext))%block.BlockSize()
	padded := append([]byte(nil), []byte(plaintext)...)
	padded = append(padded, bytes.Repeat([]byte{byte(padding)}, padding)...)
	ciphertext := make([]byte, len(padded))
	for offset := 0; offset < len(padded); offset += block.BlockSize() {
		block.Encrypt(ciphertext[offset:offset+block.BlockSize()], padded[offset:offset+block.BlockSize()])
	}
	return hex.EncodeToString(ciphertext)
}

func printResponse(stage string, response apiResponse) {
	raw, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		fmt.Printf("[%s] success=%v code=%d message=%s\n", stage, response.Success, response.Code, response.Message)
		return
	}
	fmt.Printf("[%s] %s\n", stage, raw)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
