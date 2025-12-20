package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"singbox-launcher/core/parsers"
)

// DecodeSubscriptionContent is now in parsers package
// Import "singbox-launcher/core/parsers" to use it

// FetchSubscription fetches subscription content from URL and decodes it
// Returns decoded content and error if fetch or decode fails
func FetchSubscription(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), NetworkRequestTimeout)
	defer cancel()

	client := CreateHTTPClient(NetworkRequestTimeout)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid server detecting sing-box and returning JSON config
	req.Header.Set("User-Agent", SubscriptionUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		if IsNetworkError(err) {
			return nil, fmt.Errorf("network error: %s", GetNetworkErrorMessage(err))
		}
		return nil, fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription server returned status %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read subscription content: %w", err)
	}

	if len(content) == 0 {
		return nil, fmt.Errorf("subscription returned empty content")
	}

	// Log preview of raw content for debugging
	const previewLen = 200
	preview := string(content)
	if len(preview) > previewLen {
		preview = preview[:previewLen] + "..."
	}
	log.Printf("[DEBUG] FetchSubscription: Raw content preview (first %d bytes): %q", len(content), preview)

	// Use parsers.DecodeSubscriptionContent for decoding
	decoded, err := parsers.DecodeSubscriptionContent(content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode subscription content: %w", err)
	}

	return decoded, nil
}

// ParserConfig represents the configuration structure from @ParserConfig block
// Clean structure for version 3 (legacy versions are migrated automatically)
type ParserConfig struct {
	ParserConfig struct {
		Version   int              `json:"version,omitempty"`
		Proxies   []ProxySource    `json:"proxies"`
		Outbounds []OutboundConfig `json:"outbounds"`
		Parser    struct {
			Reload      string `json:"reload,omitempty"`       // Интервал автоматического обновления
			LastUpdated string `json:"last_updated,omitempty"` // Время последнего обновления (RFC3339, UTC)
		} `json:"parser,omitempty"`
	} `json:"ParserConfig"`
}

// ParserConfigVersion is the current version of ParserConfig format
const ParserConfigVersion = 4

// SubscriptionUserAgent is the User-Agent string used for fetching subscriptions
// Using neutral User-Agent to avoid server detecting sing-box and returning JSON config
const SubscriptionUserAgent = "SubscriptionParserClient"

// NormalizeParserConfig normalizes ParserConfig structure:
// - Ensures version is set to ParserConfigVersion
// - Sets default reload to "4h" if not specified
// - Optionally updates last_updated timestamp (if updateLastUpdated is true)
// Note: Migration is handled in ExtractParserConfig
// This function works with already-migrated clean ParserConfig
func NormalizeParserConfig(parserConfig *ParserConfig, updateLastUpdated bool) {
	if parserConfig == nil {
		return
	}

	// Ensure version is set to current version (always update to latest)
	parserConfig.ParserConfig.Version = ParserConfigVersion

	// Ensure parser object exists (create if missing)
	// Set default reload to "4h" if not specified
	if parserConfig.ParserConfig.Parser.Reload == "" {
		parserConfig.ParserConfig.Parser.Reload = "4h"
	}

	// Optionally update last_updated timestamp
	if updateLastUpdated {
		parserConfig.ParserConfig.Parser.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	}
}

// ProxySource represents a proxy subscription source
type ProxySource struct {
	Source      string              `json:"source,omitempty"`
	Connections []string            `json:"connections,omitempty"`
	Skip        []map[string]string `json:"skip,omitempty"`
	Outbounds   []OutboundConfig    `json:"outbounds,omitempty"`   // Local outbounds for this source (version 4)
	TagPrefix   string              `json:"tag_prefix,omitempty"`  // Prefix to add to all node tags from this source
	TagPostfix  string              `json:"tag_postfix,omitempty"` // Postfix to add to all node tags from this source
	TagMask     string              `json:"tag_mask,omitempty"`    // Mask to replace entire tag (ignores tag_prefix and tag_postfix if set)
}

