package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const xhttpNumericSubscription = `proxies:
  - name: XHTTP numeric options
    type: vless
    server: 127.0.0.1
    port: 9
    uuid: 01234567-89ab-cdef-0123-456789abcdef
    network: xhttp
    xhttp-opts:
      path: /test
      mode: packet-up
      sc-max-each-post-bytes: 1000000
      x-padding-bytes: "100-1000"
      xmux:
        h-max-request-times: 1000000
`

func xhttpNumericConfig(t *testing.T) Config {
	t.Helper()
	servers, err := ParseSubscription([]byte(xhttpNumericSubscription), "test")
	if err != nil {
		t.Fatal(err)
	}
	c := DefaultConfig()
	c.Servers, c.Selected = servers, servers[0].ID
	c.Settings.TUN, c.Settings.SystemProxy = false, false
	c.Settings.GroupMode = "select"
	return c
}

func checkXHTTPNumbers(t *testing.T, data []byte) {
	t.Helper()
	// This opt-in check uses a temporary config and never starts TUN or a proxy.
	if binary := os.Getenv("MIHOMO_TEST_BINARY"); binary != "" {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(ctx, binary, "-t", "-d", dir, "-f", path).CombinedOutput(); err != nil {
			t.Fatalf("Mihomo rejected XHTTP config: %v\n%s", err, output)
		}
	}
	for _, value := range []string{"sc-max-each-post-bytes: 1000000", "h-max-request-times: 1000000", "x-padding-bytes: 100-1000"} {
		if !strings.Contains(string(data), value) {
			t.Errorf("missing %q in generated YAML", value)
		}
	}
}

func TestXHTTPNumbersAfterConfigRoundTrips(t *testing.T) {
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown() })
	c := xhttpNumericConfig(t)
	if err = m.Save(c); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(m.dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeConfig(saved)
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(m.dir, "backup.json")
	if err = m.Export(backupPath); err != nil {
		t.Fatal(err)
	}
	backup, err := ReadBackup(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	for name, config := range map[string]Config{"import": c, "snapshot": m.Config(), "saved JSON": decoded, "backup": backup} {
		t.Run(name, func(t *testing.T) {
			data, err := config.YAML("127.0.0.1:19099", "test", 19098)
			if err != nil {
				t.Fatal(err)
			}
			checkXHTTPNumbers(t, data)
		})
	}
}

func TestXHTTPNumbersInLatencyProbe(t *testing.T) {
	m, runner, _ := testManager(t)
	c := xhttpNumericConfig(t)
	c.Revision = m.Config().Revision
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	proxy, err := m.Config().Servers[0].Proxy()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.probeSavedServer(context.Background(), "unused-fake-core", proxy, false, 1000, "https://example.invalid"); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(runner.lastConfig)
	if err != nil {
		t.Fatal(err)
	}
	checkXHTTPNumbers(t, data)
}

func TestXHTTPNumbersInReload(t *testing.T) {
	m, _, _, _ := setupLiveSettings(t)
	var payload string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"mixed-port":45678,"mode":"rule","tun":{"enable":false}}`))
			return
		}
		var body struct{ Payload string }
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		payload = body.Payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	m.endpoint = server.URL
	c := xhttpNumericConfig(t)
	c.Revision = m.Config().Revision
	if err := m.SaveLive(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	// Reload from the saved JSON snapshot, as on reconnect/restart.
	if err := m.ReloadConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	checkXHTTPNumbers(t, []byte(payload))
}
