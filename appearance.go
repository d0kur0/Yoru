package main

import (
	"encoding/json"
	"errors"
	"github.com/wailsapp/wails/v3/pkg/application"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type Appearance struct {
	mu     sync.Mutex
	mode   string
	path   string
	app    *application.App
	window *application.WebviewWindow
}
type AppearanceState struct {
	Mode     string `json:"mode"`
	Resolved string `json:"resolved"`
	Platform string `json:"platform"`
}

func newAppearance() *Appearance {
	a := &Appearance{mode: "dark"}
	if dir, err := os.UserConfigDir(); err == nil {
		a.path = filepath.Join(dir, "Yoru", "appearance.json")
		if data, err := os.ReadFile(a.path); err == nil {
			var mode string
			if json.Unmarshal(data, &mode) == nil && validTheme(mode) {
				a.mode = mode
			}
		}
	}
	return a
}
func validTheme(mode string) bool { return mode == "dark" || mode == "light" || mode == "system" }
func (a *Appearance) Get() AppearanceState {
	a.mu.Lock()
	mode := a.mode
	a.mu.Unlock()
	resolved := mode
	if mode == "system" {
		resolved = "light"
		if a.app != nil && a.app.Env.IsDarkMode() {
			resolved = "dark"
		}
	}
	return AppearanceState{mode, resolved, runtime.GOOS}
}
func (a *Appearance) Set(mode string) (AppearanceState, error) {
	if !validTheme(mode) {
		return AppearanceState{}, errors.New("unknown theme")
	}
	a.mu.Lock()
	if a.path != "" {
		if err := os.MkdirAll(filepath.Dir(a.path), 0700); err != nil {
			a.mu.Unlock()
			return AppearanceState{}, err
		}
		data, _ := json.Marshal(mode)
		if err := os.WriteFile(a.path+".tmp", data, 0600); err != nil {
			a.mu.Unlock()
			return AppearanceState{}, err
		}
		if err := os.Rename(a.path+".tmp", a.path); err != nil {
			a.mu.Unlock()
			return AppearanceState{}, err
		}
	}
	a.mode = mode
	a.mu.Unlock()
	a.apply()
	return a.Get(), nil
}
func (a *Appearance) apply() {
	if a.window == nil {
		return
	}
	state := a.Get()
	colour := application.NewRGB(245, 245, 245)
	if state.Resolved == "dark" {
		colour = application.NewRGB(7, 7, 10)
	}
	a.window.SetBackgroundColour(colour)
	data, _ := json.Marshal(state)
	a.window.ExecJS(`window.dispatchEvent(new CustomEvent('yoru-appearance',{detail:` + string(data) + `}));`)
}
