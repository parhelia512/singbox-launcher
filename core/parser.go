package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedNode represents a parsed proxy node
type ParsedNode struct {
	Tag      string
	Scheme   string
	Server   string
	Port     int
	UUID     string
	Flow     string
	Label    string
	Comment  string
	Query    url.Values
	Outbound map[string]interface{}
}

// updateParserProgress safely calls UpdateParserProgressFunc if it's not nil
func updateParserProgress(ac *AppController, progress float64, status string) {
	if ac.UpdateParserProgressFunc != nil {
		ac.UpdateParserProgressFunc(progress, status)
	}
}

// ParseSubscriptionContent parses subscription content and handles duplicate tags
// Parameters:
//   - content: raw subscription content (can be base64 encoded or plain text)
//   - skipFilters: filters to skip certain nodes
//   - logPrefix: prefix for log messages (e.g., "Parser" or "ConfigWizard")
//   - sourceURL: URL of the subscription source (for better error logging, can be empty)
//
// Returns:
//   - []*ParsedNode: list of parsed nodes with unique tags
//   - map[string]int: tag counts for statistics (originalTag -> count)
//   - map[string]string: reverse mapping (renamedTag -> originalTag) for performance
//   - error: parsing error (if content is empty)
func ParseSubscriptionContent(content []byte, skipFilters []map[string]string, logPrefix string, sourceURL string) ([]*ParsedNode, map[string]int, map[string]string, error) {
	allNodes := make([]*ParsedNode, 0)
	tagCounts := make(map[string]int)
	tagReverseMap := make(map[string]string) // renamedTag -> originalTag for fast lookup

	if logPrefix == "" {
		logPrefix = "Parser"
	}

	// Check if content is empty (advanced check from parser)
	if len(content) == 0 {
		if sourceURL != "" {
			return nil, nil, nil, fmt.Errorf("subscription content is empty from %s", sourceURL)
		}
		return nil, nil, nil, fmt.Errorf("subscription content is empty")
	}

	log.Printf("%s: Initializing tag deduplication tracker", logPrefix)

	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		node, err := ParseNode(line, skipFilters)
		if err != nil {
			if sourceURL != "" {
				log.Printf("%s: Warning: Failed to parse node from %s: %v", logPrefix, sourceURL, err)
			} else {
				log.Printf("%s: Warning: Failed to parse node: %v", logPrefix, err)
			}
			continue
		}

		if node != nil {
			// Store original tag before any renaming
			originalTag := node.Tag

			// Make tag unique if it already exists
			// Check if tag already exists before incrementing
			if tagCounts[originalTag] > 0 {
				// Tag already exists, make it unique
				tagCounts[originalTag]++
				node.Tag = fmt.Sprintf("%s-%d", originalTag, tagCounts[originalTag])
				// Store reverse mapping for fast lookup
				tagReverseMap[node.Tag] = originalTag
				log.Printf("%s: Duplicate tag '%s' found (occurrence #%d), renamed to '%s'", logPrefix, originalTag, tagCounts[originalTag], node.Tag)
			} else {
				// First occurrence, just mark it
				tagCounts[originalTag] = 1
				// Store mapping for original tag (maps to itself)
				tagReverseMap[originalTag] = originalTag
				log.Printf("%s: First occurrence of tag '%s'", logPrefix, originalTag)
			}

			allNodes = append(allNodes, node)
		}
	}

	// Log statistics about duplicates
	duplicateCount := 0
	for tag, count := range tagCounts {
		if count > 1 {
			duplicateCount++
			log.Printf("%s: Tag '%s' had %d occurrences (renamed duplicates)", logPrefix, tag, count)
		}
	}
	if duplicateCount > 0 {
		log.Printf("%s: Found %d tags with duplicates, all have been renamed", logPrefix, duplicateCount)
	} else {
		log.Printf("%s: No duplicate tags found, all tags are unique", logPrefix)
	}

	return allNodes, tagCounts, tagReverseMap, nil
}

