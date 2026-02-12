package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FolderManager handles tracked folder operations.
type FolderManager struct {
	config *ConfigManager
}

// NewFolderManager creates a FolderManager.
func NewFolderManager(config *ConfigManager) *FolderManager {
	return &FolderManager{config: config}
}

// Add adds a folder to the tracked list. The path is resolved to an absolute path.
// Returns an error if the path doesn't exist or is already tracked.
func (fm *FolderManager) Add(path string) error {
	absPath, err := resolveFolderPath(path)
	if err != nil {
		return err
	}

	cfg, err := fm.config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Check for duplicates
	for _, f := range cfg.Folders {
		if f.Path == absPath {
			return fmt.Errorf("folder already tracked: %s", absPath)
		}
	}

	cfg.Folders = append(cfg.Folders, TrackedFolder{
		Path:    absPath,
		AddedAt: time.Now().UTC(),
	})

	if err := fm.config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

// Remove removes a folder from the tracked list.
// Does not delete any files on disk.
func (fm *FolderManager) Remove(path string) error {
	absPath, err := resolvePathNoValidation(path)
	if err != nil {
		return err
	}

	cfg, err := fm.config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	found := false
	folders := make([]TrackedFolder, 0, len(cfg.Folders))
	for _, f := range cfg.Folders {
		if f.Path == absPath {
			found = true
			continue
		}
		folders = append(folders, f)
	}

	if !found {
		return fmt.Errorf("folder not tracked: %s", absPath)
	}

	cfg.Folders = folders
	if err := fm.config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

// List returns all tracked folders.
func (fm *FolderManager) List() ([]TrackedFolder, error) {
	cfg, err := fm.config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg.Folders, nil
}

// resolveFolderPath resolves a path to absolute and validates it exists.
func resolveFolderPath(path string) (string, error) {
	if path == "" {
		path = "."
	}

	// Expand ~
	path = expandPath(path)

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %s", absPath)
		}
		return "", fmt.Errorf("checking path: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}

	return absPath, nil
}

// resolvePathNoValidation resolves a path to absolute without checking existence.
// Used for remove operations where the directory may have been deleted.
func resolvePathNoValidation(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	path = expandPath(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}
	return absPath, nil
}
