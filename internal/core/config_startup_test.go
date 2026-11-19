package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnreadableConfigIsMovedAsideInsteadOfBlockingStartup(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Config().Revision != DefaultConfig().Revision || !strings.Contains(m.Status(context.Background()).Error, "config.invalid-") {
		t.Fatalf("expected defaults and visible error, got %q", m.Status(context.Background()).Error)
	}
	aside, _ := filepath.Glob(filepath.Join(dir, "config.invalid-*.json"))
	if len(aside) != 1 {
		t.Fatalf("broken config must be preserved: %v", aside)
	}
	if data, _ := os.ReadFile(aside[0]); string(data) != "{broken" {
		t.Fatal("preserved file changed")
	}
}

func TestLargeConfigSurvivesRestart(t *testing.T) {
	m, _, _ := testManager(t)
	c := m.Config()
	for i := 0; i < 6000; i++ {
		s := c.Servers[0]
		s.ID, s.Name = fmt.Sprintf("bulk-%d", i), fmt.Sprintf("Bulk server %d with a long descriptive name", i)
		c.Servers = append(c.Servers, s)
	}
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(m.dir, "config.json")); err != nil || info.Size() <= 2<<20 {
		t.Fatalf("fixture must exceed the old 2 MiB limit: %v", err)
	}
	reopened, err := New(m.dir)
	if err != nil || len(reopened.Config().Servers) != 6001 || reopened.Status(context.Background()).Error != "" {
		t.Fatalf("large config not reloaded: %v", err)
	}
}
