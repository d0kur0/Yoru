package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReloadRoutingPreservesTUNAndRollsBackFailures(t *testing.T) {
	for _, failure := range []string{"", "api", "disk"} {
		t.Run(failure, func(t *testing.T) {
			m, r, _, _ := setupLiveSettings(t)
			before := m.Config()
			var payloads []map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodGet {
					_, _ = w.Write([]byte(`{"mixed-port":45678,"mode":"rule","tun":{"enable":false}}`))
					return
				}
				if req.URL.Query().Get("force") != "false" {
					t.Error("forced reload")
				}
				var body struct {
					Payload string `json:"payload"`
				}
				_ = json.NewDecoder(req.Body).Decode(&body)
				var doc map[string]any
				if err := yaml.Unmarshal([]byte(body.Payload), &doc); err != nil {
					t.Error(err)
				}
				payloads = append(payloads, doc)
				if failure == "api" && len(payloads) == 1 {
					w.WriteHeader(400)
					return
				}
				w.WriteHeader(204)
			}))
			defer server.Close()
			m.endpoint = server.URL
			c := m.Config()
			c.Rules = append(c.Rules, Rule{ID: "live-rule", Type: "DOMAIN", Value: "test.example", Action: "REJECT"})
			c.Sets = append(c.Sets, RouteSet{ID: "live-set", Name: "Test", Enabled: true, Action: "DIRECT", CIDRs: []string{"192.0.2.0/24"}, BypassTUN: true})
			if failure == "disk" {
				p := filepath.Join(m.dir, "config.json")
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(p, 0700); err != nil {
					t.Fatal(err)
				}
			}
			process, started := m.process, m.started
			err := m.SaveLive(context.Background(), c)
			if (err != nil) != (failure != "") {
				t.Fatalf("error = %v", err)
			}
			if m.process != process || m.started != started || r.starts != 1 {
				t.Fatal("core restarted")
			}
			oldYAML, _ := before.YAML("127.0.0.1:1234", "secret", 45678)
			var old map[string]any
			_ = yaml.Unmarshal(oldYAML, &old)
			for _, doc := range payloads {
				if !reflect.DeepEqual(doc["tun"], old["tun"]) {
					t.Fatal("TUN configuration changed")
				}
			}
			if failure != "" {
				if len(payloads) != 2 || !reflect.DeepEqual(payloads[1]["rules"], old["rules"]) || m.Config().Revision != before.Revision {
					t.Fatal("rollback failed")
				}
			} else {
				if !m.pendingTUN || m.Config().Revision != before.Revision+1 {
					t.Fatal("pending TUN change lost")
				}
				if err := m.ReloadConfig(context.Background()); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(payloads[1]["tun"], old["tun"]) {
					t.Fatal("second reload applied pending TUN")
				}
			}
		})
	}
}

func TestSaveLiveReloadsOrderOnlyChangeAndRollsBack(t *testing.T) {
	for _, failure := range []string{"", "api", "disk"} {
		t.Run(failure, func(t *testing.T) {
			m, runner, _, _ := setupLiveSettings(t)
			base := m.Config()
			base.Sets = []RouteSet{{ID: "broad", Name: "Broad", Enabled: true, Domains: []string{"+.example.com"}, Action: "DIRECT"}}
			base.Rules = []Rule{{ID: "exact", Type: "DOMAIN", Value: "specific.example.com", Action: "PROXY"}}
			if err := m.SaveLive(context.Background(), base); err != nil {
				t.Fatal(err)
			}
			before := m.Config()
			if !reflect.DeepEqual(before.RouteOrder, []RouteRef{{"set", "broad"}, {"rule", "exact"}}) {
				t.Fatalf("legacy priority changed: %+v", before.RouteOrder)
			}
			var payloads []map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodGet {
					_, _ = w.Write([]byte(`{"mixed-port":45678,"mode":"rule","tun":{"enable":false}}`))
					return
				}
				var body struct {
					Payload string `json:"payload"`
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				var doc map[string]any
				if err := yaml.Unmarshal([]byte(body.Payload), &doc); err != nil {
					t.Error(err)
				}
				payloads = append(payloads, doc)
				if failure == "api" && len(payloads) == 1 {
					w.WriteHeader(400)
					return
				}
				w.WriteHeader(204)
			}))
			defer server.Close()
			m.endpoint = server.URL
			updated := m.Config()
			updated.RouteOrder[0], updated.RouteOrder[1] = updated.RouteOrder[1], updated.RouteOrder[0]
			if failure == "disk" {
				path := filepath.Join(m.dir, "config.json")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			process := m.process
			err := m.SaveLive(context.Background(), updated)
			if (err != nil) != (failure != "") {
				t.Fatalf("SaveLive error = %v", err)
			}
			if m.process != process || runner.starts != 1 || len(payloads) == 0 {
				t.Fatal("order edit did not hot reload the current core")
			}
			appliedRules := payloads[0]["rules"].([]any)
			if appliedRules[0] != "DOMAIN,specific.example.com,PROXY" || appliedRules[1] != "DOMAIN-SUFFIX,example.com,DIRECT" {
				t.Fatalf("order edit did not reach Mihomo: %v", appliedRules)
			}
			if failure == "" {
				if len(payloads) != 1 || !reflect.DeepEqual(m.Config().RouteOrder, updated.RouteOrder) {
					t.Fatal("applied order was not saved")
				}
				return
			}
			oldYAML, err := before.YAML("127.0.0.1:1234", "secret", 45678)
			if err != nil {
				t.Fatal(err)
			}
			var old map[string]any
			_ = yaml.Unmarshal(oldYAML, &old)
			if len(payloads) != 2 || !reflect.DeepEqual(payloads[1]["rules"], old["rules"]) ||
				!reflect.DeepEqual(m.Config().RouteOrder, before.RouteOrder) || m.Config().Revision != before.Revision {
				t.Fatal("failed order edit was not rolled back")
			}
		})
	}
}
