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

func TestConfigManager_SaveCloneURLOverride(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManagerWithDir(dir)

	// Save initial config.
	cfg := defaultConfig()
	if err := cm.Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Save an override.
	if err := cm.SaveCloneURLOverride("pandadoc-studio/skills", "git@github.com-work:pandadoc-studio/skills.git"); err != nil {
		t.Fatalf("SaveCloneURLOverride() error: %v", err)
	}

	// Load and verify.
	loaded, err := cm.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Settings.CloneURLOverrides == nil {
		t.Fatal("CloneURLOverrides is nil after save")
	}
	got := loaded.Settings.CloneURLOverrides["pandadoc-studio/skills"]
	want := "git@github.com-work:pandadoc-studio/skills.git"
	if got != want {
		t.Errorf("override = %q, want %q", got, want)
	}
}

func TestConfigManager_SaveCloneURLOverride_MultipleSaves(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManagerWithDir(dir)

	// Save initial config.
	if err := cm.Save(defaultConfig()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Save two different overrides.
	if err := cm.SaveCloneURLOverride("org/repo-a", "git@github.com:org/repo-a.git"); err != nil {
		t.Fatalf("first SaveCloneURLOverride() error: %v", err)
	}
	if err := cm.SaveCloneURLOverride("org/repo-b", "git@github.com:org/repo-b.git"); err != nil {
		t.Fatalf("second SaveCloneURLOverride() error: %v", err)
	}

	loaded, err := cm.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded.Settings.CloneURLOverrides) != 2 {
		t.Errorf("expected 2 overrides, got %d", len(loaded.Settings.CloneURLOverrides))
	}
}

func TestConfigManager_SaveCloneURLOverride_UpdateExisting(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManagerWithDir(dir)

	if err := cm.Save(defaultConfig()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Save then update the same key.
	_ = cm.SaveCloneURLOverride("org/repo", "git@github.com:org/repo.git")
	_ = cm.SaveCloneURLOverride("org/repo", "git@github.com-work:org/repo.git")

	loaded, err := cm.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	got := loaded.Settings.CloneURLOverrides["org/repo"]
	want := "git@github.com-work:org/repo.git"
	if got != want {
		t.Errorf("override = %q, want %q", got, want)
	}
}

func TestConfigManager_SaveCloneURLOverride_EmptyInputs(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManagerWithDir(dir)

	if err := cm.Save(defaultConfig()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Empty key or URL should be no-ops.
	if err := cm.SaveCloneURLOverride("", "git@github.com:o/r.git"); err != nil {
		t.Errorf("empty key should not error, got: %v", err)
	}
	if err := cm.SaveCloneURLOverride("org/repo", ""); err != nil {
		t.Errorf("empty URL should not error, got: %v", err)
	}

	loaded, err := cm.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded.Settings.CloneURLOverrides) > 0 {
		t.Errorf("expected no overrides, got %v", loaded.Settings.CloneURLOverrides)
	}
}

func TestConfigManager_SaveAndLoad_WithOverrides(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManagerWithDir(dir)

	cfg := &Config{
		Folders: []TrackedFolder{
			{Path: "/project"},
		},
		Settings: Settings{
			AutoAddCurrentDir: true,
			CloneURLOverrides: map[string]string{
				"org/repo": "git@github.com:org/repo.git",
			},
		},
	}

	if err := cm.Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := cm.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(loaded.Settings.CloneURLOverrides) != 1 {
		t.Fatalf("expected 1 override, got %d", len(loaded.Settings.CloneURLOverrides))
	}
	if loaded.Settings.CloneURLOverrides["org/repo"] != "git@github.com:org/repo.git" {
		t.Errorf("override value mismatch")
	}
}
