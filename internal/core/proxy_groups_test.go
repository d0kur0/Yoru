package core

import (
	"context"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProxyGroupModesAndPinnedRules(t *testing.T) {
	for _, mode := range []string{"select", "fallback", "url-test", "load-balance"} {
		t.Run(mode, func(t *testing.T) {
			c := validConfig()
			c.Servers = []Server{{ID: "a", Name: "A", Host: "127.0.0.1", Port: 19999, Protocol: "Shadowsocks", Transport: "TCP", Cipher: "aes-128-gcm", Secret: "test-secret"}}
			c.Settings.GroupMode = mode
			c.Settings.HealthInterval = 45
			c.Settings.HealthTimeout = 7000
			tolerance := 80
			c.Settings.HealthTolerance = &tolerance
			c.Settings.HealthLazy = true
			c.Settings.BalanceStrategy = "sticky-sessions"
			c.Rules = []Rule{{Type: "PROCESS-NAME", Value: "game.exe", Action: "PROXY", Target: "a"}, {Type: "DOMAIN", Value: "fast.example", Action: "PROXY", Target: "@fastest"}, {Type: "DOMAIN", Value: "missing.example", Action: "PROXY", Target: "removed"}}
			data, err := c.YAML("127.0.0.1:19099", "test", 19098)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Groups []map[string]any `yaml:"proxy-groups"`
				Rules  []string         `yaml:"rules"`
			}
			if err = yaml.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			g := doc.Groups[0]
			if g["type"] != mode {
				t.Fatal(g)
			}
			if mode == "select" {
				if g["url"] != nil {
					t.Fatal("manual group probes")
				}
			} else if g["interval"] != 45 || g["timeout"] != 7000 || g["lazy"] != true {
				t.Fatal(g)
			}
			if mode == "url-test" && g["tolerance"] != 80 {
				t.Fatal(g)
			}
			if mode == "load-balance" && g["strategy"] != "sticky-sessions" {
				t.Fatal(g)
			}
			if doc.Groups[1]["type"] != "url-test" || doc.Groups[1]["tolerance"] != 80 {
				t.Fatal(doc.Groups)
			}
			if doc.Rules[0] != "PROCESS-NAME,game.exe,server-a" || doc.Rules[1] != "DOMAIN,fast.example,AUTO" || doc.Rules[2] != "DOMAIN,missing.example,REJECT" {
				t.Fatal(doc.Rules)
			}
			if binary := os.Getenv("MIHOMO_TEST_BINARY"); binary != "" {
				dir := t.TempDir()
				path := filepath.Join(dir, "config.yaml")
				if err = os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if out, e := exec.CommandContext(ctx, binary, "-t", "-d", dir, "-f", path).CombinedOutput(); e != nil {
					t.Fatalf("core validation: %v %s", e, out)
				}
			}
		})
	}
}

func TestProxyGroupValidationAndMigration(t *testing.T) {
	c := validConfig()
	if c.GroupMode() != "fallback" {
		t.Fatal("legacy fallback changed")
	}
	off := false
	c.Settings.Failover = &off
	if c.GroupMode() != "select" {
		t.Fatal("legacy manual changed")
	}
	c.Settings.GroupMode = "load-balance"
	if c.GroupMode() != "load-balance" {
		t.Fatal("mode ignored")
	}
	for _, mutate := range []func(*Config){func(c *Config) { c.Settings.GroupMode = "bad" }, func(c *Config) { c.Settings.HealthInterval = -1 }, func(c *Config) { c.Settings.HealthTimeout = 999 }, func(c *Config) { n := -1; c.Settings.HealthTolerance = &n }, func(c *Config) { c.Settings.BalanceStrategy = "bad" }, func(c *Config) { c.Rules = []Rule{{Type: "DOMAIN", Value: "a.example", Action: "DIRECT", Target: "a"}} }} {
		x := validConfig()
		mutate(&x)
		if x.Validate() == nil {
			t.Fatal("invalid group settings accepted")
		}
	}
	c = validConfig()
	c.Rules = []Rule{{Type: "DOMAIN", Value: "game.example", Action: "PROXY", Target: "a"}}
	c.Servers = nil
	c.Selected = ""
	data, err := c.YAML("127.0.0.1:19099", "test", 19098)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "DOMAIN,game.example,REJECT") {
		t.Fatal("missing pinned server escaped")
	}
}
