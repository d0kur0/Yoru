package core

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var processIcons = struct {
	sync.Mutex
	items map[string]string
}{items: make(map[string]string)}

// ProcessIcon reads an associated local application icon, never executes the application.
func ProcessIcon(ctx context.Context, path string) string {
	if !filepath.IsAbs(path) || strings.HasPrefix(path, `\\`) || strings.ContainsRune(path, 0) {
		return ""
	}
	processIcons.Lock()
	defer processIcons.Unlock()
	if icon, ok := processIcons.items[path]; ok {
		return icon
	}
	if len(processIcons.items) >= 256 {
		return ""
	}
	processIcons.items[path] = ""
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if !strings.EqualFold(filepath.Ext(path), ".exe") {
			return ""
		}
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; Add-Type -AssemblyName System.Drawing; $i=[System.Drawing.Icon]::ExtractAssociatedIcon($env:MIHOMO_ICON_PATH); if($i){try{$b=$i.ToBitmap();try{$s=New-Object IO.MemoryStream;try{$b.Save($s,[System.Drawing.Imaging.ImageFormat]::Png);[Convert]::ToBase64String($s.ToArray())}finally{$s.Dispose()}}finally{$b.Dispose()}}finally{$i.Dispose()}}`)
	case "darwin":
		cmd = exec.CommandContext(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", `ObjC.import('AppKit'); ObjC.import('Foundation'); var p=$.NSProcessInfo.processInfo.environment.objectForKey('MIHOMO_ICON_PATH'); var i=$.NSWorkspace.sharedWorkspace.iconForFile(p); var b=$.NSBitmapImageRep.imageRepWithData(i.TIFFRepresentation); ObjC.unwrap(b.representationUsingTypeProperties($.NSBitmapImageFileTypePNG,$.NSDictionary.dictionary).base64EncodedStringWithOptions(0));`)
	default:
		return ""
	}
	cmd.Env = append(os.Environ(), "MIHOMO_ICON_PATH="+path)
	hideConsole(cmd)
	output, err := cmd.Output()
	if err != nil || len(output) > 512*1024 {
		return ""
	}
	encoded := strings.TrimSpace(string(output))
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		return ""
	}
	icon := "data:image/png;base64," + encoded
	processIcons.items[path] = icon
	return icon
}
