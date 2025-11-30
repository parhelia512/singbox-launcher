package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/core"
)

// CreateControlTab creates and returns the content for the "Control" tab.
func CreateControlTab(ac *core.AppController) fyne.CanvasObject {
	ac.StatusLabel = widget.NewLabelWithData(ac.StatusText)

	ac.StartButton = widget.NewButton("Start VPN (Sing-Box)", func() {
		ac.StartSingBox()
	})
	ac.StopButton = widget.NewButton("Stop VPN (Sing-Box)", func() {
		ac.StopSingBox()
	})
	exitButton := widget.NewButton("Exit", ac.GracefulExit)

	return container.NewVBox(
		widget.NewLabel("Main Control"),
		ac.StatusLabel,
		ac.StartButton,
		ac.StopButton,
		exitButton,
	)
}