// UpdateConfigFromSubscriptions updates config.json by fetching subscriptions and parsing nodes
func UpdateConfigFromSubscriptions(ac *AppController) error {
	log.Printf("Parser: Starting configuration update...")
	log.Printf("Parser: Config file path: %s", ac.ConfigPath)

	// Step 1: Extract configuration
	config, err := ExtractParcerConfig(ac.ConfigPath)
	if err != nil {
		updateParserProgress(ac, -1, fmt.Sprintf("Error: %v", err))
		return fmt.Errorf("failed to extract parser config: %w", err)
	}

	// Update progress: Step 1 completed
	updateParserProgress(ac, 5, "Parsed ParcerConfig block")

	// Wait 0.1 sec before showing connection message
	time.Sleep(100 * time.Millisecond)

	// Show connection message
	updateParserProgress(ac, 5, "Connecting...")

	// Small delay before starting to fetch subscriptions
	time.Sleep(100 * time.Millisecond)

	// Step 2: Load and parse subscriptions
	allNodes := make([]*ParsedNode, 0)
	successfulSubscriptions := 0
	totalSubscriptions := len(config.ParserConfig.Proxies)

	// Map to track unique tags and their counts
	tagCounts := make(map[string]int)
	log.Printf("Parser: Initializing tag deduplication tracker")

	updateParserProgress(ac, 20, fmt.Sprintf("Loading subscriptions (0/%d)...", totalSubscriptions))

	for i, proxySource := range config.ParserConfig.Proxies {
		log.Printf("Parser: Downloading subscription %d/%d from: %s", i+1, totalSubscriptions, proxySource.Source)

		// Update progress: downloading subscription
		progress := 20 + float64(i)*50.0/float64(totalSubscriptions)
		updateParserProgress(ac, progress, fmt.Sprintf("Downloading subscription %d/%d: %s", i+1, totalSubscriptions, proxySource.Source))

		content, err := FetchSubscription(proxySource.Source)
		if err != nil {
			log.Printf("Parser: Error: Failed to fetch subscription from %s: %v", proxySource.Source, err)
			continue
		}

		// Early check for empty content (optimization - skip parsing if empty)
		if len(content) == 0 {
			log.Printf("Parser: Warning: Subscription from %s returned empty content", proxySource.Source)
			continue
		}

		// Update progress: parsing subscription
		progress = 20 + float64(i)*50.0/float64(totalSubscriptions) + 10.0/float64(totalSubscriptions)
		updateParserProgress(ac, progress, fmt.Sprintf("Parsing subscription %d/%d: %s", i+1, totalSubscriptions, proxySource.Source))

		// Parse subscription content using shared function
		nodesFromThisSubscription, subscriptionTagCounts, subscriptionTagReverseMap, err := ParseSubscriptionContent(content, proxySource.Skip, "Parser", proxySource.Source)
		if err != nil {
			log.Printf("Parser: Error parsing subscription from %s: %v", proxySource.Source, err)
			continue
		}

		// Merge tag counts from this subscription into global tagCounts
		// This ensures duplicates are handled globally across all subscriptions
		// Use reverse mapping for fast lookup of original tags
		for _, node := range nodesFromThisSubscription {
			// Use reverse mapping for O(1) lookup instead of O(n) search
			originalTag, found := subscriptionTagReverseMap[node.Tag]
			if !found {
				// Fallback: extract original tag by removing suffix (e.g., "Tag-2" -> "Tag")
				originalTag = node.Tag
				if idx := strings.LastIndex(originalTag, "-"); idx > 0 {
					// Check if suffix is a number
					if _, err := strconv.Atoi(originalTag[idx+1:]); err == nil {
						originalTag = originalTag[:idx]
					}
				}
				log.Printf("Parser: Warning: Could not find original tag for '%s' in reverse map, using extracted '%s'", node.Tag, originalTag)
			}

			// Now check if this original tag exists globally
			if tagCounts[originalTag] > 0 {
				// Tag already exists from previous subscription, rename it
				tagCounts[originalTag]++
				node.Tag = fmt.Sprintf("%s-%d", originalTag, tagCounts[originalTag])
				log.Printf("Parser: Global duplicate tag '%s' found (occurrence #%d), renamed to '%s'", originalTag, tagCounts[originalTag], node.Tag)
			} else {
				// First occurrence of this tag globally, mark it
				// Use the count from subscription (which may be > 1 if there were duplicates within subscription)
				if count, exists := subscriptionTagCounts[originalTag]; exists {
					tagCounts[originalTag] = count
				} else {
					tagCounts[originalTag] = 1
				}
			}
		}

		allNodes = append(allNodes, nodesFromThisSubscription...)
		nodesFromThisSubscriptionCount := len(nodesFromThisSubscription)

		if nodesFromThisSubscriptionCount > 0 {
			successfulSubscriptions++
			log.Printf("Parser: Successfully parsed %d nodes from %s", nodesFromThisSubscriptionCount, proxySource.Source)
		} else {
			log.Printf("Parser: Warning: No valid nodes parsed from %s", proxySource.Source)
		}

		// Update progress after parsing subscription
		progress = 20 + float64(i+1)*50.0/float64(totalSubscriptions)
		updateParserProgress(ac, progress, fmt.Sprintf("Processed subscriptions: %d/%d, nodes: %d", i+1, totalSubscriptions, len(allNodes)))
	}

	// Check if we successfully loaded at least one subscription
	if successfulSubscriptions == 0 {
		updateParserProgress(ac, -1, "Error: failed to load any subscriptions")
		return fmt.Errorf("failed to load any subscriptions - check internet connection and subscription URLs")
	}

	log.Printf("Parser: Parsed %d nodes from subscriptions", len(allNodes))

	// Log global statistics about duplicates across all subscriptions
	globalDuplicateCount := 0
	for tag, count := range tagCounts {
		if count > 1 {
			globalDuplicateCount++
			log.Printf("Parser: Global tag '%s' had %d total occurrences across all subscriptions (renamed duplicates)", tag, count)
		}
	}
	if globalDuplicateCount > 0 {
		log.Printf("Parser: Found %d tags with duplicates across all subscriptions, all have been renamed", globalDuplicateCount)
	} else {
		log.Printf("Parser: No duplicate tags found across all subscriptions, all tags are unique")
	}

	updateParserProgress(ac, 70, fmt.Sprintf("Processed nodes: %d. Generating JSON...", len(allNodes)))

	// Step 3: Generate JSON for nodes and selectors using shared function
	updateParserProgress(ac, 75, "Generating JSON for nodes...")

	selectorsJSON, err := GenerateOutboundsJSON(allNodes, config.ParserConfig.Outbounds)
	if err != nil {
		updateParserProgress(ac, -1, fmt.Sprintf("Error: %v", err))
		return fmt.Errorf("failed to generate outbounds JSON: %w", err)
	}

	updateParserProgress(ac, 85, "Generated JSON successfully")

	// Step 4: Write to file
	updateParserProgress(ac, 90, "Writing to config file...")

	content := strings.Join(selectorsJSON, "\n")
	if err := writeToConfig(ac.ConfigPath, content); err != nil {
		updateParserProgress(ac, -1, fmt.Sprintf("Write error: %v", err))
		return fmt.Errorf("failed to write to config: %w", err)
	}

	log.Printf("Parser: Done! File %s successfully updated.", ac.ConfigPath)

	updateParserProgress(ac, 100, "Configuration updated successfully!")

	return nil
}

