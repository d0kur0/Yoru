package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/d0kur0/Yoru/internal/core"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// JSON keeps the persisted schema independent of generated UI model constructors.
type VPN struct {
	manager *core.Manager
	app     *application.App
	window  *application.WebviewWindow
}

func encoded(v any) (string, error)        { b, e := json.Marshal(v); return string(b), e }
func (v *VPN) LoadConfig() (string, error) { return encoded(v.manager.Config()) }
func (v *VPN) SaveConfig(ctx context.Context, data string) (string, error) {
	c, e := core.DecodeConfig([]byte(data))
	if e != nil {
		return "", e
	}
	if e = v.manager.SaveLive(ctx, c); e != nil {
		return "", e
	}
	return encoded(v.manager.Config())
}
func (v *VPN) Status(ctx context.Context) (string, error) { return encoded(v.manager.Status(ctx)) }
func (v *VPN) InstallCore(ctx context.Context) (string, error) {
	i, e := v.manager.Install(ctx)
	if e != nil {
		return "", e
	}
	return encoded(i)
}
func (v *VPN) Start(ctx context.Context) error { return v.manager.Start(ctx) }
func (v *VPN) Stop() error                     { return v.manager.Stop() }
func (v *VPN) PreviewYAML() (string, error)    { return v.manager.Preview() }
func (v *VPN) ParseServerLink(link string) (string, error) {
	s, e := core.ParseLink(link)
	if e != nil {
		return "", e
	}
	return encoded(s)
}
func (v *VPN) UpdateSubscription(ctx context.Context, id string) (string, error) {
	c, e := v.manager.UpdateSubscription(ctx, id)
	if e != nil {
		return "", e
	}
	return encoded(c)
}
func (v *VPN) TestLatency(ctx context.Context) (string, error) {
	r, e := v.manager.TestLatency(ctx)
	if e != nil {
		return "", e
	}
	return encoded(r)
}
func (v *VPN) CloseConnection(ctx context.Context, id string) error {
	return v.manager.CloseConnection(ctx, id)
}
func (v *VPN) Logs() string { return v.manager.Logs() }
func (v *VPN) ListProcesses(ctx context.Context) (string, error) {
	p, e := core.ListProcesses(ctx)
	if e != nil {
		return "", e
	}
	return encoded(p)
}
func (v *VPN) RecoverProxy() error { return v.manager.RecoverProxy() }
func (v *VPN) ExportBackup() (bool, error) {
	path, e := v.app.Dialog.SaveFile().SetFilename("mihomo-settings.json").AddFilter("Настройки JSON", "*.json").AttachToWindow(v.window).PromptForSingleSelection()
	if e != nil || path == "" {
		return false, e
	}
	return true, v.manager.Export(path)
}
func (v *VPN) ImportBackup() (string, error) {
	path, e := v.app.Dialog.OpenFile().SetTitle("Импорт настроек").AddFilter("Настройки JSON", "*.json").AttachToWindow(v.window).PromptForSingleSelection()
	if e != nil || path == "" {
		return "", e
	}
	c, e := core.ReadBackup(path)
	if e != nil {
		return "", e
	}
	return encoded(c)
}

func (v *VPN) TestServerLatency(ctx context.Context, id string) (int, error) {
	return v.manager.TestServerLatency(ctx, id)
}

func (v *VPN) FetchSubscription(ctx context.Context, data string) (string, error) {
	var source core.Subscription
	if err := json.Unmarshal([]byte(data), &source); err != nil {
		return "", err
	}
	servers, err := v.manager.FetchSubscription(ctx, source)
	if err != nil {
		return "", err
	}
	return encoded(servers)
}

func (v *VPN) ProcessIcon(ctx context.Context, path string) string {
	return core.ProcessIcon(ctx, path)
}

func (v *VPN) TestRunningServerLatency(ctx context.Context, id string) (int, error) {
	return v.manager.TestRunningServerLatency(ctx, id)
}

// ImportRouteSets only reads the file; the UI previews it before SaveConfig.
func (v *VPN) ImportRouteSets() (string, error) {
	path, err := v.app.Dialog.OpenFile().SetTitle("Импорт наборов маршрутизации").AddFilter("Наборы Yoru (JSON)", "*.json").AttachToWindow(v.window).PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	sets, err := core.ReadRouteSets(path)
	if err != nil {
		return "", err
	}
	return encoded(sets)
}

// An empty id exports every routing set, preserving their order.
func (v *VPN) ExportRouteSets(id string) (bool, error) {
	sets := v.manager.Config().Sets
	if id != "" {
		found := false
		for _, set := range sets {
			if set.ID == id {
				sets = []core.RouteSet{set}
				found = true
				break
			}
		}
		if !found {
			return false, errors.New("Набор не найден")
		}
	}
	if len(sets) == 0 {
		return false, errors.New("Нет наборов для экспорта")
	}
	filename := "yoru-route-sets.json"
	if id != "" {
		filename = "yoru-route-set.json"
	}
	path, err := v.app.Dialog.SaveFile().SetFilename(filename).AddFilter("Наборы Yoru (JSON)", "*.json").AttachToWindow(v.window).PromptForSingleSelection()
	if err != nil || path == "" {
		return false, err
	}
	err = core.WriteRouteSets(path, sets)
	return err == nil, err
}
