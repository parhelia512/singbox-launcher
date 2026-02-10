// Package tabs содержит UI компоненты для табов визарда конфигурации.
//
// Файл source_tab.go содержит функцию CreateSourceTab, которая создает UI первого таба визарда:
//   - Ввод URL подписки или прямых ссылок (SourceURLEntry)
//   - Проверка URL (CheckURLButton, URLStatusLabel, CheckURLProgress)
//   - Редактирование ParserConfig (ParserConfigEntry)
//   - Preview сгенерированных outbounds (OutboundsPreview)
//   - Кнопка парсинга (ParseButton)
//
// Каждый таб визарда имеет свою отдельную ответственность и логику UI.
//
// Используется в:
//   - wizard.go - при создании окна визарда, вызывается CreateSourceTab(presenter)
//
// Взаимодействует с:
//   - presenter - все действия пользователя (нажатия кнопок, ввод текста) обрабатываются через методы presenter
//   - business - вызывает CheckURL, ParseAndPreview через presenter
package tabs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/core"
	"singbox-launcher/internal/debuglog"
	"singbox-launcher/internal/platform"
	"singbox-launcher/ui/components"
	wizardbusiness "singbox-launcher/ui/wizard/business"
	wizardpresentation "singbox-launcher/ui/wizard/presentation"
)