// ParseNode parses a single node URI and applies skip filters (exported for use in UI)
func ParseNode(uri string, skipFilters []map[string]string) (*ParsedNode, error) {
	// Determine scheme
	scheme := ""
	uriToParse := uri

	// Handle VMess base64 format
	if strings.HasPrefix(uri, "vmess://") {
		scheme = "vmess"
		// VMess might be in base64 format, decode if needed
		base64Part := strings.TrimPrefix(uri, "vmess://")
		decoded, err := DecodeSubscriptionContent([]byte(base64Part))
		if err == nil && len(decoded) > 0 {
			// Try to parse as JSON VMess config
			var vmessConfig map[string]interface{}
			if err := json.Unmarshal(decoded, &vmessConfig); err == nil {
				// Convert VMess JSON to URI format (simplified)
				// For now, we'll handle it as a special case
				return parseVMessJSON(vmessConfig, skipFilters)
			}
		}
	} else if strings.HasPrefix(uri, "vless://") {
		scheme = "vless"
	} else if strings.HasPrefix(uri, "trojan://") {
		scheme = "trojan"
	} else if strings.HasPrefix(uri, "ss://") {
		scheme = "ss"
	} else {
		return nil, fmt.Errorf("unsupported scheme")
	}

	// Parse URI
	parsedURL, err := url.Parse(uriToParse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI: %w", err)
	}

	// Extract components
	node := &ParsedNode{
		Scheme: scheme,
		Server: parsedURL.Hostname(),
		Query:  parsedURL.Query(),
	}

	// Extract port
	port := parsedURL.Port()
	if port == "" {
		// Default ports
		switch scheme {
		case "vless", "vmess":
			node.Port = 443
		case "trojan":
			node.Port = 443
		case "ss":
			node.Port = 443
		}
	} else {
		if p, err := strconv.Atoi(port); err == nil {
			node.Port = p
		} else {
			node.Port = 443 // Fallback
		}
	}

	// Extract UUID/user
	if parsedURL.User != nil {
		node.UUID = parsedURL.User.Username()
	}

	// Extract fragment (label)
	node.Label = parsedURL.Fragment

	// For some formats, label might be in path or userinfo
	if node.Label == "" {
		// Try to extract from path (some formats use path for label)
		if parsedURL.Path != "" && parsedURL.Path != "/" {
			node.Label = strings.TrimPrefix(parsedURL.Path, "/")
		} else if parsedURL.User != nil {
			// Some formats encode label in username
			node.Label = parsedURL.User.Username()
		}
	}

	// Extract tag and comment from label
	node.Tag, node.Comment = extractTagAndComment(node.Label)

	// Normalize flag
	node.Tag = normalizeFlagTag(node.Tag)

	// Extract flow
	node.Flow = parsedURL.Query().Get("flow")

	// Apply skip filters
	if shouldSkipNode(node, skipFilters) {
		return nil, nil // Node should be skipped
	}

	// Build outbound JSON based on scheme
	node.Outbound = buildOutbound(node)

	return node, nil
}

