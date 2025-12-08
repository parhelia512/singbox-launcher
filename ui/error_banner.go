package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ErrorBanner displays a red error banner in the UI
type ErrorBanner struct {
	container *fyne.Container
	text      *widget.Label
	rect      *canvas.Rectangle
}

// NewErrorBanner creates a new error banner widget
func NewErrorBanner(message string) *ErrorBanner {
	text := widget.NewLabel(message)
	text.Wrapping = fyne.TextWrapWord
	text.Alignment = fyne.TextAlignCenter

	rect := canvas.NewRectangle(color.NRGBA{R: 255, G: 200, B: 200, A: 255})
	rect.SetMinSize(fyne.NewSize(0, 40))

	content := container.NewStack(
		rect,
		container.NewPadded(text),
	)

	return &ErrorBanner{
		container: content,
		text:      text,
		rect:      rect,
	}
}

// GetContainer returns the container for embedding in UI
func (eb *ErrorBanner) GetContainer() *fyne.Container {
	return eb.container
}

// SetMessage updates the error message
func (eb *ErrorBanner) SetMessage(message string) {
	eb.text.SetText("❌ " + message)
	eb.container.Refresh()
}

// Hide hides the error banner
func (eb *ErrorBanner) Hide() {
	eb.container.Hide()
}

// Show shows the error banner
func (eb *ErrorBanner) Show() {
	eb.container.Show()
}

// IsVisible returns whether the banner is visible
func (eb *ErrorBanner) IsVisible() bool {
	return eb.container.Visible()
}
