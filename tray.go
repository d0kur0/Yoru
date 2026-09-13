package main

import (
	_ "embed"
	"github.com/d0kur0/Yoru/internal/core"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func showMainWindow(window *application.WebviewWindow) {
	window.UnMinimise()
	window.Show()
	window.Focus()
}

func setupTray(app *application.App, window *application.WebviewWindow, manager *core.Manager) {
	tray := app.SystemTray.New()
	tray.SetTooltip(product.Name)
	if runtime.GOOS == "darwin" {
		// Template alpha lets macOS choose the menu-bar colour for either theme.
		tray.SetTemplateIcon(trayIcon(true))
	} else {
		tray.SetIcon(trayIcon(false))
	}

	menu := application.NewMenu()
	menu.Add("Открыть " + product.Name).OnClick(func(*application.Context) { showMainWindow(window) })
	menu.Add("Скрыть окно").OnClick(func(*application.Context) { window.Hide() })
	menu.AddSeparator()
	menu.Add("Выход").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)
	if runtime.GOOS == "windows" {
		tray.OnClick(func() { showMainWindow(window) })
	} else {
		tray.OnClick(tray.ShowMenu)
	}
	tray.OnRightClick(tray.ShowMenu)

	// Keep the webview and its state alive; explicit application Quit bypasses this hook.
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if manager.Config().Settings.Tray {
			event.Cancel()
			window.Hide()
		} else {
			app.Quit()
		}
	})
	window.OnWindowEvent(events.Common.WindowMinimise, func(*application.WindowEvent) {
		window.Hide()
	})
}

//go:embed assets/tray.png
var colourTrayIcon []byte

//go:embed assets/tray-template.png
var templateTrayIcon []byte

func trayIcon(template bool) []byte {
	if template {
		return templateTrayIcon
	}
	return colourTrayIcon
}
