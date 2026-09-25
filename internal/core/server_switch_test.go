package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

type switchController struct {
	mu       sync.Mutex
	payloads []map[string]any
	failNext bool
}

func (c *switchController) handler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/configs" {
		http.NotFound(w, req)
		return
	}
	switch req.Method {
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mixed-port": 45678,
			"mode":       "rule",
			"tun":        map[string]bool{"enable": false},
		})
	case http.MethodPut:
		if req.URL.Query().Get("force") != "false" {
			http.Error(w, "unexpected forced reload", http.StatusBadRequest)
			return
		}
		var body struct {
			Payload string `json:"payload"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var doc map[string]any
		if err := yaml.Unmarshal([]byte(body.Payload), &doc); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		c.mu.Lock()
		c.payloads = append(c.payloads, doc)
		fail := c.failNext
		c.failNext = false
		c.mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
	}
}

func (c *switchController) takePayloads() []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := c.payloads
	c.payloads = nil
	return result
}

func (c *switchController) failOnce() {
	c.mu.Lock()
	c.failNext = true
	c.mu.Unlock()
}

func setupServerSwitch(t *testing.T, mode string) (*Manager, *fakeRunner, *switchController, map[string]any) {
	t.Helper()
	m, runner, _, _ := setupLiveSettings(t)
	config := m.Config()
	second := config.Servers[0]
	second.ID, second.Name = "b", "B"
	config.Servers = append(config.Servers, second)
	config.Settings.GroupMode = mode
	if err := m.Save(config); err != nil {
		t.Fatal(err)
	}
	controller := &switchController{}
	server := httptest.NewServer(http.HandlerFunc(controller.handler))
	t.Cleanup(server.Close)
	m.endpoint = server.URL
	if err := m.ReloadConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	baseline := controller.takePayloads()
	if len(baseline) != 1 {
		t.Fatalf("baseline reloads = %d, want 1", len(baseline))
	}
	return m, runner, controller, baseline[0]
}

func switchGroup(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	groups, ok := doc["proxy-groups"].([]any)
	if !ok || len(groups) == 0 {
		t.Fatalf("missing proxy group: %#v", doc["proxy-groups"])
	}
	group, ok := groups[0].(map[string]any)
	if !ok {
		t.Fatalf("invalid proxy group: %#v", groups[0])
	}
	return group
}

func checkSwitchPayload(t *testing.T, doc, baseline map[string]any) {
	t.Helper()
	if !reflect.DeepEqual(doc["tun"], baseline["tun"]) {
		t.Fatal("reload changed TUN configuration")
	}
	if doc["mixed-port"] != baseline["mixed-port"] || doc["mixed-port"] != 45678 {
		t.Fatalf("reload changed mixed port: %#v", doc["mixed-port"])
	}
}

func TestServerSelectionAndGroupSettingsApplyWithoutCoreRestart(t *testing.T) {
	modes := []string{"select", "fallback", "url-test", "load-balance"}
	for i, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			m, runner, controller, baseline := setupServerSwitch(t, mode)
			process, started := m.process, m.started

			config := m.Config()
			config.Selected = "b"
			if err := m.SaveLive(context.Background(), config); err != nil {
				t.Fatal(err)
			}
			payloads := controller.takePayloads()
			if len(payloads) != 1 {
				t.Fatalf("selection reloads = %d, want 1", len(payloads))
			}
			checkSwitchPayload(t, payloads[0], baseline)
			group := switchGroup(t, payloads[0])
			if group["type"] != mode || !reflect.DeepEqual(group["proxies"], []any{"server-b", "server-a"}) {
				t.Fatalf("selection not applied: %#v", group)
			}

			config = m.Config()
			nextMode := modes[(i+1)%len(modes)]
			config.Settings.GroupMode = nextMode
			config.Settings.HealthInterval = 45
			if err := m.SaveLive(context.Background(), config); err != nil {
				t.Fatal(err)
			}
			payloads = controller.takePayloads()
			if len(payloads) != 1 {
				t.Fatalf("group settings reloads = %d, want 1", len(payloads))
			}
			checkSwitchPayload(t, payloads[0], baseline)
			group = switchGroup(t, payloads[0])
			if group["type"] != nextMode || !reflect.DeepEqual(group["proxies"], []any{"server-b", "server-a"}) {
				t.Fatalf("group settings not applied: %#v", group)
			}
			if nextMode != "select" && group["interval"] != 45 {
				t.Fatalf("health interval not applied: %#v", group)
			}
			if m.process != process || m.started != started || runner.starts != 1 {
				t.Fatal("core restarted during server switch")
			}
			if !reflect.DeepEqual(m.applied, runtimeSnapshot(m.Config())) {
				t.Fatal("running configuration snapshot not updated")
			}
		})
	}
}

func TestFailedServerSelectionRollsBackWithoutCoreRestart(t *testing.T) {
	for _, failure := range []string{"api", "disk"} {
		t.Run(failure, func(t *testing.T) {
			m, runner, controller, baseline := setupServerSwitch(t, "fallback")
			before := m.Config()
			process, started := m.process, m.started
			if failure == "api" {
				controller.failOnce()
			} else {
				path := filepath.Join(m.dir, "config.json")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			config := m.Config()
			config.Selected = "b"
			if err := m.SaveLive(context.Background(), config); err == nil {
				t.Fatal("failed switch reported success")
			}
			if !reflect.DeepEqual(m.Config(), before) || !reflect.DeepEqual(m.applied, runtimeSnapshot(before)) {
				t.Fatal("failed switch changed saved or applied configuration")
			}
			payloads := controller.takePayloads()
			if len(payloads) != 2 {
				t.Fatalf("reloads = %d, want failed apply and rollback", len(payloads))
			}
			for _, doc := range payloads {
				checkSwitchPayload(t, doc, baseline)
			}
			if got := switchGroup(t, payloads[1])["proxies"]; !reflect.DeepEqual(got, switchGroup(t, baseline)["proxies"]) {
				t.Fatalf("rollback did not restore priority: %#v", got)
			}
			if m.process != process || m.started != started || runner.starts != 1 {
				t.Fatal("core restarted during failed switch")
			}
		})
	}
}

// Opt-in integration smoke. Both nodes point to unused loopback ports; no TUN,
// system proxy, or live VPN endpoint is touched.
func TestServerSwitchWithBundledMihomo(t *testing.T) {
	if os.Getenv("YORU_LIVE_CORE") != "1" {
		t.Skip("set YORU_LIVE_CORE=1 for the real core smoke test")
	}
	if !HasBundledCore() {
		t.Skip("bundled Mihomo is unavailable")
	}
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown() })
	m.platform = &fakePlatform{}
	config := validConfig()
	config.Settings.TUN, config.Settings.SystemProxy = false, false
	config.Settings.GroupMode = "select"
	config.Servers[0].Host, config.Servers[0].Port = "127.0.0.1", 9
	config.Servers[0].Transport, config.Servers[0].Flow = "TCP", ""
	config.Servers[0].PublicKey, config.Servers[0].ShortID, config.Servers[0].SNI = "", "", ""
	second := config.Servers[0]
	second.ID, second.Name, second.Port = "b", "B", 10
	config.Servers = append(config.Servers, second)
	if err = m.Save(config); err != nil {
		t.Fatal(err)
	}
	if err = m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	process, started, endpoint := m.process, m.started, m.endpoint
	var settings liveSettings
	if err = m.api(context.Background(), http.MethodGet, "/configs", nil, &settings); err != nil {
		t.Fatal(err)
	}
	readGroup := func() struct {
		Type string   `json:"type"`
		Now  string   `json:"now"`
		All  []string `json:"all"`
	} {
		t.Helper()
		var group struct {
			Type string   `json:"type"`
			Now  string   `json:"now"`
			All  []string `json:"all"`
		}
		if err := m.api(context.Background(), http.MethodGet, "/proxies/PROXY", nil, &group); err != nil {
			t.Fatal(err)
		}
		return group
	}
	if group := readGroup(); group.Type != "Selector" || group.Now != "server-a" {
		t.Fatalf("initial selection: %+v", group)
	}
	config = m.Config()
	config.Selected = "b"
	if err = m.SaveLive(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	if group := readGroup(); group.Type != "Selector" || group.Now != "server-b" || !reflect.DeepEqual(group.All, []string{"server-b", "server-a"}) {
		t.Fatalf("manual switch did not apply: %+v", group)
	}
	for _, mode := range []struct{ config, runtime string }{
		{"fallback", "Fallback"},
		{"url-test", "URLTest"},
		{"load-balance", "LoadBalance"},
		{"select", "Selector"},
	} {
		config = m.Config()
		config.Settings.GroupMode = mode.config
		if err = m.SaveLive(context.Background(), config); err != nil {
			t.Fatalf("%s: %v", mode.config, err)
		}
		group := readGroup()
		if group.Type != mode.runtime || !reflect.DeepEqual(group.All, []string{"server-b", "server-a"}) {
			t.Fatalf("%s group not applied: %+v", mode.config, group)
		}
		if mode.config == "select" && group.Now != "server-b" {
			t.Fatalf("manual selection lost after mode cycle: %+v", group)
		}
		var current liveSettings
		if err = m.api(context.Background(), http.MethodGet, "/configs", nil, &current); err != nil {
			t.Fatal(err)
		}
		if current.MixedPort != settings.MixedPort || current.TUN.Enable != settings.TUN.Enable || m.process != process || m.started != started || m.endpoint != endpoint {
			t.Fatalf("%s interrupted core/listener lifecycle", mode.config)
		}
	}
}
