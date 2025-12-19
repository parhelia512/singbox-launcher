//go:build !darwin
// +build !darwin

package platform

// GetSystemSOCKSProxy is a stub for non-macOS platforms.
// On non-darwin systems system-wide SOCKS proxy detection is not implemented,
// so return empty host, port 0, enabled=false and no error.
func GetSystemSOCKSProxy() (host string, port int, enabled bool, err error) {
	return "", 0, false, nil
}
