package core

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestEditedServerProxyReloadsLiveCore(t *testing.T) {
	m, runner, controller, baseline := setupServerSwitch(t, "select")
	process, started := m.process, m.started
	c := m.Config()
	c.Servers[0].Host = "new-endpoint.example"
	if err := m.SaveLive(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	payloads := controller.takePayloads()
	if len(payloads) != 1 {
		t.Fatalf("proxy edit reloads = %d, want 1", len(payloads))
	}
	checkSwitchPayload(t, payloads[0], baseline)
	proxies := payloads[0]["proxies"].([]any)
	if got := proxies[0].(map[string]any)["server"]; got != "new-endpoint.example" {
		t.Fatalf("live proxy still uses old endpoint: %v", got)
	}
	if m.process != process || m.started != started || runner.starts != 1 || !m.nodeApplied("a") {
		t.Fatal("edit restarted core or left running proxy stale")
	}
}

func TestServerEditWithModeAndProxyReloadsLiveCore(t *testing.T) {
	m, _, controller, _ := setupServerSwitch(t, "select")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/proxies/GLOBAL" {
			if req.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"now":"DIRECT"}`))
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
			return
		}
		controller.handler(w, req)
	}))
	t.Cleanup(server.Close)
	m.endpoint = server.URL
	c := m.Config()
	c.Servers[0].Host = "new-endpoint.example"
	c.Settings.Mode = "global"
	c.Settings.SystemProxy = true
	if err := m.SaveLive(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	payloads := controller.takePayloads()
	if len(payloads) != 1 || payloads[0]["mode"] != "global" || payloads[0]["proxies"].([]any)[0].(map[string]any)["server"] != "new-endpoint.example" {
		t.Fatalf("routing and mode not applied together: %+v", payloads)
	}
	if !m.Config().Settings.SystemProxy || m.restoreProxy == nil {
		t.Fatal("system proxy setting not applied with routing reload")
	}
}

func TestServerEditWithTUNChangeFailsBeforeMutation(t *testing.T) {
	m, _, controller, _ := setupServerSwitch(t, "select")
	before := m.Config()
	applied := append([]byte(nil), m.applied...)
	c := m.Config()
	c.Servers[0].Host = "new-endpoint.example"
	c.Settings.TUN = !c.Settings.TUN
	if err := m.SaveLive(context.Background(), c); err == nil || !strings.Contains(err.Error(), "TUN") {
		t.Fatalf("combined TUN change was not rejected: %v", err)
	}
	if !reflect.DeepEqual(m.Config(), before) || !bytes.Equal(m.applied, applied) || len(controller.takePayloads()) != 0 {
		t.Fatal("combined TUN change modified saved or running configuration")
	}
}

func TestSubscriptionRefreshReloadsLiveCoreAndRollsBackFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		label := "success"
		if fail {
			label = "api-failure"
		}
		t.Run(label, func(t *testing.T) {
			m, runner, controller, baseline := setupServerSwitch(t, "select")
			c := m.Config()
			c.Subscriptions = []Subscription{{ID: "sub", Name: "Source", URL: "https://provider.example/profile", Interval: "24"}}
			if err := m.SaveLive(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			before := m.Config()
			applied := append([]byte(nil), m.applied...)
			process, started := m.process, m.started
			m.downloadClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("trojan://password@new.example:443#New"))}, nil
			})}
			if fail {
				controller.failOnce()
			}
			updated, err := m.UpdateSubscription(context.Background(), "sub")
			if (err != nil) != fail {
				t.Fatalf("refresh error=%v, fail=%t", err, fail)
			}
			payloads := controller.takePayloads()
			want := 1
			if fail {
				want = 2 // failed application, then old runtime rollback
			}
			if len(payloads) != want {
				t.Fatalf("reloads = %d, want %d", len(payloads), want)
			}
			checkSwitchPayload(t, payloads[0], baseline)
			if m.process != process || m.started != started || runner.starts != 1 {
				t.Fatal("subscription refresh restarted core")
			}
			if fail {
				if !reflect.DeepEqual(m.Config(), before) || !bytes.Equal(m.applied, applied) || !reflect.DeepEqual(payloads[1]["proxies"], baseline["proxies"]) {
					t.Fatal("failed subscription refresh did not roll back disk and runtime")
				}
				return
			}
			if len(updated.Servers) != 3 || updated.Revision != before.Revision+1 || !m.nodeApplied(updated.Servers[2].ID) {
				t.Fatalf("new subscription node is not active: %+v", updated)
			}
			proxies := payloads[0]["proxies"].([]any)
			if got := proxies[2].(map[string]any)["server"]; got != "new.example" {
				t.Fatalf("new node missing from live core: %v", got)
			}
		})
	}
}
