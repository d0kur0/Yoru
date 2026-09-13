package main

func (v *VPN) CopyLogText(text string) bool {
	if len(text) > 128<<10 {
		return false
	}
	return v.app.Clipboard.SetText(text)
}

func (v *VPN) ClearLogs() error { return v.manager.ClearLogs() }
func (v *VPN) ExportLogs() (bool, error) {
	path, e := v.app.Dialog.SaveFile().SetFilename("mihomo-logs.zip").AddFilter("Журналы ZIP", "*.zip").AttachToWindow(v.window).PromptForSingleSelection()
	if e != nil || path == "" {
		return false, e
	}
	return true, v.manager.ExportLogs(path)
}
