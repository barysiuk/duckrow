package core

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// createTestManifest writes a duckrow.json manifest to the given directory.
func createTestManifest(t *testing.T, dir string, manifest RegistryManifest) {
	t.Helper()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "duckrow.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadManifest(t *testing.T) {
	t.Run("parses valid manifest", func(t *testing.T) {
		dir := t.TempDir()
		expected := RegistryManifest{
			Name:        "test-registry",
			Description: "A test registry",
			Skills: []SkillEntry{
				{
					Name:        "skill-a",
					Description: "Skill A",
					Source:      "owner/repo",
					Version:     "1.0.0",
				},
				{
					Name:        "skill-b",
					Description: "Skill B",
					Source:      "owner/other-repo",
				},
			},
		}
		createTestManifest(t, dir, expected)

		manifest, err := readManifest(dir)
		if err != nil {
			t.Fatalf("readManifest() error = %v", err)
		}

		if manifest.Name != "test-registry" {
			t.Errorf("Name = %q, want %q", manifest.Name, "test-registry")
		}
		if manifest.Description != "A test registry" {
			t.Errorf("Description = %q, want %q", manifest.Description, "A test registry")
		}
		if len(manifest.Skills) != 2 {
			t.Fatalf("len(Skills) = %d, want 2", len(manifest.Skills))
		}
		if manifest.Skills[0].Name != "skill-a" {
			t.Errorf("Skills[0].Name = %q, want %q", manifest.Skills[0].Name, "skill-a")
		}
		if manifest.Skills[0].Version != "1.0.0" {
			t.Errorf("Skills[0].Version = %q, want %q", manifest.Skills[0].Version, "1.0.0")
		}
		if manifest.Skills[1].Source != "owner/other-repo" {
			t.Errorf("Skills[1].Source = %q, want %q", manifest.Skills[1].Source, "owner/other-repo")
		}
	})

	t.Run("error when file not found", func(t *testing.T) {
		dir := t.TempDir()
		_, err := readManifest(dir)
		if err == nil {
			t.Fatal("expected error for missing manifest")
		}
	})

	t.Run("error when invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "duckrow.json"), []byte("{invalid}"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := readManifest(dir)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("handles empty skills array", func(t *testing.T) {
		dir := t.TempDir()
		createTestManifest(t, dir, RegistryManifest{
			Name:   "empty-registry",
			Skills: []SkillEntry{},
		})

		manifest, err := readManifest(dir)
		if err != nil {
			t.Fatalf("readManifest() error = %v", err)
		}
		if len(manifest.Skills) != 0 {
			t.Errorf("expected 0 skills, got %d", len(manifest.Skills))
		}
	})
}

func TestRegistryManager_LoadManifest(t *testing.T) {
	t.Run("loads manifest from registry dir", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		// Create a fake registry clone
		regDir := filepath.Join(registriesDir, "my-org")
		createTestManifest(t, regDir, RegistryManifest{
			Name:        "my-org",
			Description: "My org skills",
			Skills: []SkillEntry{
				{Name: "lint-rules", Description: "Linting", Source: "org/lint"},
			},
		})

		manifest, err := rm.LoadManifest("my-org")
		if err != nil {
			t.Fatalf("LoadManifest() error = %v", err)
		}

		if manifest.Name != "my-org" {
			t.Errorf("Name = %q, want %q", manifest.Name, "my-org")
		}
		if len(manifest.Skills) != 1 {
			t.Errorf("len(Skills) = %d, want 1", len(manifest.Skills))
		}
	})

	t.Run("error when registry not found", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		_, err := rm.LoadManifest("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent registry")
		}
	})
}

func TestRegistryManager_LoadAllManifests(t *testing.T) {
	registriesDir := t.TempDir()
	rm := NewRegistryManager(registriesDir)

	// Create two fake registry clones
	createTestManifest(t, filepath.Join(registriesDir, "org-a"), RegistryManifest{
		Name:   "org-a",
		Skills: []SkillEntry{{Name: "s1", Description: "S1", Source: "a/s1"}},
	})
	createTestManifest(t, filepath.Join(registriesDir, "org-b"), RegistryManifest{
		Name:   "org-b",
		Skills: []SkillEntry{{Name: "s2", Description: "S2", Source: "b/s2"}},
	})

	registries := []Registry{
		{Name: "org-a", Repo: "git@example.com:a.git"},
		{Name: "org-b", Repo: "git@example.com:b.git"},
		{Name: "org-missing", Repo: "git@example.com:missing.git"},
	}

	results := rm.LoadAllManifests(registries)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results["org-a"].Name != "org-a" {
		t.Errorf("org-a manifest name = %q", results["org-a"].Name)
	}
	if results["org-b"].Name != "org-b" {
		t.Errorf("org-b manifest name = %q", results["org-b"].Name)
	}
}

