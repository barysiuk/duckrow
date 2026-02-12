package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigManager_DefaultConfig(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManagerWithDir(dir)

	cfg, err := cm.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if len(cfg.Folders) != 0 {
		t.Errorf("expected 0 folders, got %d", len(cfg.Folders))
	}
	if len(cfg.Registries) != 0 {
		t.Errorf("expected 0 registries, got %d", len(cfg.Registries))
	}
	if !cfg.Settings.AutoAddCurrentDir {
		t.Error("expected autoAddCurrentDir to be true by default")
	}
}

func TestConfigManager_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManagerWithDir(dir)

	cfg := &Config{
		Folders: []TrackedFolder{
			{Path: "/home/user/project1"},
			{Path: "/home/user/project2"},
		},
		Registries: []Registry{
			{Name: "internal", Repo: "git@github.com:org/registry.git"},
		},
		Settings: Settings{
			AutoAddCurrentDir:   false,
			DisableAllTelemetry: true,
		},
	}

	if err := cm.Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cm.ConfigPath()); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Load back
	loaded, err := cm.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded.Folders) != 2 {
		t.Errorf("expected 2 folders, got %d", len(loaded.Folders))
	}
	if loaded.Folders[0].Path != "/home/user/project1" {
		t.Errorf("expected folder path '/home/user/project1', got %q", loaded.Folders[0].Path)
	}
	if len(loaded.Registries) != 1 {
		t.Errorf("expected 1 registry, got %d", len(loaded.Registries))
	}
	if loaded.Registries[0].Name != "internal" {
		t.Errorf("expected registry name 'internal', got %q", loaded.Registries[0].Name)
	}
	if loaded.Settings.AutoAddCurrentDir {
		t.Error("expected autoAddCurrentDir to be false")
	}
	if !loaded.Settings.DisableAllTelemetry {
		t.Error("expected disableAllTelemetry to be true")
	}
}

func TestConfigManager_ConfigDir(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManagerWithDir(dir)

	if cm.ConfigDir() != dir {
		t.Errorf("ConfigDir() = %q, want %q", cm.ConfigDir(), dir)
	}
	if cm.ConfigPath() != filepath.Join(dir, "config.json") {
		t.Errorf("ConfigPath() = %q, want %q", cm.ConfigPath(), filepath.Join(dir, "config.json"))
	}
	if cm.RegistriesDir() != filepath.Join(dir, "registries") {
		t.Errorf("RegistriesDir() = %q, want %q", cm.RegistriesDir(), filepath.Join(dir, "registries"))
	}
}

func TestConfigManager_SaveCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "config")
	cm := NewConfigManagerWithDir(dir)

	cfg := defaultConfig()
	if err := cm.Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("config directory not created: %v", err)
	}
}

func TestConfigManager_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManagerWithDir(dir)

	// Write invalid JSON
	if err := os.WriteFile(cm.ConfigPath(), []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := cm.Load()
	if err == nil {
		t.Error("Load() should return error for corrupt config")
	}
}
