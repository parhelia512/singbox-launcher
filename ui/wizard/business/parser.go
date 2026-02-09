// Package business содержит бизнес-логику визарда конфигурации.
//
// Файл parser.go является оркестратором/координатором, который вызывает реальные парсеры
// из core-модулей, но сам не содержит логику парсинга. Его функции:
//   - CheckURL - координирует проверку URL через subscription.FetchSubscription, subscription.ParseNode
//   - ParseAndPreview - координирует генерацию outbounds через ConfigService.GenerateOutboundsFromParserConfig
//   - ApplyURLToParserConfig - применяет URL к ParserConfig (работает со структурами config.ParserConfig)
//   - SerializeParserConfig - сериализует через config.NormalizeParserConfig
//
// Файл работает в контексте визарда (использует WizardModel и UIUpdater для обновления GUI).
// Координирует вызовы реальных парсеров из core/config/subscription и core/config.
// Интегрирован с GUI через UIUpdater (обновляет GUI прогресс, статусы и preview).
//
// Реальная логика парсинга находится в:
//   - core/config/parser - парсинг @ParserConfig блоков из файлов
//   - core/config/subscription - парсинг URL подписок и прямых ссылок
//   - core/config - генерация outbounds из ParserConfig
package business

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"singbox-launcher/core/config"
	"singbox-launcher/core/config/subscription"
	"singbox-launcher/internal/debuglog"
	wizardmodels "singbox-launcher/ui/wizard/models"
	wizardutils "singbox-launcher/ui/wizard/utils"
)

// CheckURL validates subscription URLs or direct links and updates the model through UIUpdater.
// It checks availability of subscription URLs and validates direct links.
func CheckURL(model *wizardmodels.WizardModel, updater UIUpdater) error {
	timing := debuglog.StartTiming("checkURL")
	defer timing.EndWithDefer()

	input := strings.TrimSpace(model.SourceURLs)
	if input == "" {
		debuglog.DebugLog("checkURL: Empty input, returning early")
		updater.UpdateURLStatus("❌ Please enter a URL or direct link")
		updater.UpdateCheckURLButtonText("Check")
		updater.UpdateCheckURLProgress(-1)
		return fmt.Errorf("empty input")
	}

	// Initialize UI state
	initializeCheckURLUI(updater)

	// Process all input lines
	inputLines := strings.Split(input, "\n")
	debuglog.DebugLog("checkURL: Processing %d input lines", len(inputLines))

	// Pre-allocate slices with estimated capacity
	estimatedPreview := min(len(inputLines), wizardutils.MaxPreviewLines)
	estimatedErrors := len(inputLines) / 4
	if estimatedErrors < 1 {
		estimatedErrors = 1
	}
	previewLines := make([]string, 0, estimatedPreview)
	errors := make([]string, 0, estimatedErrors)

	// Process each line
	totalValid := processAllInputLines(inputLines, updater, &previewLines, &errors)

	// Build and display result
	buildAndDisplayCheckResult(totalValid, previewLines, errors, updater)

	return nil
}

// initializeCheckURLUI инициализирует UI для проверки URL.
func initializeCheckURLUI(updater UIUpdater) {
	updater.UpdateURLStatus("⏳ Checking...")
	updater.UpdateCheckURLButtonText("")
	updater.UpdateCheckURLProgress(0.0)
}

// processAllInputLines обрабатывает все входные строки.
// Возвращает общее количество валидных ссылок.
func processAllInputLines(inputLines []string, updater UIUpdater, previewLines *[]string, errors *[]string) int {
	totalValid := 0

	for i, line := range inputLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Update progress
		updateCheckProgress(updater, i+1, len(inputLines))

		// Process line based on type
		validCount := processInputLine(line, i+1, len(inputLines), previewLines, errors, totalValid)
		totalValid += validCount
	}

	debuglog.DebugLog("checkURL: Processed all lines (total valid: %d, errors: %d)", totalValid, len(*errors))
	return totalValid
}

// updateCheckProgress обновляет прогресс проверки.
func updateCheckProgress(updater UIUpdater, current, total int) {
	progress := float64(current) / float64(total)
	updater.UpdateURLStatus(fmt.Sprintf("⏳ Checking... (%d/%d)", current, total))
	updater.UpdateCheckURLProgress(progress)
}

