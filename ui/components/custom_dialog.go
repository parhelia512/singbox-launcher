// Package components содержит переиспользуемые UI компоненты.
//
// Файл custom_dialog.go содержит функцию NewCustom, которая создает диалог с упрощенным API.
// Функция принимает mainContent (центр) и buttons (низ), сама собирает Border.
// Если dismissText не пустой, создается кнопка закрытия и добавляется в buttons контейнер слева,
// и ESC работает аналогично нажатию на эту кнопку.
package components

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// NewCustom создает диалог с упрощенным API.
// Принимает mainContent (центр) и buttons (низ), сама собирает Border.
// dismissText - текст кнопки закрытия (пустая строка = без кнопки закрытия).
// Если dismissText не пустой, создается кнопка и добавляется в buttons контейнер слева,
// и ESC работает аналогично нажатию на эту кнопку.
func NewCustom(title string, mainContent fyne.CanvasObject, buttons fyne.CanvasObject, dismissText string, parent fyne.Window) dialog.Dialog {
	var d dialog.Dialog

	// Если buttons пусто, создаем пустой контейнер
	if buttons == nil {
		buttons = container.NewHBox()
	}

	// Если dismissText не пустой, создаем кнопку закрытия и размещаем её слева, buttons справа
	if dismissText != "" {
		closeButton := widget.NewButton(dismissText, func() {
			if d != nil {
				d.Hide()
			}
		})
		// Используем Border для размещения: closeButton слева, buttons справа
		buttons = container.NewBorder(nil, nil, closeButton, buttons, nil)
	}

	// Собираем Border: top=nil, bottom=buttons (с кнопкой dismissText слева, если указан), left=nil, right=nil, center=mainContent
	content := container.NewBorder(
		nil,         // top
		buttons,     // bottom (кнопка с dismissText слева, если указан)
		nil,         // left
		nil,         // right
		mainContent, // center
	)

	d = dialog.NewCustomWithoutButtons(title, content, parent)

	// Если dismissText не пустой, добавляем обработку ESC
	if dismissText != "" {
		originalOnTypedKey := parent.Canvas().OnTypedKey()
		parent.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
			if key.Name == fyne.KeyEscape && d != nil {
				d.Hide()
				// Восстанавливаем оригинальный обработчик
				if originalOnTypedKey != nil {
					parent.Canvas().SetOnTypedKey(originalOnTypedKey)
				} else {
					parent.Canvas().SetOnTypedKey(nil)
				}
				return
			}
			// Пробрасываем другие клавиши оригинальному обработчику
			if originalOnTypedKey != nil {
				originalOnTypedKey(key)
			}
		})

		// Восстанавливаем обработчик при закрытии диалога
		d.SetOnClosed(func() {
			if originalOnTypedKey != nil {
				parent.Canvas().SetOnTypedKey(originalOnTypedKey)
			} else {
				parent.Canvas().SetOnTypedKey(nil)
			}
		})
	}

	return d
}
