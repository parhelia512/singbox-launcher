package ui

import (
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/core"
)

// CoreDashboardTab управляет вкладкой Core Dashboard
type CoreDashboardTab struct {
	controller *core.AppController

	// UI элементы
	statusEmojiLabel        *widget.Label // Эмодзи статуса (отдельно для выравнивания)
	statusLabel             *widget.Label // Текст статуса (без эмодзи)
	versionLabel            *widget.Label
	versionButton           *widget.Button // Кликабельная версия (только Windows)
	downloadButton          *widget.Button
	downloadProgress        *widget.ProgressBar // Прогресс-бар для скачивания
	downloadContainer       fyne.CanvasObject   // Контейнер для кнопки/прогресс-бара
	errorLabel              *widget.Label       // Для отображения ошибок под версией
	startButton             *widget.Button      // Кнопка Start
	stopButton              *widget.Button      // Кнопка Stop
	wintunStatusLabel       *widget.Label       // Статус wintun.dll
	wintunDownloadButton    *widget.Button      // Кнопка скачивания wintun.dll
	wintunDownloadProgress  *widget.ProgressBar // Прогресс-бар для скачивания wintun.dll
	wintunDownloadContainer fyne.CanvasObject   // Контейнер для кнопки/прогресс-бара wintun

	// Данные
	stopAutoUpdate           chan bool
	fullBinaryPath           string
	lastUpdateSuccess        bool // Отслеживаем успех последнего обновления версии
	downloadInProgress       bool // Флаг процесса скачивания sing-box
	wintunDownloadInProgress bool // Флаг процесса скачивания wintun.dll
}

// CreateCoreDashboardTab создает и возвращает вкладку Core Dashboard
func CreateCoreDashboardTab(ac *core.AppController) fyne.CanvasObject {
	tab := &CoreDashboardTab{
		controller:     ac,
		stopAutoUpdate: make(chan bool),
	}

	// Блок статуса с кнопками в одну строку
	statusRow := tab.createStatusRow()

	// Блок версии и пути
	versionBlock := tab.createVersionBlock()

	// Блок wintun.dll (только для Windows)
	var wintunBlock fyne.CanvasObject
	if runtime.GOOS == "windows" {
		wintunBlock = tab.createWintunBlock()
	}

	// Основной контейнер - все элементы в VBox, кнопка Exit в конце
	contentItems := []fyne.CanvasObject{
		statusRow,
		widget.NewSeparator(),
		versionBlock,
	}
	if runtime.GOOS == "windows" && wintunBlock != nil {
		contentItems = append(contentItems, wintunBlock) // Убрали separator перед wintunBlock
	}

	// Горизонтальная линия и кнопка Exit в конце списка
	contentItems = append(contentItems, widget.NewSeparator())
	exitButton := widget.NewButton("Exit", ac.GracefulExit)
	contentItems = append(contentItems, exitButton)

	content := container.NewVBox(contentItems...)

	// Регистрируем callback для обновления статуса при изменении RunningState
	tab.controller.UpdateCoreStatusFunc = func() {
		fyne.Do(func() {
			tab.updateRunningStatus()
		})
	}

	// Первоначальное обновление
	tab.updateBinaryStatus() // Проверяет наличие бинарника и вызывает updateRunningStatus
	tab.updateVersionInfo()
	if runtime.GOOS == "windows" {
		tab.updateWintunStatus() // Проверяет наличие wintun.dll
	}

	// Запускаем автообновление версии
	tab.startAutoUpdate()

	return content
}

