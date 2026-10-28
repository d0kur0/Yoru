package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func remoteConfig() Config {
	c := validConfig()
	c.Servers[0].ID, c.Servers[0].Host, c.Selected = "new", "new.example", "new"
	c.Servers[0].Subscription = "new-source"
	c.Subscriptions = []Subscription{{ID: "new-source", Name: "New subscription", URL: "https://provider.example/sub", Interval: "24"}}
	c.Rules = []Rule{{ID: "remote-rule", Type: "DOMAIN-SUFFIX", Value: "direct.example", Action: "DIRECT"}}
	c.Sets = []RouteSet{{ID: "remote-set", Name: "Work", Enabled: true, Domains: []string{"+.work.example"}, Action: "DIRECT", Resolvers: []string{"192.0.2.53"}}}
	c.RouteOrder = []RouteRef{{"rule", "remote-rule"}, {"set", "remote-set"}}
	c.DNS.Servers = []string{"https://dns.google/dns-query"}
	c.Settings.GroupMode, c.Settings.HealthInterval = "url-test", 60
	c.Revision = 999
	return c
}

func configResponse(t *testing.T, m *Manager, c Config) {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	m.downloadClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(b))}, nil
	})}
}

func TestURLSyncClearsOmittedRuleFields(t *testing.T) {
	m, _, _ := testManager(t)
	before := m.Config()
	before.Rules = []Rule{{ID: "old-ip", Type: "IP-CIDR", Value: "192.0.2.0/24", Action: "DIRECT", NoResolve: true}}
	before.RouteOrder = nil
	if err := m.Save(before); err != nil {
		t.Fatal(err)
	}
	configResponse(t, m, remoteConfig())
	got, err := m.SyncConfigURL(context.Background(), "https://config.example/profile", m.Config().Revision)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rules[0].NoResolve {
		t.Fatal("domain rule inherited noResolve from replaced IP rule")
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SyncConfigURL(context.Background(), got.ConfigSyncURL, got.Revision); err != nil {
		t.Fatal("repeat sync failed", err)
	}
}

func TestURLSyncReplacesFullConfigAndKeepsPrivateRestorableBackup(t *testing.T) {
	m, _, _ := testManager(t)
	before := m.Config()
	remote := remoteConfig()
	remote.Settings.TUN = !before.Settings.TUN
	remote.Settings.SystemProxy = !before.Settings.SystemProxy
	remote.Settings.Autostart = !before.Settings.Autostart
	remote.Settings.Minimized = !before.Settings.Minimized
	remote.Settings.AutoConnect = !before.Settings.AutoConnect
	remote.Settings.Tray = !before.Settings.Tray
	remote.TUNOptions.Interface = "remote-interface"
	remote.ConfigSyncURL = "https://untrusted.example/redirect-source"
	configResponse(t, m, remote)
	const source = "https://config.example/personal-secret"
	got, err := m.SyncConfigURL(context.Background(), source, before.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConfigSyncURL != source || got.Revision != before.Revision+1 {
		t.Fatal("source or local revision not preserved")
	}
	if got.Selected != "new" || !reflect.DeepEqual(got.Servers, remote.Servers) || !reflect.DeepEqual(got.Subscriptions, remote.Subscriptions) || !reflect.DeepEqual(got.Rules, remote.Rules) || !reflect.DeepEqual(got.Sets, remote.Sets) || !reflect.DeepEqual(got.RouteOrder, remote.RouteOrder) || !reflect.DeepEqual(got.DNS, remote.DNS) || got.Settings.GroupMode != "url-test" || got.Settings.HealthInterval != 60 {
		t.Fatal("full routing/subscriptions/DNS settings did not replace local config")
	}
	if got.Settings.TUN != before.Settings.TUN || got.Settings.SystemProxy != before.Settings.SystemProxy || got.Settings.Autostart != before.Settings.Autostart || got.Settings.Minimized != before.Settings.Minimized || got.Settings.AutoConnect != before.Settings.AutoConnect || got.Settings.Tray != before.Settings.Tray || got.TUNOptions.Interface != before.TUNOptions.Interface {
		t.Fatal("remote config changed device settings")
	}
	backupPath := filepath.Join(m.dir, configSyncBackup)
	backup, err := ReadBackup(backupPath)
	if err != nil || !reflect.DeepEqual(backup, before) {
		t.Fatal("previous configuration is not restorable", err)
	}
	loaded, err := New(m.dir)
	if err != nil || !reflect.DeepEqual(loaded.Config(), got) {
		t.Fatal("configuration/source did not persist", err)
	}
	// A second explicit update uses the remembered URL and replaces the backup.
	remote.DNS.Mode = "redir-host"
	configResponse(t, m, remote)
	updated, err := m.SyncConfigURL(context.Background(), got.ConfigSyncURL, got.Revision)
	if err != nil || updated.DNS.Mode != "redir-host" {
		t.Fatal("same URL refresh failed", err)
	}
	backup, err = ReadBackup(backupPath)
	if err != nil || !reflect.DeepEqual(backup, got) {
		t.Fatal("refresh backup did not capture previous import", err)
	}
}

func TestRemoteConfigRejectsPartialAndCorruptRouting(t *testing.T) {
	for _, mutation := range []func(*Config){
		func(c *Config) { c.RouteOrder = append(c.RouteOrder, c.RouteOrder[0]) },
		func(c *Config) { c.RouteOrder[0].ID = "missing" },
		func(c *Config) { c.RouteOrder = c.RouteOrder[:1] },
		func(c *Config) { c.Rules = append(c.Rules, c.Rules[0]) },
		func(c *Config) { c.Rules[0].Type = "unsupported" },
		func(c *Config) { c.DNS.Servers = []string{"invalid"} },
		func(c *Config) { c.Selected = "missing" },
	} {
		c := remoteConfig()
		mutation(&c)
		b, _ := json.Marshal(c)
		if _, err := DecodeRemoteConfig(b); err == nil {
			t.Fatal("corrupt remote config accepted")
		}
	}
	for _, b := range []string{`{}`, `null`, `{"servers":[]}`, `proxies: []`, strings.Repeat(" ", (2<<20)+1)} {
		if _, err := DecodeRemoteConfig([]byte(b)); err == nil {
			t.Fatal("partial or oversized remote config accepted")
		}
	}
}

func TestURLSyncErrorsNeverReplaceOrExposePersonalURL(t *testing.T) {
	const source = "https://config.example/private-token"
	for _, test := range []struct {
		name      string
		transport transportFunc
	}{
		{"network", func(*http.Request) (*http.Response, error) { return nil, errors.New(source) }},
		{"HTTP", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(source))}, nil
		}},
		{"bad-config", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}},
		{"too-large", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", (2<<20)+1)))}, nil
		}},
		{"redirect", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://other.example/"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, _, _ := testManager(t)
			before := m.Config()
			m.downloadClient = &http.Client{Transport: test.transport}
			_, err := m.SyncConfigURL(context.Background(), source, before.Revision)
			if err == nil || strings.Contains(err.Error(), "private-token") || strings.Contains(err.Error(), "config.example") {
				t.Fatal("missing or sensitive error", err)
			}
			if !reflect.DeepEqual(m.Config(), before) {
				t.Fatal("failed request replaced configuration")
			}
			if _, err := os.Stat(filepath.Join(m.dir, configSyncBackup)); !os.IsNotExist(err) {
				t.Fatal("failed fetch created a backup")
			}
		})
	}
}

