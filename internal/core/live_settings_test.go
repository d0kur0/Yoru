package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type liveController struct {
	mu      sync.Mutex
	state   liveSettings
	global  string
	denyTUN bool
}

func setupLiveSettings(t *testing.T) (*Manager, *fakeRunner, *fakePlatform, *liveController) {
	t.Helper()
	m, r, p := testManager(t)
	c := m.Config()
	c.Settings.TUN, c.Settings.SystemProxy = false, false
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	controller := &liveController{global: "DIRECT"}
	controller.state.Mode, controller.state.MixedPort = "rule", 45678
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		controller.mu.Lock()
		defer controller.mu.Unlock()
		switch req.URL.Path {
		case "/configs":
			if req.Method == http.MethodPatch {
				var patch struct {
					Mode *string `json:"mode"`
					TUN  *struct {
						Enable bool `json:"enable"`
					} `json:"tun"`
				}
				if err := json.NewDecoder(req.Body).Decode(&patch); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				if patch.Mode != nil {
					controller.state.Mode = *patch.Mode
				}
				if patch.TUN != nil && !controller.denyTUN {
					controller.state.TUN.Enable = patch.TUN.Enable
				}
				w.WriteHeader(204)
			} else {
				_ = json.NewEncoder(w).Encode(controller.state)
			}
		case "/proxies/GLOBAL":
			if req.Method == http.MethodPut {
				var choice struct {
					Name string `json:"name"`
				}
				_ = json.NewDecoder(req.Body).Decode(&choice)
				controller.global = choice.Name
				w.WriteHeader(204)
			} else {
				_ = json.NewEncoder(w).Encode(map[string]string{"now": controller.global})
			}
		default:
			w.WriteHeader(204)
		}
	}))
	t.Cleanup(server.Close)
	m.endpoint = server.URL
	return m, r, p, controller
}