// createStatusRow создает строку со статусом и кнопками
func (tab *CoreDashboardTab) createStatusRow() fyne.CanvasObject {
	statusTitle := widget.NewLabel("Core Status")
	statusTitle.Importance = widget.MediumImportance
	statusTitle.Alignment = fyne.TextAlignLeading // Выравнивание текста

	// Отдельный label для эмодзи (для правильного выравнивания)
	tab.statusEmojiLabel = widget.NewLabel("")
	tab.statusEmojiLabel.Wrapping = fyne.TextWrapOff
	tab.statusEmojiLabel.Alignment = fyne.TextAlignLeading

	tab.statusLabel = widget.NewLabel("Checking...")
	tab.statusLabel.Wrapping = fyne.TextWrapOff       // Отключаем перенос текста
	tab.statusLabel.Alignment = fyne.TextAlignLeading // Выравнивание текста
	tab.statusLabel.Importance = widget.MediumImportance

	startButton := widget.NewButton("Start", func() {
		tab.controller.StartSingBox()
		// Статус обновится автоматически через UpdateCoreStatusFunc
	})

	stopButton := widget.NewButton("Stop", func() {
		tab.controller.StopSingBox()
		// Статус обновится автоматически через UpdateCoreStatusFunc
	})

	// Сохраняем ссылки на кнопки для обновления блокировок
	tab.startButton = startButton
	tab.stopButton = stopButton

	// Статус в одну строку: иконка (без отступов), заголовок, текст статуса
	statusContainer := container.NewHBox(
		tab.statusEmojiLabel, // Иконка перед "Core Status" без отступов
		statusTitle,          // "Core Status"
		tab.statusLabel,      // Текст статуса
	)

	// Кнопки на новой строке по центру
	buttonsContainer := container.NewCenter(
		container.NewHBox(startButton, stopButton),
	)

	// Возвращаем контейнер со статусом и кнопками, с пустыми строками до и после кнопок
	return container.NewVBox(
		statusContainer,
		widget.NewLabel(""), // Пустая строка перед кнопками
		buttonsContainer,
		widget.NewLabel(""), // Пустая строка после кнопок
	)
}

// createVersionBlock создает блок с версией
func (tab *CoreDashboardTab) createVersionBlock() fyne.CanvasObject {
	versionTitle := widget.NewLabel("Sing-box Ver.")
	versionTitle.Importance = widget.MediumImportance

	tab.versionLabel = widget.NewLabel("Loading version...")
	tab.versionLabel.Wrapping = fyne.TextWrapOff

	// На Windows делаем версию кликабельной для открытия проводника
	if runtime.GOOS == "windows" {
		tab.versionButton = widget.NewButton("Loading version...", func() {
			tab.openBinaryInExplorer()
		})
		tab.versionButton.Importance = widget.LowImportance
		tab.versionButton.Hide() // Скрываем по умолчанию, покажем когда будет версия
	}

	// Кнопка Download/Update справа от версии
	tab.downloadButton = widget.NewButton("Download", func() {
		tab.handleDownload()
	})
	tab.downloadButton.Importance = widget.MediumImportance
	tab.downloadButton.Disable() // По умолчанию отключена, пока не проверим наличие бинарника

	// Прогресс-бар для скачивания (скрыт по умолчанию)
	tab.downloadProgress = widget.NewProgressBar()
	tab.downloadProgress.Hide()
	tab.downloadProgress.SetValue(0)

	// Контейнер для кнопки/прогресс-бара - они занимают одно место, переключаются через Show/Hide
	// Используем Stack с Max для прогресс-бара, чтобы он мог расширяться и был достаточно большим
	// Структура такая же, как у wintun
	progressContainer := container.NewMax(tab.downloadProgress)
	tab.downloadContainer = container.NewStack(tab.downloadButton, progressContainer)

	// Объединяем версию и контейнер с кнопкой/прогресс-баром в одну строку
	var versionDisplay fyne.CanvasObject
	if runtime.GOOS == "windows" && tab.versionButton != nil {
		// На Windows используем кликабельную кнопку
		versionDisplay = tab.versionButton
	} else {
		// На других платформах просто label
		versionDisplay = tab.versionLabel
	}

	// Используем HBox как у wintun, чтобы прогресс-бар был такого же размера
	// Версия слева, контейнер с кнопкой/прогресс-баром справа
	// Структура точно такая же, как у wintun
	versionInfoContainer := container.NewHBox(
		versionDisplay,
		tab.downloadContainer,
	)

	// Label для ошибок (скрыт по умолчанию)
	tab.errorLabel = widget.NewLabel("")
	tab.errorLabel.Wrapping = fyne.TextWrapWord
	tab.errorLabel.Importance = widget.DangerImportance
	tab.errorLabel.Hide()

	return container.NewVBox(
		container.NewHBox(versionTitle, versionInfoContainer),
		tab.errorLabel,
	)
}

// updateBinaryStatus проверяет наличие бинарника и обновляет статус
func (tab *CoreDashboardTab) updateBinaryStatus() {
	// Проверяем, существует ли бинарник
	if _, err := tab.controller.GetInstalledCoreVersion(); err != nil {
		tab.statusEmojiLabel.SetText("❌")
		tab.statusLabel.SetText("Error: sing-box not found")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
		// Обновляем иконку трея (красная при ошибке)
		tab.controller.UpdateUI()
		return
	}
	// Если бинарник найден, обновляем статус запуска
	tab.updateRunningStatus()
	// Обновляем иконку трея (может измениться с красной на черную/зеленую)
	tab.controller.UpdateUI()
}

