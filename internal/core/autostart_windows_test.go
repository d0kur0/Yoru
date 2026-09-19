package core

import (
	"encoding/xml"
	"testing"
)

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
