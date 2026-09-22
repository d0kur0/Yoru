package core

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestAutostartTaskFileEncoding(t *testing.T) {
	path := `C:\Программы & Tools\夜🌙\Yoru.exe`
	data := autostartTaskFile(path, "S-1-5-21-123", true)
	if len(data)%2 != 0 || data[0] != 0xff || data[1] != 0xfe {
		t.Fatal("missing UTF-16LE BOM")
	}
	units := make([]uint16, (len(data)-2)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[2+i*2:])
	}
	decoded := string(utf16.Decode(units))
	want := strings.Replace(autostartTaskXML(path, "S-1-5-21-123", true), `encoding="UTF-8"`, `encoding="UTF-16"`, 1)
	if decoded != want {
		t.Fatal("XML encoding corrupted path or declaration")
	}
}

// Opt-in integration check: register, read back, delete. Never execute the task.
func TestAutostartTaskWindowsImport(t *testing.T) {
	if os.Getenv("YORU_TASK_SMOKE") != "1" {
		t.Skip("set YORU_TASK_SMOKE=1 for Windows Task Scheduler check")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("Yoru-encoding-test-%d", time.Now().UnixNano())
	path := filepath.Join(t.TempDir(), "task.xml")
	exe := `C:\Программы & Tools\夜🌙\Yoru.exe`
	if err := os.WriteFile(path, autostartTaskFile(exe, user.User.Sid.String(), true), 0600); err != nil {
		t.Fatal(err)
	}
	defer func() {
		out, err := exec.Command("schtasks.exe", "/delete", "/tn", name, "/f").CombinedOutput()
		if err != nil {
			t.Logf("cleanup: %s %v", out, err)
		}
	}()
	if out, err := exec.Command("schtasks.exe", "/create", "/tn", name, "/xml", path).CombinedOutput(); err != nil {
		t.Fatalf("register: %s %v", out, err)
	}
	out, err := exec.Command("schtasks.exe", "/query", "/tn", name, "/xml").CombinedOutput()
	if err != nil {
		t.Fatalf("read back: %s %v", out, err)
	}
	if !strings.Contains(string(out), "--minimized") {
		t.Fatal("task action lost arguments")
	}
}

func TestAutostartTaskKeepsExecutableAndInteractiveUser(t *testing.T) {
	for _, minimized := range []bool{false, true} {
		var task struct {
			User    string `xml:"Principals>Principal>UserId"`
			Logon   string `xml:"Principals>Principal>LogonType"`
			Level   string `xml:"Principals>Principal>RunLevel"`
			Command string `xml:"Actions>Exec>Command"`
			Args    string `xml:"Actions>Exec>Arguments"`
			Limit   string `xml:"Settings>ExecutionTimeLimit"`
		}
		path := `C:\Apps & Tools\Yoru.exe`
		if err := xml.Unmarshal([]byte(autostartTaskXML(path, "S-1-5-21-123", minimized)), &task); err != nil {
			t.Fatal(err)
		}
		if task.Command != path || task.User != "S-1-5-21-123" || task.Logon != "InteractiveToken" || task.Level != "HighestAvailable" || task.Limit != "PT0S" {
			t.Fatalf("invalid interactive app task: %+v", task)
		}
		want := ""
		if minimized {
			want = "--minimized"
		}
		if task.Args != want {
			t.Fatalf("args = %q, want %q", task.Args, want)
		}
	}
}
