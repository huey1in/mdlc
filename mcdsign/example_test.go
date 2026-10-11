package mcdsign_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/huey1in/mdlc/mcdsign"
)

func ExampleSigner_SignHTTPRequest() {
	signer, err := mcdsign.NewSigner(mcdsign.Credentials{
		AccessKey: "example-access-key",
		SecretKey: "example-secret-key",
	})
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodPost, "https://example.test/resource", strings.NewReader(`{"example":true}`))
	if err != nil {
		panic(err)
	}
	defer req.Body.Close()
	if _, err := signer.SignHTTPRequest(req, nil, mcdsign.SignOptions{}); err != nil {
		panic(err)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(req.Header.Get("Authorization") != "")
	fmt.Println(string(body))
	// Output:
	// true
	// {"example":true}
}
