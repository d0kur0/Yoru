package core

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestBundleTokenOnlyOnReleaseMetadata(t *testing.T) {
	const token = "test-build-token"
	const assetURL = "https://release-assets.githubusercontent.com/asset"
	var requests []struct{ url, authorization string }
	client := bundleHTTPClient(token)
	client.Transport = bundleReleaseAuthTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, struct{ url, authorization string }{req.URL.String(), req.Header.Get("Authorization")})
			if req.URL.String() == bundleReleaseAPIURL {
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": []string{assetURL}},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
		token: token,
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, bundleReleaseAPIURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if req.Header.Get("Authorization") != "" {
		t.Fatal("original request mutated; redirect could inherit token")
	}
	for _, address := range []string{
		"https://github.com/MetaCubeX/mihomo/releases/download/v1.19.30/mihomo-darwin-arm64-v1.19.30.gz",
		"https://raw.githubusercontent.com/MetaCubeX/mihomo/v1.19.30/LICENSE",
		"https://api.github.com/repos/MetaCubeX/mihomo/releases/latest",
		"http://api.github.com/repos/MetaCubeX/mihomo/releases/tags/v1.19.30",
	} {
		res, err := client.Get(address)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
	}
	if len(requests) != 6 || requests[0].url != bundleReleaseAPIURL || requests[0].authorization != "Bearer "+token || requests[1].url != assetURL {
		t.Fatalf("unexpected requests: %+v", requests)
	}
	for _, request := range requests[1:] {
		if request.authorization != "" {
			t.Fatalf("token leaked to %s", request.url)
		}
	}
}
