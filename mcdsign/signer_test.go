package mcdsign

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSignerConcurrentUse(t *testing.T) {
	signer, err := NewSigner(Credentials{AccessKey: "test-access", SecretKey: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	newRequest := func() *Request {
		return &Request{
			Method:  "POST",
			URL:     "https://example.test/resource?value=example",
			Body:    []byte(`{"example":true}`),
			Headers: map[string]string{"ct": "102", "language": "cn"},
		}
	}
	options := SignOptions{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	want, err := signer.Sign(newRequest(), options)
	if err != nil {
		t.Fatal(err)
	}
	const workers = 32
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := signer.Sign(newRequest(), options)
			if err != nil {
				results <- err
				return
			}
			if got.Authorization != want.Authorization || got.Digest != want.Digest || got.CanonicalRequest != want.CanonicalRequest {
				results <- fmt.Errorf("shared signer produced inconsistent results")
			}
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		t.Error(err)
	}
}
