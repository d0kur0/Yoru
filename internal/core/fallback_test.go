package core

import (
	"gopkg.in/yaml.v3"
	"reflect"
	"testing"
)

func TestManualModeAndPersistentPriority(t *testing.T) {
	c := validConfig()
	b := c.Servers[0]
	b.ID = "b"
	b.Name = "B"
	d := b
	d.ID = "c"
	d.Name = "C"
	c.Servers = append(c.Servers, b, d)
	c.Settings.ServerOrder = []string{"missing", "c", "c", "a", "b"}
	if !reflect.DeepEqual(c.OrderedServerIDs(), []string{"a", "c", "b"}) {
		t.Fatal(c.OrderedServerIDs())
	}
	disabled := false
	c.Settings.Failover = &disabled
	data, err := c.YAML("127.0.0.1:19099", "test", 19098)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Groups []map[string]any `yaml:"proxy-groups"`
	}
	if err = yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Groups[0]["type"] != "select" || doc.Groups[0]["interval"] != nil {
		t.Fatal("manual mode still probes or fails over")
	}
}

func TestFallbackPriorityAndHealthCheck(t *testing.T) {
	c := validConfig()
	second := c.Servers[0]
	second.ID = "b"
	second.Name = "B"
	third := c.Servers[0]
	third.ID = "c"
	third.Name = "C"
	c.Servers = append(c.Servers, second, third)
	c.Selected = "b"
	c.Settings.LatencyURL = "https://example.com/check"
	data, err := c.YAML("127.0.0.1:19099", "test", 19098)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Groups []struct {
			Type, URL         string
			Proxies           []string
			Interval, Timeout int
			Lazy              bool
		} `yaml:"proxy-groups"`
	}
	if err = yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	g := doc.Groups[0]
	if g.Type != "fallback" || g.URL != c.Settings.LatencyURL || g.Interval != 30 || g.Timeout != 5000 || g.Lazy {
		t.Fatalf("wrong health check: %+v", g)
	}
	want := []string{"server-b", "server-a", "server-c"}
	for i, name := range want {
		if g.Proxies[i] != name {
			t.Fatalf("wrong priority: %v", g.Proxies)
		}
	}
	for _, name := range g.Proxies {
		if name == "DIRECT" {
			t.Fatal("failover must not bypass VPN")
		}
	}
}
