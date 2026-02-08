// Package dialogs содержит диалоговые окна визарда конфигурации.
//
// Файл save_state_dialog.go содержит функцию ShowSaveStateDialog, которая создает диалоговое окно
// для сохранения состояния визарда под новым ID:
//   - Поле ввода ID (обязательное) с валидацией
//   - Поле ввода комментария (необязательное)
//   - Предупреждение, если ID уже существует
//   - Buttons: "Save", "Cancel"
//
// Диалог используется в двух сценариях:
//   - При нажатии кнопки "Save As"
//   - При нажатии кнопки "Save", если state.json уже существует (для сохранения предыдущего состояния)
//
// Используется в:
//   - presentation/presenter_state.go - при сохранении состояния
package dialogs

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	wizardbusiness "singbox-launcher/ui/wizard/business"
	wizardmodels "singbox-launcher/ui/wizard/models"
	wizardpresentation "singbox-launcher/ui/wizard/presentation"
)

// SaveStateResult представляет результат диалога сохранения состояния.
type SaveStateResult struct {
	Action  string // "save", "cancel"
	ID      string
	Comment string
}

// ShowSaveStateDialog показывает диалог сохранения состояния.
// Возвращает результат через callback.
func ShowSaveStateDialog(presenter *wizardpresentation.WizardPresenter, onResult func(SaveStateResult)) {
	guiState := presenter.GUIState()
	if guiState.Window == nil {
		onResult(SaveStateResult{Action: "cancel"})
		return
	}

	// Input fields
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("Enter state ID (a-z, A-Z, 0-9, -, _)")

	commentEntry := widget.NewMultiLineEntry()
	commentEntry.SetPlaceHolder("Comment (optional)")
	commentEntry.Wrapping = fyne.TextWrapWord

	// Предупреждение о существующем ID
	warningLabel := widget.NewLabel("")
	warningLabel.Hide()

	// ID validation function
	validateID := func() (string, error) {
		id := idEntry.Text
		if id == "" {
			return "", fmt.Errorf("ID cannot be empty")
		}
		if err := wizardmodels.ValidateStateID(id); err != nil {
			return "", err
		}
		return id, nil
	}

	// Функция проверки существования ID
	checkIDExists := func(id string) bool {
		fileServiceAdapter := &wizardbusiness.FileServiceAdapter{FileService: presenter.Controller().FileService}
		stateStore := wizardbusiness.NewStateStore(fileServiceAdapter)
		return stateStore.StateExists(id)
	}

	// Обновление предупреждения при изменении ID
	idEntry.OnChanged = func(text string) {
		id, err := validateID()
		if err != nil {
			warningLabel.Hide()
			return
		}
		if checkIDExists(id) {
			warningLabel.SetText("State with this ID already exists. It will be overwritten.")
			warningLabel.Show()
		} else {
			warningLabel.Hide()
		}
	}

	// Buttons
	var dialogWindow dialog.Dialog
	saveButton := widget.NewButton("Save", func() {
		id, err := validateID()
		if err != nil {
			dialog.ShowError(err, guiState.Window)
			return
		}

		comment := commentEntry.Text
		if dialogWindow != nil {
			dialogWindow.Hide()
		}
		onResult(SaveStateResult{
			Action:  "save",
			ID:      id,
			Comment: comment,
		})
	})
	saveButton.Importance = widget.HighImportance

	cancelButton := widget.NewButton("Cancel", func() {
		if dialogWindow != nil {
			dialogWindow.Hide()
		}
		onResult(SaveStateResult{Action: "cancel"})
	})

	// Fields container
	fieldsContainer := container.NewVBox(
		widget.NewLabel("State ID:"),
		idEntry,
		widget.NewLabel("Comment:"),
		container.NewScroll(commentEntry),
		warningLabel,
	)

	// Buttons container
	buttonsContainer := container.NewHBox(
		layout.NewSpacer(),
		cancelButton,
		saveButton,
	)

	// Main content
	content := container.NewBorder(
		nil,
		buttonsContainer,
		nil,
		nil,
		fieldsContainer,
	)

	// Create dialog without close button
	dialogWindow = dialog.NewCustomWithoutButtons("Save State", content, guiState.Window)
	dialogWindow.Resize(fyne.NewSize(400, 300))
	dialogWindow.Show()

	// Focus on ID field
	idEntry.FocusGained()
}