// extractTagAndComment extracts tag and comment from label
func extractTagAndComment(label string) (tag, comment string) {
	parts := strings.Split(label, "|")
	if len(parts) > 1 {
		tag = parts[0]
		comment = label // Full label as comment
	} else {
		tag = label
		comment = label
	}
	return strings.TrimSpace(tag), strings.TrimSpace(comment)
}

// normalizeFlagTag normalizes flag tags (e.g., 🇪🇳 -> 🇬🇧)
func normalizeFlagTag(tag string) string {
	return strings.ReplaceAll(tag, "🇪🇳", "🇬🇧")
}

// shouldSkipNode checks if node should be skipped based on filters
func shouldSkipNode(node *ParsedNode, skipFilters []map[string]string) bool {
	// OR between filters: if at least one matches, skip
	for _, filter := range skipFilters {
		// AND inside filter: all keys must match
		allKeysMatch := true
		for key, pattern := range filter {
			value := getNodeValue(node, key)
			if !matchesPattern(value, pattern) {
				allKeysMatch = false
				break
			}
		}
		if allKeysMatch {
			return true // Skip node
		}
	}
	return false // Don't skip
}

// getNodeValue extracts value from node by key
func getNodeValue(node *ParsedNode, key string) string {
	switch key {
	case "tag":
		return node.Tag
	case "host":
		return node.Server
	case "label":
		return node.Label
	case "scheme":
		return node.Scheme
	case "fragment":
		return node.Label // fragment == label
	case "comment":
		return node.Comment
	default:
		return ""
	}
}

// matchesPattern checks if value matches pattern
func matchesPattern(value, pattern string) bool {
	// Negation literal: !literal
	if strings.HasPrefix(pattern, "!") && !strings.HasPrefix(pattern, "!/") {
		literal := strings.TrimPrefix(pattern, "!")
		return value != literal
	}

	// Negation regex: !/regex/i
	if strings.HasPrefix(pattern, "!/") && strings.HasSuffix(pattern, "/i") {
		regexStr := strings.TrimPrefix(pattern, "!/")
		regexStr = strings.TrimSuffix(regexStr, "/i")
		re, err := regexp.Compile("(?i)" + regexStr)
		if err != nil {
			log.Printf("Parser: Invalid regex pattern %s: %v", pattern, err)
			return false
		}
		return !re.MatchString(value)
	}

	// Regex: /regex/i
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/i") {
		regexStr := strings.TrimPrefix(pattern, "/")
		regexStr = strings.TrimSuffix(regexStr, "/i")
		re, err := regexp.Compile("(?i)" + regexStr)
		if err != nil {
			log.Printf("Parser: Invalid regex pattern %s: %v", pattern, err)
			return false
		}
		return re.MatchString(value)
	}

	// Literal match
	return value == pattern
}

