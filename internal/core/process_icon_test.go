package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProcessIconRejectsNonlocalPaths(t *testing.T) {
	for _, path := range []string{"", "app.exe", `\\server\share\app.exe`, "https://example.org/app.exe"} {
		if ProcessIcon(context.Background(), path) != "" {
			t.Fatalf("accepted %q", path)
		}
	}
}

func TestWindowsProcessIcon(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows shell icon extraction")
	}
	path := filepath.Join(os.Getenv("WINDIR"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	icon := ProcessIcon(context.Background(), path)
	if !strings.HasPrefix(icon, "data:image/png;base64,") {
		t.Fatal("system executable icon was not extracted")
	}
	if cached := ProcessIcon(context.Background(), path); cached != icon {
		t.Fatal("icon cache mismatch")
	}
}
