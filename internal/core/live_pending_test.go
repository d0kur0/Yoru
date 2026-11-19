package core

import (
	"context"
	"testing"
)

func TestUIOnlyChangesDoNotReportPending(t *testing.T) {
	m, _, _ := testManager(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := m.Config()
	c.Settings.Tray = !c.Settings.Tray
	c.Servers[0].Flag = "🇩🇪"
	if err := m.SaveLive(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if m.Status(context.Background()).Pending {
		t.Fatal("UI-only change reported as pending")
	}
	c = m.Config()
	c.Logging.Level = "debug"
	if err := m.SaveLive(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if !m.Status(context.Background()).Pending {
		t.Fatal("core log level change must stay pending until restart")
	}
}
