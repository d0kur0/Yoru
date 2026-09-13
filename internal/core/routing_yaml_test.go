package core

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// Decode generated YAML into an independent contract, not back into Config.
type yamlContract struct {
	Mode  string   `yaml:"mode"`
	Rules []string `yaml:"rules"`
	DNS   struct {
		Nameserver   []string            `yaml:"nameserver"`
		NodeResolver []string            `yaml:"proxy-server-nameserver"`
		Policies     map[string][]string `yaml:"nameserver-policy"`
		FakeIPFilter []string            `yaml:"fake-ip-filter"`
		Mode         string              `yaml:"enhanced-mode"`
	} `yaml:"dns"`
	TUN struct {
		Excluded []string `yaml:"route-exclude-address"`
		Hijack   []string `yaml:"dns-hijack"`
		Strict   bool     `yaml:"strict-route"`
	} `yaml:"tun"`
	Groups []struct {
		Name    string   `yaml:"name"`
		Proxies []string `yaml:"proxies"`
	} `yaml:"proxy-groups"`
}

func compiled(t *testing.T, c Config) yamlContract {
	t.Helper()
	b, e := c.YAML("127.0.0.1:49150", "test-only-secret", 49151)
	if e != nil {
		t.Fatal(e)
	}
	var out yamlContract
	if e = yaml.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	again, e := c.YAML("127.0.0.1:49150", "test-only-secret", 49151)
	if e != nil || string(again) != string(b) {
		t.Fatal("non-deterministic YAML")
	}
	return out
}
func TestSplitDNSAndCiscoYAML(t *testing.T) {
	// These are configuration fixtures, not live assertions about Cisco DNS reachability.
	for _, upstreams := range [][]string{{"10.20.0.53", "10.20.0.54"}, {"127.0.0.1:1053"}} {
		t.Run(upstreams[0], func(t *testing.T) {
			c := validConfig()
			c.DefaultAction = "DIRECT"
			c.Rules = []Rule{{ID: "work", Type: "DOMAIN-SUFFIX", Value: "company.example", Action: "DIRECT"}, {ID: "work2", Type: "DOMAIN-SUFFIX", Value: "service.example", Action: "DIRECT"}, {ID: "ai", Type: "DOMAIN-SUFFIX", Value: "openai.com", Action: "PROXY"}, {ID: "app", Type: "PROCESS-NAME", Value: "Cursor.exe", Action: "PROXY"}}
			c.DNS.Servers = []string{"https://dns.mullvad.net/dns-query", "https://dns10.quad9.net/dns-query"}
			for _, zone := range []string{"+.company.example", "+.internal.example", "+.service.example", "+.local"} {
				for _, server := range upstreams {
					c.DNS.Policies = append(c.DNS.Policies, Policy{zone, server})
				}
			}
			for _, server := range c.DNS.Servers {
				c.DNS.Policies = append(c.DNS.Policies, Policy{"vpn.company.example", server})
			}
			c.DNS.Exclusions = []string{"+.company.example", "+.internal.example", "+.service.example", "+.local"}
			c.Settings.RouteExclusions = []string{"10.20.0.53/32", "10.20.0.54/32", "192.168.255.0/24", "172.16.0.0/12"}
			got := compiled(t, c)
			want := []string{"DOMAIN-SUFFIX,company.example,DIRECT", "DOMAIN-SUFFIX,service.example,DIRECT", "DOMAIN-SUFFIX,openai.com,PROXY", "PROCESS-NAME,Cursor.exe,PROXY", "MATCH,DIRECT"}
			if got.Mode != "rule" || !reflect.DeepEqual(got.Rules, want) {
				t.Fatal("route priority or opt-in VPN changed", got.Rules)
			}
			for _, zone := range c.DNS.Exclusions {
				if !reflect.DeepEqual(got.DNS.Policies[zone], upstreams) {
					t.Fatalf("private zone %s mixed with public DNS: %v", zone, got.DNS.Policies[zone])
				}
			}
			if !reflect.DeepEqual(got.DNS.Policies["vpn.company.example"], c.DNS.Servers) {
				t.Fatal("Cisco gateway public DNS exception lost")
			}
			if !reflect.DeepEqual(got.DNS.NodeResolver, c.DNS.Servers) {
				t.Fatal("VPN node resolver depends on corporate DNS")
			}
			if !reflect.DeepEqual(got.DNS.FakeIPFilter, c.DNS.Exclusions) {
				t.Fatal("work domains lost real-IP exceptions")
			}
			if !reflect.DeepEqual(got.TUN.Excluded, c.Settings.RouteExclusions) || got.TUN.Strict {
				t.Fatal("TUN exclusion or non-strict routing lost")
			}
			if !reflect.DeepEqual(got.TUN.Hijack, []string{"any:53", "tcp://any:53"}) {
				t.Fatal("intercept DNS 53, never the helper's 1053", got.TUN.Hijack)
			}
		})
	}
}
func TestRouteDefaultIsExplicitAndPreservesUserChoice(t *testing.T) {
	if DefaultConfig().DefaultAction != "DIRECT" {
		t.Fatal("new installations must opt into VPN per rule")
	}
	for _, choice := range []string{"DIRECT", "PROXY", "REJECT"} {
		c := validConfig()
		c.DefaultAction = choice
		c.Rules = []Rule{{ID: "work", Type: "DOMAIN-SUFFIX", Value: "company.example", Action: "DIRECT"}}
		out := compiled(t, c)
		if !reflect.DeepEqual(out.Rules, []string{"DOMAIN-SUFFIX,company.example,DIRECT", "MATCH," + choice}) {
			t.Fatal(out.Rules)
		}
	}
}
func TestSelectedServerAndIPv6YAML(t *testing.T) {
	c := validConfig()
	second := c.Servers[0]
	second.ID = "b"
	second.Name = "B"
	c.Servers = append(c.Servers, second)
	c.Selected = "b"
	c.Settings.IPv6 = true
	c.Rules = []Rule{{ID: "v6", Type: "IP-CIDR6", Value: "fd00::/8", Action: "DIRECT"}}
	c.Settings.RouteExclusions = []string{"fd00::/8", "10.20.0.53/32"}
	out := compiled(t, c)
	if len(out.Groups) != 1 || !reflect.DeepEqual(out.Groups[0].Proxies, []string{"server-b", "server-a"}) {
		t.Fatal("selection lost")
	}
	if out.Rules[0] != "IP-CIDR6,fd00::/8,DIRECT" || !reflect.DeepEqual(out.TUN.Excluded, c.Settings.RouteExclusions) {
		t.Fatal("IPv6 route lost")
	}
}
func TestDNSHijackIsNotEmittedWhenDNSIsDisabled(t *testing.T) {
	c := validConfig()
	c.DNS.Enabled = false
	if len(compiled(t, c).TUN.Hijack) != 0 {
		t.Fatal("DNS disabled but port 53 still intercepted")
	}
	c.DNS.Enabled = true
	c.DNS.Hijack = false
	if len(compiled(t, c).TUN.Hijack) != 0 {
		t.Fatal("hijack switch ignored")
	}
}
func TestInvalidResolverPortsRejected(t *testing.T) {
	for _, address := range []string{"10.20.0.53:abc", "10.20.0.53:0", "127.0.0.1:65536", "https://dns.example:99999/dns-query"} {
		c := validConfig()
		c.DNS.Policies = []Policy{{"+.company.example", address}}
		if _, e := c.YAML("127.0.0.1:9090", "test", 7890); e == nil {
			t.Fatalf("invalid DNS accepted: %s", address)
		}
	}
}
