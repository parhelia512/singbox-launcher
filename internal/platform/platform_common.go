package platform

import (
	"os"
	"path/filepath"
)

// GetConfigPath returns the path to config.json
func GetConfigPath(execDir string) string {
	return filepath.Join(execDir, "bin", "config.json")
}

// GetBinDir returns the path to bin directory
func GetBinDir(execDir string) string {
	return filepath.Join(execDir, "bin")
}

// GetLogsDir returns the path to logs directory
func GetLogsDir(execDir string) string {
	return filepath.Join(execDir, "logs")
}

// EnsureDirectories creates necessary directories if they don't exist
func EnsureDirectories(execDir string) error {
	dirs := []string{
		GetLogsDir(execDir),
		GetBinDir(execDir),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

