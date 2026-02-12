package core

import (
	"runtime"

	"fyne.io/fyne/v2"

	"singbox-launcher/internal/debuglog"
	"singbox-launcher/internal/dialogs"
	"singbox-launcher/internal/platform"
)

// addHideDockMenuItem adds "Hide app from Dock" toggle menu item (macOS only)
func (ac *AppController) addHideDockMenuItem(menuItems []*fyne.MenuItem) []*fyne.MenuItem {
	if runtime.GOOS != "darwin" {
		return menuItems
	}

	hideDockEnabled := ac.UIService.HideAppFromDock
	hideDockLabel := "Hide app from Dock"
	if hideDockEnabled {
		hideDockLabel = "✓ " + hideDockLabel
	}

	menuItems = append(menuItems, fyne.NewMenuItem(hideDockLabel, func() {
		// Toggle the preference
		ac.UIService.HideAppFromDock = !ac.UIService.HideAppFromDock

		// Apply the change immediately on macOS
		if runtime.GOOS == "darwin" {
			if ac.UIService.HideAppFromDock {
				platform.HideDockIcon()
				// Also hide the main window when hiding from Dock
				if ac.UIService.MainWindow != nil {
					ac.UIService.MainWindow.Hide()
				}
				debuglog.InfoLog("Tray: Hide app from Dock enabled — Dock hidden and window hidden")
			} else {
				platform.RestoreDockIcon()
				// Restore and show the main window when unchecking (or focus wizard if open)
				if ac.hasUI() {
					ac.UIService.ShowMainWindowOrFocusWizard()
				}
				debuglog.InfoLog("Tray: Hide app from Dock disabled — Dock restored and window shown")
			}
		}

		if ac.UIService.UpdateTrayMenuFunc != nil {
			ac.UIService.UpdateTrayMenuFunc()
		}
	}))
	menuItems = append(menuItems, fyne.NewMenuItemSeparator())

	return menuItems
}

