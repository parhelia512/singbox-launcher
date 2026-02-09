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
	"singbox-launcher/ui/components"
	wizardbusiness "singbox-launcher/ui/wizard/business"
	wizardmodels "singbox-launcher/ui/wizard/models"
)

// SaveConfig сохраняет конфигурацию асинхронно с прогресс-баром.
func (p *WizardPresenter) SaveConfig() {
	p.SyncGUIToModel()

	// Validate input before starting save operation
	if !p.validateSaveInput() {
		return
	}

	// Check if save operation is already in progress
	if !p.checkSaveOperationState() {
		return
	}

	debuglog.InfoLog("SaveConfig: starting save operation")

	// Устанавливаем флаг синхронно ДО запуска горутины, чтобы избежать race condition
	p.guiState.SaveInProgress = true
	p.SetSaveState("", 0.0)

	go p.executeSaveOperation()
}

// validateSaveInput проверяет входные данные перед сохранением.
func (p *WizardPresenter) validateSaveInput() bool {
	if strings.TrimSpace(p.model.ParserConfigJSON) == "" {
		debuglog.WarnLog("SaveConfig: ParserConfig is empty")
		dialog.ShowError(fmt.Errorf("ParserConfig is empty"), p.guiState.Window)
		return false
	}
	if strings.TrimSpace(p.model.SourceURLs) == "" {
		debuglog.WarnLog("SaveConfig: SourceURLs is empty")
		dialog.ShowError(fmt.Errorf("VLESS URL is empty"), p.guiState.Window)
		return false
	}
	return true
}

// checkSaveOperationState проверяет состояние операции сохранения.
func (p *WizardPresenter) checkSaveOperationState() bool {
	if p.guiState.SaveInProgress {
		debuglog.WarnLog("SaveConfig: Save operation already in progress")
		dialog.ShowInformation("Saving", "Save operation already in progress... Please wait.", p.guiState.Window)
		return false
	}
	if p.model.AutoParseInProgress {
		debuglog.WarnLog("SaveConfig: Parsing in progress")
		dialog.ShowInformation("Parsing", "Parsing in progress... Please wait.", p.guiState.Window)
		return false
	}
	return true
}

// executeSaveOperation выполняет операцию сохранения в отдельной горутине.
func (p *WizardPresenter) executeSaveOperation() {
	defer p.finalizeSaveOperation()

	// Step 0: Wait for parsing if needed (0.05-0.1)
	if !p.waitForParsingIfNeeded() {
		return
	}

	// Step 1: Build config (0.2-0.4)
	configText, err := p.buildConfigForSave()
	if err != nil {
		return
	}

	// Step 2: Save file (0.4-0.5)
	configPath, err := p.saveConfigFile(configText)
	if err != nil {
		return
	}

	// Step 3: Validate config with sing-box (0.5-0.6)
	validationErr := p.validateConfigFile(configPath)

	// Step 4: Save state.json and show success dialog (0.6-0.9)
	p.saveStateAndShowSuccessDialog(configPath, validationErr)

	// Step 5: Completion (0.9-1.0)
	p.completeSaveOperation()
}

// finalizeSaveOperation завершает операцию сохранения и восстанавливает UI.
func (p *WizardPresenter) finalizeSaveOperation() {
	debuglog.InfoLog("SaveConfig: save operation completed (or failed)")
	// Всегда восстанавливаем кнопку Save, даже при ошибке
	p.SetSaveState("Save", -1)
	// Сбрасываем флаг парсинга на случай, если он завис
	if p.model.AutoParseInProgress {
		p.model.AutoParseInProgress = false
	}
}

