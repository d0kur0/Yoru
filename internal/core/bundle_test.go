package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFirstConnectInstallsBundledCoreOffline(t *testing.T) {
	if !HasBundledCore() {
		t.Skip("run cmd/bundle-core to prepare platform payload")
	}
	m, runner, _ := testManager(t)
	m.releaseURL = "http://127.0.0.1:1/unreachable"
	if e := os.Remove(filepath.Join(m.dir, "core.json")); e != nil {
		t.Fatal(e)
	}
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	i, e := m.installed(true)
	if e != nil {
		t.Fatal(e)
	}
	if i.Version != BundledVersion {
		t.Fatal("wrong bundled version")
	}
	if runner == nil {
		t.Fatal("missing fake runner")
	}
	if e = m.Stop(); e != nil {
		t.Fatal(e)
	}
}

func TestBundledCoreDoesNotReplaceExistingInstallation(t *testing.T) {
	m, _, _ := testManager(t)
	if e := m.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	i, e := m.installed(true)
	if e != nil || i.Version != "test" {
		t.Fatal("existing version replaced", e)
	}
}
