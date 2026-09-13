package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppearancePersistsAndRejectsInvalidMode(t *testing.T) {
	a := &Appearance{mode: "dark", path: filepath.Join(t.TempDir(), "appearance.json")}
	for _, mode := range []string{"light", "dark", "system"} {
		state, err := a.Set(mode)
		if err != nil || state.Mode != mode {
			t.Fatalf("Set(%q): %v, %v", mode, state, err)
		}
		data, err := os.ReadFile(a.path)
		if err != nil || string(data) != `"`+mode+`"` {
			t.Fatalf("persist %q: %s, %v", mode, data, err)
		}
	}
	if _, err := a.Set("invalid"); err == nil {
		t.Fatal("invalid mode accepted")
	}
	if a.Get().Mode != "system" {
		t.Fatal("invalid mode changed preference")
	}
}

func TestAppearanceStorageFailureKeepsMode(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	a := &Appearance{mode: "dark", path: filepath.Join(blocker, "appearance.json")}
	if _, err := a.Set("light"); err == nil {
		t.Fatal("storage failure ignored")
	}
	if a.Get().Mode != "dark" {
		t.Fatal("failed write changed preference")
	}
}