// processInputLine обрабатывает одну входную строку.
// Возвращает количество валидных ссылок, найденных в этой строке.
// currentValidCount используется для правильной нумерации в previewLines.
func processInputLine(line string, lineNum, totalLines int, previewLines *[]string, errors *[]string, currentValidCount int) int {
	lineStartTime := time.Now()
		linePreview := line
		if len(line) > 50 {
			linePreview = line[:50] + "..."
		}
	debuglog.DebugLog("checkURL: Processing line %d/%d: %s", lineNum, totalLines, linePreview)

		if subscription.IsSubscriptionURL(line) {
		return processSubscriptionURL(line, lineNum, totalLines, previewLines, errors, lineStartTime, currentValidCount)
	} else if subscription.IsDirectLink(line) {
		return processDirectLink(line, lineNum, totalLines, previewLines, errors, lineStartTime, currentValidCount)
	} else {
		debuglog.DebugLog("checkURL: Unknown format for line %d/%d: %s", lineNum, totalLines, line)
		*errors = append(*errors, fmt.Sprintf("Unknown format: %s", line))
		return 0
	}
}

// processSubscriptionURL обрабатывает subscription URL.
// Возвращает количество валидных ссылок, найденных в подписке.
func processSubscriptionURL(
	line string,
	lineNum, totalLines int,
	previewLines *[]string,
	errors *[]string,
	lineStartTime time.Time,
	currentValidCount int,
) int {
			// Validate URL before fetching
			if err := ValidateURL(line); err != nil {
		debuglog.DebugLog("checkURL: Invalid subscription URL %d/%d: %v", lineNum, totalLines, err)
		*errors = append(*errors, fmt.Sprintf("Invalid subscription URL: %v", err))
		return 0
			}

	// Fetch subscription
			fetchStartTime := time.Now()
	debuglog.DebugLog("checkURL: Fetching subscription %d/%d: %s", lineNum, totalLines, line)
			content, err := subscription.FetchSubscription(line)
			fetchDuration := time.Since(fetchStartTime)
			if err != nil {
		debuglog.DebugLog("checkURL: Failed to fetch subscription %d/%d (took %v): %v", lineNum, totalLines, fetchDuration, err)
		*errors = append(*errors, fmt.Sprintf("Failed to fetch %s: %v", line, err))
		return 0
			}

			// Validate response size
			if err := ValidateHTTPResponseSize(int64(len(content))); err != nil {
		debuglog.DebugLog("checkURL: Subscription response too large %d/%d: %v", lineNum, totalLines, err)
		*errors = append(*errors, fmt.Sprintf("Subscription response too large: %v", err))
		return 0
			}

	debuglog.DebugLog("checkURL: Fetched subscription %d/%d: %d bytes in %v", lineNum, totalLines, len(content), fetchDuration)

	// Parse subscription content and count valid links
	validCount := parseSubscriptionContent(content, lineNum, totalLines, previewLines, errors, lineStartTime, currentValidCount)
	if validCount == 0 {
		*errors = append(*errors, fmt.Sprintf("Subscription %s contains no valid proxy links", line))
	}

	return validCount
}

// parseSubscriptionContent парсит содержимое подписки и подсчитывает валидные ссылки.
func parseSubscriptionContent(
	content []byte,
	lineNum, totalLines int,
	previewLines *[]string,
	errors *[]string,
	lineStartTime time.Time,
	currentValidCount int,
) int {
			parseStartTime := time.Now()
			subLines := strings.Split(string(content), "\n")
	debuglog.DebugLog("checkURL: Parsing subscription %d/%d: %d lines", lineNum, totalLines, len(subLines))

			validInSub := 0
	validCount := currentValidCount

			for _, subLine := range subLines {
				subLine = strings.TrimSpace(subLine)
				if subLine != "" && subscription.IsDirectLink(subLine) {
					validInSub++
			validCount++
			if len(*previewLines) < wizardutils.MaxPreviewLines {
				*previewLines = append(*previewLines, fmt.Sprintf("%d. %s", validCount, subLine))
					}
				}
			}

			parseDuration := time.Since(parseStartTime)
			debuglog.DebugLog("checkURL: Parsed subscription %d/%d: %d valid links in %v (line processing took %v total)",
		lineNum, totalLines, validInSub, parseDuration, time.Since(lineStartTime))

	return validInSub
}