// buildOutbound builds outbound JSON structure for node
func buildOutbound(node *ParsedNode) map[string]interface{} {
	outbound := make(map[string]interface{})
	outbound["tag"] = node.Tag
	outbound["type"] = node.Scheme
	outbound["server"] = node.Server
	outbound["server_port"] = node.Port

	if node.Scheme == "vless" {
		outbound["uuid"] = node.UUID
		if node.Flow != "" {
			outbound["flow"] = node.Flow
		}

		// Build TLS structure with correct field order
		// Store TLS data for ordered serialization
		sni := node.Query.Get("sni")
		if sni == "" {
			sni = node.Server // Fallback to server hostname
		}
		fp := node.Query.Get("fp")
		if fp == "" {
			fp = "random"
		}
		pbk := node.Query.Get("pbk")
		sid := node.Query.Get("sid")

		// Store TLS components for ordered serialization
		tlsData := map[string]interface{}{
			"enabled":     true,
			"server_name": sni,
			"utls": map[string]interface{}{
				"enabled":     true,
				"fingerprint": fp,
			},
		}

		// Only add Reality section if public_key is not empty
		// Sing-box requires valid public_key when Reality is enabled
		if pbk != "" {
			tlsData["reality"] = map[string]interface{}{
				"enabled":    true,
				"public_key": pbk,
				"short_id":   sid,
			}
		}

		outbound["tls"] = tlsData
	} else if node.Scheme == "vmess" {
		outbound["uuid"] = node.UUID
		// Add VMess-specific fields if needed
	} else if node.Scheme == "trojan" {
		outbound["password"] = node.UUID
		// Add Trojan-specific fields if needed
	} else if node.Scheme == "ss" {
		// Add Shadowsocks-specific fields if needed
	}

	return outbound
}

// GenerateNodeJSON generates JSON string for a node with correct field order (exported for use in UI)
func GenerateNodeJSON(node *ParsedNode) (string, error) {
	// Build JSON with correct field order
	var parts []string

	// 1. tag
	parts = append(parts, fmt.Sprintf(`"tag":%q`, node.Tag))

	// 2. type
	parts = append(parts, fmt.Sprintf(`"type":%q`, node.Scheme))

	// 3. server
	parts = append(parts, fmt.Sprintf(`"server":%q`, node.Server))

	// 4. server_port
	parts = append(parts, fmt.Sprintf(`"server_port":%d`, node.Port))

	// 5. uuid (for vless/vmess) or password (for trojan)
	if node.Scheme == "vless" || node.Scheme == "vmess" {
		parts = append(parts, fmt.Sprintf(`"uuid":%q`, node.UUID))
	} else if node.Scheme == "trojan" {
		parts = append(parts, fmt.Sprintf(`"password":%q`, node.UUID))
	}

	// 6. flow (if present)
	if node.Flow != "" {
		parts = append(parts, fmt.Sprintf(`"flow":%q`, node.Flow))
	}

	// 7. tls (if present) - with correct field order
	if tlsData, ok := node.Outbound["tls"].(map[string]interface{}); ok {
		var tlsParts []string

		// enabled
		if enabled, ok := tlsData["enabled"].(bool); ok {
			tlsParts = append(tlsParts, fmt.Sprintf(`"enabled":%v`, enabled))
		}

		// server_name
		if serverName, ok := tlsData["server_name"].(string); ok {
			tlsParts = append(tlsParts, fmt.Sprintf(`"server_name":%q`, serverName))
		}

		// utls
		if utls, ok := tlsData["utls"].(map[string]interface{}); ok {
			var utlsParts []string
			if utlsEnabled, ok := utls["enabled"].(bool); ok {
				utlsParts = append(utlsParts, fmt.Sprintf(`"enabled":%v`, utlsEnabled))
			}
			if fingerprint, ok := utls["fingerprint"].(string); ok {
				utlsParts = append(utlsParts, fmt.Sprintf(`"fingerprint":%q`, fingerprint))
			}
			utlsJSON := "{" + strings.Join(utlsParts, ",") + "}"
			tlsParts = append(tlsParts, fmt.Sprintf(`"utls":%s`, utlsJSON))
		}

		// reality
		if reality, ok := tlsData["reality"].(map[string]interface{}); ok {
			var realityParts []string
			if realityEnabled, ok := reality["enabled"].(bool); ok {
				realityParts = append(realityParts, fmt.Sprintf(`"enabled":%v`, realityEnabled))
			}
			if publicKey, ok := reality["public_key"].(string); ok {
				realityParts = append(realityParts, fmt.Sprintf(`"public_key":%q`, publicKey))
			}
			if shortID, ok := reality["short_id"].(string); ok {
				realityParts = append(realityParts, fmt.Sprintf(`"short_id":%q`, shortID))
			}
			realityJSON := "{" + strings.Join(realityParts, ",") + "}"
			tlsParts = append(tlsParts, fmt.Sprintf(`"reality":%s`, realityJSON))
		}

		tlsJSON := "{" + strings.Join(tlsParts, ",") + "}"
		parts = append(parts, fmt.Sprintf(`"tls":%s`, tlsJSON))
	}

	// Build final JSON
	jsonStr := "{" + strings.Join(parts, ",") + "}"
	return fmt.Sprintf("\t// %s\n\t%s,", node.Label, jsonStr), nil
}

