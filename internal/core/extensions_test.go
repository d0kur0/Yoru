package core

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestProcessMasksAndNoResolve(t *testing.T) {
	c := DefaultConfig()
	c.Rules = []Rule{{ID: "name", Type: "PROCESS-NAME-WILDCARD", Value: "*Chrome*", Action: "PROXY"}, {ID: "path", Type: "PROCESS-PATH-WILDCARD", Value: "/Applications/Google Chrome.app/*", Action: "DIRECT"}, {ID: "ip", Type: "IP-CIDR", Value: "10.0.0.0/8", Action: "DIRECT", NoResolve: true}}
	data, e := c.YAML("127.0.0.1:9000", "test", 7890)
	if e != nil {
		t.Fatal(e)
	}
	for _, expected := range []string{"PROCESS-NAME-WILDCARD,*Chrome*,PROXY", "PROCESS-PATH-WILDCARD,/Applications/Google Chrome.app/*,DIRECT", "IP-CIDR,10.0.0.0/8,DIRECT,no-resolve"} {
		if !bytes.Contains(data, []byte(expected)) {
			t.Fatalf("missing %s", expected)
		}
	}
}
func TestListProcesses(t *testing.T) {
	items, e := ListProcesses(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range items {
		if p.PID == os.Getpid() {
			if p.Name == "" {
				t.Fatal("empty name")
			}
			return
		}
	}
	t.Fatal("own process missing")
}
func TestLogRotationAndDisabled(t *testing.T) {
	l := &rotatingLog{dir: t.TempDir()}
	l.Configure(LogSettings{true, "info", 1, 2, 7})
	l.Write(bytes.Repeat([]byte("x"), 3<<20))
	for i := 0; i < 2; i++ {
		info, e := os.Stat(l.name(i))
		if e != nil || info.Size() > 1<<20 {
			t.Fatalf("rotation: %v", e)
		}
	}
	if _, e := os.Stat(l.name(2)); !os.IsNotExist(e) {
		t.Fatal("excess file")
	}
	l.Configure(LogSettings{false, "info", 1, 2, 7})
	l.Write([]byte("disabled"))
	if l.String() != "" {
		t.Fatal("collection still active")
	}
}
func TestWorkSetExpansion(t *testing.T) {
	c := DefaultConfig()
	preset := WorkPreset()
	preset.Enabled = true
	c.Sets = []RouteSet{preset}
	c.Rules = []Rule{{Type: "PROCESS-NAME", Value: "browser.exe", Action: "PROXY"}}
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	x := c.expanded()
	if x.Rules[0].Value != "company.example" || x.Rules[0].Action != "DIRECT" {
		t.Fatal("work precedence")
	}
	if len(x.DNS.Policies) != 8 {
		t.Fatal("split DNS")
	}
	if len(c.DNS.Policies) != 0 {
		t.Fatal("expanded mutated source")
	}
}

func TestImportYAMLKeepsRouting(t *testing.T) {
	source := []byte(`mode: rule
proxies:
  - {name: node, type: trojan, server: example.com, port: 443, password: test-only, sni: example.com}
proxy-groups:
  - {name: Proxy, type: select, proxies: [node, DIRECT]}
dns:
  enable: true
  enhanced-mode: fake-ip
  nameserver: [https://dns.quad9.net/dns-query]
  nameserver-policy:
    '+.example.org': [192.168.1.1, 192.168.1.2]
tun:
  enable: true
  dns-hijack: [any:1053]
rules:
  - DOMAIN-SUFFIX,example.org,DIRECT
  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve
  - PROCESS-NAME-WILDCARD,*Chrome*,Proxy
  - MATCH,Proxy
`)
	imported, e := ImportYAML(source)
	if e != nil {
		t.Fatal(e)
	}
	c := imported.Config
	if c.DefaultAction != "PROXY" || len(c.DNS.Policies) != 2 || !c.Rules[1].NoResolve || c.Rules[2].Type != "PROCESS-NAME-WILDCARD" {
		t.Fatal("lost imported behavior")
	}
	if c.Settings.SystemProxy || c.Settings.AutoConnect {
		t.Fatal("import changed system startup")
	}
	if len(imported.Warnings) == 0 {
		t.Fatal("missing normalization warning")
	}
	data, e := c.YAML("127.0.0.1:9999", "test", 7890)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(data, []byte("any:53")) || bytes.Contains(data, []byte("any:1053")) {
		t.Fatal("wrong DNS interception")
	}
}
