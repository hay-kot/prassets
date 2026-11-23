package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const appName = "prassets"

// ConfigDir returns the XDG config directory for the application.
func ConfigDir() (string, error) {
	return appDir("XDG_CONFIG_HOME", ".config")
}

// DataDir returns the XDG data directory for the application.
func DataDir() (string, error) {
	return appDir("XDG_DATA_HOME", ".local", "share")
}

// CacheDir returns the XDG cache directory for the application.
func CacheDir() (string, error) {
	return appDir("XDG_CACHE_HOME", ".cache")
}

// Expand replaces a leading "~" or "~/" in p with the user's home directory.
func Expand(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}

func appDir(envName string, fallbackParts ...string) (string, error) {
	if dir := os.Getenv(envName); dir != "" {
		return filepath.Join(dir, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	parts := append([]string{home}, fallbackParts...)
	return filepath.Join(append(parts, appName)...), nil
}
