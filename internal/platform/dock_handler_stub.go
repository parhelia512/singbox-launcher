//go:build !darwin
// +build !darwin

package platform

import "singbox-launcher/internal/debuglog"

// HideDockIcon is a no-op on non-macOS platforms
func HideDockIcon() {
	debuglog.DebugLog("platform: HideDockIcon is not implemented on non-darwin platforms")
}

// RestoreDockIcon is a no-op on non-macOS platforms
func RestoreDockIcon() {
	debuglog.DebugLog("platform: RestoreDockIcon is not implemented on non-darwin platforms")
}
