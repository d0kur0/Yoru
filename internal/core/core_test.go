package core

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func validConfig() Config {
	c := DefaultConfig()
	c.Servers = []Server{{ID: "a", Name: "A", Host: "example.com", Port: 443, Protocol: "VLESS", Transport: "Reality", Secret: "01234567-89ab-cdef-0123-456789abcdef", PublicKey: "test-public-key", ShortID: "abcd", SNI: "example.com", Flow: "xtls-rprx-vision"}} // gitleaks:allow -- synthetic sequential UUID fixture
	c.Selected = "a"
	return c
}
func TestConfigCompilesCompleteRouting(t *testing.T) {
	c := validConfig()
	c.Rules = []Rule{{ID: "r", Type: "DOMAIN-SUFFIX", Value: "example.org", Action: "DIRECT"}}
	c.DNS.Policies = []Policy{{"+.corp.example", "192.168.1.1"}, {"+.other.example", "10.1.0.2"}, {"+.corp.example", "192.168.1.2"}}
	b, e := c.YAML("127.0.0.1:45001", "secret", 45002)
	if e != nil {
		t.Fatal(e)
	}
	var got map[string]any
	if e = yaml.Unmarshal(b, &got); e != nil {
		t.Fatal(e)
	}
	if got["external-controller"] != "127.0.0.1:45001" || got["secret"] != "secret" || got["mixed-port"] != 45002 || got["allow-lan"] != false {
		t.Fatal("controller isolation lost")
	}
	rules := got["rules"].([]any)
	if rules[0] != "DOMAIN-SUFFIX,example.org,DIRECT" || rules[1] != "MATCH,DIRECT" {
		t.Fatal(rules)
	}
	proxy := got["proxies"].([]any)[0].(map[string]any)
	if proxy["uuid"] != c.Servers[0].Secret || proxy["flow"] != "xtls-rprx-vision" || proxy["reality-opts"] == nil {
		t.Fatal(proxy)
	}
	dns := got["dns"].(map[string]any)
	policies := dns["nameserver-policy"].(map[string]any)
	if len(policies["+.corp.example"].([]any)) != 2 || policies["+.other.example"] == nil {
		t.Fatal(policies)
	}
	tun := got["tun"].(map[string]any)
	if len(tun["route-exclude-address"].([]any)) != 3 || len(tun["dns-hijack"].([]any)) != 2 {
		t.Fatal(tun)
	}
}
func TestConfigRejectsBrokenInput(t *testing.T) {
	for name, mutate := range map[string]func(*Config){
		"rule injection": func(c *Config) {
			c.Rules = []Rule{{ID: "r", Type: "DOMAIN", Value: "ok,REJECT\nMATCH,DIRECT", Action: "DIRECT"}}
		},
		"unknown rule":      func(c *Config) { c.Rules = []Rule{{ID: "r", Type: "SCRIPT", Value: "x", Action: "DIRECT"}} },
		"bad cidr":          func(c *Config) { c.Settings.RouteExclusions = []string{"999.0.0.0/8"} },
		"bad dns":           func(c *Config) { c.DNS.Servers = []string{"https://"} },
		"missing secret":    func(c *Config) { c.Servers[0].Secret = "" },
		"duplicate id":      func(c *Config) { c.Servers = append(c.Servers, c.Servers[0]) },
		"unknown selection": func(c *Config) { c.Selected = "missing" },
		"external file":     func(c *Config) { c.Servers[0].Options = map[string]any{"private-key": "/tmp/key"} },
	} {
		t.Run(name, func(t *testing.T) {
			c := validConfig()
			mutate(&c)
			if c.Validate() == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
}
func TestLinkAndSubscriptions(t *testing.T) {
	link := "vless://01234567-89ab-cdef-0123-456789abcdef@example.com:443?security=reality&pbk=key&sid=abcd&sni=site.example&fp=chrome&flow=xtls-rprx-vision#Berlin"
	s, e := ParseLink(link)
	if e != nil || s.Flow != "xtls-rprx-vision" || s.SNI != "site.example" || s.PublicKey != "key" {
		t.Fatalf("%+v %v", s, e)
	}
	servers, e := ParseSubscription([]byte(link+"\n"), "sub")
	if e != nil || len(servers) != 1 || servers[0].Subscription != "sub" {
		t.Fatal(e)
	}
	yamlData := []byte("proxies:\n  - name: node\n    type: trojan\n    server: example.com\n    port: 443\n    password: pass\n    sni: example.org\n    network: grpc\n    grpc-opts:\n      grpc-service-name: tunnel\n")
	servers, e = ParseSubscription(yamlData, "sub")
	if e != nil {
		t.Fatal(e)
	}
	p, e := servers[0].Proxy()
	if e != nil || p["grpc-opts"] == nil || p["sni"] != "example.org" {
		t.Fatal(p, e)
	}
	if _, e = ParseSubscription([]byte("proxies:\n - {name: broken, type: unsupported}"), "sub"); e == nil {
		t.Fatal("unsupported subscription silently dropped")
	}
}
func compressed(t *testing.T, name string, data []byte) (Asset, []byte) {
	t.Helper()
	var b bytes.Buffer
	if strings.HasSuffix(name, ".zip") {
		z := zip.NewWriter(&b)
		f, e := z.Create("../../mihomo.exe")
		if e != nil {
			t.Fatal(e)
		}
		_, _ = f.Write(data)
		_ = z.Close()
	} else {
		z := gzip.NewWriter(&b)
		_, _ = z.Write(data)
		_ = z.Close()
	}
	sum := sha256.Sum256(b.Bytes())
	return Asset{Name: name, Digest: "sha256:" + hex.EncodeToString(sum[:])}, b.Bytes()
}
func TestArchiveDigestAndExtraction(t *testing.T) {
	for _, ext := range []string{".gz", ".zip"} {
		a, b := compressed(t, "mihomo"+ext, []byte("test binary"))
		got, e := unpack(a, b)
		if e != nil || string(got) != "test binary" {
			t.Fatal(e)
		}
		b[len(b)-1] ^= 1
		if _, e = unpack(a, b); e == nil {
			t.Fatal("bad checksum accepted")
		}
	}
}
func TestReleaseAssets(t *testing.T) {
	for _, os := range []string{"windows", "darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			name, e := assetName(os, arch, "v1.19.30")
			if e != nil || !strings.HasPrefix(name, "mihomo-"+os+"-") {
				t.Fatal(name, e)
			}
		}
	}
	if _, e := assetName("windows", "amd64", "../../escape"); e == nil {
		t.Fatal("path traversal accepted")
	}
}

type fakePlatform struct {
	enabled, restored int
	fail              bool
}

func (*fakePlatform) Autostart(bool, bool) error { return nil }
func (p *fakePlatform) Proxy(int) (func() error, error) {
	if p.fail {
		return nil, errors.New("proxy denied")
	}
	p.enabled++
	return func() error { p.restored++; return nil }, nil
}

type fakeProcess struct {
	server *httptest.Server
	done   chan struct{}
	once   sync.Once
}

func (p *fakeProcess) Wait() error { <-p.done; return nil }
func (p *fakeProcess) Stop() error { p.once.Do(func() { p.server.Close(); close(p.done) }); return nil }

type fakeRunner struct {
	starts, validations int
	validationError     bool
	p                   *fakeProcess
	token               string
}

func (r *fakeRunner) Validate(context.Context, string, []string) error {
	r.validations++
	if r.validationError {
		return errors.New("invalid config")
	}
	return nil
}
func (r *fakeRunner) Start(_ string, args []string, _ io.Writer) (Process, error) {
	r.starts++
	b, e := os.ReadFile(args[len(args)-1])
	if e != nil {
		return nil, e
	}
	var c map[string]any
	if e = yaml.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	r.token = c["secret"].(string)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer "+r.token {
			w.WriteHeader(401)
			return
		}
		switch {
		case req.URL.Path == "/version":
			_, _ = io.WriteString(w, `{"version":"test"}`)
		case req.URL.Path == "/connections":
			_, _ = io.WriteString(w, `{"downloadTotal":2048,"uploadTotal":256,"connections":[{"id":"c","metadata":{"process":"browser","processPath":"/usr/bin/browser","host":"example.org","destinationIP":"192.0.2.1","network":"tcp"},"rule":"DOMAIN","chains":["server-a","PROXY"],"download":1024,"upload":128}]}`)
		case strings.HasSuffix(req.URL.Path, "/delay"):
			_, _ = io.WriteString(w, `{"delay":42}`)
		default:
			w.WriteHeader(204)
		}
	}))
	_ = server.Listener.Close()
	server.Listener, e = net.Listen("tcp", c["external-controller"].(string))
	if e != nil {
		return nil, e
	}
	server.Start()
	r.p = &fakeProcess{server: server, done: make(chan struct{})}
	return r.p, nil
}
func testManager(t *testing.T) (*Manager, *fakeRunner, *fakePlatform) {
	t.Helper()
	m, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	r := &fakeRunner{}
	p := &fakePlatform{}
	m.runner = r
	m.platform = p
	bin := []byte("this is NOT an executable")
	sum := sha256.Sum256(bin)
	path := filepath.Join(m.dir, "core", "test", "mihomo")
	if e = atomicWrite(path, bin, 0700); e != nil {
		t.Fatal(e)
	}
	manifest, _ := json.Marshal(Installation{"test", path, hex.EncodeToString(sum[:])})
	_ = atomicWrite(filepath.Join(m.dir, "core.json"), manifest, 0600)
	if e = m.Save(validConfig()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = m.Stop() })
	return m, r, p
}
func TestLifecycleOnlyOwnedProcess(t *testing.T) {
	m, r, p := testManager(t)
	c := m.Config()
	c.Settings.SystemProxy = true
	if e := m.Save(c); e != nil {
		t.Fatal(e)
	}
	if r.starts != 0 || m.Status(context.Background()).Running {
		t.Fatal("passive operations started a process")
	}
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if r.validations != 1 || r.starts != 1 || len(r.token) != 64 {
		t.Fatal("unvalidated or unauthenticated start")
	}
	s := m.Status(context.Background())
	if !s.Running || s.DownloadTotal != 2048 || len(s.Connections) != 1 || s.Connections[0].App != "browser" || s.Connections[0].ProcessPath != "/usr/bin/browser" {
		t.Fatalf("%+v", s)
	}
	if e := m.Start(context.Background()); e == nil || r.starts != 1 {
		t.Fatal("duplicate child")
	}
	c = m.Config()
	c.Settings.Mode = "direct"
	if e := m.Save(c); e != nil {
		t.Fatal(e)
	}
	if !m.Status(context.Background()).Pending {
		t.Fatal("changes silently applied")
	}
	if delay, err := m.TestServerLatency(context.Background(), "a"); err != nil || delay != 42 {
		t.Fatalf("selected node delay: %d, %v", delay, err)
	}
	if delay, err := m.TestServerLatency(context.Background(), "not-applied"); err != nil || delay != -2 {
		t.Fatalf("missing node: %d %v", delay, err)
	}
	latency, e := m.TestLatency(context.Background())
	if e != nil || latency["a"] != 42 {
		t.Fatal(latency, e)
	}
	c = m.Config()
	c.Servers[0].Port++
	if e = m.Save(c); e != nil {
		t.Fatal(e)
	}
	if delay, err := m.TestServerLatency(context.Background(), "a"); err != nil || delay != -2 {
		t.Fatalf("changed saved node: %d %v", delay, err)
	}
	if delay, err := m.TestRunningServerLatency(context.Background(), "a"); err != nil || delay != 42 {
		t.Fatalf("running node must remain measurable with pending edits: %d %v", delay, err)
	}
	if delay, err := m.TestRunningServerLatency(context.Background(), "missing"); err != nil || delay != -2 {
		t.Fatalf("unknown running node: %d %v", delay, err)
	}
	if e = m.Stop(); e != nil {
		t.Fatal(e)
	}
	if m.Status(context.Background()).Running || p.enabled != 1 || p.restored != 1 {
		t.Fatal("stop did not restore")
	}
	_ = m.Stop()
	if p.restored != 1 {
		t.Fatal("double restore")
	}
}
func TestValidationFailureDoesNotStart(t *testing.T) {
	m, r, p := testManager(t)
	r.validationError = true
	if m.Start(context.Background()) == nil {
		t.Fatal("expected validation error")
	}
	if r.starts != 0 || p.enabled != 0 {
		t.Fatal("effects after failed validation")
	}
}
func TestProxyFailureStopsChild(t *testing.T) {
	m, r, p := testManager(t)
	p.fail = true
	c := m.Config()
	c.Settings.SystemProxy = true
	_ = m.Save(c)
	if m.Start(context.Background()) == nil {
		t.Fatal("expected proxy failure")
	}
	select {
	case <-r.p.done:
	default:
		t.Fatal("child left running")
	}
}
func TestChangedBinaryNeverExecuted(t *testing.T) {
	m, r, _ := testManager(t)
	i, _ := m.installed(false)
	_ = os.WriteFile(i.Path, []byte("changed"), 0700)
	if m.Start(context.Background()) == nil || r.starts != 0 || r.validations != 0 {
		t.Fatal("tampered binary used")
	}
}
func TestConfigSurvivesReloadWithoutStarting(t *testing.T) {
	m, _, _ := testManager(t)
	loaded, e := New(m.dir)
	if e != nil {
		t.Fatal(e)
	}
	if loaded.Config().Servers[0].Secret != m.Config().Servers[0].Secret || loaded.process != nil {
		t.Fatal("persistence or startup")
	}
}

