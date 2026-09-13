package main

import (
	"context"
	"embed"
	"github.com/d0kur0/Yoru/internal/core"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	manager, err := core.New(filepath.Join(configDir, "MihomoDesktop"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mainWindow atomic.Pointer[application.WebviewWindow]
	appearance := newAppearance()
	vpn := &VPN{manager: manager}
	app := application.New(application.Options{
		Name:        product.Name,
		Services:    []application.Service{application.NewService(appearance), application.NewService(vpn)},
		Description: "Клиент Mihomo для Windows и macOS.",
		OnShutdown:  func() { cancel(); _ = manager.Shutdown() },
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:         application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "local.tiho.desktop",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if window := mainWindow.Load(); window != nil {
					showMainWindow(window)
				}
			},
		},
	})
	traySupported := runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	background := application.NewRGB(24, 24, 29)
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: product.Name,
		Name:  "main",
		Width: 1160, Height: 760,
		Frameless: true,
		MinWidth:  800, MinHeight: 640,
		BackgroundColour: background,
		URL:              "/",
		Hidden:           traySupported && (slices.Contains(os.Args[1:], "--minimized") || manager.Config().Settings.Minimized),
		Windows:          application.WindowsWindow{Theme: application.SystemDefault},
		Mac:              application.MacWindow{Appearance: application.DefaultAppearance, TitleBar: application.MacTitleBar{Hide: true}},
	})
	appearance.app, appearance.window = app, window
	vpn.app, vpn.window = app, window

	mainWindow.Store(window)
	if traySupported {
		setupTray(app, window, manager)
	}
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go func() {
			if manager.Config().Settings.AutoConnect {
				if err := manager.Start(ctx); err != nil {
					log.Printf("Autoconnect: %v", err)
				}
			}
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					manager.RefreshDue(ctx)
				}
			}
		}()
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