// GenerateSelector generates JSON string for a selector (exported for use in UI)
func GenerateSelector(allNodes []*ParsedNode, outboundConfig OutboundConfig) (string, error) {
	// Filter nodes based on outbounds.proxies
	filteredNodes := filterNodesForSelector(allNodes, outboundConfig.Outbounds.Proxies)

	if len(filteredNodes) == 0 {
		log.Printf("Parser: No nodes matched filter for selector %s", outboundConfig.Tag)
		return "", nil
	}

	// Build outbounds list with unique tags
	outboundsList := make([]string, 0)
	seenTags := make(map[string]bool)
	duplicateCountInSelector := 0

	// Add addOutbounds first
	if len(outboundConfig.Outbounds.AddOutbounds) > 0 {
		log.Printf("Parser: Adding %d addOutbounds to selector '%s'", len(outboundConfig.Outbounds.AddOutbounds), outboundConfig.Tag)
		for _, tag := range outboundConfig.Outbounds.AddOutbounds {
			if !seenTags[tag] {
				outboundsList = append(outboundsList, tag)
				seenTags[tag] = true
			} else {
				duplicateCountInSelector++
				log.Printf("Parser: Skipping duplicate tag '%s' in addOutbounds for selector '%s'", tag, outboundConfig.Tag)
			}
		}
	}

	// Add filtered node tags (without duplicates)
	log.Printf("Parser: Processing %d filtered nodes for selector '%s'", len(filteredNodes), outboundConfig.Tag)
	for _, node := range filteredNodes {
		if !seenTags[node.Tag] {
			outboundsList = append(outboundsList, node.Tag)
			seenTags[node.Tag] = true
		} else {
			duplicateCountInSelector++
			log.Printf("Parser: Skipping duplicate tag '%s' in filtered nodes for selector '%s'", node.Tag, outboundConfig.Tag)
		}
	}

	if duplicateCountInSelector > 0 {
		log.Printf("Parser: Removed %d duplicate tags from selector '%s' outbounds list", duplicateCountInSelector, outboundConfig.Tag)
	}
	log.Printf("Parser: Selector '%s' will have %d unique outbounds", outboundConfig.Tag, len(outboundsList))

	// Determine default
	defaultTag := ""
	if len(outboundConfig.Outbounds.PreferredDefault) > 0 {
		// Find first node matching preferredDefault filter
		preferredFilter := convertFilterToStringMap(outboundConfig.Outbounds.PreferredDefault)
		for _, node := range filteredNodes {
			if matchesFilter(node, preferredFilter) {
				defaultTag = node.Tag
				break
			}
		}
	}

	// If no preferredDefault match, use first node
	if defaultTag == "" && len(filteredNodes) > 0 {
		defaultTag = filteredNodes[0].Tag
	}

	// Build selector JSON with correct field order
	var parts []string

	// 1. tag
	parts = append(parts, fmt.Sprintf(`"tag":%q`, outboundConfig.Tag))

	// 2. type
	parts = append(parts, fmt.Sprintf(`"type":%q`, outboundConfig.Type))

	// 3. default (if present) - BEFORE outbounds
	if defaultTag != "" {
		parts = append(parts, fmt.Sprintf(`"default":%q`, defaultTag))
	}

	// 4. outbounds
	outboundsJSON, _ := json.Marshal(outboundsList)
	parts = append(parts, fmt.Sprintf(`"outbounds":%s`, string(outboundsJSON)))

	// 5. interrupt_exist_connections (if present)
	if val, ok := outboundConfig.Options["interrupt_exist_connections"]; ok {
		if boolVal, ok := val.(bool); ok {
			parts = append(parts, fmt.Sprintf(`"interrupt_exist_connections":%v`, boolVal))
		} else {
			valJSON, _ := json.Marshal(val)
			parts = append(parts, fmt.Sprintf(`"interrupt_exist_connections":%s`, string(valJSON)))
		}
	}

	// 6. Other options (in order they appear)
	for key, value := range outboundConfig.Options {
		if key != "interrupt_exist_connections" {
			valJSON, _ := json.Marshal(value)
			parts = append(parts, fmt.Sprintf(`%q:%s`, key, string(valJSON)))
		}
	}

	// Build final JSON
	jsonStr := "{" + strings.Join(parts, ",") + "}"

	// Add comment if present
	result := ""
	if outboundConfig.Comment != "" {
		result = fmt.Sprintf("\t// %s\n", outboundConfig.Comment)
	}
	result += fmt.Sprintf("\t%s,", jsonStr)

	return result, nil
}

