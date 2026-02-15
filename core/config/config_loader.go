package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/muhammadmuzzammil1998/jsonc"
)

// GetSelectorGroupsFromConfig extracts selector group names from config.json
func GetSelectorGroupsFromConfig(configPath string) ([]string, string, error) {
	cleanData, err := getConfigJSON(configPath)
	if err != nil {
		return nil, "", err
	}
	var jsonData map[string]interface{}
	if err := json.Unmarshal(cleanData, &jsonData); err != nil {
		return nil, "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract selector groups from outbounds
	outbounds, ok := jsonData["outbounds"].([]interface{})
	if !ok {
		return []string{"proxy-out"}, "", nil
	}

	var selectorGroups []string
	var defaultSelector string

	// Get default from route.final
	if route, ok := jsonData["route"].(map[string]interface{}); ok {
		if final, ok := route["final"].(string); ok {
			defaultSelector = final
		}
	}

	// Find all selector type outbounds
	for _, outbound := range outbounds {
		outboundMap, ok := outbound.(map[string]interface{})
		if !ok {
			continue
		}

		outboundType, _ := outboundMap["type"].(string)
		if outboundType == "selector" {
			if tag, ok := outboundMap["tag"].(string); ok {
				// Skip if already in list
				found := false
				for _, existing := range selectorGroups {
					if existing == tag {
						found = true
						break
					}
				}
				if !found {
					selectorGroups = append(selectorGroups, tag)
				}
			}
		}
	}

	// If no selectors found, return default
	if len(selectorGroups) == 0 {
		selectorGroups = []string{"proxy-out"}
	}

	// If defaultSelector is not in the list, use first one
	if defaultSelector != "" {
		found := false
		for _, group := range selectorGroups {
			if group == defaultSelector {
				found = true
				break
			}
		}
		if !found {
			defaultSelector = selectorGroups[0]
		}
	} else {
		defaultSelector = selectorGroups[0]
	}

	return selectorGroups, defaultSelector, nil
}

var reTrailingCommas = regexp.MustCompile(`,(\s*[\]\}])`)

func removeTrailingCommas(data []byte) []byte {
	return reTrailingCommas.ReplaceAll(data, []byte("$1"))
}

// getConfigJSON reads config and returns JSON safe to parse (JSONC + trailing commas removed).
// Trailing commas are removed before and after jsonc so jsonc never sees invalid input and we still fix cases like , // comment \n ].
func getConfigJSON(configPath string) ([]byte, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	data = removeTrailingCommas(data) // before jsonc so it doesn't fail on simple ,]
	cleanData := jsonc.ToJSON(data)
	cleanData = removeTrailingCommas(cleanData) // after jsonc for cases like , // comment \n ]
	return cleanData, nil
}

// GetTunInterfaceName extracts TUN interface name from config.json
// Returns empty string if no TUN interface is configured
func GetTunInterfaceName(configPath string) (string, error) {
	cleanData, err := getConfigJSON(configPath)
	if err != nil {
		return "", err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(cleanData, &config); err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}

	inbounds, ok := config["inbounds"].([]interface{})
	if !ok {
		return "", nil // No inbounds section, no TUN interface
	}

	for _, inbound := range inbounds {
		inboundMap, ok := inbound.(map[string]interface{})
		if !ok {
			continue
		}

		if inboundMap["type"] == "tun" {
			if interfaceName, ok := inboundMap["interface_name"].(string); ok && interfaceName != "" {
				return interfaceName, nil
			}
		}
	}

	return "", nil // No TUN interface found in config
}

// ConfigHasTun returns true if config has any TUN inbound (used to decide if privilege escalation is needed on macOS).
func ConfigHasTun(configPath string) (bool, error) {
	cleanData, err := getConfigJSON(configPath)
	if err != nil {
		return false, err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(cleanData, &config); err != nil {
		return false, fmt.Errorf("failed to parse config: %w", err)
	}
	inbounds, ok := config["inbounds"].([]interface{})
	if !ok {
		return false, nil
	}
	for _, inbound := range inbounds {
		inboundMap, ok := inbound.(map[string]interface{})
		if !ok {
			continue
		}
		if inboundMap["type"] == "tun" {
			return true, nil
		}
	}
	return false, nil
}
