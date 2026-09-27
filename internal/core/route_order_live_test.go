package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"
)

// Uses an isolated bundled core and a loopback HTTP server. It never enables
// TUN/system proxy or reads the installed application's configuration.
func TestMixedRouteOrderWithBundledMihomo(t *testing.T) {
	if os.Getenv("YORU_LIVE_CORE") != "1" {
		t.Skip("set YORU_LIVE_CORE=1 for the real core smoke test")
	}
	if !HasBundledCore() {
		t.Fatal("bundle Mihomo before running the live test")
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "mixed-order-ok")
	}))
	t.Cleanup(origin.Close)
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown() })
	m.platform = &fakePlatform{}
	c := validConfig()
	c.Settings.TUN, c.Settings.SystemProxy = false, false
	c.Settings.GroupMode = "select"
	c.Servers[0].Host, c.Servers[0].Port = "127.0.0.1", 9
	c.Servers[0].Transport, c.Servers[0].Flow = "TCP", ""
	c.Servers[0].PublicKey, c.Servers[0].ShortID, c.Servers[0].SNI = "", "", ""
	c.Rules = []Rule{{ID: "local-exception", Type: "IP-CIDR", Value: "127.0.0.1/32", Action: "DIRECT", NoResolve: true}}
	c.Sets = []RouteSet{{ID: "local-block", Name: "Loopback fixture", Enabled: true, CIDRs: []string{"127.0.0.0/8"}, Action: "REJECT"}}
	c.RouteOrder = []RouteRef{{Kind: "rule", ID: "local-exception"}, {Kind: "set", ID: "local-block"}}
	if err = m.Save(c); err != nil {
		t.Fatal(err)
	}
	if err = m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	process, started := m.process, m.started
	var settings liveSettings
	if err = m.api(context.Background(), http.MethodGet, "/configs", nil, &settings); err != nil {
		t.Fatal(err)
	}
	proxy, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", settings.MixedPort))
	transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	probe := func(wantAllowed bool) {
		t.Helper()
		response, requestErr := client.Get(origin.URL)
		var body []byte
		if response != nil {
			body, _ = io.ReadAll(response.Body)
			_ = response.Body.Close()
		}
		allowed := requestErr == nil && response.StatusCode == http.StatusOK && string(body) == "mixed-order-ok"
		if allowed != wantAllowed {
			t.Fatalf("allowed=%v, want %v; request error=%v, response=%q", allowed, wantAllowed, requestErr, body)
		}
	}
	probe(true)
	for _, ruleFirst := range []bool{false, true} {
		c = m.Config()
		c.RouteOrder[0], c.RouteOrder[1] = c.RouteOrder[1], c.RouteOrder[0]
		if err = m.SaveLive(context.Background(), c); err != nil {
			t.Fatal(err)
		}
		var actual struct {
			Rules []struct {
				Proxy string `json:"proxy"`
			} `json:"rules"`
		}
		if err = m.api(context.Background(), http.MethodGet, "/rules", nil, &actual); err != nil {
			t.Fatal(err)
		}
		first := "REJECT"
		if ruleFirst {
			first = "DIRECT"
		}
		if len(actual.Rules) != 3 || actual.Rules[0].Proxy != first {
			t.Fatalf("core priority did not change: %+v", actual)
		}
		probe(ruleFirst)
		if process != m.process || started != m.started || m.Status(context.Background()).Pending {
			t.Fatal("priority change restarted the core or remains pending")
		}
	}
	data, err := os.ReadFile(m.dir + "/config.json")
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := DecodeConfig(data)
	if err != nil || !reflect.DeepEqual(persisted.RouteOrder, m.Config().RouteOrder) {
		t.Fatalf("live priority was not persisted: %v", err)
	}
}