func TestRegistryManager_ListSkills(t *testing.T) {
	registriesDir := t.TempDir()
	rm := NewRegistryManager(registriesDir)

	createTestManifest(t, filepath.Join(registriesDir, "org-a"), RegistryManifest{
		Name: "org-a",
		Skills: []SkillEntry{
			{Name: "s1", Description: "S1", Source: "a/s1"},
			{Name: "s2", Description: "S2", Source: "a/s2"},
		},
	})
	createTestManifest(t, filepath.Join(registriesDir, "org-b"), RegistryManifest{
		Name: "org-b",
		Skills: []SkillEntry{
			{Name: "s3", Description: "S3", Source: "b/s3"},
		},
	})

	registries := []Registry{
		{Name: "org-a", Repo: "git@example.com:a.git"},
		{Name: "org-b", Repo: "git@example.com:b.git"},
	}

	skills := rm.ListSkills(registries)
	if len(skills) != 3 {
		t.Fatalf("len(skills) = %d, want 3", len(skills))
	}

	// Verify registry association
	if skills[0].RegistryName != "org-a" {
		t.Errorf("skills[0].RegistryName = %q, want %q", skills[0].RegistryName, "org-a")
	}
	if skills[0].Skill.Name != "s1" {
		t.Errorf("skills[0].Skill.Name = %q, want %q", skills[0].Skill.Name, "s1")
	}
	if skills[2].RegistryName != "org-b" {
		t.Errorf("skills[2].RegistryName = %q, want %q", skills[2].RegistryName, "org-b")
	}
}

func TestRegistryManager_Remove(t *testing.T) {
	t.Run("removes registry clone", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		regDir := filepath.Join(registriesDir, "to-delete")
		createTestManifest(t, regDir, RegistryManifest{Name: "to-delete"})

		err := rm.Remove("to-delete")
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		if dirExists(regDir) {
			t.Error("registry directory still exists after removal")
		}
	})

	t.Run("error when registry not found", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		err := rm.Remove("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent registry")
		}
	})

	t.Run("error when name is empty", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		err := rm.Remove("")
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})
}

// Integration tests that require git — skipped with -short
func TestRegistryManager_Add_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	t.Run("clones public registry with manifest", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		// Create a local bare repo with a manifest to avoid network dependency
		bareRepo := t.TempDir()
		setupTestGitRepo(t, bareRepo)

		manifest, err := rm.Add(bareRepo)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		if manifest.Name != "test-org" {
			t.Errorf("manifest.Name = %q, want %q", manifest.Name, "test-org")
		}

		// Verify clone exists at named location
		namedDir := filepath.Join(registriesDir, "test-org")
		if !dirExists(namedDir) {
			t.Error("registry not cloned to expected location")
		}

		// Verify manifest can be loaded
		loaded, err := rm.LoadManifest("test-org")
		if err != nil {
			t.Fatalf("LoadManifest() after Add() error = %v", err)
		}
		if loaded.Name != "test-org" {
			t.Errorf("loaded manifest Name = %q", loaded.Name)
		}
	})

	t.Run("error when repo has no manifest", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		// Create a repo without duckrow.json
		bareRepo := t.TempDir()
		setupBareGitRepo(t, bareRepo)

		_, err := rm.Add(bareRepo)
		if err == nil {
			t.Fatal("expected error for repo without manifest")
		}
	})

	t.Run("error when URL is empty", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		_, err := rm.Add("")
		if err == nil {
			t.Fatal("expected error for empty URL")
		}
	})
}

func TestRegistryManager_Refresh_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	t.Run("refreshes registry", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		// Create and add a local repo
		bareRepo := t.TempDir()
		setupTestGitRepo(t, bareRepo)

		_, err := rm.Add(bareRepo)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		// Refresh should succeed (even though nothing changed)
		manifest, err := rm.Refresh("test-org")
		if err != nil {
			t.Fatalf("Refresh() error = %v", err)
		}

		if manifest.Name != "test-org" {
			t.Errorf("refreshed manifest.Name = %q", manifest.Name)
		}
	})

	t.Run("error for nonexistent registry", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		_, err := rm.Refresh("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent registry")
		}
	})

	t.Run("error when name is empty", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		_, err := rm.Refresh("")
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})
}

func TestRegistryManager_Add_CloneError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}

	t.Run("returns CloneError for unreachable URL", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		_, err := rm.Add("https://github.com/nonexistent-owner-xyz/nonexistent-repo-xyz.git")
		if err == nil {
			t.Fatal("expected error for unreachable URL")
		}

		ce, ok := IsCloneError(err)
		if !ok {
			t.Fatalf("expected *CloneError, got %T: %v", err, err)
		}

		if ce.URL != "https://github.com/nonexistent-owner-xyz/nonexistent-repo-xyz.git" {
			t.Errorf("CloneError.URL = %q", ce.URL)
		}
		if ce.Protocol != "https" {
			t.Errorf("CloneError.Protocol = %q, want %q", ce.Protocol, "https")
		}
	})

	t.Run("remote URL matches after Add", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		bareRepo := t.TempDir()
		setupTestGitRepo(t, bareRepo)

		_, err := rm.Add(bareRepo)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		// Verify the remote URL of the clone matches the source
		cloneDir := filepath.Join(registriesDir, "test-org")
		remoteURL := gitRemoteURL(cloneDir)
		if remoteURL != bareRepo {
			t.Errorf("remote URL = %q, want %q", remoteURL, bareRepo)
		}
	})
}

// setupTestGitRepo creates a local git repo with a duckrow.json manifest.
func setupTestGitRepo(t *testing.T, dir string) {
	t.Helper()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("checkout", "-b", "main")

	manifest := RegistryManifest{
		Name:        "test-org",
		Description: "Test organization skills",
		Skills: []SkillEntry{
			{
				Name:        "test-skill",
				Description: "A test skill",
				Source:      "test-org/skills",
				Version:     "1.0.0",
			},
		},
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "duckrow.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	runGit("add", ".")
	runGit("commit", "-m", "initial")
}

// setupBareGitRepo creates a local git repo without a manifest.
func setupBareGitRepo(t *testing.T, dir string) {
	t.Helper()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("checkout", "-b", "main")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	runGit("add", ".")
	runGit("commit", "-m", "initial")
}