func TestLiveStatusSettingsApplyWithoutReconnect(t *testing.T) {
	m, r, p, controller := setupLiveSettings(t)
	process, started := m.process, m.started
	for _, mode := range []string{"global", "direct", "rule"} {
		c := m.Config()
		c.Settings.TUN = true
		c.Settings.Mode = mode
		if err := m.SaveLive(context.Background(), c); err != nil {
			t.Fatal(err)
		}
		controller.mu.Lock()
		if !controller.state.TUN.Enable || controller.state.Mode != mode {
			t.Error("not applied to running core")
		}
		if mode == "global" && controller.global != "PROXY" {
			t.Error("global traffic bypasses VPN group")
		}
		controller.mu.Unlock()
	}
	for _, enabled := range []bool{true, false} {
		c := m.Config()
		c.Settings.SystemProxy = enabled
		if err := m.SaveLive(context.Background(), c); err != nil {
			t.Fatal(err)
		}
		if (m.restoreProxy != nil) != enabled {
			t.Fatal("proxy state mismatch")
		}
	}
	c := m.Config()
	c.Settings.TUN = false
	if err := m.SaveLive(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if r.starts != 1 || process != m.process || started != m.started {
		t.Fatal("core restarted")
	}
	if p.enabled != 1 || p.restored != 1 {
		t.Fatal("proxy lifecycle mismatch")
	}
	var applied Config
	_ = json.Unmarshal(m.applied, &applied)
	if applied.Settings.TUN || applied.Settings.SystemProxy || applied.Settings.Mode != "rule" || applied.Revision != m.config.Revision {
		t.Fatal("applied snapshot stale")
	}
}

func TestFailedLiveChangesRollbackAndDoNotSave(t *testing.T) {
	for _, failure := range []string{"tun", "proxy", "disk"} {
		t.Run(failure, func(t *testing.T) {
			m, r, p, controller := setupLiveSettings(t)
			c := m.Config()
			revision := c.Revision
			c.Settings.TUN, c.Settings.SystemProxy, c.Settings.Mode = true, true, "global"
			controller.denyTUN = failure == "tun"
			p.fail = failure == "proxy"
			if failure == "disk" {
				path := filepath.Join(m.dir, "config.json")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.SaveLive(context.Background(), c); err == nil {
				t.Fatal("failure reported as success")
			}
			actual := m.Config()
			if actual.Settings.TUN || actual.Settings.SystemProxy || actual.Settings.Mode != "rule" || actual.Revision != revision {
				t.Fatal("failed settings saved")
			}
			controller.mu.Lock()
			defer controller.mu.Unlock()
			if controller.state.TUN.Enable || controller.state.Mode != "rule" || controller.global != "DIRECT" {
				t.Fatal("runtime not rolled back")
			}
			if m.restoreProxy != nil || r.starts != 1 {
				t.Fatal("proxy left active or core restarted")
			}
		})
	}
}

func TestLiveSettingsKeepOtherPendingEdits(t *testing.T) {
	m, _, _, _ := setupLiveSettings(t)
	c := m.Config()
	c.Servers[0].Port++
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	c = m.Config()
	c.Settings.TUN = true
	if err := m.SaveLive(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	var applied Config
	_ = json.Unmarshal(m.applied, &applied)
	if !applied.Settings.TUN || applied.Servers[0].Port == c.Servers[0].Port {
		t.Fatal("unrelated pending edit applied")
	}
}

// Explicit local smoke test; no TUN, system proxy, or remote VPN endpoint.
func TestLiveModesWithBundledMihomo(t *testing.T) {
	if os.Getenv("YORU_LIVE_CORE") != "1" {
		t.Skip("set YORU_LIVE_CORE=1 for the real core smoke test")
	}
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown() })
	m.platform = &fakePlatform{}
	c := validConfig()
	c.Settings.TUN, c.Settings.SystemProxy = false, false
	c.Servers[0].Host, c.Servers[0].Port = "127.0.0.1", 9
	c.Servers[0].Transport, c.Servers[0].Flow = "TCP", ""
	c.Servers[0].PublicKey, c.Servers[0].ShortID, c.Servers[0].SNI = "", "", ""
	if err = m.Save(c); err != nil {
		t.Fatal(err)
	}
	if err = m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	started, process := m.started, m.process
	for _, mode := range []string{"global", "direct", "rule"} {
		c = m.Config()
		c.Settings.Mode = mode
		if err = m.SaveLive(context.Background(), c); err != nil {
			t.Fatal(mode, err)
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			_, _ = io.Copy(conn, conn)
		}
	}()
	var settings liveSettings
	if err = m.api(context.Background(), "GET", "/configs", nil, &settings); err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", settings.MixedPort), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", listener.Addr(), listener.Addr())
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: "CONNECT"})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	echo := func(value string) {
		t.Helper()
		if _, err := fmt.Fprintln(conn, value); err != nil {
			t.Fatal(err)
		}
		line, err := reader.ReadString('\n')
		if err != nil || line != value+"\n" {
			t.Fatalf("connection interrupted: %q %v", line, err)
		}
	}
	echo("before reload")
	c = m.Config()
	c.Rules = append(c.Rules, Rule{ID: "live-routing", Type: "DOMAIN", Value: "reload.example", Action: "REJECT"})
	if err = m.SaveLive(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	var rules struct {
		Rules []struct {
			Payload string `json:"payload"`
			Proxy   string `json:"proxy"`
		} `json:"rules"`
	}
	if err = m.api(context.Background(), "GET", "/rules", nil, &rules); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rule := range rules.Rules {
		if rule.Payload == "reload.example" && rule.Proxy == "REJECT" {
			found = true
		}
	}
	if !found {
		t.Fatal("real core did not receive routing rule")
	}
	if err = m.ReloadConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	echo("after reload")
	if m.process != process || m.started != started {
		t.Fatal("core restarted")
	}
	if m.Status(context.Background()).Pending {
		t.Fatal("live mode reported as pending")
	}
}