// OutboundConfig represents an outbound selector configuration (version 3)
// Clean structure without legacy fields - used in main codebase
// WizardConfig represents the wizard configuration for outbounds
// Supports both old format ("wizard":"hide") and new format ("wizard":{"hide":true, "required":2})
type WizardConfig struct {
	Hide     bool `json:"hide,omitempty"`     // Hide outbound from wizard second tab
	Required int  `json:"required,omitempty"` // Optional: 0 or missing=ignore, 1=check presence only, >1=strict match from template
}

type OutboundConfig struct {
	Tag              string                 `json:"tag"`
	Type             string                 `json:"type"`
	Options          map[string]interface{} `json:"options,omitempty"`
	Filters          map[string]interface{} `json:"filters,omitempty"`
	AddOutbounds     []string               `json:"addOutbounds,omitempty"`
	PreferredDefault map[string]interface{} `json:"preferredDefault,omitempty"`
	Comment          string                 `json:"comment,omitempty"`
	Wizard           interface{}            `json:"wizard,omitempty"` // Supports both "hide" (string) and {"hide":true, "required":2} (object) for backward compatibility
}

// IsWizardHidden checks if outbound should be hidden from wizard
// Supports both old format ("wizard":"hide") and new format ("wizard":{"hide":true})
func (oc *OutboundConfig) IsWizardHidden() bool {
	if oc.Wizard == nil {
		return false
	}

	// Old format: "wizard":"hide"
	if wizardStr, ok := oc.Wizard.(string); ok {
		return wizardStr == "hide"
	}

	// New format: "wizard":{"hide":true, ...}
	if wizardMap, ok := oc.Wizard.(map[string]interface{}); ok {
		if hideVal, ok := wizardMap["hide"]; ok {
			if hideBool, ok := hideVal.(bool); ok {
				return hideBool
			}
		}
	}

	return false
}

// GetWizardRequired returns the required value from wizard config
// Only checks wizard.required from new format ("wizard": {"hide": true, "required": 2})
func (oc *OutboundConfig) GetWizardRequired() int {
	if oc.Wizard != nil {
		if wizardMap, ok := oc.Wizard.(map[string]interface{}); ok {
			if requiredVal, ok := wizardMap["required"]; ok {
				if requiredInt, ok := requiredVal.(float64); ok {
					return int(requiredInt)
				}
			}
		}
	}

	// No required field found - return 0 (ignore)
	return 0
}

// ExtractParserConfig extracts the @ParserConfig block from config.json
// Returns the parsed ParserConfig structure and error if extraction or parsing fails
// Uses ConfigMigrator for handling legacy versions and migrations
// Uses parsers.ExtractParserConfigBlock for regex parsing
func ExtractParserConfig(configPath string) (*ParserConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config.json: %w", err)
	}

	// Use parsers for regex extraction
	jsonContent, err := parsers.ExtractParserConfigBlock(data)
	if err != nil {
		return nil, err
	}

	// Extract version from JSON to check if migration is needed
	currentVersion := ExtractVersion(jsonContent)

	var parserConfig *ParserConfig

	// If version is already current, parse directly without migration
	if currentVersion == ParserConfigVersion {
		if err := json.Unmarshal([]byte(jsonContent), &parserConfig); err != nil {
			return nil, fmt.Errorf("failed to parse @ParserConfig JSON: %w", err)
		}
	} else {
		// Version needs migration or is 0 - use migrator (it will handle version 0 and check for too new versions)
		migrator := NewConfigMigrator()
		var err error
		parserConfig, err = migrator.MigrateRaw(jsonContent, currentVersion, ParserConfigVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to migrate config: %w", err)
		}
	}

	log.Printf("ExtractParserConfig: Successfully extracted @ParserConfig (version %d) with %d proxy sources and %d outbounds",
		parserConfig.ParserConfig.Version,
		len(parserConfig.ParserConfig.Proxies),
		len(parserConfig.ParserConfig.Outbounds))

	return parserConfig, nil
}
