package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRouteOrderNormalizesLegacyAndStaleReferences(t *testing.T) {
	c := validConfig()
	c.Sets = []RouteSet{
		{ID: "broad", Name: "Broad", Enabled: true, Domains: []string{"+.example.com"}, Action: "PROXY"},
		{ID: "off", Name: "Off", Action: "DIRECT"},
	}
	c.Rules = []Rule{
		{ID: "legacy-rule-1", Type: "DOMAIN", Value: "kept.example", Action: "DIRECT"},
		{Type: "DOMAIN", Value: "new.example", Action: "REJECT"},
		{ID: "legacy-rule-1", Type: "DOMAIN", Value: "duplicate.example", Action: "DIRECT"},
	}
	c.RouteOrder = []RouteRef{
		{Kind: "rule", ID: "legacy-rule-1"},
		{Kind: "set", ID: "deleted"},
		{Kind: "rule", ID: "legacy-rule-1"},
		{Kind: "bad", ID: "broad"},
		{Kind: "set", ID: "off"},
	}
	originalRules := append([]Rule{}, c.Rules...)
	callerRules := c.Rules
	c.normalize()
	if !reflect.DeepEqual(c.Rules, []Rule{
		originalRules[0],
		{ID: "legacy-rule-2", Type: "DOMAIN", Value: "new.example", Action: "REJECT"},
		{ID: "legacy-rule-3", Type: "DOMAIN", Value: "duplicate.example", Action: "DIRECT"},
	}) {
		t.Fatalf("missing or duplicate IDs were not repaired: %+v", c.Rules)
	}
	want := []RouteRef{{"rule", "legacy-rule-1"}, {"set", "off"}, {"set", "broad"}, {"rule", "legacy-rule-2"}, {"rule", "legacy-rule-3"}}
	if !reflect.DeepEqual(c.RouteOrder, want) {
		t.Fatalf("normalized order = %+v", c.RouteOrder)
	}
	if callerRules[1].ID != "" || callerRules[2].ID != "legacy-rule-1" {
		t.Fatal("normalization modified the caller's rule slice")
	}
	c.normalize()
	if !reflect.DeepEqual(c.RouteOrder, want) {
		t.Fatal("normalization is not stable")
	}
}

func TestRouteOrderLegacyJSONAndRoundTrip(t *testing.T) {
	c := validConfig()
	c.Sets = []RouteSet{{ID: "set", Name: "Set", Enabled: true, Domains: []string{"+.example.com"}, Action: "PROXY"}}
	c.Rules = []Rule{{Type: "DOMAIN", Value: "exact.example.com", Action: "DIRECT"}}
	legacy, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(legacy, &document); err != nil {
		t.Fatal(err)
	}
	delete(document, "routeOrder")
	legacy, _ = json.Marshal(document)
	loaded, err := DecodeConfig(legacy)
	if err != nil {
		t.Fatal(err)
	}
	want := []RouteRef{{"set", "set"}, {"rule", loaded.Rules[0].ID}}
	if loaded.Rules[0].ID == "" || !reflect.DeepEqual(loaded.RouteOrder, want) {
		t.Fatalf("legacy order or rule identity lost: %+v", loaded)
	}
	if got := compiled(t, loaded).Rules; !reflect.DeepEqual(got, []string{
		"DOMAIN-SUFFIX,example.com,PROXY", "DOMAIN,exact.example.com,DIRECT", "MATCH,DIRECT",
	}) {
		t.Fatalf("legacy priority changed: %v", got)
	}
	encoded, _ := json.Marshal(loaded)
	again, err := DecodeConfig(encoded)
	if err != nil || !reflect.DeepEqual(again.RouteOrder, want) || again.Rules[0].ID != loaded.Rules[0].ID {
		t.Fatalf("order/identity JSON round trip failed: %v %+v", err, again.RouteOrder)
	}
	m, _, _ := testManager(t)
	loaded.Revision = m.Config().Revision
	if err := m.Save(loaded); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(m.dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := DecodeConfig(saved)
	if err != nil || !reflect.DeepEqual(persisted.RouteOrder, want) || persisted.Rules[0].ID != loaded.Rules[0].ID {
		t.Fatalf("migrated priority was not persisted: %v %+v", err, persisted.RouteOrder)
	}
}

func TestRouteOrderInterleavesAndKeepsSetEffects(t *testing.T) {
	c := validConfig()
	c.Sets = []RouteSet{
		{ID: "broad", Name: "Broad", Enabled: true, Domains: []string{"+.example.com"}, CIDRs: []string{"192.0.2.0/24"}, Resolvers: []string{"9.9.9.9"}, RealIP: true, BypassTUN: true, Action: "DIRECT"},
		{ID: "off", Name: "Off", Enabled: false, Domains: []string{"+.off.example"}, Action: "REJECT"},
	}
	c.Rules = []Rule{
		{ID: "exact", Type: "DOMAIN", Value: "specific.example.com", Action: "PROXY"},
		{ID: "last", Type: "DOMAIN-KEYWORD", Value: "tail", Action: "REJECT"},
	}
	c.RouteOrder = []RouteRef{{"rule", "exact"}, {"set", "off"}, {"set", "broad"}, {"rule", "last"}}
	want := []string{
		"DOMAIN,specific.example.com,PROXY",
		"DOMAIN-SUFFIX,example.com,DIRECT",
		"IP-CIDR,192.0.2.0/24,DIRECT,no-resolve",
		"DOMAIN-KEYWORD,tail,REJECT",
		"MATCH,DIRECT",
	}
	if got := compiled(t, c).Rules; !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed first-match priority = %v", got)
	}
	c.RouteOrder[0], c.RouteOrder[2] = c.RouteOrder[2], c.RouteOrder[0]
	before, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if got := compiled(t, c).Rules; got[0] != "DOMAIN-SUFFIX,example.com,DIRECT" {
		t.Fatalf("moving broad set did not change priority: %v", got)
	}
	out := compiled(t, c)
	after, _ := json.Marshal(c)
	if string(before) != string(after) {
		t.Fatal("expansion modified the source config")
	}
	if !reflect.DeepEqual(out.DNS.Policies["+.example.com"], []string{"9.9.9.9"}) ||
		!reflect.DeepEqual(out.DNS.FakeIPFilter[len(out.DNS.FakeIPFilter)-1:], []string{"+.example.com"}) ||
		!reflect.DeepEqual(out.TUN.Excluded[len(out.TUN.Excluded)-2:], []string{"192.0.2.0/24", "9.9.9.9/32"}) {
		t.Fatal("DNS, real-IP or TUN bypass effects were lost")
	}
}

func TestRouteSetExportKeepsVisibleRelativePriority(t *testing.T) {
	c := DefaultConfig()
	c.Sets = []RouteSet{
		{ID: "a", Name: "First stored", Action: "DIRECT", Enabled: true},
		{ID: "b", Name: "First visible", Action: "REJECT", Enabled: false},
	}
	c.Rules = []Rule{{ID: "r", Type: "DOMAIN", Value: "example.com", Action: "DIRECT"}}
	c.RouteOrder = []RouteRef{{"set", "b"}, {"rule", "r"}, {"set", "a"}}
	data, err := EncodeRouteSets(c.OrderedRouteSets())
	if err != nil {
		t.Fatal(err)
	}
	imported, err := DecodeRouteSets(data)
	if err != nil || len(imported) != 2 || imported[0].Name != "First visible" || imported[1].Name != "First stored" {
		t.Fatalf("portable export changed relative priority: %+v, %v", imported, err)
	}
	if c.Sets[0].ID != "a" {
		t.Fatal("export mutated stored records")
	}
}