// updateRunningStatus обновляет статус Running/Stopped на основе RunningState
func (tab *CoreDashboardTab) updateRunningStatus() {
	// Проверяем, существует ли бинарник (если нет - показываем ошибку)
	if _, err := tab.controller.GetInstalledCoreVersion(); err != nil {
		tab.statusEmojiLabel.SetText("❌")
		tab.statusLabel.SetText("Error: sing-box not found")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
		// Блокируем кнопки если бинарника нет
		if tab.startButton != nil {
			tab.startButton.Disable()
		}
		if tab.stopButton != nil {
			tab.stopButton.Disable()
		}
		return
	}

	// Обновляем статус на основе RunningState
	if tab.controller.RunningState.IsRunning() {
		tab.statusEmojiLabel.SetText("✅")
		tab.statusLabel.SetText("Running")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
		// Блокируем Start, разблокируем Stop
		if tab.startButton != nil {
			tab.startButton.Disable()
		}
		if tab.stopButton != nil {
			tab.stopButton.Enable()
		}
	} else {
		tab.statusEmojiLabel.SetText("⏸️")
		tab.statusLabel.SetText("Stopped")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
		// Блокируем Stop, разблокируем Start
		if tab.startButton != nil {
			tab.startButton.Enable()
		}
		if tab.stopButton != nil {
			tab.stopButton.Disable()
		}
	}
}

// updateVersionInfo обновляет информацию о версии, возвращает ошибку если есть
func (tab *CoreDashboardTab) updateVersionInfo() error {
	// Получаем полный путь для открытия в проводнике
	tab.fullBinaryPath = tab.controller.SingboxPath

	// Получаем установленную версию
	installedVersion, err := tab.controller.GetInstalledCoreVersion()
	if err != nil {
		// Версия не отображается, если ошибка
		if runtime.GOOS == "windows" && tab.versionButton != nil {
			tab.versionButton.Hide()
		}
		tab.versionLabel.SetText(" — ")
		tab.versionLabel.Importance = widget.MediumImportance
		tab.versionLabel.Show()
		// Показываем ошибку снизу
		tab.errorLabel.SetText(fmt.Sprintf("Error: %s", err.Error()))
		tab.errorLabel.Show()
		// Если бинарника нет - кнопка должна называться "Download"
		// Пытаемся получить последнюю версию для кнопки
		latest, latestErr := tab.controller.GetLatestCoreVersion()
		if latestErr == nil && latest != "" {
			tab.downloadButton.SetText(fmt.Sprintf("Download v%s", latest))
		} else {
			tab.downloadButton.SetText("Download")
		}
		tab.downloadButton.Enable()
		tab.downloadButton.Importance = widget.HighImportance
		tab.downloadButton.Show() // Показываем кнопку Download при ошибке
		return err
	}

	// Скрываем ошибку, если бинарник найден
	tab.errorLabel.Hide()

	// Получаем информацию о версиях
	versionInfo := tab.controller.GetCoreVersionInfo()

	// Обновляем отображение версии
	if runtime.GOOS == "windows" && tab.versionButton != nil {
		// На Windows используем кликабельную кнопку
		tab.versionButton.SetText(installedVersion)
		tab.versionButton.Importance = widget.LowImportance
		tab.versionButton.Show()
		tab.versionLabel.Hide()
	} else {
		// На других платформах просто label
		tab.versionLabel.SetText(installedVersion)
		tab.versionLabel.Importance = widget.SuccessImportance
		tab.versionLabel.Show()
	}

	// Обновляем кнопку - показываем только если есть обновление
	if versionInfo.LatestVersion != "" && versionInfo.UpdateAvailable {
		// Если есть обновление - показываем кнопку "Update"
		tab.downloadButton.SetText(fmt.Sprintf("Update v%s", versionInfo.LatestVersion))
		tab.downloadButton.Enable()
		tab.downloadButton.Importance = widget.HighImportance
		tab.downloadButton.Show()
	} else {
		// Если файл есть, но версия актуальна - скрываем кнопку
		tab.downloadButton.Hide()
	}

	return nil
}

