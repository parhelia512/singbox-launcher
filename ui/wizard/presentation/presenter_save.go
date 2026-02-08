// Package presentation содержит слой представления визарда конфигурации.
//
// Файл presenter_save.go содержит методы для сохранения конфигурации:
//   - SaveConfig - асинхронное сохранение конфигурации с прогресс-баром и проверками
//
// SaveConfig выполняет следующие шаги:
//  1. Проверяет, что ParserConfig и SourceURLs заполнены
//  2. При необходимости запускает парсинг конфигурации (если PreviewNeedsParse)
//  3. Генерирует финальную конфигурацию из шаблона и модели (BuildTemplateConfig)
//  4. Сохраняет конфигурацию в файл с созданием бэкапа (SaveConfigWithBackup)
//  5. Показывает диалог успешного сохранения и закрывает визард
//
// Все операции выполняются асинхронно в отдельной горутине с обновлением прогресс-бара.
//
// Сохранение конфигурации - это отдельная ответственность с сложной логикой.
// Содержит координацию нескольких бизнес-операций (парсинг, генерация, сохранение).
// Управляет прогресс-баром и диалогами на разных этапах сохранения.
//
// Используется в:
//   - wizard.go - SaveConfig вызывается при нажатии кнопки "Save" в визарде
//
// Использует:
//   - business/generator.go - BuildTemplateConfig для генерации конфигурации
//   - business/saver.go - SaveConfigWithBackup для сохранения файла
//   - presenter_async.go - TriggerParseForPreview для парсинга при необходимости
package presentation

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/internal/debuglog"
	wizardbusiness "singbox-launcher/ui/wizard/business"
	wizardmodels "singbox-launcher/ui/wizard/models"
)

