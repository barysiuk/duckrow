package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFolderManager_Add(t *testing.T) {
	configDir := t.TempDir()
	cm := NewConfigManagerWithDir(configDir)
	fm := NewFolderManager(cm)

	// Create a folder to track
	folderPath := t.TempDir()

	if err := fm.Add(folderPath); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	folders, err := fm.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(folders))
	}
	if folders[0].Path != folderPath {
		t.Errorf("expected path %q, got %q", folderPath, folders[0].Path)
	}
	if folders[0].AddedAt.IsZero() {
		t.Error("expected AddedAt to be set")
	}
}

func TestFolderManager_AddDuplicate(t *testing.T) {
	configDir := t.TempDir()
	cm := NewConfigManagerWithDir(configDir)
	fm := NewFolderManager(cm)

	folderPath := t.TempDir()

	if err := fm.Add(folderPath); err != nil {
		t.Fatalf("first Add() error: %v", err)
	}
	if err := fm.Add(folderPath); err == nil {
		t.Error("expected error when adding duplicate folder")
	}
}

func TestFolderManager_AddNonexistent(t *testing.T) {
	configDir := t.TempDir()
	cm := NewConfigManagerWithDir(configDir)
	fm := NewFolderManager(cm)

	err := fm.Add("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error when adding nonexistent path")
	}
}

func TestFolderManager_AddFile(t *testing.T) {
	configDir := t.TempDir()
	cm := NewConfigManagerWithDir(configDir)
	fm := NewFolderManager(cm)

	// Create a file (not a directory)
	filePath := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := fm.Add(filePath)
	if err == nil {
		t.Error("expected error when adding a file instead of directory")
	}
}

func TestFolderManager_Remove(t *testing.T) {
	configDir := t.TempDir()
	cm := NewConfigManagerWithDir(configDir)
	fm := NewFolderManager(cm)

	folder1 := t.TempDir()
	folder2 := t.TempDir()

	if err := fm.Add(folder1); err != nil {
		t.Fatalf("Add(folder1) error: %v", err)
	}
	if err := fm.Add(folder2); err != nil {
		t.Fatalf("Add(folder2) error: %v", err)
	}

	if err := fm.Remove(folder1); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	folders, _ := fm.List()
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder after remove, got %d", len(folders))
	}
	if folders[0].Path != folder2 {
		t.Errorf("expected remaining folder %q, got %q", folder2, folders[0].Path)
	}
}

func TestFolderManager_RemoveNotTracked(t *testing.T) {
	configDir := t.TempDir()
	cm := NewConfigManagerWithDir(configDir)
	fm := NewFolderManager(cm)

	err := fm.Remove("/not/tracked")
	if err == nil {
		t.Error("expected error when removing untracked folder")
	}
}

func TestFolderManager_ListEmpty(t *testing.T) {
	configDir := t.TempDir()
	cm := NewConfigManagerWithDir(configDir)
	fm := NewFolderManager(cm)

	folders, err := fm.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(folders) != 0 {
		t.Errorf("expected 0 folders, got %d", len(folders))
	}
}

func TestFolderManager_AddCurrentDir(t *testing.T) {
	configDir := t.TempDir()
	cm := NewConfigManagerWithDir(configDir)
	fm := NewFolderManager(cm)

	// Add with empty string should resolve to cwd
	cwd, _ := os.Getwd()
	if err := fm.Add(""); err != nil {
		t.Fatalf("Add('') error: %v", err)
	}

	folders, _ := fm.List()
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(folders))
	}
	if folders[0].Path != cwd {
		t.Errorf("expected path %q, got %q", cwd, folders[0].Path)
	}
}