func TestStaleConfigCannotOverwriteBackgroundUpdate(t *testing.T) {
	m, _, _ := testManager(t)
	old := m.Config()
	fresh := m.Config()
	fresh.Settings.Mode = "direct"
	if e := m.Save(fresh); e != nil {
		t.Fatal(e)
	}
	old.Settings.Mode = "global"
	if m.Save(old) == nil {
		t.Fatal("lost-update accepted")
	}
	if m.Config().Settings.Mode != "direct" {
		t.Fatal("new config overwritten")
	}
}
func TestSubscriptionUpdateIsAtomic(t *testing.T) {
	m, _, _ := testManager(t)
	c := m.Config()
	c.Subscriptions = []Subscription{{ID: "sub", Name: "Source", URL: "https://provider.example/profile", Interval: "24"}}
	if e := m.Save(c); e != nil {
		t.Fatal(e)
	}
	payload := `trojan://test-password@node.example:443#Node`
	m.downloadClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(payload))}, nil
	})}
	updated, e := m.UpdateSubscription(context.Background(), "sub")
	if e != nil {
		t.Fatal(e)
	}
	if len(updated.Servers) != 2 || updated.Selected != "a" || updated.Revision != m.Config().Revision {
		t.Fatal("manual server or revision lost")
	}
	id := updated.Servers[1].ID
	updated, e = m.UpdateSubscription(context.Background(), "sub")
	if e != nil || len(updated.Servers) != 2 || updated.Servers[1].ID != id {
		t.Fatal("update duplicated servers", e)
	}
	payload = "invalid subscription"
	before := m.Config().Revision
	if _, e = m.UpdateSubscription(context.Background(), "sub"); e == nil {
		t.Fatal("invalid subscription accepted")
	}
	if m.Config().Revision != before || len(m.Config().Servers) != 2 {
		t.Fatal("failed update modified config")
	}
}
func TestWebsocketLinkPreservesNoTLSAndPath(t *testing.T) {
	s, e := ParseLink("vless://01234567-89ab-cdef-0123-456789abcdef@node.example:80?type=ws&security=none&path=%2Fsocket&host=cdn.example")
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.Proxy()
	if e != nil || p["tls"] != false {
		t.Fatal(p, e)
	}
	opts := p["ws-opts"].(map[string]any)
	if opts["path"] != "/socket" {
		t.Fatal(opts)
	}
}
func TestReadLimitRejectsOversizedResponse(t *testing.T) {
	if _, e := readLimit(strings.NewReader("too large"), 3); e == nil {
		t.Fatal("limit ignored")
	}
}

