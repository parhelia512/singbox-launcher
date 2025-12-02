package ui

import (
	"log"
	"time"

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
	// Create progress bar and status label for parser
	parserProgressBar := widget.NewProgressBar()
	parserProgressBar.Hide()
	parserStatusLabel := widget.NewLabel("")
	parserStatusLabel.Hide()
	parserStatusLabel.Wrapping = fyne.TextWrapWord

	// Store references in controller
	ac.ParserProgressBar = parserProgressBar
	ac.ParserStatusLabel = parserStatusLabel

	// Create update button with progress display
	updateButton := widget.NewButton("Update Config", func() {
		// Show progress bar and status immediately
		parserProgressBar.SetValue(0)
		parserProgressBar.Show()
		parserStatusLabel.SetText("Извлечение конфигурации...")
		parserStatusLabel.Show()
		
		// Update progress to 0% immediately
		if ac.UpdateParserProgressFunc != nil {
			ac.UpdateParserProgressFunc(0, "Извлечение конфигурации...")
		}
		
		// Run parser in goroutine to avoid blocking UI
		go func() {
			// Wait 0.1 sec to show 0%
			time.Sleep(100 * time.Millisecond)
			
			// Run parser (it will handle its own progress updates)
			ac.RunParser()
		}()
	})

	// Set up callback for progress updates
	ac.UpdateParserProgressFunc = func(progress float64, status string) {
		fyne.Do(func() {
			if progress < 0 {
				// Error state - show error but keep progress bar visible
				parserProgressBar.SetValue(0)
				parserStatusLabel.SetText(status)
			} else if progress >= 100 {
				// Success - show completion
				parserProgressBar.SetValue(1.0)
				parserStatusLabel.SetText(status)
				// Hide after completion
				go func() {
					time.Sleep(3 * time.Second)
					fyne.Do(func() {
						parserProgressBar.Hide()
						parserStatusLabel.Hide()
					})
				}()
			} else {
				// Normal progress
				parserProgressBar.SetValue(progress / 100.0)
				parserStatusLabel.SetText(status)
			}
		})
	}

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

	checkUpdatesButton := widget.NewButton("Check for Updates", func() {
		ac.CheckForUpdates()
	})

	return container.NewVBox(
		logsButton,
		updateButton,
		parserProgressBar,
		parserStatusLabel,
		configButton,
		killButton,
		widget.NewSeparator(),
		checkUpdatesButton,
	)
}

