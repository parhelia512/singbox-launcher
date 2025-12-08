package core

import (
	"fmt"
	"log"
	"runtime"

	"singbox-launcher/internal/dialogs"
)

// CheckForUpdates checks for application updates
// This is a placeholder for future update functionality
func (ac *AppController) CheckForUpdates() {
	log.Println("CheckForUpdates: Update checking not yet implemented")

	dialogs.ShowInfo(ac.MainWindow, "Updates",
		"Automatic updates are not yet implemented.\n\n"+
			"Please check GitHub releases for updates:\n"+
			"https://github.com/Leadaxe/singbox-launcher/releases")
}

// UpdateAvailable shows a dialog when an update is available
func (ac *AppController) UpdateAvailable(version string, downloadURL string) {
	message := fmt.Sprintf(
		"New version %s is available!\n\n"+
			"Download: %s\n\n"+
			"Would you like to download it now?",
		version,
		downloadURL,
	)

	dialogs.ShowConfirm(ac.MainWindow, "Update Available", message, func(download bool) {
		if download {
			// Open download URL in browser
			// This would require platform.OpenURL which is in platform package
			// For now, just show info
			dialogs.ShowInfo(ac.MainWindow, "Download", "Please download the update from:\n"+downloadURL)
		}
	})
}

// GetCurrentVersion returns the current application version
func GetCurrentVersion() string {
	// This should be set during build using -ldflags
	// For now, return a placeholder
	return "1.0.0"
}

// GetUpdateURL returns the URL to check for updates based on platform
func GetUpdateURL() string {
	baseURL := "https://github.com/Leadaxe/singbox-launcher/releases/latest"

	// Platform-specific download URLs would go here
	switch runtime.GOOS {
	case "windows":
		return baseURL + "/download/singbox-launcher-windows-amd64.exe"
	case "darwin":
		return baseURL + "/download/singbox-launcher-darwin-amd64"
	case "linux":
		return baseURL + "/download/singbox-launcher-linux-amd64"
	default:
		return baseURL
	}
}
