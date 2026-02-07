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
	"log"
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

	updater.UpdateURLStatus("⏳ Checking...")
	updater.UpdateCheckURLButtonText("")
	updater.UpdateCheckURLProgress(0.0)

	// Split input into lines for processing
	inputLines := strings.Split(input, "\n")
	debuglog.DebugLog("checkURL: Processing %d input lines", len(inputLines))
	totalValid := 0
	previewLines := make([]string, 0)
	errors := make([]string, 0)

	for i, line := range inputLines {
		lineStartTime := time.Now()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		linePreview := line
		if len(line) > 50 {
			linePreview = line[:50] + "..."
		}
		debuglog.DebugLog("checkURL: Processing line %d/%d: %s", i+1, len(inputLines), linePreview)

		progress := float64(i+1) / float64(len(inputLines))
		updater.UpdateURLStatus(fmt.Sprintf("⏳ Checking... (%d/%d)", i+1, len(inputLines)))
		updater.UpdateCheckURLProgress(progress)

		if subscription.IsSubscriptionURL(line) {
			// Validate URL before fetching
			if err := ValidateURL(line); err != nil {
				debuglog.DebugLog("checkURL: Invalid subscription URL %d/%d: %v", i+1, len(inputLines), err)
				errors = append(errors, fmt.Sprintf("Invalid subscription URL: %v", err))
				continue
			}

			// This is a subscription URL - check availability
			fetchStartTime := time.Now()
			debuglog.DebugLog("checkURL: Fetching subscription %d/%d: %s", i+1, len(inputLines), line)
			content, err := subscription.FetchSubscription(line)
			fetchDuration := time.Since(fetchStartTime)
			if err != nil {
				debuglog.DebugLog("checkURL: Failed to fetch subscription %d/%d (took %v): %v", i+1, len(inputLines), fetchDuration, err)
				errors = append(errors, fmt.Sprintf("Failed to fetch %s: %v", line, err))
				continue
			}

			// Validate response size
			if err := ValidateHTTPResponseSize(int64(len(content))); err != nil {
				debuglog.DebugLog("checkURL: Subscription response too large %d/%d: %v", i+1, len(inputLines), err)
				errors = append(errors, fmt.Sprintf("Subscription response too large: %v", err))
				continue
			}

			debuglog.DebugLog("checkURL: Fetched subscription %d/%d: %d bytes in %v", i+1, len(inputLines), len(content), fetchDuration)

			// Check subscription content
			parseStartTime := time.Now()
			subLines := strings.Split(string(content), "\n")
			debuglog.DebugLog("checkURL: Parsing subscription %d/%d: %d lines", i+1, len(inputLines), len(subLines))
			validInSub := 0
			for _, subLine := range subLines {
				subLine = strings.TrimSpace(subLine)
				if subLine != "" && subscription.IsDirectLink(subLine) {
					validInSub++
					totalValid++
					if len(previewLines) < wizardutils.MaxPreviewLines {
						previewLines = append(previewLines, fmt.Sprintf("%d. %s", totalValid, subLine))
					}
				}
			}
			parseDuration := time.Since(parseStartTime)
			debuglog.DebugLog("checkURL: Parsed subscription %d/%d: %d valid links in %v (line processing took %v total)",
				i+1, len(inputLines), validInSub, parseDuration, time.Since(lineStartTime))
			if validInSub == 0 {
				errors = append(errors, fmt.Sprintf("Subscription %s contains no valid proxy links", line))
			}
		} else if subscription.IsDirectLink(line) {
			// Validate URI before parsing
			if err := ValidateURI(line); err != nil {
				debuglog.DebugLog("checkURL: Invalid URI format %d/%d: %v", i+1, len(inputLines), err)
				errors = append(errors, fmt.Sprintf("Invalid URI format: %v", err))
				continue
			}

			// This is a direct link - validate parsing
			parseStartTime := time.Now()
			debuglog.DebugLog("checkURL: Parsing direct link %d/%d", i+1, len(inputLines))
			_, err := subscription.ParseNode(line, nil)
			parseDuration := time.Since(parseStartTime)
			if err != nil {
				debuglog.DebugLog("checkURL: Invalid direct link %d/%d (took %v): %v", i+1, len(inputLines), parseDuration, err)
				errors = append(errors, fmt.Sprintf("Invalid direct link: %v", err))
			} else {
				totalValid++
				debuglog.DebugLog("checkURL: Valid direct link %d/%d (took %v)", i+1, len(inputLines), parseDuration)
				if len(previewLines) < wizardutils.MaxPreviewLines {
					previewLines = append(previewLines, fmt.Sprintf("%d. %s", totalValid, line))
				}
			}
		} else {
			debuglog.DebugLog("checkURL: Unknown format for line %d/%d: %s", i+1, len(inputLines), line)
			errors = append(errors, fmt.Sprintf("Unknown format: %s", line))
		}
	}

	debuglog.DebugLog("checkURL: Processed all lines (total valid: %d, errors: %d)", totalValid, len(errors))

	if totalValid == 0 {
		errorMsg := "❌ No valid proxy links found"
		if len(errors) > 0 {
			errorMsg += "\n" + strings.Join(errors[:min(3, len(errors))], "\n")
		}
		updater.UpdateURLStatus(errorMsg)
	} else {
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
	updater.UpdateCheckURLButtonText("Check")
	updater.UpdateCheckURLProgress(-1)
	return nil
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
		log.Printf("parseAndPreview: error applying URL to ParserConfig: %v", err)
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

	if input == "" {
		debuglog.DebugLog("applyURLToParserConfig: input is empty, returning early")
		return fmt.Errorf("input is empty")
	}
	text := strings.TrimSpace(model.ParserConfigJSON)
	if text == "" {
		debuglog.DebugLog("applyURLToParserConfig: ParserConfigJSON text is empty, returning early")
		return fmt.Errorf("parserConfigJSON is empty")
	}

	parseStartTime := time.Now()
	var parserConfig config.ParserConfig
	if err := json.Unmarshal([]byte(text), &parserConfig); err != nil {
		timing.LogTiming("parse ParserConfig", time.Since(parseStartTime))
		debuglog.DebugLog("applyURLToParserConfig: Failed to parse ParserConfig: %v", err)
		return fmt.Errorf("failed to parse ParserConfig: %w", err)
	}
	timing.LogTiming("parse ParserConfig", time.Since(parseStartTime))
	debuglog.DebugLog("applyURLToParserConfig: Parsed ParserConfig (outbounds: %d)",
		len(parserConfig.ParserConfig.Outbounds))

	// Separate subscriptions and direct links
	splitStartTime := time.Now()
	lines := strings.Split(input, "\n")
	debuglog.DebugLog("applyURLToParserConfig: Split input into %d lines", len(lines))
	subscriptions := make([]string, 0)
	connections := make([]string, 0)

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

	// Preserve existing local outbounds, tag_prefix, and tag_postfix for each source
	// Use source URL as key for matching
	existingOutboundsMap := make(map[string][]config.OutboundConfig)
	existingTagPrefixMap := make(map[string]string)
	existingTagPostfixMap := make(map[string]string)
	// Preserve all ProxySource entries without source (with connections)
	existingConnectionsProxies := make([]config.ProxySource, 0)
	for _, existingProxy := range parserConfig.ParserConfig.Proxies {
		if existingProxy.Source != "" {
			existingOutboundsMap[existingProxy.Source] = existingProxy.Outbounds
			if existingProxy.TagPrefix != "" {
				existingTagPrefixMap[existingProxy.Source] = existingProxy.TagPrefix
			}
			if existingProxy.TagPostfix != "" {
				existingTagPostfixMap[existingProxy.Source] = existingProxy.TagPostfix
			}
		} else if len(existingProxy.Connections) > 0 {
			// Preserve all ProxySource entries with connections but no source
			existingConnectionsProxies = append(existingConnectionsProxies, existingProxy)
		}
	}

	// Create new ProxySource array
	newProxies := make([]config.ProxySource, 0)

	// Automatically add tag_prefix with sequential number only if there are multiple subscriptions
	autoAddPrefix := len(subscriptions) > 1

	// Helper function to restore tag_prefix and tag_postfix
	restoreTagPrefixAndPostfix := func(proxySource *config.ProxySource, lookupKey string, logContext string) {
		if existingTagPrefix, ok := existingTagPrefixMap[lookupKey]; ok {
			proxySource.TagPrefix = existingTagPrefix
			debuglog.DebugLog("applyURLToParserConfig: Restored tag_prefix '%s' for %s", existingTagPrefix, logContext)
		}
		if existingTagPostfix, ok := existingTagPostfixMap[lookupKey]; ok {
			proxySource.TagPostfix = existingTagPostfix
			debuglog.DebugLog("applyURLToParserConfig: Restored tag_postfix '%s' for %s", existingTagPostfix, logContext)
		}
	}

	// Create separate ProxySource for each subscription
	for idx, sub := range subscriptions {
		proxySource := config.ProxySource{
			Source: sub,
		}
		// Restore local outbounds if they existed for this source
		if existingOutbounds, ok := existingOutboundsMap[sub]; ok {
			proxySource.Outbounds = existingOutbounds
			debuglog.DebugLog("applyURLToParserConfig: Restored %d local outbounds for subscription: %s", len(existingOutbounds), sub)
		}
		// Restore tag_prefix and tag_postfix
		restoreTagPrefixAndPostfix(&proxySource, sub, fmt.Sprintf("subscription: %s", sub))
		// Automatically add tag_prefix if not restored and auto-add is enabled
		if proxySource.TagPrefix == "" && autoAddPrefix {
			proxySource.TagPrefix = GenerateTagPrefix(idx + 1)
			debuglog.DebugLog("applyURLToParserConfig: Added automatic tag_prefix '%s' for subscription: %s", proxySource.TagPrefix, sub)
		}
		newProxies = append(newProxies, proxySource)
	}

	// Helper function to check if two connection arrays match (order-independent)
	connectionsMatch := func(conn1, conn2 []string) bool {
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

	// If there are new direct links from input, try to match with existing or create new
	if len(connections) > 0 {
		// Try to match with existing connections proxy by comparing connections
		matched := false
		for _, existingConnectionsProxy := range existingConnectionsProxies {
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
				matched = true
				debuglog.DebugLog("applyURLToParserConfig: Matched existing connections proxy, preserved tag_prefix '%s', tag_postfix '%s', tag_mask '%s'",
					matchedProxy.TagPrefix, matchedProxy.TagPostfix, matchedProxy.TagMask)
				break
			}
		}
		if !matched {
			// New connections - add as new ProxySource
			proxySource := config.ProxySource{
				Connections: connections,
			}
			debuglog.DebugLog("applyURLToParserConfig: Adding new ProxySource with %d connections", len(connections))
			newProxies = append(newProxies, proxySource)
		}
		// Don't preserve other existing ProxySource entries with connections - user removed them
		debuglog.DebugLog("applyURLToParserConfig: Not preserving %d other connection ProxySources (user removed them)", len(existingConnectionsProxies)-1)
	}
	// If user removed all connections (len(connections) == 0), don't add any connection ProxySources
	// This allows user to clear connections by deleting them from GUI

	// If there are no subscriptions or connections, create empty array
	if len(newProxies) == 0 {
		newProxies = []config.ProxySource{{}}
	}

	// Update proxies array
	parserConfig.ParserConfig.Proxies = newProxies
	debuglog.DebugLog("applyURLToParserConfig: Created %d proxy sources (%d subscriptions, %d with connections)",
		len(newProxies), len(subscriptions), len(connections))

	serializeStartTime := time.Now()
	serialized, err := SerializeParserConfig(&parserConfig)
	if err != nil {
		timing.LogTiming("serialize ParserConfig", time.Since(serializeStartTime))
		debuglog.DebugLog("applyURLToParserConfig: Failed to serialize ParserConfig: %v", err)
		return fmt.Errorf("failed to serialize ParserConfig: %w", err)
	}
	timing.LogTiming("serialize ParserConfig", time.Since(serializeStartTime))
	debuglog.DebugLog("applyURLToParserConfig: Serialized ParserConfig (result length: %d bytes, outbounds before: %d)",
		len(serialized), len(parserConfig.ParserConfig.Outbounds))

	updater.UpdateParserConfig(serialized)
	model.ParserConfig = &parserConfig
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