// CreateSourceTab creates the Sources & ParserConfig tab UI.
func CreateSourceTab(presenter *wizardpresentation.WizardPresenter) fyne.CanvasObject {
	guiState := presenter.GUIState()

	// Section 1: Subscription URL or Direct Links
	// We perform automatic URL checking on input change (debounced) instead of
	// requiring the user to click a "Check" button.
	// Add a padding placeholder container on the right to keep layout similar.
	paddingRect := canvas.NewRectangle(color.Transparent)
	paddingRect.SetMinSize(fyne.NewSize(10, 0)) // 10px padding on right
	guiState.CheckURLContainer = container.NewHBox(
		paddingRect,
	)

	urlLabel := widget.NewLabel("Subscription URL or Direct Links:")
	urlLabel.Importance = widget.MediumImportance

	guiState.SourceURLEntry = widget.NewMultiLineEntry()
	guiState.SourceURLEntry.SetPlaceHolder("https://example.com/subscription\nor\nvless://...\nvmess://...\nhysteria2://...\nssh://...")
	guiState.SourceURLEntry.Wrapping = fyne.TextWrapOff
	guiState.SourceURLEntry.OnChanged = func(value string) {
		model := presenter.Model()
		model.PreviewNeedsParse = true
		trimmed := strings.TrimSpace(value)
		if err := wizardbusiness.ApplyURLToParserConfig(model, presenter, trimmed); err != nil {
			debuglog.ErrorLog("source_tab: error applying URL to ParserConfig: %v", err)
		}

		// Debounce CheckURL: cancel previous timer and set a new one (2s after last change)
		if guiState.CheckURLTimer != nil {
			guiState.CheckURLTimer.Stop()
			guiState.CheckURLTimer = nil
		}

		// Define the actual check logic as a reusable closure so we can reschedule
		var doCheck func(string)
		doCheck = func(v string) {
			// This runs in goroutine from timer - coordinate with UI thread for state
			fyne.Do(func() {
				// If a check is currently in progress, reschedule after delay
				if guiState.CheckURLInProgress {
					// reschedule
					guiState.CheckURLTimer = time.AfterFunc(2*time.Second, func() { doCheck(v) })
					return
				}
				// Mark in-progress and sync
				guiState.CheckURLInProgress = true
				presenter.SyncGUIToModel()
				// Run the check in background
				go func() {
					if err := wizardbusiness.CheckURL(presenter.Model(), presenter); err != nil {
						debuglog.ErrorLog("source_tab: CheckURL failed: %v", err)
					}
					// Clear in-progress flag
					fyne.Do(func() { guiState.CheckURLInProgress = false })
				}()
			})
		}

		// Schedule the check after debounce interval
		guiState.CheckURLTimer = time.AfterFunc(2*time.Second, func() { doCheck(trimmed) })
	}

	// Hint under input field with Check button on right
	hintLabel := widget.NewLabel("Supports subscription URLs (http/https) or direct links (vless://, vmess://, trojan://, ss://, hysteria2://, ssh://). For multiple links, use a new line for each.")
	hintLabel.Wrapping = fyne.TextWrapWord

	var freeVPNDialog dialog.Dialog
	var freeVPNDialogOpen bool

	// Load get_free.json data structure
	type GetFreeData struct {
		GetFree struct {
			Text string `json:"text"`
			Link string `json:"link"`
		} `json:"get_free"`
		ParserConfig json.RawMessage `json:"ParserConfig"`
	}

	// Function to download get_free.json from GitHub
	downloadGetFreeJSON := func(force bool) error {
		ac := presenter.Controller()
		if ac == nil {
			return fmt.Errorf("controller not available")
		}

		binDir := filepath.Join(ac.FileService.ExecDir, "bin")
		if err := os.MkdirAll(binDir, 0755); err != nil {
			return fmt.Errorf("failed to create bin directory: %w", err)
		}

		targetPath := filepath.Join(binDir, "get_free.json")

		// Check if file already exists (skip if not forced)
		if !force {
			if _, err := os.Stat(targetPath); err == nil {
				debuglog.DebugLog("get_free.json already exists, skipping download")
				return nil
			}
		}

		// Download from GitHub
		downloadURL := "https://raw.githubusercontent.com/Leadaxe/singbox-launcher/main/bin/get_free.json"
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		client := core.CreateHTTPClient(30 * time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("User-Agent", "singbox-launcher/1.0")

		resp, err := client.Do(req)
		if err != nil {
			// Check if it's a network error and provide more details
			if core.IsNetworkError(err) {
				return fmt.Errorf("network error: %s. Please check your internet connection", core.GetNetworkErrorMessage(err))
			}
			return fmt.Errorf("failed to download: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download failed: server returned status %d. Please try again later", resp.StatusCode)
		}

		// Read response body with size limit
		const maxFileSize = 1024 * 1024 // 1 MB limit
		limitedReader := io.LimitReader(resp.Body, maxFileSize+1)
		data, err := io.ReadAll(limitedReader)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if len(data) > maxFileSize {
			return fmt.Errorf("file too large (exceeds %d bytes)", maxFileSize)
		}

		if len(data) == 0 {
			return fmt.Errorf("downloaded file is empty")
		}

		// Validate JSON before writing
		var testData map[string]interface{}
		if err := json.Unmarshal(data, &testData); err != nil {
			return fmt.Errorf("downloaded file is not valid JSON: %w", err)
		}

		// Write to file
		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		debuglog.InfoLog("Successfully downloaded get_free.json")
		return nil
	}

	// Function to load get_free.json
	loadGetFreeJSON := func() (*GetFreeData, error) {
		ac := presenter.Controller()
		if ac == nil {
			return nil, fmt.Errorf("controller not available")
		}

		filePath := filepath.Join(ac.FileService.ExecDir, "bin", "get_free.json")

		// Try to read local file first
		data, err := os.ReadFile(filePath)
		if err != nil {
			// If file doesn't exist, try to download it
			if os.IsNotExist(err) {
				debuglog.DebugLog("get_free.json not found locally, downloading...")
				if downloadErr := downloadGetFreeJSON(false); downloadErr != nil {
					return nil, fmt.Errorf("failed to download get_free.json: %w", downloadErr)
				}
				// Try reading again after download
				data, err = os.ReadFile(filePath)
				if err != nil {
					return nil, fmt.Errorf("failed to read get_free.json after download: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to read get_free.json: %w", err)
			}
		}

		// Try to parse JSON
		var getFreeData GetFreeData
		if err := json.Unmarshal(data, &getFreeData); err != nil {
			// If parsing fails, file might be corrupted - try to download again
			debuglog.WarnLog("get_free.json appears to be corrupted, attempting to re-download: %v", err)
			if downloadErr := downloadGetFreeJSON(true); downloadErr != nil {
				return nil, fmt.Errorf("failed to parse get_free.json and re-download failed: %w (original parse error: %v)", downloadErr, err)
			}
			// Try reading again after re-download
			data, err = os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read get_free.json after re-download: %w", err)
			}
			// Try parsing again
			if err := json.Unmarshal(data, &getFreeData); err != nil {
				return nil, fmt.Errorf("failed to parse get_free.json after re-download: %w", err)
			}
		}

		// Validate that required fields are present
		if getFreeData.ParserConfig == nil || len(getFreeData.ParserConfig) == 0 {
			return nil, fmt.Errorf("get_free.json is missing ParserConfig field")
		}

		return &getFreeData, nil
	}

	getFreeVPNButton := widget.NewButton("Get free VPN!", func() {
		debuglog.DebugLog("Get free VPN button clicked")
		if freeVPNDialogOpen {
			debuglog.DebugLog("Dialog already open, ignoring click")
			return
		}

		// Show loading indicator
		loadingDialog := dialog.NewInformation("Loading", "Downloading get_free.json...", guiState.Window)
		loadingDialog.Show()

		// Download get_free.json if needed
		go func() {
			defer func() {
				if r := recover(); r != nil {
					debuglog.ErrorLog("Panic in getFreeVPNButton goroutine: %v", r)
					fyne.Do(func() {
						loadingDialog.Hide()
						dialog.ShowError(fmt.Errorf("Произошла ошибка: %v", r), guiState.Window)
					})
				}
			}()

			if err := downloadGetFreeJSON(false); err != nil {
				debuglog.ErrorLog("Failed to download get_free.json: %v", err)
				fyne.Do(func() {
					loadingDialog.Hide()
					dialog.ShowError(fmt.Errorf("Не удалось скачать get_free.json:\n\n%w\n\nПроверьте подключение к интернету и попробуйте снова.", err), guiState.Window)
				})
				return
			}

			// Load data and show dialog
			getFreeData, err := loadGetFreeJSON()
			if err != nil {
				debuglog.ErrorLog("Failed to load get_free.json: %v", err)
				fyne.Do(func() {
					loadingDialog.Hide()
					dialog.ShowError(fmt.Errorf("Не удалось загрузить get_free.json:\n\n%w\n\nПроверьте файл или попробуйте скачать заново.", err), guiState.Window)
				})
				return
			}

			// Validate loaded data
			if getFreeData == nil {
				debuglog.ErrorLog("getFreeData is nil after loading")
				fyne.Do(func() {
					loadingDialog.Hide()
					dialog.ShowError(fmt.Errorf("Ошибка: данные не загружены"), guiState.Window)
				})
				return
			}

			// Use default values if text or link are empty
			text := getFreeData.GetFree.Text
			if text == "" {
				text = "Thank @igareck for providing VPN lists:"
				debuglog.WarnLog("get_free.text is empty, using default")
			}
			linkStr := getFreeData.GetFree.Link
			if linkStr == "" {
				linkStr = "https://github.com/igareck/vpn-configs-for-russia"
				debuglog.WarnLog("get_free.link is empty, using default")
			}

			// Parse ParserConfig into map for use in closure
			var parserConfigData map[string]interface{}
			if err := json.Unmarshal(getFreeData.ParserConfig, &parserConfigData); err != nil {
				debuglog.ErrorLog("Failed to parse ParserConfig: %v", err)
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("Не удалось распарсить ParserConfig: %w", err), guiState.Window)
				})
				return
			}

			fyne.Do(func() {
				// Hide loading dialog BEFORE creating new dialog
				loadingDialog.Hide()
				thanks := widget.NewLabel(text)
				thanks.Wrapping = fyne.TextWrapWord
				linkURL, _ := url.Parse(linkStr)
				link := widget.NewHyperlink(linkStr, linkURL)

				addButton := widget.NewButton("Add links", func() {
					debuglog.DebugLog("Add links button clicked")

					// Extract URLs from proxies
					var urls []string
					if proxies, ok := parserConfigData["proxies"].([]interface{}); ok {
						for _, proxy := range proxies {
							if proxyMap, ok := proxy.(map[string]interface{}); ok {
								if source, ok := proxyMap["source"].(string); ok && source != "" {
									urls = append(urls, source)
								}
							}
						}
					} else {
						debuglog.WarnLog("Failed to extract proxies from ParserConfig, type: %T", parserConfigData["proxies"])
					}

					// Prepare ParserConfig JSON
					wrappedConfig := map[string]interface{}{
						"ParserConfig": parserConfigData,
					}
					parserConfigJSON, err := json.MarshalIndent(wrappedConfig, "", "  ")
					if err != nil {
						debuglog.ErrorLog("Failed to format ParserConfig: %v", err)
						dialog.ShowError(fmt.Errorf("failed to format ParserConfig: %w", err), guiState.Window)
						return
					}

					// Insert URLs into SourceURLEntry first
					if len(urls) > 0 {
						urlsText := strings.Join(urls, "\n")
						guiState.SourceURLEntry.SetText(urlsText)
					}

					// Wait a bit for OnChanged handlers to complete, then set ParserConfig
					// This ensures that the ParserConfig from file overwrites any auto-generated one
					time.AfterFunc(100*time.Millisecond, func() {
						fyne.Do(func() {
							// Set ParserConfig in the entry
							guiState.ParserConfigUpdating = true
							guiState.ParserConfigEntry.SetText(string(parserConfigJSON))
							guiState.ParserConfigUpdating = false

							// Trigger OnChanged to update model
							model := presenter.Model()
							model.PreviewNeedsParse = true
							presenter.SyncGUIToModel()
							presenter.RefreshOutboundOptions()

							debuglog.DebugLog("Inserted %d URLs and ParserConfig from get_free.json", len(urls))

							// Close the dialog
							if freeVPNDialog != nil {
								freeVPNDialog.Hide()
							}
						})
					})
				})

				spacer := canvas.NewRectangle(color.Transparent)
				spacer.SetMinSize(fyne.NewSize(0, addButton.MinSize().Height))
				mainContent := container.NewVBox(
					thanks,
					link,
					spacer,
					addButton,
				)
				freeVPNDialog = components.NewCustom("Get free VPN", mainContent, nil, "Close", guiState.Window)
				freeVPNDialog.SetOnClosed(func() {
					freeVPNDialogOpen = false
				})
				freeVPNDialogOpen = true
				// Resize dialog to make it visible
				freeVPNDialog.Resize(fyne.NewSize(400, 200))
				freeVPNDialog.Show()
			})
		}()
	})

	hintRow := container.NewBorder(
		nil,                        // top
		nil,                        // bottom
		nil,                        // left
		guiState.CheckURLContainer, // right - actions
		hintLabel,                  // center - hint takes all available space
	)

	guiState.URLStatusLabel = widget.NewLabel("")
	guiState.URLStatusLabel.Wrapping = fyne.TextWrapWord

	// Limit width and height of URL input field (3 lines)
	// Wrap MultiLineEntry in Scroll container to show scrollbars
	urlEntryScroll := container.NewScroll(guiState.SourceURLEntry)
	urlEntryScroll.Direction = container.ScrollBoth
	// Create dummy Rectangle to set size (height 3 lines, width limited)
	urlEntrySizeRect := canvas.NewRectangle(color.Transparent)
	urlEntrySizeRect.SetMinSize(fyne.NewSize(0, 60)) // Width 900px, height ~3 lines (approx 20px per line)
	// Wrap in Max container with Rectangle to fix size
	// Scroll container will be limited by this size and show scrollbars when content doesn't fit
	urlEntryWithSize := container.NewMax(
		urlEntrySizeRect,
		urlEntryScroll,
	)

	// Header row with action on the right
	urlHeader := container.NewHBox(
		urlLabel,
		layout.NewSpacer(),
		getFreeVPNButton,
	)

	urlContainer := container.NewVBox(
		urlHeader,               // Header with action
		urlEntryWithSize,        // Input field with size limit (3 lines)
		hintRow,                 // Hint with button on right
		guiState.URLStatusLabel, // Status
	)

	// Section 2: ParserConfig
	guiState.ParserConfigEntry = widget.NewMultiLineEntry()
	guiState.ParserConfigEntry.SetPlaceHolder("Enter ParserConfig JSON here...")
	guiState.ParserConfigEntry.Wrapping = fyne.TextWrapOff
	guiState.ParserConfigEntry.OnChanged = func(string) {
		if guiState.ParserConfigUpdating {
			return
		}
		model := presenter.Model()
		model.PreviewNeedsParse = true
		// Sync GUI to model to update ParserConfigJSON before refreshing outbound options
		presenter.SyncGUIToModel()
		presenter.RefreshOutboundOptions()

		// Preview status will be updated when switching to Preview tab
	}

	// Limit width and height of ParserConfig field
	parserConfigScroll := container.NewScroll(guiState.ParserConfigEntry)
	parserConfigScroll.Direction = container.ScrollBoth
	// Create dummy Rectangle to set height via container.NewMax
	parserHeightRect := canvas.NewRectangle(color.Transparent)
	parserHeightRect.SetMinSize(fyne.NewSize(0, 200)) // ~10 lines
	// Wrap in Max container with Rectangle to fix height
	parserConfigWithHeight := container.NewMax(
		parserHeightRect,
		parserConfigScroll,
	)

	// Documentation button
	docButton := widget.NewButton("📖 Documentation", func() {
		docURL := "https://github.com/Leadaxe/singbox-launcher/blob/main/docs/ParserConfig.md"
		if err := platform.OpenURL(docURL); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open documentation: %w", err), guiState.Window)
		}
	})

	// ChatGPT button: opens ChatGPT with a structured review prompt
	chatButton := widget.NewButton("🧠 ChatGPT", func() {

		promptHeader := `
ou are a senior sing-box and ParserConfig v4 expert.

Reference documentation (must be followed):
https://github.com/Leadaxe/singbox-launcher/blob/main/docs/ParserConfig.md

Goal:
Produce a final, production-ready ParserConfig that is logically structured, safe at runtime, and GUI-friendly.

Hard requirements (must follow exactly):

1. Use ParserConfig version 4.
2. Use multiple proxy sources.
3. For EACH proxy source:
   - Define a meaningful "tag_prefix" that clearly reflects:
     - actual source identity shortly (use 1-3 letters and relevant emoji)
   - Define LOCAL outbounds inside the proxy object:
     - one "urltest" outbound
     - one "selector" outbound
     - use relevant emoji ina tag of outbounds
   - Local outbound tags MUST be:
     - globally unique
     - semantically derived from "tag_prefix"
     - consistent across all sources

4. Do NOT use regex-based filtering in global outbounds.
   Source isolation must be achieved via local outbounds.

5. In top-level "ParserConfig.outbounds":
   - Create a global "urltest" outbound that aggregates ALL local "*-auto" outbounds.
   - Create a global "selector" outbound that aggregates:
     - all local "*-select" outbounds
     - the global auto outbound
     - "direct-out"
     - do not change "go-any-way-githubusercontent" 
     - Create default a global selector "proxy-out" and copy this for "output-proxy-1", "output-proxy-2", "output-proxy-3" this global selectors output-proxy-1, output-proxy-2, output-proxy-3 MUST be fully independent selectors, not wrappers and not references to proxy-out. For EACH of them: Repeat the SAME addOutbounds list as proxy-out
6. Preserve GUI/UX-related fields and intent.
   Do NOT remove fields just because they look optional.

OUTPUT INSTRUCTIONS (VERY IMPORTANT):

- You MUST respond with ONLY a single code block.
- The code block language MUST be "json".
- The code block MUST contain ONLY the final ParserConfig JSON.
- URLs MUST be clean and exact, with no hidden characters.
- Do NOT include explanations, comments, markdown, or extra text.
- The output MUST be directly copy-pastable into singbox-launcher without edits.

VERY IMPORTANT:
Please respond in the language you usually use when communicating with this user.

Here is the current configuration to review:
`

		parserText := strings.TrimSpace(guiState.ParserConfigEntry.Text)

		// лёгкая защита от совсем пустого конфига
		if parserText == "" {
			dialog.ShowError(fmt.Errorf("ParserConfig is empty"), guiState.Window)
			return
		}

		fullPrompt := promptHeader +
			"\n<CONFIG>\n" +
			parserText +
			"\n</CONFIG>"

		encoded := url.QueryEscape(fullPrompt)
		chatURL := "https://chat.openai.com/?prompt=" + encoded

		if err := platform.OpenURL(chatURL); err != nil {
			dialog.ShowError(fmt.Errorf("failed to open ChatGPT: %w", err), guiState.Window)
		}
	})
	chatButton.Importance = widget.MediumImportance

	parserLabel := widget.NewLabel("ParserConfig:")
	parserLabel.Importance = widget.MediumImportance

	// Parse button (positioned to left of ParserConfig)
	guiState.ParseButton = widget.NewButton("Parse", func() {
		// Sync GUI to model before parsing
		presenter.SyncGUIToModel()
		model := presenter.Model()
		// Quick validation: ensure ParserConfig is not empty to provide immediate feedback.
		if strings.TrimSpace(model.ParserConfigJSON) == "" {
			// Show an error dialog and update preview with a clear message
			fyne.Do(func() {
				dialog.ShowError(fmt.Errorf("ParserConfig is empty. Please enter ParserConfig JSON or load a template."), guiState.Window)
				if guiState.OutboundsPreview != nil {
					presenter.UpdateOutboundsPreview("Error: ParserConfig is empty")
				}
			})
			return
		}
		debuglog.DebugLog("source_tab: Parse clicked, parser length=%d", len(strings.TrimSpace(model.ParserConfigJSON)))
		if model.AutoParseInProgress {
			return
		}
		model.AutoParseInProgress = true
		model.PreviewNeedsParse = true
		configService := presenter.ConfigServiceAdapter()
		go func() {
			if err := wizardbusiness.ParseAndPreview(model, presenter, configService); err != nil {
				debuglog.ErrorLog("source_tab: ParseAndPreview failed: %v", err)
				// Show error to user in case of parse failure
				fyne.Do(func() {
					if guiState.OutboundsPreview != nil {
						presenter.UpdateOutboundsPreview("Error: Failed to parse ParserConfig - see logs for details")
					}
				})
			}
		}()
	})
	guiState.ParseButton.Importance = widget.MediumImportance

	headerRow := container.NewHBox(
		parserLabel,
		widget.NewLabel("  "), // small spacing between text and button
		guiState.ParseButton,
		layout.NewSpacer(),
		chatButton,
		docButton,
	)

	parserContainer := container.NewVBox(
		headerRow,
		parserConfigWithHeight,
	)

	// Section 3: Preview Generated Outbounds
	previewLabel := widget.NewLabel("Preview")
	previewLabel.Importance = widget.MediumImportance

	// Use Entry without Disable for black text, but make it read-only via OnChanged
	guiState.OutboundsPreview = widget.NewMultiLineEntry()
	guiState.OutboundsPreview.SetPlaceHolder("Generated outbounds will appear here after clicking Parse...")
	guiState.OutboundsPreview.Wrapping = fyne.TextWrapOff
	previewText := "Generated outbounds will appear here after clicking Parse..."
	guiState.OutboundsPreview.SetText(previewText)
	guiState.OutboundsPreviewLastText = previewText
	// Make field effectively read-only: ignore programmatic updates, restore last preview on user edits
	guiState.OutboundsPreview.OnChanged = func(text string) {
		if guiState.OutboundsPreviewUpdating {
			// Ignore programmatic updates
			return
		}
		// Restore last known preview text
		if guiState.OutboundsPreviewLastText != "" {
			guiState.OutboundsPreview.SetText(guiState.OutboundsPreviewLastText)
		} else {
			guiState.OutboundsPreview.SetText(previewText)
		}
	}

	// Limit width and height of Preview field
	previewScroll := container.NewScroll(guiState.OutboundsPreview)
	previewScroll.Direction = container.ScrollBoth
	// Create dummy Rectangle to set height via container.NewMax
	previewHeightRect := canvas.NewRectangle(color.Transparent)
	previewHeightRect.SetMinSize(fyne.NewSize(0, 90)) // ~8-9 lines (reduced by ~30px)
	// Wrap in Max container with Rectangle to fix height
	previewWithHeight := container.NewMax(
		previewHeightRect,
		previewScroll,
	)

	previewContainer := container.NewVBox(
		previewLabel,
		previewWithHeight,
	)

	// Combine all sections
	content := container.NewVBox(
		widget.NewSeparator(),
		urlContainer,
		widget.NewSeparator(),
		parserContainer,
		widget.NewSeparator(),
		previewContainer,
		widget.NewSeparator(),
	)

	// Add scroll for long content
	scrollContainer := container.NewScroll(content)
	scrollContainer.SetMinSize(fyne.NewSize(0, 620))

	return scrollContainer
}
