package dialogs

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShowError shows an error dialog to the user
func ShowError(window fyne.Window, err error) {
	fyne.Do(func() {
		dialog.ShowError(err, window)
	})
}

// ShowErrorText shows an error dialog with a text message
func ShowErrorText(window fyne.Window, title, message string) {
	fyne.Do(func() {
		dialog.ShowError(fmt.Errorf("%s: %s", title, message), window)
	})
}

// ShowInfo shows an information dialog to the user
func ShowInfo(window fyne.Window, title, message string) {
	fyne.Do(func() {
		dialog.ShowInformation(title, message, window)
	})
}

// ShowCustom shows a custom dialog with custom content
func ShowCustom(window fyne.Window, title, dismiss string, content fyne.CanvasObject) {
	fyne.Do(func() {
		dialog.ShowCustom(title, dismiss, content, window)
	})
}

// ShowConfirm shows a confirmation dialog
func ShowConfirm(window fyne.Window, title, message string, onConfirm func(bool)) {
	fyne.Do(func() {
		dialog.ShowConfirm(title, message, onConfirm, window)
	})
}

// ShowProcessKillConfirmation shows a dialog asking user if they want to kill a running process.
// onKill is called in a goroutine when user clicks "Kill Process".
func ShowProcessKillConfirmation(window fyne.Window, onKill func()) {
	fyne.Do(func() {
		var d dialog.Dialog
		killButton := widget.NewButton("Kill Process", nil)
		closeButton := widget.NewButton("Close This Warning", nil)
		content := container.NewVBox(
			widget.NewLabel("Sing-Box appears to be already running.\nWould you like to kill the existing process?"),
			killButton,
			closeButton,
		)
		d = dialog.NewCustomWithoutButtons("Warning", content, window)
		killButton.OnTapped = func() {
			go onKill()
			d.Hide()
		}
		closeButton.OnTapped = func() { d.Hide() }
		d.Show()
	})
}

// ShowAutoHideInfo shows a temporary notification and dialog that auto-hides after 2 seconds
func ShowAutoHideInfo(app fyne.App, window fyne.Window, title, message string) {
	app.SendNotification(&fyne.Notification{Title: title, Content: message})
	fyne.Do(func() {
		d := dialog.NewCustomWithoutButtons(title, widget.NewLabel(message), window)
		d.Show()
		go func() {
			<-time.After(2 * time.Second)
			fyne.Do(func() { d.Hide() })
		}()
	})
}
