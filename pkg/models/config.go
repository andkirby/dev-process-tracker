package models

import (
	"os"
	"path/filepath"
)

// ConfigPaths provides paths for config and data directories
type ConfigPaths struct {
	ConfigDir    string
	RegistryFile string
	LogsDir      string
}

// GetConfigPaths returns paths for devpt configuration
func GetConfigPaths() (ConfigPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return ConfigPaths{}, err
	}

	configDir := filepath.Join(home, ".config", "devpt")
	return ConfigPaths{
		ConfigDir:    configDir,
		RegistryFile: filepath.Join(configDir, "registry.json"),
		LogsDir:      filepath.Join(configDir, "logs"),
	}, nil
}

// EnsureDirs creates necessary configuration directories
func (cp ConfigPaths) EnsureDirs() error {
	dirs := []string{cp.ConfigDir, cp.LogsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}