func TestBackupRoundtripIncludesSecretsAndDNS(t *testing.T) {
	m, _, _ := testManager(t)
	path := filepath.Join(t.TempDir(), "backup.json")
	if e := m.Export(path); e != nil {
		t.Fatal(e)
	}
	c, e := ReadBackup(path)
	if e != nil {
		t.Fatal(e)
	}
	want, _ := json.Marshal(m.Config())
	got, _ := json.Marshal(c)
	if !bytes.Equal(want, got) {
		t.Fatal("backup lost configuration")
	}
	if e = os.WriteFile(path, []byte(`{"format":"unknown","version":1}`), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadBackup(path); e == nil {
		t.Fatal("unknown format accepted")
	}
}
func TestCrashRestoresProxy(t *testing.T) {
	m, r, p := testManager(t)
	c := m.Config()
	c.Settings.SystemProxy = true
	_ = m.Save(c)
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	_ = r.p.Stop()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		done := m.process == nil && p.restored == 1
		m.mu.Unlock()
		if done {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("crash cleanup did not happen")
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestInstallDownloadsButNeverExecutes(t *testing.T) {
	m, r, _ := testManager(t)
	name, e := assetName(runtime.GOOS, runtime.GOARCH, "v1.2.3")
	if e != nil {
		t.Fatal(e)
	}
	a, b := compressed(t, name, []byte("a downloaded binary"))
	a.URL = "https://github.com/MetaCubeX/mihomo/releases/download/v1.2.3/" + name
	release, _ := json.Marshal(Release{"v1.2.3", []Asset{a}})
	m.downloadClient = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		body := release
		if req.URL.String() == a.URL {
			body = b
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
	installed, e := m.Install(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if installed.Version != "v1.2.3" || r.starts != 0 || r.validations != 0 {
		t.Fatal("installer executed binary")
	}
	if _, e = m.installed(true); e != nil {
		t.Fatal(e)
	}
}
