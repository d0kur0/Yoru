package core

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func routeSetFixture() RouteSet {
	return RouteSet{ID: "local-id", Name: "Company", Enabled: true, Domains: []string{"+.company.example"}, Processes: []string{"client.exe"}, CIDRs: []string{"10.0.0.0/8"}, Action: "DIRECT", Resolvers: []string{"10.20.0.53"}, RealIP: true, BypassTUN: true}
}

func TestRouteSetFileRoundTrip(t *testing.T) {
	original := routeSetFixture()
	disabled := original
	disabled.ID = "second"
	disabled.Name = "Off"
	disabled.Enabled = false
	data, err := EncodeRouteSets([]RouteSet{original, disabled})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("local-id")) || bytes.Contains(data, []byte(`"id"`)) {
		t.Fatal("export includes local identifiers")
	}
	imported, err := DecodeRouteSets(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported) != 2 || imported[0].ID == original.ID || imported[0].ID == imported[1].ID {
		t.Fatal("import must assign independent identifiers")
	}
	imported[0].ID = original.ID
	imported[1].ID = disabled.ID
	if !reflect.DeepEqual(imported, []RouteSet{original, disabled}) {
		t.Fatalf("settings lost: %#v", imported)
	}
	path := filepath.Join(t.TempDir(), "sets.json")
	if err = WriteRouteSets(path, []RouteSet{original}); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadRouteSets(path); err != nil {
		t.Fatal(err)
	}
	if original.ID != "local-id" {
		t.Fatal("export mutated original")
	}
}

func TestRouteSetFileRejectsInvalidDocuments(t *testing.T) {
	valid, _ := EncodeRouteSets([]RouteSet{routeSetFixture()})
	cases := map[string][]byte{
		"backup":   []byte(`{"format":"mihomo-desktop","version":1,"config":{}}`),
		"future":   bytes.Replace(valid, []byte(`"version": 1`), []byte(`"version": 2`), 1),
		"unknown":  bytes.Replace(valid, []byte(`"name":`), []byte(`"password":"secret","name":`), 1),
		"cidr":     bytes.ReplaceAll(valid, []byte("10.0.0.0/8"), []byte("invalid")),
		"resolver": bytes.ReplaceAll(valid, []byte("10.20.0.53"), []byte("not a DNS")),
		"action":   bytes.ReplaceAll(valid, []byte("DIRECT"), []byte("PROXY")),
		"domain":   bytes.ReplaceAll(valid, []byte("+.company.example"), []byte("https://company.example")),
		"empty":    []byte(`{"format":"yoru-route-sets","version":1,"sets":[]}`),
		"trailing": append(append([]byte{}, valid...), []byte(`{}`)...),
		"oversize": []byte(strings.Repeat(" ", 1<<20+1)),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeRouteSets(data); err == nil {
				t.Fatal("invalid file accepted")
			}
		})
	}
	// Windows editors may write a UTF-8 BOM.
	if _, err := DecodeRouteSets(append([]byte{0xef, 0xbb, 0xbf}, valid...)); err != nil {
		t.Fatal(err)
	}
}

func TestRouteSetFileDNSConflictAndAtomicExport(t *testing.T) {
	a := routeSetFixture()
	b := a
	b.ID = "another"
	b.Resolvers = []string{"10.20.0.54"}
	data, _ := json.Marshal(RouteSetsFile{routeSetsFormat, 1, []RouteSet{a, b}})
	if _, err := DecodeRouteSets(data); err == nil {
		t.Fatal("conflicting enabled DNS accepted")
	}
	path := filepath.Join(t.TempDir(), "existing.json")
	os.WriteFile(path, []byte("original"), 0600)
	if err := WriteRouteSets(path, []RouteSet{a, b}); err == nil {
		t.Fatal("invalid export accepted")
	}
	data, _ = os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal("failed export damaged existing file")
	}
	// A DNS conflict with current settings is rejected by the normal config save.
	config := DefaultConfig()
	config.Sets = []RouteSet{a}
	config.DNS.Policies = []Policy{{"+.company.example", "10.20.0.54"}}
	if err := config.Validate(); err == nil {
		t.Fatal("current DNS conflict accepted")
	}
}

func TestRouteSetKeywordsCompileAndRoundTrip(t *testing.T) {
	c := validConfig()
	c.Sets = []RouteSet{
		{ID: "sites", Name: "VPN sites", Enabled: true, Domains: []string{"+.example.com"}, Keywords: []string{"ubisoft", "chatgpt"}, Action: "PROXY", RealIP: true, Resolvers: []string{"9.9.9.9"}},
		{ID: "direct", Name: "Direct sites", Enabled: true, Domains: []string{"+.ru"}, Action: "DIRECT"},
		{ID: "apps", Name: "VPN apps", Enabled: true, Processes: []string{"chrome.exe"}, Action: "PROXY"},
	}
	c.Rules = nil
	c.DefaultAction = "PROXY"
	got := compiled(t, c)
	want := []string{"DOMAIN-SUFFIX,example.com,PROXY", "DOMAIN-KEYWORD,ubisoft,PROXY", "DOMAIN-KEYWORD,chatgpt,PROXY", "DOMAIN-SUFFIX,ru,DIRECT", "PROCESS-NAME,chrome.exe,PROXY", "MATCH,PROXY"}
	if !reflect.DeepEqual(got.Rules, want) {
		t.Fatalf("priority or type changed: %v", got.Rules)
	}
	for _, keyword := range c.Sets[0].Keywords {
		if _, ok := got.DNS.Policies[keyword]; ok {
			t.Fatal("keyword became an exact DNS policy")
		}
		for _, filter := range got.DNS.FakeIPFilter {
			if filter == keyword {
				t.Fatal("keyword became an exact fake-ip filter")
			}
		}
	}
	data, err := EncodeRouteSets(c.Sets)
	if err != nil {
		t.Fatal(err)
	}
	sets, err := DecodeRouteSets(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sets[0].Keywords, c.Sets[0].Keywords) {
		t.Fatal("keywords lost during import/export")
	}
	c.Sets[0].Enabled = false
	for _, rule := range compiled(t, c).Rules {
		if strings.HasPrefix(rule, "DOMAIN-KEYWORD,") {
			t.Fatal("disabled keywords still active")
		}
	}
}

func TestRouteSetRejectsInvalidKeywords(t *testing.T) {
	for _, keyword := range []string{"", "http://example.com", "two words", "example,PROXY", "bad\nvalue"} {
		c := DefaultConfig()
		s := routeSetFixture()
		s.Keywords = []string{keyword}
		c.Sets = []RouteSet{s}
		if err := c.Validate(); err == nil {
			t.Errorf("accepted keyword %q", keyword)
		}
	}
}