// waitForParsingIfNeeded ожидает завершения парсинга, если он необходим.
// Возвращает false, если операция была отменена или произошла ошибка.
func (p *WizardPresenter) waitForParsingIfNeeded() bool {
	if !p.model.PreviewNeedsParse && !p.model.AutoParseInProgress {
		return true
	}

	p.UpdateSaveProgress(0.05)

	// Start parsing if not already in progress
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

	// Wait for parsing to complete
	maxWaitTime := 60 * time.Second
	startTime := time.Now()
	iterations := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for p.model.AutoParseInProgress {
		// Check if save operation was cancelled
		if !p.guiState.SaveInProgress {
			debuglog.DebugLog("presenter_save: Save operation cancelled during parsing wait")
			return false
		}
		if time.Since(startTime) > maxWaitTime {
			p.UpdateUI(func() {
				dialog.ShowError(fmt.Errorf("Parsing timeout: operation took too long"), p.guiState.Window)
			})
			return false
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
	return true
}

// buildConfigForSave строит конфигурацию из шаблона и модели.
// Возвращает текст конфигурации или ошибку.
func (p *WizardPresenter) buildConfigForSave() (string, error) {
	p.UpdateSaveProgress(0.2)

	// Check if save operation was cancelled
	if !p.guiState.SaveInProgress {
		debuglog.DebugLog("presenter_save: Save operation cancelled before building config")
		return "", fmt.Errorf("save operation cancelled")
	}

	debuglog.InfoLog("SaveConfig: building template config")
	text, err := wizardbusiness.BuildTemplateConfig(p.model, false)
	if err != nil {
		debuglog.ErrorLog("SaveConfig: BuildTemplateConfig failed: %v", err)
		p.UpdateUI(func() {
			dialog.ShowError(err, p.guiState.Window)
		})
		return "", err
	}

	debuglog.InfoLog("SaveConfig: template config built successfully, length: %d", len(text))
	p.UpdateSaveProgress(0.4)
	return text, nil
}

// saveConfigFile сохраняет конфигурацию в файл с созданием бэкапа.
// Возвращает путь к сохраненному файлу или ошибку.
func (p *WizardPresenter) saveConfigFile(configText string) (string, error) {
	// Check if save operation was cancelled
	if !p.guiState.SaveInProgress {
		debuglog.DebugLog("presenter_save: Save operation cancelled before saving file")
		return "", fmt.Errorf("save operation cancelled")
	}

	fileService := &wizardbusiness.FileServiceAdapter{
		FileService: p.controller.FileService,
	}
	debuglog.InfoLog("SaveConfig: saving config file")
	path, err := wizardbusiness.SaveConfigWithBackup(fileService, configText)
	if err != nil {
		debuglog.ErrorLog("SaveConfig: SaveConfigWithBackup failed: %v", err)
		p.UpdateUI(func() {
			dialog.ShowError(err, p.guiState.Window)
		})
		return "", err
	}

	debuglog.InfoLog("SaveConfig: config saved to %s", path)
	p.UpdateSaveProgress(0.5)
	return path, nil
}

// validateConfigFile валидирует сохраненный конфиг с помощью sing-box.
// Возвращает ошибку валидации, если она есть.
func (p *WizardPresenter) validateConfigFile(configPath string) error {
	// Check if save operation was cancelled
	if !p.guiState.SaveInProgress {
		debuglog.DebugLog("presenter_save: Save operation cancelled before validation")
		return fmt.Errorf("save operation cancelled")
	}

	singBoxPath := ""
	if p.controller.FileService != nil {
		singBoxPath = p.controller.FileService.SingboxPath
	}

	validationErr := wizardbusiness.ValidateConfigWithSingBox(configPath, singBoxPath)
	p.UpdateSaveProgress(0.6)
	return validationErr
}

// saveStateAndShowSuccessDialog сохраняет state.json и показывает диалог успешного сохранения.
func (p *WizardPresenter) saveStateAndShowSuccessDialog(configPath string, validationErr error) {
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
		debuglog.InfoLog("SaveConfig: completed - config.json=%s, state.json=%s", configPath, statePath)

		// Show success dialog
		p.showSaveSuccessDialog(configPath, validationErr)
	})
	p.UpdateSaveProgress(0.9)
}

// showSaveSuccessDialog показывает диалог успешного сохранения.
func (p *WizardPresenter) showSaveSuccessDialog(configPath string, validationErr error) {
	// Build message with validation status
	message := fmt.Sprintf("Config written to %s", configPath)
	if validationErr != nil {
		message += fmt.Sprintf("\n\n⚠️ Validation warning:\n%v\n\nPlease check the config manually.", validationErr)
	} else {
		message += "\n\n✅ Validation: Passed"
	}

	// Determine dialog title
	title := "Config Saved"
	if validationErr != nil {
		title = "Config Saved (with warnings)"
	}

	// Create dialog with OK button that closes both dialog and wizard
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

	buttonsRow := container.NewHBox(
		layout.NewSpacer(),
		okButton,
	)

	messageLabel := widget.NewLabel(message)

	d = components.NewCustom(title, messageLabel, buttonsRow, "", p.guiState.Window)
	d.Show()
}

// completeSaveOperation завершает операцию сохранения с небольшой задержкой.
func (p *WizardPresenter) completeSaveOperation() {
	<-time.After(100 * time.Millisecond)
	p.UpdateSaveProgress(1.0)
	<-time.After(200 * time.Millisecond)
}
