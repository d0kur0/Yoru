//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestWindowsInstallerParameters(t *testing.T) {
	got, err := windowsInstallerParameters(`C:\Program Files\Custom Yoru\Yoru.exe`, 4271)
	if err != nil {
		t.Fatal(err)
	}
	if want := `/UPDATEPID=4271 /D=C:\Program Files\Custom Yoru`; got != want {
		t.Fatalf("parameters = %q, want %q", got, want)
	}
	if !strings.HasSuffix(got, `/D=C:\Program Files\Custom Yoru`) {
		t.Fatal("/D must be the final NSIS argument")
	}
}

func TestWindowsInstallerParametersRejectsUnsafeTarget(t *testing.T) {
	for _, test := range []struct {
		name string
		exe  string
		pid  int
	}{
		{"relative path", `bin\Yoru.exe`, 42},
		{"development binary", `C:\dev\Yoru-next.exe`, 42},
		{"root directory", `C:\Yoru.exe`, 42},
		{"missing pid", `C:\Apps\Yoru.exe`, 0},
		{"newline", "C:\\Apps\nOther\\Yoru.exe", 42},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := windowsInstallerParameters(test.exe, test.pid); err == nil {
				t.Fatal("unsafe target accepted")
			}
		})
	}
}
