package core

import (
	"bytes"
	"testing"
)

func TestLatencyURLSettings(t *testing.T) {
	c := DefaultConfig()
	before := runtimeSnapshot(c)
	c.Settings.LatencyURL = "https://example.com/check"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.LatencyTestURL() != c.Settings.LatencyURL {
		t.Fatal("custom probe ignored")
	}
	if bytes.Equal(before, runtimeSnapshot(c)) {
		t.Fatal("fallback health URL change must mark config pending")
	}
	for _, value := range []string{"file:///etc/passwd", "https://user:password@example.com", "https://example.com/#part", "https://"} {
		c.Settings.LatencyURL = value
		if c.Validate() == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}
