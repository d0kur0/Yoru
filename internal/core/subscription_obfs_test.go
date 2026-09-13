package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDraftSubscriptionIsAtomic(t *testing.T) {
	m, _, _ := testManager(t)
	before := m.Config()
	body := "hy2://password@example.com:443?obfs=salamander&obfs-password=secret"
	m.downloadClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	source := Subscription{ID: "new", URL: "https://example.com/sub"}
	servers, err := m.FetchSubscription(context.Background(), source)
	if err != nil || len(servers) != 1 || servers[0].Subscription != "new" {
		t.Fatal("draft import failed", err)
	}
	if !reflect.DeepEqual(before, m.Config()) {
		t.Fatal("fetch mutated configuration")
	}
	body = "hy2://password@example.com:443?obfs=salamander"
	if _, err = m.FetchSubscription(context.Background(), source); err == nil || !strings.Contains(err.Error(), "Строка 1") {
		t.Fatal("missing inline error context", err)
	}
	if !reflect.DeepEqual(before, m.Config()) {
		t.Fatal("failed fetch mutated configuration")
	}
}

func TestHysteriaSecurityParameters(t *testing.T) {
	s, err := ParseLink("hy2://password@example.com?obfs=salamander&obfs-password=a%26b&insecure=1&pinSHA256=abcdef&ech=encoded-config")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Proxy()
	if err != nil {
		t.Fatal(err)
	}
	if p["obfs-password"] != "a&b" || p["skip-cert-verify"] != true || p["fingerprint"] != "abcdef" || p["ech-opts"] == nil {
		t.Fatal("lost security options")
	}
	for _, query := range []string{"obfs=unknown", "obfs=salamander", "obfs-password=secret", "insecure=invalid"} {
		if _, err := ParseLink("hy2://password@example.com?" + query); err == nil {
			t.Fatal("accepted invalid options", query)
		}
	}
}

// Opt-in check of an existing private subscription. Never prints credentials or config.
func TestPrivateSubscriptionValidation(t *testing.T) {
	path := os.Getenv("MIHOMO_SUBSCRIPTION_CHECK_CONFIG")
	if path == "" {
		t.Skip("private subscription check is opt-in")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("read failed")
	}
	var c Config
	if json.Unmarshal(data, &c) != nil || len(c.Subscriptions) == 0 {
		t.Fatal("invalid local config")
	}
	m, _, _ := testManager(t)
	m.downloadClient = &http.Client{}
	servers, err := m.FetchSubscription(context.Background(), c.Subscriptions[len(c.Subscriptions)-1])
	if err != nil {
		t.Fatal("subscription import failed", err)
	}
	cfg := DefaultConfig()
	cfg.Servers = servers
	cfg.Selected = servers[0].ID
	cfg.Settings.TUN = false
	yaml, err := cfg.YAML("127.0.0.1:19099", "test", 19098)
	if err != nil {
		t.Fatal("yaml generation failed")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if os.WriteFile(file, yaml, 0600) != nil {
		t.Fatal("write failed")
	}
	binary := os.Getenv("MIHOMO_TEST_BINARY")
	if binary == "" {
		t.Fatal("core path required")
	}
	if exec.Command(binary, "-t", "-d", dir, "-f", file).Run() != nil {
		t.Fatal("core validation failed")
	}
	t.Logf("Imported and validated %d nodes; core not started", len(servers))
}