// GenerateOutboundsJSON generates JSON strings for all nodes and selectors
// Parameters:
//   - allNodes: list of parsed nodes
//   - outboundConfigs: list of selector configurations
//
// Returns:
//   - []string: JSON strings for nodes and selectors (ready to write to config)
//   - error: error if no nodes or generation fails
func GenerateOutboundsJSON(allNodes []*ParsedNode, outboundConfigs []OutboundConfig) ([]string, error) {
	// Check if we have any nodes before proceeding
	if len(allNodes) == 0 {
		return nil, fmt.Errorf("no nodes found - cannot generate outbounds")
	}

	selectorsJSON := make([]string, 0)

	// First, generate JSON for all nodes
	for _, node := range allNodes {
		nodeJSON, err := GenerateNodeJSON(node)
		if err != nil {
			log.Printf("GenerateOutboundsJSON: Warning: Failed to generate JSON for node %s: %v", node.Tag, err)
			continue
		}
		selectorsJSON = append(selectorsJSON, nodeJSON)
	}

	// Check if we have any node JSON before generating selectors
	if len(selectorsJSON) == 0 {
		return nil, fmt.Errorf("failed to generate JSON for any nodes")
	}

	// Then, generate selectors
	for _, outboundConfig := range outboundConfigs {
		selectorJSON, err := GenerateSelector(allNodes, outboundConfig)
		if err != nil {
			log.Printf("GenerateOutboundsJSON: Warning: Failed to generate selector %s: %v", outboundConfig.Tag, err)
			continue
		}
		if selectorJSON != "" {
			selectorsJSON = append(selectorsJSON, selectorJSON)
		}
	}

	// Final check: ensure we have content
	if len(selectorsJSON) == 0 {
		return nil, fmt.Errorf("no content generated - cannot generate empty result")
	}

	return selectorsJSON, nil
}

