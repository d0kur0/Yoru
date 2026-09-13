package main

import (
	"github.com/d0kur0/Yoru/internal/core"
	"os"
)

func (v *VPN) ImportYAML() (string, error) {
	path, e := v.app.Dialog.OpenFile().SetTitle("Импорт конфигурации Mihomo").AddFilter("YAML", "*.yaml;*.yml;*.txt").AttachToWindow(v.window).PromptForSingleSelection()
	if e != nil || path == "" {
		return "", e
	}
	data, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	result, e := core.ImportYAML(data)
	if e != nil {
		return "", e
	}
	return encoded(result)
}
func (v *VPN) WorkPreset() (string, error) { return encoded(core.WorkPreset()) }
