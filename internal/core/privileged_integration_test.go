package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakePrivilegedRunner struct {
	*fakeRunner
	t          *testing.T
	denied     bool
	authorized int
}

func (r *fakePrivilegedRunner) StartPrivileged(ctx context.Context, installation Installation, c Config, controller, secret string, port int, output io.Writer) (Process, error) {
	r.authorized++
	if r.denied {
		return nil, errors.New("authorization cancelled")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := c.YAML(controller, secret, port)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(r.t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, b, 0600); err != nil {
		return nil, err
	}
	return r.fakeRunner.Start(installation.Path, []string{"-f", p}, output)
}

func TestPrivilegedCoreStartLeavesGUIUnprivileged(t *testing.T) {
	m, runner, platform := testManager(t)
	r := &fakePrivilegedRunner{fakeRunner: runner, t: t}
	m.runner = r
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := m.Status(context.Background())
	if r.authorized != 1 || !status.Running || !status.Elevated || status.NeedsElevation || platform.IsElevated() {
		t.Fatalf("unexpected privileged core status: calls=%d status=%+v", r.authorized, status)
	}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if m.Status(context.Background()).Elevated {
		t.Fatal("stopped core retained elevated status")
	}
}

func TestOrdinaryCoreDoesNotRequestAuthorization(t *testing.T) {
	m, runner, _ := testManager(t)
	r := &fakePrivilegedRunner{fakeRunner: runner, t: t}
	m.runner = r
	c := m.Config()
	c.Settings.TUN = false
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.authorized != 0 || runner.starts != 1 || m.Status(context.Background()).Elevated {
		t.Fatal("ordinary connection requested privileges")
	}
}

func TestLiveTUNAuthorizationRestartsOrRestoresPriorConnection(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "approved", true: "cancelled"}[denied], func(t *testing.T) {
			m, runner, _ := testManager(t)
			r := &fakePrivilegedRunner{fakeRunner: runner, t: t, denied: denied}
			m.runner = r
			c := m.Config()
			c.Settings.TUN = false
			if err := m.Save(c); err != nil {
				t.Fatal(err)
			}
			if err := m.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			previous := runner.p
			c = m.Config()
			c.Settings.TUN = true
			err := m.SaveLive(context.Background(), c)
			if denied && (err == nil || !strings.Contains(err.Error(), "authorization cancelled")) {
				t.Fatalf("missing cancellation error: %v", err)
			}
			if !denied && err != nil {
				t.Fatal(err)
			}
			select {
			case <-previous.done:
			default:
				t.Fatal("old ordinary child still running")
			}
			if r.authorized != 1 || runner.starts != 2 || !m.Status(context.Background()).Running {
				t.Fatalf("connection was not restored/replaced: auth=%d starts=%d", r.authorized, runner.starts)
			}
			if m.Config().Settings.TUN == denied || m.Status(context.Background()).Elevated == denied {
				t.Fatal("persisted/runtime state contradicts authorization outcome")
			}
			if denied && m.Status(context.Background()).Pending {
				t.Fatal("cancelled TUN toggle left a false pending change")
			}
			stored, readErr := os.ReadFile(filepath.Join(m.dir, "config.json"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			persisted, decodeErr := DecodeConfig(stored)
			if decodeErr != nil || persisted.Settings.TUN == denied {
				t.Fatalf("disk state contradicts authorization outcome: %v", decodeErr)
			}
		})
	}
}

func TestCancelledTUNKeepsPreviouslyPendingSettingsUnapplied(t *testing.T) {
	m, runner, _ := testManager(t)
	r := &fakePrivilegedRunner{fakeRunner: runner, t: t, denied: true}
	m.runner = r
	c := m.Config()
	c.Settings.TUN = false
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	c = m.Config()
	c.Servers[0].Host = "pending.example"
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	c = m.Config()
	c.Settings.TUN = true
	if err := m.SaveLive(context.Background(), c); err == nil {
		t.Fatal("missing cancellation error")
	}
	if m.Config().Servers[0].Host != "pending.example" || m.Config().Settings.TUN || !m.Status(context.Background()).Pending {
		t.Fatal("saved pending settings were lost or incorrectly applied")
	}
	proxies := runner.lastConfig["proxies"].([]any)
	if proxies[0].(map[string]any)["server"] != "example.com" {
		t.Fatal("cancellation applied a previously pending server edit")
	}
}

func TestPrivilegedLiveReloadRejectsNestedFileReferences(t *testing.T) {
	m, runner, _ := testManager(t)
	r := &fakePrivilegedRunner{fakeRunner: runner, t: t}
	m.runner = r
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := m.Config()
	before := c.Revision
	c.Servers[0].Options = map[string]any{"xhttp-opts": map[string]any{"download-settings": map[string]any{"Certificate": "/private/root.pem"}}}
	if err := m.SaveLive(context.Background(), c); err == nil {
		t.Fatal("privileged live reload accepted a nested file reference")
	}
	if m.Config().Revision != before || !m.Status(context.Background()).Running {
		t.Fatal("rejected reload changed the saved or running connection")
	}
}
