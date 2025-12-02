package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"singbox-launcher/core"
)

// App manages the UI structure and tabs
type App struct {
	window fyne.Window
	core   *core.AppController
	tabs   *container.AppTabs
}

// NewApp creates a new App instance
func NewApp(window fyne.Window, controller *core.AppController) *App {
	app := &App{
		window: window,
		core:   controller,
	}

	// Create tabs - Core is first (opens on startup)
	app.tabs = container.NewAppTabs(
		container.NewTabItem("Core", CreateCoreDashboardTab(controller)),
		container.NewTabItem("Diagnostics", CreateDiagnosticsTab(controller)),
		container.NewTabItem("Tools", CreateToolsTab(controller)),
		container.NewTabItem("Clash API", CreateClashAPITab(controller)),
	)

	// Set tab selection handler
	app.tabs.OnSelected = func(item *container.TabItem) {
		if item.Text == "Clash API" {
			controller.RefreshAPIFunc()
		}
	}

	return app
}

// GetTabs returns the tabs container
func (a *App) GetTabs() *container.AppTabs {
	return a.tabs
}

// GetWindow returns the main window
func (a *App) GetWindow() fyne.Window {
	return a.window
}

// GetController returns the core controller
func (a *App) GetController() *core.AppController {
	return a.core
}