// CreateTrayMenu creates the system tray menu with proxy selection submenu
func (ac *AppController) CreateTrayMenu() *fyne.Menu {
	/**
	@TODO:if ac.APIService == nil { кажется это приводит к дублированию кода, может лучше бы делать if ac.APIService != nil {
	*/
	if ac.APIService == nil {
		// Return minimal menu if APIService is not initialized
		menuItems := []*fyne.MenuItem{}

		// On macOS, add a separator at the beginning to fix menu positioning
		// This prevents the first item from being hidden behind the scroll arrow
		// by increasing the menu height and ensuring proper positioning
		if runtime.GOOS == "darwin" {
			menuItems = append(menuItems, fyne.NewMenuItemSeparator())
		}

		menuItems = append(menuItems,
			fyne.NewMenuItem("Open", func() {
				if ac.UIService != nil {
					platform.RestoreDockIcon()
					ac.UIService.ShowMainWindowOrFocusWizard()
				}
			}),
			fyne.NewMenuItemSeparator(),
		)

		if runtime.GOOS == "darwin" {
			menuItems = ac.addHideDockMenuItem(menuItems)
		}
		menuItems = append(menuItems, fyne.NewMenuItem("Quit", ac.GracefulExit))
		return fyne.NewMenu("Singbox Launcher", menuItems...)
	}

	// Get proxies from current group
	proxies := ac.APIService.GetProxiesList()
	activeProxy := ac.APIService.GetActiveProxyName()
	selectedGroup := ac.APIService.GetSelectedClashGroup()
	_, _, clashAPIEnabled := ac.APIService.GetClashAPIConfig()

	// Auto-load proxies if list is empty and API is enabled
	// Note: AutoLoadProxies has internal guard to prevent multiple simultaneous loads
	if clashAPIEnabled && selectedGroup != "" && len(proxies) == 0 {
		// Only auto-load if sing-box is running
		if ac.RunningState.IsRunning() {
			// Check if auto-load is already in progress to avoid duplicate calls
			ac.APIService.AutoLoadMutex.Lock()
			alreadyInProgress := ac.APIService.AutoLoadInProgress
			ac.APIService.AutoLoadMutex.Unlock()

			if !alreadyInProgress {
				// Start auto-loading in background (non-blocking)
				go ac.AutoLoadProxies()
			}
		}
	}

	// Create proxy submenu items
	var proxyMenuItems []*fyne.MenuItem
	if clashAPIEnabled && selectedGroup != "" && len(proxies) > 0 {
		for i := range proxies {
			proxy := proxies[i]
			proxyName := proxy.Name
			isActive := proxyName == activeProxy

			// Create local copy for closure
			pName := proxyName
			menuItem := fyne.NewMenuItem(proxyName, func() {
				// Switch to selected proxy
				go func() {
					err := ac.APIService.SwitchProxy(selectedGroup, pName)
					fyne.Do(func() {
						if err != nil {
							debuglog.ErrorLog("CreateTrayMenu: Failed to switch proxy: %v", err)
							if ac.hasUI() {
								dialogs.ShowError(ac.UIService.MainWindow, err)
							}
						}
						// OnProxySwitched callback is already called in APIService.SwitchProxy
					})
				}()
			})

			// Mark active proxy with checkmark
			if isActive {
				menuItem.Label = "✓ " + proxyName
			}

			proxyMenuItems = append(proxyMenuItems, menuItem)
		}
	} else {
		// Show disabled item if no proxies available
		disabledItem := fyne.NewMenuItem("No proxies available", nil)
		disabledItem.Disabled = true
		proxyMenuItems = append(proxyMenuItems, disabledItem)
	}

	// Create proxy submenu
	proxySubmenu := fyne.NewMenu("Select Proxy", proxyMenuItems...)

	// Get button state from centralized function
	buttonState := ac.GetVPNButtonState()

	// Create main menu items
	menuItems := []*fyne.MenuItem{}

	// On macOS, add a separator at the beginning to fix menu positioning
	// This prevents the first item from being hidden behind the scroll arrow
	// by increasing the menu height and ensuring proper positioning
	if runtime.GOOS == "darwin" {
		menuItems = append(menuItems, fyne.NewMenuItemSeparator())
	}

	menuItems = append(menuItems,
		fyne.NewMenuItem("Open", func() {
			if ac.hasUI() {
				platform.RestoreDockIcon()
				ac.UIService.ShowMainWindowOrFocusWizard()
			}
		}),
		fyne.NewMenuItemSeparator(),
	)

	// Add Start/Stop VPN buttons based on centralized state
	if buttonState.StartEnabled {
		menuItems = append(menuItems, fyne.NewMenuItem("Start VPN", func() { StartSingBoxProcess() }))
	} else {
		startItem := fyne.NewMenuItem("Start VPN", nil)
		startItem.Disabled = true
		menuItems = append(menuItems, startItem)
	}

	if buttonState.StopEnabled {
		menuItems = append(menuItems, fyne.NewMenuItem("Stop VPN", func() { StopSingBoxProcess() }))
	} else {
		stopItem := fyne.NewMenuItem("Stop VPN", nil)
		stopItem.Disabled = true
		menuItems = append(menuItems, stopItem)
	}

	menuItems = append(menuItems, fyne.NewMenuItemSeparator())

	// Add proxy submenu if Clash API is enabled
	if clashAPIEnabled && selectedGroup != "" {
		selectProxyItem := fyne.NewMenuItem("Select Proxy", nil)
		selectProxyItem.ChildMenu = proxySubmenu
		menuItems = append(menuItems, selectProxyItem)
		menuItems = append(menuItems, fyne.NewMenuItemSeparator())
	}

	// Add "Hide app from Dock" toggle (macOS only) before Quit
	if runtime.GOOS == "darwin" {
		menuItems = ac.addHideDockMenuItem(menuItems)
	}

	// Add Quit item
	menuItems = append(menuItems, fyne.NewMenuItem("Quit", ac.GracefulExit))

	return fyne.NewMenu("Singbox Launcher", menuItems...)
}