// processDirectLink обрабатывает прямую ссылку.
// Возвращает 1, если ссылка валидна, иначе 0.
func processDirectLink(
	line string,
	lineNum, totalLines int,
	previewLines *[]string,
	errors *[]string,
	lineStartTime time.Time,
	currentValidCount int,
) int {
			// Validate URI before parsing
			if err := ValidateURI(line); err != nil {
		debuglog.DebugLog("checkURL: Invalid URI format %d/%d: %v", lineNum, totalLines, err)
		*errors = append(*errors, fmt.Sprintf("Invalid URI format: %v", err))
		return 0
			}

	// Validate parsing
			parseStartTime := time.Now()
	debuglog.DebugLog("checkURL: Parsing direct link %d/%d", lineNum, totalLines)
			_, err := subscription.ParseNode(line, nil)
			parseDuration := time.Since(parseStartTime)

			if err != nil {
		debuglog.DebugLog("checkURL: Invalid direct link %d/%d (took %v): %v", lineNum, totalLines, parseDuration, err)
		*errors = append(*errors, fmt.Sprintf("Invalid direct link: %v", err))
		return 0
	}

	debuglog.DebugLog("checkURL: Valid direct link %d/%d (took %v)", lineNum, totalLines, parseDuration)
	if len(*previewLines) < wizardutils.MaxPreviewLines {
		validCount := currentValidCount + 1
		*previewLines = append(*previewLines, fmt.Sprintf("%d. %s", validCount, line))
				}
	return 1
}

// buildAndDisplayCheckResult строит и отображает результат проверки.
func buildAndDisplayCheckResult(totalValid int, previewLines []string, errors []string, updater UIUpdater) {
	if totalValid == 0 {
		buildErrorResult(errors, updater)
	} else {
		buildSuccessResult(totalValid, previewLines, errors, updater)
	}

	// Restore UI state
	updater.UpdateCheckURLButtonText("Check")
	updater.UpdateCheckURLProgress(-1)
}

// buildErrorResult строит сообщение об ошибке.
func buildErrorResult(errors []string, updater UIUpdater) {
		errorMsg := "❌ No valid proxy links found"
		if len(errors) > 0 {
			errorMsg += "\n" + strings.Join(errors[:min(3, len(errors))], "\n")
		}
		updater.UpdateURLStatus(errorMsg)
}