// filterNodesForSelector filters nodes based on outbounds.proxies filter
// Filter can be a single object (AND between keys) or array of objects (OR between objects, AND inside)
func filterNodesForSelector(allNodes []*ParsedNode, filter interface{}) []*ParsedNode {
	if filter == nil {
		return allNodes // No filter, return all nodes
	}

	filtered := make([]*ParsedNode, 0)

	// Check if filter is an array
	if filterArray, ok := filter.([]interface{}); ok {
		// OR between filter objects
		for _, node := range allNodes {
			for _, filterObj := range filterArray {
				if filterMap, ok := filterObj.(map[string]interface{}); ok {
					filterStrMap := convertFilterToStringMap(filterMap)
					if matchesFilter(node, filterStrMap) {
						filtered = append(filtered, node)
						break // Node matched at least one filter, add it
					}
				}
			}
		}
	} else if filterMap, ok := filter.(map[string]interface{}); ok {
		// Single filter object (AND between keys)
		filterStrMap := convertFilterToStringMap(filterMap)
		for _, node := range allNodes {
			if matchesFilter(node, filterStrMap) {
				filtered = append(filtered, node)
			}
		}
	}

	return filtered
}

// parseVMessJSON parses VMess JSON configuration
func parseVMessJSON(vmessConfig map[string]interface{}, skipFilters []map[string]string) (*ParsedNode, error) {
	node := &ParsedNode{
		Scheme: "vmess",
		Query:  make(url.Values),
	}

	// Extract common fields
	if add, ok := vmessConfig["add"].(string); ok {
		node.Server = add
	}
	if port, ok := vmessConfig["port"].(float64); ok {
		node.Port = int(port)
	} else {
		node.Port = 443
	}
	if id, ok := vmessConfig["id"].(string); ok {
		node.UUID = id
	}
	if ps, ok := vmessConfig["ps"].(string); ok {
		node.Label = ps
		node.Tag, node.Comment = extractTagAndComment(ps)
		node.Tag = normalizeFlagTag(node.Tag)
	}

	// Extract TLS settings
	if tls, ok := vmessConfig["tls"].(string); ok && tls == "tls" {
		if sni, ok := vmessConfig["sni"].(string); ok {
			node.Query.Set("sni", sni)
		}
	}

	// Apply skip filters
	if shouldSkipNode(node, skipFilters) {
		return nil, nil
	}

	// Build outbound
	node.Outbound = buildOutbound(node)
	return node, nil
}

// convertFilterToStringMap converts filter map to string map
func convertFilterToStringMap(filter map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range filter {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}

// matchesFilter checks if node matches filter (AND between keys)
func matchesFilter(node *ParsedNode, filter map[string]string) bool {
	for key, pattern := range filter {
		value := getNodeValue(node, key)
		if !matchesPattern(value, pattern) {
			return false // At least one key doesn't match
		}
	}
	return true // All keys match
}

// writeToConfig writes content between @ParserSTART and @ParserEND markers
func writeToConfig(configPath string, content string) error {
	log.Printf("writeToConfig: Writing to file: %s", configPath)
	log.Printf("writeToConfig: Content length: %d bytes", len(content))

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("writeToConfig: Failed to read config file: %v", err)
		return fmt.Errorf("failed to read config file: %w", err)
	}

	log.Printf("writeToConfig: Read config file, size: %d bytes", len(data))
	configStr := string(data)

	// Find markers
	startMarker := "/** @ParserSTART */"
	endMarker := "/** @ParserEND */"

	startIdx := strings.Index(configStr, startMarker)
	endIdx := strings.Index(configStr, endMarker)

	log.Printf("writeToConfig: Start marker found at index: %d", startIdx)
	log.Printf("writeToConfig: End marker found at index: %d", endIdx)

	if startIdx == -1 || endIdx == -1 {
		log.Printf("writeToConfig: ERROR - Markers not found! Start: %d, End: %d", startIdx, endIdx)
		return fmt.Errorf("markers @ParserSTART or @ParserEND not found in config.json")
	}

	if endIdx <= startIdx {
		log.Printf("writeToConfig: ERROR - Invalid marker positions! Start: %d, End: %d", startIdx, endIdx)
		return fmt.Errorf("invalid marker positions")
	}

	// Build new content
	newContent := configStr[:startIdx+len(startMarker)] + "\n" + content + "\n" + configStr[endIdx:]
	log.Printf("writeToConfig: New content size: %d bytes", len(newContent))

	// Write to file
	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		log.Printf("writeToConfig: ERROR - Failed to write file: %v", err)
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Printf("writeToConfig: Successfully wrote to file: %s", configPath)
	return nil
}
