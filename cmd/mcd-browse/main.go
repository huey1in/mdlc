package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/huey1in/mdlc/mcd"
	"github.com/huey1in/mdlc/mcdsign"
	"net/http"
	"net/url"
	"os"
	"regexp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	path := flag.String("path", "/bff/store/stores/vicinity?showType=1&latitude=31.2304&longitude=121.4737", "read-only API path and query")
	output := flag.String("output", "research/apk-stores.json", "response file")
	session := flag.String("session", "mcd-apk-session.json", "session file")
	bizFrom := flag.String("biz-from", "1005", "business source header")
	bizScenario := flag.String("biz-scenario", "100", "business scenario header")
	host := flag.String("host", "api2.mcd.cn", "APK API host")
	flag.Parse()
	u, err := url.Parse(*path)
	if err != nil {
		return err
	}
	storeRead := regexp.MustCompile(`^/bff/store/stores/[0-9]+(/menu/(daypartsTimeoption|reservationTimeOption))?$`).MatchString(u.Path)
	if u.IsAbs() || u.Host != "" || (!storeRead && u.Path != "/bff/store/stores/vicinity" && u.Path != "/bff/spc/menu") {
		return fmt.Errorf("only store list and menu endpoints allowed")
	}
	raw, err := os.ReadFile(*session)
	if err != nil {
		return err
	}
	var s struct {
		SID      string `json:"sid"`
		MeddyID  string `json:"meddyId"`
		DeviceID string `json:"deviceId"`
	}
	if err = json.Unmarshal(raw, &s); err != nil {
		return err
	}
	if s.SID == "" {
		return fmt.Errorf("missing sid")
	}
	if *host != "api.mcd.cn" && *host != "api2.mcd.cn" {
		return fmt.Errorf("unsupported API host")
	}
	sdk, err := mcd.NewClient(mcd.Config{BaseURL: "https://" + *host, Signer: mcdsign.NewAndroid7036Signer(), SID: s.SID, MeddyID: s.MeddyID, DeviceID: s.DeviceID})
	if err != nil {
		return err
	}
	h := http.Header{}
	h.Set("biz_from", *bizFrom)
	h.Set("biz_scenario", *bizScenario)
	result, err := sdk.Do(context.Background(), "GET", u.Path, u.Query(), nil, h)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(*output, body, 0600); err != nil {
		return err
	}
	fmt.Printf("success=%v code=%d output=%s bytes=%d\n", result.Success, result.Code, *output, len(body))
	return nil
}