// SaveConfig сохраняет конфигурацию асинхронно с прогресс-баром.
func (p *WizardPresenter) SaveConfig() {
	p.SyncGUIToModel()

	if strings.TrimSpace(p.model.ParserConfigJSON) == "" {
		debuglog.WarnLog("SaveConfig: ParserConfig is empty")
		dialog.ShowError(fmt.Errorf("ParserConfig is empty"), p.guiState.Window)
		return
	}
	if strings.TrimSpace(p.model.SourceURLs) == "" {
		debuglog.WarnLog("SaveConfig: SourceURLs is empty")
		dialog.ShowError(fmt.Errorf("VLESS URL is empty"), p.guiState.Window)
		return
	}
	if p.guiState.SaveInProgress {
		debuglog.WarnLog("SaveConfig: Save operation already in progress")
		dialog.ShowInformation("Saving", "Save operation already in progress... Please wait.", p.guiState.Window)
		return
	}
	if p.model.AutoParseInProgress {
		debuglog.WarnLog("SaveConfig: Parsing in progress")
		dialog.ShowInformation("Parsing", "Parsing in progress... Please wait.", p.guiState.Window)
		return
	}

	debuglog.InfoLog("SaveConfig: starting save operation")

	// Устанавливаем флаг синхронно ДО запуска горутины, чтобы избежать race condition
	p.guiState.SaveInProgress = true
	p.SetSaveState("", 0.0)
	go func() {
		defer func() {
			debuglog.InfoLog("SaveConfig: save operation completed (or failed)")
			// Всегда восстанавливаем кнопку Save, даже при ошибке
			p.SetSaveState("Save", -1)
			// Сбрасываем флаг парсинга на случай, если он завис
			if p.model.AutoParseInProgress {
				p.model.AutoParseInProgress = false
			}
		}()

		// Step 0: Check and wait for parsing if needed (0.05-0.1)
		if p.model.PreviewNeedsParse || p.model.AutoParseInProgress {
			p.UpdateSaveProgress(0.05)

			if !p.model.AutoParseInProgress {
				p.model.AutoParseInProgress = true
				configService := &wizardbusiness.ConfigServiceAdapter{
					CoreConfigService: p.controller.ConfigService,
				}
				go func() {
					if err := wizardbusiness.ParseAndPreview(p.model, p, configService); err != nil {
						debuglog.ErrorLog("presenter_save: ParseAndPreview failed: %v", err)
					}
				}()
			}

			maxWaitTime := 60 * time.Second
			startTime := time.Now()
			iterations := 0
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for p.model.AutoParseInProgress {
				// Check if save operation was cancelled
				if !p.guiState.SaveInProgress {
					debuglog.DebugLog("presenter_save: Save operation cancelled during parsing wait")
					return
				}
				if time.Since(startTime) > maxWaitTime {
					p.UpdateUI(func() {
						dialog.ShowError(fmt.Errorf("Parsing timeout: operation took too long"), p.guiState.Window)
					})
					return
				}
				select {
				case <-ticker.C:
					iterations++
					progressRange := 0.05 // 0.05 to 0.1
					baseProgress := 0.05
					cycleProgress := float64(iterations%40) / 40.0
					currentProgress := baseProgress + cycleProgress*progressRange
					p.UpdateSaveProgress(currentProgress)
				}
			}
			p.UpdateSaveProgress(0.1)
		}

		// Step 1: Build config (0.2-0.4)
		p.UpdateSaveProgress(0.2)
		// Check if save operation was cancelled
		if !p.guiState.SaveInProgress {
			debuglog.DebugLog("presenter_save: Save operation cancelled before building config")
			return
		}
		debuglog.InfoLog("SaveConfig: building template config")
		text, err := wizardbusiness.BuildTemplateConfig(p.model, false)
		if err != nil {
			debuglog.ErrorLog("SaveConfig: BuildTemplateConfig failed: %v", err)
			p.UpdateUI(func() {
				dialog.ShowError(err, p.guiState.Window)
			})
			return
		}
		debuglog.InfoLog("SaveConfig: template config built successfully, length: %d", len(text))
		p.UpdateSaveProgress(0.4)

		// Step 2: Save file (0.4-0.5)
		// Check if save operation was cancelled
		if !p.guiState.SaveInProgress {
			debuglog.DebugLog("presenter_save: Save operation cancelled before saving file")
			return
		}
		fileService := &wizardbusiness.FileServiceAdapter{
			FileService: p.controller.FileService,
		}
		debuglog.InfoLog("SaveConfig: saving config file")
		path, err := wizardbusiness.SaveConfigWithBackup(fileService, text)
		if err != nil {
			debuglog.ErrorLog("SaveConfig: SaveConfigWithBackup failed: %v", err)
			p.UpdateUI(func() {
				dialog.ShowError(err, p.guiState.Window)
			})
			return
		}
		debuglog.InfoLog("SaveConfig: config saved to %s", path)
		p.UpdateSaveProgress(0.5)

		// Step 3: Validate config with sing-box (0.5-0.6)
		// Check if save operation was cancelled
		if !p.guiState.SaveInProgress {
			debuglog.DebugLog("presenter_save: Save operation cancelled before validation")
			return
		}
		singBoxPath := ""
		if p.controller.FileService != nil {
			singBoxPath = p.controller.FileService.SingboxPath
		}

		validationErr := wizardbusiness.ValidateConfigWithSingBox(path, singBoxPath)
		p.UpdateSaveProgress(0.6)

		// Step 4: Save state.json (0.6-0.9)
		// Check if save operation was cancelled
		if !p.guiState.SaveInProgress {
			debuglog.DebugLog("presenter_save: Save operation cancelled before saving state")
			return
		}
		p.UpdateSaveProgress(0.7)

		// Получаем путь к state.json для логирования
		statesDir := filepath.Join(p.controller.FileService.ExecDir, "bin", wizardbusiness.WizardStatesDir)
		statePath := filepath.Join(statesDir, wizardmodels.StateFileName)

		p.UpdateUI(func() {
			// Update config status in Core Dashboard
			if p.controller.UIService != nil && p.controller.UIService.UpdateConfigStatusFunc != nil {
				p.controller.UIService.UpdateConfigStatusFunc()
			}

			// Сохраняем текущее состояние в state.json после успешного сохранения конфигурации
			// Сохранение происходит всегда, независимо от hasChanges
			debuglog.InfoLog("SaveConfig: saving state.json to %s", statePath)
			if err := p.SaveCurrentState(); err != nil {
				debuglog.WarnLog("presenter_save: failed to save state after config save: %v", err)
			} else {
				debuglog.InfoLog("SaveConfig: state.json saved successfully to %s", statePath)
			}

			// Логируем итоговую информацию о сохранении
			debuglog.InfoLog("SaveConfig: completed - config.json=%s, state.json=%s", path, statePath)

			// Show result with validation status and close wizard
			message := fmt.Sprintf("Config written to %s", path)
			if validationErr != nil {
				message += fmt.Sprintf("\n\n⚠️ Validation warning:\n%v\n\nPlease check the config manually.", validationErr)
			} else {
				message += "\n\n✅ Validation: Passed"
			}

			// Show dialog with OK button that closes both dialog and wizard
			title := "Config Saved"
			if validationErr != nil {
				title = "Config Saved (with warnings)"
			}

			var d dialog.Dialog
			okButton := widget.NewButton("OK", func() {
				// Close dialog first
				if d != nil {
					d.Hide()
				}
				// Close wizard window only (not the main application)
				if p.guiState.Window != nil {
					p.guiState.Window.Close()
				}
			})
			okButton.Importance = widget.HighImportance

			content := container.NewVBox(
				widget.NewLabel(message),
				container.NewHBox(
					layout.NewSpacer(),
					okButton,
				),
			)

			d = dialog.NewCustomWithoutButtons(title, content, p.guiState.Window)
			d.Show()
		})
		p.UpdateSaveProgress(0.9)

		// Step 5: Completion (0.9-1.0)
		<-time.After(100 * time.Millisecond)
		p.UpdateSaveProgress(1.0)
		<-time.After(200 * time.Millisecond)
	}()
}
