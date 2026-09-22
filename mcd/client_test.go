package mcd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"github.com/huey1in/mdlc/mcdsign"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	c, err := NewClient(Config{BaseURL: s.URL, Credentials: mcdsign.Credentials{AccessKey: "test", SecretKey: "secret"}, AESKey: "0123456789abcdef", DeviceID: "device"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestMenuEmptyAndReservation(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("biz_from") != "1006" || r.Header.Get("biz_scenario") != "101" {
			t.Error("wrong business headers")
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "hmac-auth-v1#test#") {
			t.Error("custom credentials unused")
		}
		if r.URL.Query().Get("time") == "10:45" {
			io.WriteString(w, `{"success":true,"code":200,"data":{"menu":[{"categoryName":"test"}]}}`)
		} else {
			io.WriteString(w, `{"success":true,"code":200}`)
		}
	})
	o := MenuOptions{StoreCode: "1450586", DayPartCode: "8"}
	if _, err := c.Menu(context.Background(), o); !errors.Is(err, ErrNoData) {
		t.Fatalf("error=%v", err)
	}
	o.Date = "2026-09-23"
	o.Time = "10:45"
	r, err := c.Menu(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Menu []json.RawMessage }
	if err = r.Decode(&data); err != nil || len(data.Menu) != 1 {
		t.Fatal("missing menu")
	}
}
func TestErrorsAndCancellation(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"success":false,"code":706010,"message":"rejected","traceId":"trace"}`)
	})
	_, err := c.NearbyStores(context.Background(), 31, 121)
	var api *APIError
	if !errors.As(err, &api) || api.Code != 706010 || api.TraceID != "trace" {
		t.Fatalf("error=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = c.NearbyStores(ctx, 31, 121); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}
func TestValidationBeforeNetwork(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid input reached network") })
	if _, err := c.Menu(context.Background(), MenuOptions{StoreCode: "../bad"}); err == nil {
		t.Fatal("store accepted")
	}
	if _, err := c.Menu(context.Background(), MenuOptions{StoreCode: "1", Date: "2026-09-23"}); err == nil {
		t.Fatal("unpaired date accepted")
	}
	if _, err := c.NearbyStores(context.Background(), math.NaN(), 121); err == nil {
		t.Fatal("NaN accepted")
	}
	if _, err := c.SendSMS(context.Background(), "invalid", "86", ""); err == nil {
		t.Fatal("phone accepted")
	}
	if _, err := c.LoginSMS(context.Background(), "13800138000", "86", "abc123"); err == nil {
		t.Fatal("code accepted")
	}
	if _, err := c.Do(context.Background(), "GET", "https://other.test/bff/menu", nil, nil, nil); err == nil {
		t.Fatal("absolute URL accepted")
	}
}
func TestLoginSession(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
		}
		if r.URL.Path != "/bff/passport/login/mobile" || p["deviceInfoId"] != "device" || p["tel"] == "13800138000" {
			t.Error("incorrect login request")
		}
		io.WriteString(w, `{"success":true,"code":200,"data":{"sid":"session","meddyId":"member"}}`)
	})
	s, err := c.LoginSMS(context.Background(), "13800138000", "86", "123456")
	if err != nil || s.SID != "session" || s.DeviceID != "device" {
		t.Fatalf("login error=%v", err)
	}
	if c.config.SID != "" {
		t.Fatal("client session mutated")
	}
}
func TestRedirectAndMalformedResponse(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Location", "/bff/other")
		w.WriteHeader(302)
	})
	_, err := c.NearbyStores(context.Background(), 31, 121)
	var api *APIError
	if !errors.As(err, &api) || api.StatusCode != 302 || calls != 1 {
		t.Fatalf("redirect error=%v calls=%d", err, calls)
	}
	bad := testClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "not json") })
	if _, err = bad.NearbyStores(context.Background(), 31, 121); err == nil {
		t.Fatal("malformed response accepted")
	}
}
