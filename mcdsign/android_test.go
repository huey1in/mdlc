package mcdsign

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAndroidQueryCanonicalization(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		req := &Request{Method: method, URL: "https://api2.mcd.cn/bff/spc/menu?time=10%3A45&x=%28%21%7E%29&tag=b&tag=a&space=a%20b", Query: map[string]string{"ignored": "value"}}
		result, err := NewAndroid7036Signer().Sign(req, SignOptions{Time: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)})
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(result.CanonicalRequest, "\n")
		if lines[2] != "space=a%20b&tag=a&tag=b&time=10%3A45&x=(!~)" {
			t.Fatalf("query=%s", lines[2])
		}
		if !strings.HasSuffix(result.Authorization, "#ct;language;p;sid;sv;token;v;x-mcd-gw-v") {
			t.Fatal("wrong signed headers")
		}
	}
}

func TestAndroidHTTPPreservesBody(t *testing.T) {
	const body = `{"tel":"test","type":1}`
	req, err := http.NewRequest("POST", "https://api2.mcd.cn/bff/passport/verifyCode/sms/send", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.GetBody = nil
	sig, err := NewAndroid7036Signer().SignHTTPRequest(req, nil, SignOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body || req.Header.Get("Authorization") != sig.Authorization || req.Header.Get("X-HMAC-DIGEST") != sig.Digest {
		t.Fatal("wire request differs from signature")
	}
}
