package ui

import (
	"fmt"
	"log"
	"net"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/pion/stun"

	"singbox-launcher/core"
	"singbox-launcher/internal/platform"
)

// checkSTUN performs a STUN request to determine the external IP address.
func checkSTUN(serverAddr string) (string, error) {
	// Создаем UDP соединение
	conn, err := net.Dial("udp", serverAddr)
	if err != nil {
		return "", fmt.Errorf("failed to dial STUN server: %w", err)
	}
	defer conn.Close()

	// Создаем STUN клиент
	c, err := stun.NewClient(conn)
	if err != nil {
		return "", fmt.Errorf("failed to create STUN client: %w", err)
	}
	// Гарантируем корректное освобождение внутренних горутин и ресурсов клиента
	defer c.Close()

	// Создаем сообщение для запроса
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	var xorAddr stun.XORMappedAddress
	var errResult error

	// Канал для получения результата из горутины
	done := make(chan bool)

	// Выполняем запрос в горутине
	go func() {
		err = c.Do(message, func(res stun.Event) {
			if res.Error != nil {
				errResult = res.Error
				return
			}
			// Ищем XORMappedAddress в ответе
			if err := xorAddr.GetFrom(res.Message); err != nil {
				errResult = err
				return
			}
		})
		if err != nil {
			errResult = err
		}
		close(done)
	}()

	// Ждем результата или таймаута
	select {
	case <-done:
		if errResult != nil {
			return "", fmt.Errorf("STUN request failed: %w", errResult)
		}
		return xorAddr.IP.String(), nil
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("STUN request timed out")
	}
}

// CreateDiagnosticsTab creates and returns the content for the "Diagnostics" tab.
func CreateDiagnosticsTab(ac *core.AppController) fyne.CanvasObject {
	checkFilesButton := widget.NewButton("Check Files", ac.CheckFiles)

	// Кнопка для проверки STUN
	stunButton := widget.NewButton("Check STUN", func() {
		// Показываем диалог ожидания
		waitDialog := dialog.NewCustomWithoutButtons("STUN Check", widget.NewLabel("Checking, please wait..."), ac.MainWindow)
		waitDialog.Show()

		go func() {
			stunServer := "stun.l.google.com:19302"
			ip, err := checkSTUN(stunServer)

			// Закрываем диалог ожидания и показываем результат
			fyne.Do(func() {
				waitDialog.Hide()
				if err != nil {
					log.Printf("diagnosticsTab: STUN check failed: %v", err)
					ShowError(ac.MainWindow, err)
				} else {
					log.Printf("diagnosticsTab: STUN check successful, IP: %s", ip)
					// Создаем кастомный диалог с кнопкой "Copy"
					resultLabel := widget.NewLabel(fmt.Sprintf("Your External IP: %s\n(determined via [UDP]%s)", ip, stunServer))
					copyButton := widget.NewButton("Copy IP", func() {
						ac.MainWindow.Clipboard().SetContent(ip)
						ac.ShowAutoHideInfo("Copied", "IP address copied to clipboard.")
					})

					ShowCustom(ac.MainWindow, "STUN Check Result", "Close", container.NewVBox(resultLabel, copyButton))
				}
			})
		}()
	})

	// Helper function to create "Open in Browser" buttons
	openBrowserButton := func(label, url string) fyne.CanvasObject {
		return widget.NewButton(label, func() {
			if err := platform.OpenURL(url); err != nil {
				log.Printf("diagnosticsTab: Failed to open URL %s: %v", url, err)
				ShowError(ac.MainWindow, err)
			}
		})
	}

	return container.NewVBox(
		widget.NewLabel("Diagnostics"),
		checkFilesButton,
		stunButton,
		widget.NewSeparator(),
		widget.NewLabel("IP Check Services:"),
		openBrowserButton("2ip.ru", "https://2ip.ru"),
		openBrowserButton("2ip.io", "https://2ip.io"),
		openBrowserButton("2ip.me", "https://2ip.me"),
		openBrowserButton("Yandex Internet", "https://yandex.ru/internet/"),
		openBrowserButton("SpeedTest", "https://www.speedtest.net/"),
		openBrowserButton("WhatIsMyIPAddress", "https://whatismyipaddress.com"),
	)
}

