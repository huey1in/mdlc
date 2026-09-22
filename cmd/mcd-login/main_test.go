package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"github.com/huey1in/mdlc/mcdsign"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPKLoginRequests(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Error(err)
		}
		if r.Method != "POST" {
			t.Error("expected POST")
		}
		for k, v := range map[string]string{"ct": "102", "language": "cn", "v": "7.0.36.0", "biz_scenario": "500", "p": "test-partner", "user-agent": "mcdonald_Android/7.0.36.0 (Android)"} {
			if r.Header.Get(k) != v {
				t.Errorf("header %s=%q", k, r.Header.Get(k))
			}
		}
		for _, k := range []string{"referer", "xweb_xhr"} {
			if r.Header.Get(k) != "" {
				t.Errorf("unexpected browser header %s", k)
			}
		}
		if payload["tel"] != "f3d1143965ff2f9d447dfa1b437437b2" || payload["regionCode"] != "86" {
			t.Error("wrong phone or region")
		}
		if calls == 1 {
			if r.URL.Path != "/bff/passport/verifyCode/sms/send" || r.Header.Get("biz_from") != "1011" {
				t.Error("wrong SMS route")
			}
			if len(payload) != 3 || payload["type"] != float64(1) {
				t.Errorf("SMS shape: %s", body)
			}
			if r.Header.Get("X-Mcd-Sec-Verify") != "test-verification" {
				t.Error("missing verification header")
			}
		} else {
			if r.URL.Path != "/bff/passport/login/mobile" || r.Header.Get("biz_from") != "1004" {
				t.Error("wrong login route")
			}
			if len(payload) != 6 || payload["code"] != "49cb154777e5cbc8de68d4ec709a19d2" || payload["deviceInfoId"] != "test-device" || payload["citicRegister"] != false || payload["secondPhoneFlag"] != false {
				t.Errorf("login shape: %s", body)
			}
			if r.Header.Get("X-Mcd-Sec-Verify") != "" {
				t.Error("SMS-only header leaked")
			}
		}
		parts := strings.Split(r.Header.Get("Authorization"), "#")
		if len(parts) != 6 {
			t.Error("invalid authorization")
			return
		}
		if parts[1] != "zWB5ZZDSaV5jjGoVz255" || parts[5] != "ct;language;p;sid;sv;token;v;x-mcd-gw-v" {
			t.Error("wrong APK profile")
		}
		canonical := "POST\n" + r.URL.Path + "\n\n" + parts[1] + "\n" + parts[4] + "\nct:102\nlanguage:cn\np:test-partner\nsid:\nsv:v4\ntoken:\nv:7.0.36.0\nx-mcd-gw-v:1\n"
		mac := func(b []byte) string {
			h := hmac.New(sha256.New, []byte("Lch2Z94uveZQjGL6OMVUwqGNkEzZjfcVhKPwVHSu"))
			h.Write(b)
			return base64.StdEncoding.EncodeToString(h.Sum(nil))
		}
		if parts[2] != mac([]byte(canonical)) {
			t.Error("authorization mismatch")
		}
		if r.Header.Get("X-HMAC-DIGEST") != mac(body) {
			t.Error("body signature mismatch")
		}
		io.WriteString(w, `{"success":true,"code":0,"data":{"sid":"test-session"}}`)
	}))
	defer server.Close()
	c := client{config: config{baseURL: server.URL, phone: "13800138000", region: "86", deviceID: "test-device", partner: "test-partner", secVerify: "test-verification"}, http: server.Client(), signer: mcdsign.NewAndroid7036Signer()}
	if _, err := c.sendCode(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := c.verifyCode(context.Background(), "123456"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("calls=%d", calls)
	}
}

func TestLoginCompletion(t *testing.T) {
	for _, tc := range []struct {
		data  string
		valid bool
	}{
		{`{"sid":"test-session"}`, true},
		{`{"sid":"test-session","needVerify":true}`, false},
		{`{}`, false}, {`null`, false}, {`invalid`, false},
	} {
		if err := validateLoginData(json.RawMessage(tc.data)); (err == nil) != tc.valid {
			t.Errorf("data=%s error=%v", tc.data, err)
		}
	}
}