func TestURLSyncAllowsOnlyHTTPS(t *testing.T) {
	m, _, _ := testManager(t)
	for _, source := range []string{"http://example.com/token", "https://user:pass@example.com/token", "https://example.com/#token", "https://example.com:99999/token", "https://example.com/token\n"} {
		if _, err := m.SyncConfigURL(context.Background(), source, m.Config().Revision); err == nil {
			t.Fatal("unsafe URL accepted")
		}
	}
}

func TestURLSyncFetchDoesNotBlockLocalEditsAndRejectsStaleResponse(t *testing.T) {
	m, _, _ := testManager(t)
	before := m.Config()
	started, release := make(chan struct{}), make(chan struct{})
	response, _ := json.Marshal(remoteConfig())
	m.downloadClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		close(started)
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(response))}, nil
	})}
	done := make(chan error, 1)
	go func() {
		_, err := m.SyncConfigURL(context.Background(), "https://config.example/token", before.Revision)
		done <- err
	}()
	<-started
	changed := make(chan error, 1)
	go func() { c := m.Config(); c.DefaultAction = "REJECT"; changed <- m.Save(c) }()
	select {
	case err := <-changed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("network fetch held the manager lock")
	}
	close(release)
	if err := <-done; err == nil || m.Config().DefaultAction != "REJECT" {
		t.Fatal("stale network response replaced a newer local edit", err)
	}
}

func TestURLSyncBackupFailurePreventsApply(t *testing.T) {
	m, _, _ := testManager(t)
	before := m.Config()
	if err := os.Mkdir(filepath.Join(m.dir, configSyncBackup), 0700); err != nil {
		t.Fatal(err)
	}
	configResponse(t, m, remoteConfig())
	if _, err := m.SyncConfigURL(context.Background(), "https://config.example/token", before.Revision); err == nil || !reflect.DeepEqual(m.Config(), before) {
		t.Fatal("applied configuration without backup", err)
	}
}

func TestURLSyncLiveReloadAndRollback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmtBool(fail), func(t *testing.T) {
			m, runner, controller, baseline := setupServerSwitch(t, "select")
			before := m.Config()
			process := m.process
			configResponse(t, m, remoteConfig())
			if fail {
				controller.failOnce()
			}
			_, err := m.SyncConfigURL(context.Background(), "https://config.example/token", before.Revision)
			payloads := controller.takePayloads()
			if fail {
				if err == nil || !reflect.DeepEqual(m.Config(), before) || len(payloads) != 2 || !reflect.DeepEqual(payloads[1], baseline) {
					t.Fatal("failed live import did not roll back", err)
				}
			} else if err != nil || m.Config().Selected != "new" || len(payloads) != 1 {
				t.Fatal("live import failed", err)
			}
			if !fail {
				current := m.Config()
				if _, err := m.SyncConfigURL(context.Background(), current.ConfigSyncURL, current.Revision); err != nil {
					t.Fatal("repeat live sync failed", err)
				}
				if !bytes.Equal(m.applied, runtimeSnapshot(m.Config())) || len(controller.takePayloads()) != 0 {
					t.Fatal("unchanged refresh marked configuration pending or reloaded the core")
				}
			}
			if runner.starts != 1 || m.process != process {
				t.Fatal("sync restarted the core")
			}
		})
	}
}

func fmtBool(value bool) string {
	if value {
		return "failure"
	}
	return "success"
}

func TestURLSyncRejectsHTTPSDowngrade(t *testing.T) {
	m, _, _ := testManager(t)
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/token", http.StatusFound)
	}))
	defer s.Close()
	m.downloadClient = s.Client()
	if _, err := m.SyncConfigURL(context.Background(), s.URL, m.Config().Revision); err == nil {
		t.Fatal("HTTPS downgrade accepted")
	}
}
