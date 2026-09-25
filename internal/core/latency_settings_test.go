package core

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatencyURLSettings(t *testing.T) {
	c := DefaultConfig()
	if got := c.LatencyTestURL(); got != "https://www.gstatic.com/generate_204" {
		t.Fatalf("default latency URL: %s", got)
	}
	before := runtimeSnapshot(c)
	c.Settings.LatencyURL = "https://example.com/check"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.LatencyTestURL() != c.Settings.LatencyURL {
		t.Fatal("custom probe ignored")
	}
	if bytes.Equal(before, runtimeSnapshot(c)) {
		t.Fatal("fallback health URL change must mark config pending")
	}
	c.Settings.LatencyURL = "http://example.com/legacy-check"
	if err := c.Validate(); err != nil || c.LatencyTestURL() != c.Settings.LatencyURL {
		t.Fatalf("existing HTTP target changed: %v", err)
	}
	for _, value := range []string{"file:///etc/passwd", "https://user:password@example.com", "https://example.com/#part", "https://", "https://example.com:0/check", "https://example.com:65536/check"} {
		c.Settings.LatencyURL = value
		if c.Validate() == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestLatencyURLPassedUnchangedToMihomo(t *testing.T) {
	m, _, _ := testManager(t)
	c := m.Config()
	c.Settings.LatencyURL = "http://example.com/check?source=legacy&value=a%2Bb"
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/proxies/server-a/delay" || req.URL.Query().Get("url") != c.Settings.LatencyURL {
			t.Errorf("wrong delay target: %s", req.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"delay": 42})
	}))
	t.Cleanup(server.Close)
	m.endpoint = server.URL
	if delay, err := m.TestServerLatency(context.Background(), "a"); err != nil || delay != 42 {
		t.Fatalf("delay=%d err=%v", delay, err)
	}
}
