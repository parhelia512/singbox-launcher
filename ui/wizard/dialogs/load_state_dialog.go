// Package dialogs содержит диалоговые окна визарда конфигурации.
//
// Файл load_state_dialog.go содержит функцию ShowLoadStateDialog, которая создает диалоговое окно
// для выбора сохранённого состояния визарда:
//   - Отображение списка всех сохранённых состояний с метаданными
//   - Выделение state.json как "текущее"
//   - Buttons: "Load", "Configure Anew", "Cancel"
//
// Диалог используется в двух сценариях:
//   - При открытии визарда, если существует state.json или есть другие сохранённые состояния
//   - При нажатии кнопки "Read" для загрузки другого состояния
//
// Используется в:
//   - wizard.go - при открытии визарда для выбора состояния
//   - presentation/presenter_state.go - при нажатии кнопки "Read"
package dialogs

import (
	"fmt"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/internal/debuglog"
	wizardpresentation "singbox-launcher/ui/wizard/presentation"
)

// LoadStateResult представляет результат выбора в диалоге загрузки состояния.
type LoadStateResult struct {
	Action      string // "load", "new", "cancel"
	SelectedID  string // ID выбранного состояния (пусто для state.json)
}

// ShowLoadStateDialog показывает диалог выбора состояния.
// Возвращает результат через callback.
func ShowLoadStateDialog(presenter *wizardpresentation.WizardPresenter, onResult func(LoadStateResult)) {
	guiState := presenter.GUIState()
	if guiState.Window == nil {
		onResult(LoadStateResult{Action: "cancel"})
		return
	}

	// Загружаем список состояний
	stateStore := presenter.GetStateStore()
	states, err := stateStore.ListWizardStates()
	if err != nil {
		debuglog.ErrorLog("ShowLoadStateDialog: failed to list states: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to load states list: %w", err), guiState.Window)
		onResult(LoadStateResult{Action: "cancel"})
		return
	}

	// Если состояний нет, вызываем callback с "new"
	if len(states) == 0 {
		onResult(LoadStateResult{Action: "new"})
		return
	}

	// Сортируем состояния: сначала state.json, затем по дате изменения (новые сверху)
	sort.Slice(states, func(i, j int) bool {
		if states[i].IsCurrent && !states[j].IsCurrent {
			return true
		}
		if !states[i].IsCurrent && states[j].IsCurrent {
			return false
		}
		return states[i].UpdatedAt.After(states[j].UpdatedAt)
	})

	// Создаём список состояний
	var selectedIndex widget.ListItemID = -1
	if len(states) > 0 && states[0].IsCurrent {
		selectedIndex = 0
	}

	list := widget.NewList(
		func() int {
			return len(states)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel("ID"),
				widget.NewLabel("Comment"),
				widget.NewLabel("Dates"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			state := states[id]
			cont := obj.(*fyne.Container)
			labels := cont.Objects

			// Format ID with "Current" label
			idText := state.ID
			if state.ID == "" {
				idText = "state.json"
			}
			if state.IsCurrent {
				idText += " (Current)"
			}

			// Comment
			commentText := state.Comment
			if commentText == "" {
				commentText = "(no comment)"
			}

			// Dates
			datesText := fmt.Sprintf("Created: %s\nUpdated: %s",
				formatDate(state.CreatedAt),
				formatDate(state.UpdatedAt))

			labels[0].(*widget.Label).SetText(idText)
			labels[1].(*widget.Label).SetText(commentText)
			labels[2].(*widget.Label).SetText(datesText)
		},
	)

	// Отслеживаем выбранный элемент
	list.OnSelected = func(id widget.ListItemID) {
		selectedIndex = id
	}

	// Выделяем state.json по умолчанию
	if selectedIndex >= 0 && selectedIndex < widget.ListItemID(len(states)) {
		list.Select(selectedIndex)
	}

	// Declare dialog variable first so it can be used in button callbacks
	var dialogWindow dialog.Dialog

	// Buttons
	loadButton := widget.NewButton("Load", func() {
		selectedID := ""
		if selectedIndex >= 0 && selectedIndex < widget.ListItemID(len(states)) {
			selectedID = states[selectedIndex].ID
		}
		if dialogWindow != nil {
			dialogWindow.Hide()
		}
		onResult(LoadStateResult{
			Action:     "load",
			SelectedID: selectedID,
		})
	})
	loadButton.Importance = widget.HighImportance

	newButton := widget.NewButton("Configure Anew", func() {
		if dialogWindow != nil {
			dialogWindow.Hide()
		}
		onResult(LoadStateResult{Action: "new"})
	})

	cancelButton := widget.NewButton("Cancel", func() {
		if dialogWindow != nil {
			dialogWindow.Hide()
		}
		onResult(LoadStateResult{Action: "cancel"})
	})

	// Контейнер с кнопками
	buttonsContainer := container.NewHBox(
		layout.NewSpacer(),
		cancelButton,
		newButton,
		loadButton,
	)

	// Main content
	content := container.NewBorder(
		widget.NewLabel("Select state to load:"),
		buttonsContainer,
		nil,
		nil,
		container.NewScroll(list),
	)

	// Create dialog without close button (empty dismiss text)
	dialogWindow = dialog.NewCustom("Load State", "", content, guiState.Window)
	dialogWindow.Resize(fyne.NewSize(500, 400))
	dialogWindow.Show()
}

// formatDate formats date for display.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format("02.01.2006 15:04")
}