// handleDownload обрабатывает нажатие на кнопку Download
func (tab *CoreDashboardTab) handleDownload() {
	if tab.downloadInProgress {
		return // Уже идет скачивание
	}

	// Получаем информацию о версиях
	versionInfo := tab.controller.GetCoreVersionInfo()

	targetVersion := versionInfo.LatestVersion
	if targetVersion == "" {
		// Пытаемся получить последнюю версию
		latest, err := tab.controller.GetLatestCoreVersion()
		if err != nil {
			ShowError(tab.controller.MainWindow, fmt.Errorf("failed to get latest version: %w", err))
			return
		}
		targetVersion = latest
	}

	// Запускаем скачивание в отдельной горутине
	tab.downloadInProgress = true
	tab.downloadButton.Disable()
	// Скрываем кнопку и показываем прогресс-бар
	tab.downloadButton.Hide()
	tab.downloadProgress.Show()
	tab.downloadProgress.SetValue(0)

	// Создаем канал для прогресса
	progressChan := make(chan core.DownloadProgress, 10)

	// Запускаем скачивание в отдельной горутине
	go func() {
		tab.controller.DownloadCore(targetVersion, progressChan)
	}()

	// Обрабатываем прогресс в отдельной горутине
	go func() {
		for progress := range progressChan {
			fyne.Do(func() {
				// Обновляем только прогресс-бар (кнопка скрыта)
				tab.downloadProgress.SetValue(float64(progress.Progress) / 100.0)

				if progress.Status == "done" {
					tab.downloadInProgress = false
					// Скрываем прогресс-бар и показываем кнопку
					tab.downloadProgress.Hide()
					tab.downloadProgress.SetValue(0)
					tab.downloadButton.Show()
					tab.downloadButton.Enable()
					// Обновляем статусы после успешного скачивания (это уберет ошибки и обновит статус)
					tab.updateVersionInfo()
					tab.updateBinaryStatus() // Это вызовет updateRunningStatus() и обновит статус
					// Обновляем иконку трея (может измениться с красной на черную/зеленую)
					tab.controller.UpdateUI()
					ShowInfo(tab.controller.MainWindow, "Download Complete", progress.Message)
				} else if progress.Status == "error" {
					tab.downloadInProgress = false
					// Скрываем прогресс-бар и показываем кнопку
					tab.downloadProgress.Hide()
					tab.downloadProgress.SetValue(0)
					tab.downloadButton.Show()
					tab.downloadButton.Enable()
					ShowError(tab.controller.MainWindow, progress.Error)
				}
			})
		}
	}()
}

// openBinaryInExplorer открывает проводник и выделяет файл бинарника
func (tab *CoreDashboardTab) openBinaryInExplorer() {
	if tab.fullBinaryPath == "" {
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: explorer /select,"путь\к\файлу"
		cmd = exec.Command("explorer.exe", "/select,", tab.fullBinaryPath)
	} else if runtime.GOOS == "darwin" {
		// macOS: open -R "путь/к/файлу"
		cmd = exec.Command("open", "-R", tab.fullBinaryPath)
	} else {
		// Linux: xdg-open с директорией файла
		dir := filepath.Dir(tab.fullBinaryPath)
		cmd = exec.Command("xdg-open", dir)
	}

	if err := cmd.Run(); err != nil {
		// Логируем ошибку, но не показываем пользователю
		fmt.Printf("Failed to open explorer: %v\n", err)
	}
}

// startAutoUpdate запускает автообновление версии (статус управляется через RunningState)
func (tab *CoreDashboardTab) startAutoUpdate() {
	// Запускаем периодическое обновление с умной логикой
	go func() {
		rand.Seed(time.Now().UnixNano()) // Инициализация генератора случайных чисел

		for {
			select {
			case <-tab.stopAutoUpdate:
				return
			default:
				// Ждем перед следующим обновлением
				var delay time.Duration
				if tab.lastUpdateSuccess {
					// Если последнее обновление было успешным - не повторяем автоматически
					// Ждем очень долго (или можно вообще не повторять)
					delay = 10 * time.Minute
				} else {
					// Если была ошибка - повторяем через случайный интервал 20-35 секунд
					delay = time.Duration(20+rand.Intn(16)) * time.Second // 20-35 секунд
				}

				select {
				case <-time.After(delay):
					// Обновляем только версию (статус управляется через RunningState)
					var success bool
					fyne.Do(func() {
						success = tab.updateVersionInfo() == nil
					})
					tab.lastUpdateSuccess = success
				case <-tab.stopAutoUpdate:
					return
				}
			}
		}
	}()
}

