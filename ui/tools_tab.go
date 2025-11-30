package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/core"
	"singbox-launcher/internal/platform"
)

// CreateToolsTab creates and returns the content for the "Tools" tab.
func CreateToolsTab(ac *core.AppController) fyne.CanvasObject {
	logsButton := widget.NewButton("Open Logs Folder", func() {
		logsDir := platform.GetLogsDir(ac.ExecDir)
		if err := platform.OpenFolder(logsDir); err != nil {
			log.Printf("toolsTab: Failed to open logs folder: %v", err)
			ShowError(ac.MainWindow, err)
		}
	})
	updateButton := widget.NewButton("Update Config", ac.RunParser)
	configButton := widget.NewButton("Open Config Folder", func() {
		binDir := platform.GetBinDir(ac.ExecDir)
		if err := platform.OpenFolder(binDir); err != nil {
			log.Printf("toolsTab: Failed to open config folder: %v", err)
			ShowError(ac.MainWindow, err)
		}
	})
	killButton := widget.NewButton("Kill Sing-Box", func() {
		go func() {
			processName := platform.GetProcessNameForCheck()
			_ = platform.KillProcess(processName)
			fyne.Do(func() {
				ac.ShowAutoHideInfo("Kill", "Sing-Box killed if running.")
				ac.RunningState.Set(false)
			})
		}()
	})

	return container.NewVBox(
		logsButton,
		updateButton,
		configButton,
		killButton,
	)
}

