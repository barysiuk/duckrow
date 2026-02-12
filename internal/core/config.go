package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	configDirName  = ".duckrow"
	configFileName = "config.json"
)

// ConfigManager handles reading and writing the DuckRow configuration.
type ConfigManager struct {
	configDir string
	mu        sync.RWMutex
}

// NewConfigManager creates a ConfigManager using the default config path (~/.duckrow/).
func NewConfigManager() (*ConfigManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}
	return &ConfigManager{
		configDir: filepath.Join(home, configDirName),
	}, nil
}

// NewConfigManagerWithDir creates a ConfigManager using a custom config directory.
// Useful for testing.
func NewConfigManagerWithDir(dir string) *ConfigManager {
	return &ConfigManager{configDir: dir}
}

// ConfigDir returns the configuration directory path.
func (cm *ConfigManager) ConfigDir() string {
	return cm.configDir
}

// ConfigPath returns the full path to the config file.
func (cm *ConfigManager) ConfigPath() string {
	return filepath.Join(cm.configDir, configFileName)
}

// Load reads the config from disk. Returns default config if file doesn't exist.
func (cm *ConfigManager) Load() (*Config, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	path := cm.ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to disk, creating the directory if needed.
func (cm *ConfigManager) Save(cfg *Config) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if err := os.MkdirAll(cm.configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Write atomically: write to temp file then rename
	tmpPath := cm.ConfigPath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	if err := os.Rename(tmpPath, cm.ConfigPath()); err != nil {
		_ = os.Remove(tmpPath) // clean up on failure
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

// RegistriesDir returns the path where registry clones are stored.
func (cm *ConfigManager) RegistriesDir() string {
	return filepath.Join(cm.configDir, "registries")
}

func defaultConfig() *Config {
	return &Config{
		Folders:    []TrackedFolder{},
		Registries: []Registry{},
		Settings: Settings{
			AutoAddCurrentDir:   true,
			DisableAllTelemetry: false,
		},
	}
}