// createWintunBlock создает блок для отображения статуса wintun.dll
func (tab *CoreDashboardTab) createWintunBlock() fyne.CanvasObject {
	wintunTitle := widget.NewLabel("WinTun DLL")
	wintunTitle.Importance = widget.MediumImportance

	tab.wintunStatusLabel = widget.NewLabel("Checking...")
	tab.wintunStatusLabel.Wrapping = fyne.TextWrapOff

	// Кнопка скачивания wintun.dll
	tab.wintunDownloadButton = widget.NewButton("Download", func() {
		tab.handleWintunDownload()
	})
	tab.wintunDownloadButton.Importance = widget.MediumImportance
	tab.wintunDownloadButton.Disable() // По умолчанию отключена

	// Прогресс-бар для скачивания wintun.dll
	tab.wintunDownloadProgress = widget.NewProgressBar()
	tab.wintunDownloadProgress.Hide()
	tab.wintunDownloadProgress.SetValue(0)

	// Контейнер для кнопки/прогресс-бара wintun
	progressContainer := container.NewMax(tab.wintunDownloadProgress)
	tab.wintunDownloadContainer = container.NewStack(tab.wintunDownloadButton, progressContainer)

	// Объединяем статус и кнопку в одну строку
	wintunInfoContainer := container.NewHBox(
		tab.wintunStatusLabel,
		tab.wintunDownloadContainer,
	)

	return container.NewVBox(
		container.NewHBox(wintunTitle, wintunInfoContainer),
	)
}

// updateWintunStatus обновляет статус wintun.dll
func (tab *CoreDashboardTab) updateWintunStatus() {
	if runtime.GOOS != "windows" {
		return // wintun нужен только на Windows
	}

	exists, err := tab.controller.CheckWintunDLL()
	if err != nil {
		tab.wintunStatusLabel.SetText("❌ Error checking wintun.dll")
		tab.wintunStatusLabel.Importance = widget.MediumImportance
		tab.wintunDownloadButton.Disable()
		return
	}

	if exists {
		tab.wintunStatusLabel.SetText("ok")
		tab.wintunStatusLabel.Importance = widget.MediumImportance
		tab.wintunDownloadButton.Hide()
		tab.wintunDownloadProgress.Hide()
	} else {
		tab.wintunStatusLabel.SetText("❌ wintun.dll not found")
		tab.wintunStatusLabel.Importance = widget.MediumImportance
		tab.wintunDownloadButton.Show()
		tab.wintunDownloadButton.Enable()
		tab.wintunDownloadButton.SetText("Download wintun.dll")
		tab.wintunDownloadButton.Importance = widget.HighImportance
	}
}

// handleWintunDownload обрабатывает нажатие на кнопку Download wintun.dll
func (tab *CoreDashboardTab) handleWintunDownload() {
	if tab.wintunDownloadInProgress {
		return // Уже идет скачивание
	}

	tab.wintunDownloadInProgress = true
	tab.wintunDownloadButton.Disable()
	tab.wintunDownloadButton.SetText("Downloading...")
	tab.wintunDownloadProgress.Show()
	tab.wintunDownloadProgress.SetValue(0)

	go func() {
		progressChan := make(chan core.DownloadProgress, 10)

		go func() {
			tab.controller.DownloadWintunDLL(progressChan)
		}()

		for progress := range progressChan {
			fyne.Do(func() {
				tab.wintunDownloadProgress.SetValue(float64(progress.Progress) / 100.0)
				tab.wintunDownloadButton.SetText(fmt.Sprintf("Downloading... %d%%", progress.Progress))

				if progress.Status == "done" {
					tab.wintunDownloadInProgress = false
					tab.updateWintunStatus() // Обновляем статус после скачивания
					tab.wintunDownloadProgress.Hide()
					tab.wintunDownloadProgress.SetValue(0)
					tab.wintunDownloadButton.Enable()
					ShowInfo(tab.controller.MainWindow, "Download Complete", progress.Message)
				} else if progress.Status == "error" {
					tab.wintunDownloadInProgress = false
					tab.wintunDownloadProgress.Hide()
					tab.wintunDownloadProgress.SetValue(0)
					tab.wintunDownloadButton.Show()
					tab.wintunDownloadButton.Enable()
					ShowError(tab.controller.MainWindow, progress.Error)
				}
			})
		}
	}()
}