// buildSuccessResult строит сообщение об успешной проверке.
func buildSuccessResult(totalValid int, previewLines []string, errors []string, updater UIUpdater) {
		statusMsg := fmt.Sprintf("✅ Working! Found %d valid proxy link(s)", totalValid)
		if len(errors) > 0 {
			statusMsg += fmt.Sprintf("\n⚠️ %d error(s)", len(errors))
		}
		updater.UpdateURLStatus(statusMsg)

		if len(previewLines) > 0 {
			previewText := strings.Join(previewLines, "\n")
			if totalValid > len(previewLines) {
				previewText += fmt.Sprintf("\n... and %d more", totalValid-len(previewLines))
			}
			updater.UpdateOutboundsPreview(previewText)
		}
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ParseAndPreview parses ParserConfig and generates outbounds preview.
// It updates the model and UI through UIUpdater.
func ParseAndPreview(model *wizardmodels.WizardModel, updater UIUpdater, configService ConfigService) error {
	timing := debuglog.StartTiming("parseAndPreview")
	defer func() {
		timing.End()
		model.AutoParseInProgress = false
	}()

	updater.UpdateSaveButtonText("") // Hide save button during parsing
	updater.UpdateOutboundsPreview("Parsing configuration...")
	updater.UpdateCheckURLButtonText("") // Hide check URL button during parsing

	// Parse ParserConfig from field
	parseStartTime := time.Now()
	parserConfigJSON := strings.TrimSpace(model.ParserConfigJSON)
	debuglog.DebugLog("parseAndPreview: ParserConfig text length: %d bytes", len(parserConfigJSON))
	if parserConfigJSON == "" {
		debuglog.DebugLog("parseAndPreview: ParserConfig is empty, returning early")
		updater.UpdateOutboundsPreview("Error: ParserConfig is empty")
		updater.UpdateCheckURLButtonText("Check") // Restore check URL button
		updater.UpdateSaveButtonText("Save")      // Restore save button
		return fmt.Errorf("parserConfig is empty")
	}

	// Validate JSON size before parsing
	if err := ValidateJSONSize([]byte(parserConfigJSON)); err != nil {
		debuglog.DebugLog("parseAndPreview: ParserConfig JSON size validation failed: %v", err)
		updater.UpdateOutboundsPreview(fmt.Sprintf("Error: %v", err))
		updater.UpdateCheckURLButtonText("Check")
		updater.UpdateSaveButtonText("Save")
		return err
	}

	var parserConfig config.ParserConfig
	if err := json.Unmarshal([]byte(parserConfigJSON), &parserConfig); err != nil {
		timing.LogTiming("parse ParserConfig JSON", time.Since(parseStartTime))
		debuglog.DebugLog("parseAndPreview: Failed to parse ParserConfig JSON: %v", err)
		updater.UpdateOutboundsPreview(fmt.Sprintf("Error: Failed to parse ParserConfig JSON: %v", err))
		updater.UpdateCheckURLButtonText("Check")
		updater.UpdateSaveButtonText("Save")
		return fmt.Errorf("failed to parse ParserConfig JSON: %w", err)
	}

	// Validate ParserConfig structure
	if err := ValidateParserConfig(&parserConfig); err != nil {
		debuglog.DebugLog("parseAndPreview: ParserConfig validation failed: %v", err)
		updater.UpdateOutboundsPreview(fmt.Sprintf("Error: Invalid ParserConfig: %v", err))
		updater.UpdateCheckURLButtonText("Check")
		updater.UpdateSaveButtonText("Save")
		return err
	}
	timing.LogTiming("parse ParserConfig", time.Since(parseStartTime))
	debuglog.DebugLog("parseAndPreview: Parsed ParserConfig (sources: %d, outbounds: %d)",
		len(parserConfig.ParserConfig.Proxies), len(parserConfig.ParserConfig.Outbounds))

	// Check for URL or direct links
	url := strings.TrimSpace(model.SourceURLs)
	debuglog.DebugLog("parseAndPreview: URL text length: %d bytes", len(url))
	if url == "" {
		debuglog.DebugLog("parseAndPreview: URL is empty, returning early")
		updater.UpdateOutboundsPreview("Error: VLESS URL or direct links are empty")
		updater.UpdateCheckURLButtonText("Check")
		updater.UpdateSaveButtonText("Save")
		return fmt.Errorf("VLESS URL or direct links are empty")
	}

	// Update config through ApplyURLToParserConfig, which correctly separates subscriptions and connections
	applyStartTime := time.Now()
	debuglog.DebugLog("parseAndPreview: Applying URL to ParserConfig")
	if err := ApplyURLToParserConfig(model, updater, url); err != nil {
		debuglog.DebugLog("parseAndPreview: Failed to apply URL to ParserConfig: %v", err)
	}
	timing.LogTiming("apply URL to ParserConfig", time.Since(applyStartTime))

	// Reload parserConfig after update
	reloadStartTime := time.Now()
	parserConfigJSON = strings.TrimSpace(model.ParserConfigJSON)
	if parserConfigJSON != "" {
		if err := json.Unmarshal([]byte(parserConfigJSON), &parserConfig); err != nil {
			timing.LogTiming("reload ParserConfig JSON", time.Since(reloadStartTime))
			debuglog.DebugLog("parseAndPreview: Failed to parse updated ParserConfig JSON: %v", err)
			updater.UpdateOutboundsPreview(fmt.Sprintf("Error: Failed to parse updated ParserConfig JSON: %v", err))
			updater.UpdateCheckURLButtonText("Check")
			updater.UpdateSaveButtonText("Save")
			return fmt.Errorf("failed to parse updated ParserConfig JSON: %w", err)
		}
		timing.LogTiming("reload ParserConfig", time.Since(reloadStartTime))
		debuglog.DebugLog("parseAndPreview: Reloaded ParserConfig (sources: %d)",
			len(parserConfig.ParserConfig.Proxies))
	}

	// Generate all outbounds using unified function
	// This eliminates code duplication and adds support for local outbounds
	generateStartTime := time.Now()
	debuglog.DebugLog("parseAndPreview: Starting outbound generation using unified function")

	tagCounts := make(map[string]int)
	debuglog.DebugLog("parseAndPreview: Initializing tag deduplication tracker")

	var lastProgressUpdate time.Time
	progressCallback := func(p float64, s string) {
		now := time.Now()
		if now.Sub(lastProgressUpdate) < wizardutils.ProgressUpdateInterval {
			return
		}
		lastProgressUpdate = now
		updater.UpdateOutboundsPreview(s)
	}

	result, err := configService.GenerateOutboundsFromParserConfig(
		&parserConfig, tagCounts, progressCallback)
	if err != nil {
		timing.LogTiming("generate outbounds", time.Since(generateStartTime))
		debuglog.DebugLog("parseAndPreview: Failed to generate outbounds: %v", err)
		updater.UpdateOutboundsPreview(fmt.Sprintf("Error: Failed to generate outbounds: %v", err))
		updater.UpdateCheckURLButtonText("Check")
		updater.UpdateSaveButtonText("Save")
		return fmt.Errorf("failed to generate outbounds: %w", err)
	}

	subscription.LogDuplicateTagStatistics(tagCounts, "ConfigWizard")

	model.OutboundStats.NodesCount = result.NodesCount
	model.OutboundStats.LocalSelectorsCount = result.LocalSelectorsCount
	model.OutboundStats.GlobalSelectorsCount = result.GlobalSelectorsCount
	model.GeneratedOutbounds = result.OutboundsJSON

	var previewText string
	if result.NodesCount > wizardutils.MaxNodesForFullPreview {
		joinStartTime := time.Now()
		statsComment := fmt.Sprintf(`/** @ParserSTART */
	// Generated: %d nodes, %d local selectors, %d global selectors
	// Total outbounds: %d
/** @ParserEND */`,
			result.NodesCount,
			result.LocalSelectorsCount,
			result.GlobalSelectorsCount,
			len(result.OutboundsJSON))
		previewText = statsComment
		timing.LogTiming("generate statistics comment", time.Since(joinStartTime))
		debuglog.DebugLog("parseAndPreview: Generated statistics comment (nodes: %d > %d)", result.NodesCount, wizardutils.MaxNodesForFullPreview)
	} else {
		joinStartTime := time.Now()
		previewText = strings.Join(result.OutboundsJSON, "\n")
		timing.LogTiming("join JSON strings", time.Since(joinStartTime))
		debuglog.DebugLog("parseAndPreview: Joined %d JSON strings (total preview text length: %d bytes)",
			len(result.OutboundsJSON), len(previewText))
	}
	timing.LogTiming("total outbound generation", time.Since(generateStartTime))

	updater.UpdateOutboundsPreview(previewText)
	updater.UpdateCheckURLButtonText("Check")
	updater.UpdateSaveButtonText("Save")
	model.ParserConfig = &parserConfig
	model.PreviewNeedsParse = false
	// RefreshOutboundOptions will be called by presenter
	if model.TemplateData != nil && len(model.GeneratedOutbounds) > 0 {
		model.TemplatePreviewNeedsUpdate = true
		// go UpdateTemplatePreviewAsync(model, updater) // This will be called by presenter
	}
	return nil
}

// ApplyURLToParserConfig applies URL input to ParserConfig, correctly separating subscriptions and connections.
// It preserves existing local outbounds, tag_prefix, and tag_postfix for each source.
func ApplyURLToParserConfig(model *wizardmodels.WizardModel, updater UIUpdater, input string) error {
	timing := debuglog.StartTiming("applyURLToParserConfig")
	defer timing.EndWithDefer()
	debuglog.DebugLog("applyURLToParserConfig: input length: %d bytes", len(input))

	// Validate input
	if err := validateApplyURLInput(input, model.ParserConfigJSON); err != nil {
		return err
	}

	// Parse ParserConfig
	parserConfig, err := parseParserConfigForApply(model.ParserConfigJSON, timing)
	if err != nil {
		return err
	}

	// Classify input lines into subscriptions and connections
	subscriptions, connections := classifyInputLines(input, timing)

	// Preserve existing properties from current ParserConfig
	existingProps := preserveExistingProperties(parserConfig)

	// Create new ProxySource array
	newProxies := createSubscriptionProxies(subscriptions, existingProps)

	// Match or create connection proxy
	newProxies = matchOrCreateConnectionProxy(connections, existingProps, newProxies)

	// Ensure at least one empty proxy if no subscriptions or connections
	if len(newProxies) == 0 {
		newProxies = []config.ProxySource{{}}
	}

	// Update and serialize
	return updateAndSerializeParserConfig(parserConfig, newProxies, subscriptions, connections, model, updater, timing)
}

// validateApplyURLInput проверяет входные данные перед применением URL.
func validateApplyURLInput(input, parserConfigJSON string) error {
	if input == "" {
		debuglog.DebugLog("applyURLToParserConfig: input is empty, returning early")
		return fmt.Errorf("input is empty")
	}
	text := strings.TrimSpace(parserConfigJSON)
	if text == "" {
		debuglog.DebugLog("applyURLToParserConfig: ParserConfigJSON text is empty, returning early")
		return fmt.Errorf("parserConfigJSON is empty")
	}
	return nil
	}

// parseParserConfigForApply парсит ParserConfig из JSON строки.
func parseParserConfigForApply(parserConfigJSON string, timing interface{ LogTiming(string, time.Duration) }) (*config.ParserConfig, error) {
	parseStartTime := time.Now()
	var parserConfig config.ParserConfig
	text := strings.TrimSpace(parserConfigJSON)
	if err := json.Unmarshal([]byte(text), &parserConfig); err != nil {
		timing.LogTiming("parse ParserConfig", time.Since(parseStartTime))
		debuglog.DebugLog("applyURLToParserConfig: Failed to parse ParserConfig: %v", err)
		return nil, fmt.Errorf("failed to parse ParserConfig: %w", err)
	}
	timing.LogTiming("parse ParserConfig", time.Since(parseStartTime))
	debuglog.DebugLog("applyURLToParserConfig: Parsed ParserConfig (outbounds: %d)",
		len(parserConfig.ParserConfig.Outbounds))
	return &parserConfig, nil
}

// classifyInputLines классифицирует входные строки на подписки и прямые ссылки.
func classifyInputLines(input string, timing interface{ LogTiming(string, time.Duration) }) (subscriptions []string, connections []string) {
	splitStartTime := time.Now()
	lines := strings.Split(input, "\n")
	debuglog.DebugLog("applyURLToParserConfig: Split input into %d lines", len(lines))

	subscriptions = make([]string, 0)
	connections = make([]string, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if subscription.IsSubscriptionURL(line) {
			subscriptions = append(subscriptions, line)
		} else if subscription.IsDirectLink(line) {
			connections = append(connections, line)
		}
	}

	timing.LogTiming("classify lines", time.Since(splitStartTime))
	debuglog.DebugLog("applyURLToParserConfig: Classified lines: %d subscriptions, %d connections",
		len(subscriptions), len(connections))
	return subscriptions, connections
}

// existingProperties содержит сохраненные свойства существующих ProxySource.
type existingProperties struct {
	OutboundsMap         map[string][]config.OutboundConfig
	TagPrefixMap         map[string]string
	TagPostfixMap        map[string]string
	ConnectionsProxies   []config.ProxySource
}

// preserveExistingProperties сохраняет существующие свойства из текущего ParserConfig.
func preserveExistingProperties(parserConfig *config.ParserConfig) *existingProperties {
	props := &existingProperties{
		OutboundsMap:       make(map[string][]config.OutboundConfig),
		TagPrefixMap:       make(map[string]string),
		TagPostfixMap:      make(map[string]string),
		ConnectionsProxies: make([]config.ProxySource, 0),
	}

	for _, existingProxy := range parserConfig.ParserConfig.Proxies {
		if existingProxy.Source != "" {
			props.OutboundsMap[existingProxy.Source] = existingProxy.Outbounds
			if existingProxy.TagPrefix != "" {
				props.TagPrefixMap[existingProxy.Source] = existingProxy.TagPrefix
			}
			if existingProxy.TagPostfix != "" {
				props.TagPostfixMap[existingProxy.Source] = existingProxy.TagPostfix
			}
		} else if len(existingProxy.Connections) > 0 {
			// Preserve all ProxySource entries with connections but no source
			props.ConnectionsProxies = append(props.ConnectionsProxies, existingProxy)
		}
	}

	return props
	}

// createSubscriptionProxies создает ProxySource для каждой подписки.
func createSubscriptionProxies(subscriptions []string, existingProps *existingProperties) []config.ProxySource {
	newProxies := make([]config.ProxySource, 0, len(subscriptions))

	// Automatically add tag_prefix with sequential number only if there are multiple subscriptions
	autoAddPrefix := len(subscriptions) > 1

	for idx, sub := range subscriptions {
		proxySource := config.ProxySource{
			Source: sub,
		}

		// Restore local outbounds if they existed for this source
		if existingOutbounds, ok := existingProps.OutboundsMap[sub]; ok {
			proxySource.Outbounds = existingOutbounds
			debuglog.DebugLog("applyURLToParserConfig: Restored %d local outbounds for subscription: %s", len(existingOutbounds), sub)
		}

		// Restore tag_prefix and tag_postfix
		restoreTagPrefixAndPostfix(&proxySource, sub, existingProps, fmt.Sprintf("subscription: %s", sub))

		// Automatically add tag_prefix if not restored and auto-add is enabled
		if proxySource.TagPrefix == "" && autoAddPrefix {
			proxySource.TagPrefix = GenerateTagPrefix(idx + 1)
			debuglog.DebugLog("applyURLToParserConfig: Added automatic tag_prefix '%s' for subscription: %s", proxySource.TagPrefix, sub)
		}

		newProxies = append(newProxies, proxySource)
	}

	return newProxies
}

// restoreTagPrefixAndPostfix восстанавливает tag_prefix и tag_postfix из сохраненных свойств.
func restoreTagPrefixAndPostfix(proxySource *config.ProxySource, lookupKey string, existingProps *existingProperties, logContext string) {
	if existingTagPrefix, ok := existingProps.TagPrefixMap[lookupKey]; ok {
		proxySource.TagPrefix = existingTagPrefix
		debuglog.DebugLog("applyURLToParserConfig: Restored tag_prefix '%s' for %s", existingTagPrefix, logContext)
	}
	if existingTagPostfix, ok := existingProps.TagPostfixMap[lookupKey]; ok {
		proxySource.TagPostfix = existingTagPostfix
		debuglog.DebugLog("applyURLToParserConfig: Restored tag_postfix '%s' for %s", existingTagPostfix, logContext)
	}
}

// connectionsMatch проверяет, совпадают ли два массива connections (порядок не важен).
func connectionsMatch(conn1, conn2 []string) bool {
		if len(conn1) != len(conn2) {
			return false
		}
		// Create maps for comparison
		map1 := make(map[string]int)
		map2 := make(map[string]int)
		for _, c := range conn1 {
			map1[strings.TrimSpace(c)]++
		}
		for _, c := range conn2 {
			map2[strings.TrimSpace(c)]++
		}
		if len(map1) != len(map2) {
			return false
		}
		for k, v := range map1 {
			if map2[k] != v {
				return false
			}
		}
		return true
	}

// matchOrCreateConnectionProxy сопоставляет connections с существующим ProxySource или создает новый.
func matchOrCreateConnectionProxy(connections []string, existingProps *existingProperties, newProxies []config.ProxySource) []config.ProxySource {
	if len(connections) == 0 {
		// If user removed all connections, don't add any connection ProxySources
		// This allows user to clear connections by deleting them from GUI
		return newProxies
	}

		// Try to match with existing connections proxy by comparing connections
	for _, existingConnectionsProxy := range existingProps.ConnectionsProxies {
			if connectionsMatch(existingConnectionsProxy.Connections, connections) {
				// Matched existing proxy - update connections but preserve all other properties
				matchedProxy := config.ProxySource{
					Connections: connections, // Update with potentially reordered connections
					Outbounds:   existingConnectionsProxy.Outbounds,
					TagPrefix:   existingConnectionsProxy.TagPrefix,
					TagPostfix:  existingConnectionsProxy.TagPostfix,
					TagMask:     existingConnectionsProxy.TagMask,
					Skip:        existingConnectionsProxy.Skip,
				}
				newProxies = append(newProxies, matchedProxy)
				debuglog.DebugLog("applyURLToParserConfig: Matched existing connections proxy, preserved tag_prefix '%s', tag_postfix '%s', tag_mask '%s'",
					matchedProxy.TagPrefix, matchedProxy.TagPostfix, matchedProxy.TagMask)
			return newProxies
			}
		}

			// New connections - add as new ProxySource
			proxySource := config.ProxySource{
				Connections: connections,
			}
			debuglog.DebugLog("applyURLToParserConfig: Adding new ProxySource with %d connections", len(connections))
			newProxies = append(newProxies, proxySource)

		// Don't preserve other existing ProxySource entries with connections - user removed them
	if len(existingProps.ConnectionsProxies) > 0 {
		debuglog.DebugLog("applyURLToParserConfig: Not preserving %d other connection ProxySources (user removed them)", len(existingProps.ConnectionsProxies)-1)
	}

	return newProxies
}

// updateAndSerializeParserConfig обновляет ParserConfig и сериализует его.
func updateAndSerializeParserConfig(
	parserConfig *config.ParserConfig,
	newProxies []config.ProxySource,
	subscriptions []string,
	connections []string,
	model *wizardmodels.WizardModel,
	updater UIUpdater,
	timing interface{ LogTiming(string, time.Duration) },
) error {
	// Update proxies array
	parserConfig.ParserConfig.Proxies = newProxies
	debuglog.DebugLog("applyURLToParserConfig: Created %d proxy sources (%d subscriptions, %d with connections)",
		len(newProxies), len(subscriptions), len(connections))

	// Serialize
	serializeStartTime := time.Now()
	serialized, err := SerializeParserConfig(parserConfig)
	if err != nil {
		timing.LogTiming("serialize ParserConfig", time.Since(serializeStartTime))
		debuglog.DebugLog("applyURLToParserConfig: Failed to serialize ParserConfig: %v", err)
		return fmt.Errorf("failed to serialize ParserConfig: %w", err)
	}
	timing.LogTiming("serialize ParserConfig", time.Since(serializeStartTime))
	debuglog.DebugLog("applyURLToParserConfig: Serialized ParserConfig (result length: %d bytes, outbounds before: %d)",
		len(serialized), len(parserConfig.ParserConfig.Outbounds))

	// Update model and UI
	updater.UpdateParserConfig(serialized)
	model.ParserConfig = parserConfig
	model.PreviewNeedsParse = true
	return nil
}

// SerializeParserConfig serializes ParserConfig to JSON string.
func SerializeParserConfig(parserConfig *config.ParserConfig) (string, error) {
	if parserConfig == nil {
		return "", fmt.Errorf("parserConfig is nil")
	}

	// Normalize ParserConfig (migrate version, set defaults, but don't update last_updated)
	config.NormalizeParserConfig(parserConfig, false)

	// Serialize in version 2 format (version inside ParserConfig, not at top level)
	configToSerialize := map[string]interface{}{
		"ParserConfig": parserConfig.ParserConfig,
	}
	data, err := json.MarshalIndent(configToSerialize, "", IndentBase)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GenerateTagPrefix generates a tag prefix for a subscription based on its index.
// Format: "1:", "2:", "3:", etc.
// This function can be easily modified to change the prefix format.
func GenerateTagPrefix(index int) string {
	return fmt.Sprintf("%d:", index)
}

