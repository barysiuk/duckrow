package core

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// createTestRegistryClone creates a fake registry clone in the registries dir
// at the path derived from the repo URL (using RegistryDirKey).
func createTestRegistryClone(t *testing.T, registriesDir, repoURL string, manifest RegistryManifest) string {
	t.Helper()
	dirKey := RegistryDirKey(repoURL)
	regDir := filepath.Join(registriesDir, dirKey)
	createTestManifest(t, regDir, manifest)
	return regDir
}

func TestRegistryDirKey(t *testing.T) {
	t.Run("different repos produce different keys", func(t *testing.T) {
		key1 := RegistryDirKey("git@github.com:org/repo-a.git")
		key2 := RegistryDirKey("git@github.com:org/repo-b.git")
		if key1 == key2 {
			t.Errorf("expected different keys, got %q for both", key1)
		}
	})

	t.Run("same repo produces same key", func(t *testing.T) {
		key1 := RegistryDirKey("git@github.com:org/repo.git")
		key2 := RegistryDirKey("git@github.com:org/repo.git")
		if key1 != key2 {
			t.Errorf("expected same key, got %q and %q", key1, key2)
		}
	})

	t.Run("includes readable part from SSH URL", func(t *testing.T) {
		key := RegistryDirKey("git@github.com:myorg/skills.git")
		// Should contain "myorg-skills" prefix
		if len(key) < 10 {
			t.Errorf("key %q seems too short", key)
		}
	})

	t.Run("includes readable part from HTTPS URL", func(t *testing.T) {
		key := RegistryDirKey("https://github.com/myorg/skills.git")
		if len(key) < 10 {
			t.Errorf("key %q seems too short", key)
		}
	})
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

		repoURL := "git@example.com:my-org/skills.git"
		createTestRegistryClone(t, registriesDir, repoURL, RegistryManifest{
			Name:        "my-org",
			Description: "My org skills",
			Skills: []SkillEntry{
				{Name: "lint-rules", Description: "Linting", Source: "org/lint"},
			},
		})

		manifest, err := rm.LoadManifest(repoURL)
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

		_, err := rm.LoadManifest("git@example.com:nonexistent.git")
		if err == nil {
			t.Fatal("expected error for nonexistent registry")
		}
	})
}

func TestRegistryManager_LoadAllManifests(t *testing.T) {
	registriesDir := t.TempDir()
	rm := NewRegistryManager(registriesDir)

	repoA := "git@example.com:a.git"
	repoB := "git@example.com:b.git"

	createTestRegistryClone(t, registriesDir, repoA, RegistryManifest{
		Name:   "org-a",
		Skills: []SkillEntry{{Name: "s1", Description: "S1", Source: "a/s1"}},
	})
	createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
		Name:   "org-b",
		Skills: []SkillEntry{{Name: "s2", Description: "S2", Source: "b/s2"}},
	})

	registries := []Registry{
		{Name: "org-a", Repo: repoA},
		{Name: "org-b", Repo: repoB},
		{Name: "org-missing", Repo: "git@example.com:missing.git"},
	}

	results := rm.LoadAllManifests(registries)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[repoA].Name != "org-a" {
		t.Errorf("org-a manifest name = %q", results[repoA].Name)
	}
	if results[repoB].Name != "org-b" {
		t.Errorf("org-b manifest name = %q", results[repoB].Name)
	}
}

func TestRegistryManager_ListSkills(t *testing.T) {
	registriesDir := t.TempDir()
	rm := NewRegistryManager(registriesDir)

	repoA := "git@example.com:a.git"
	repoB := "git@example.com:b.git"

	createTestRegistryClone(t, registriesDir, repoA, RegistryManifest{
		Name: "org-a",
		Skills: []SkillEntry{
			{Name: "s1", Description: "S1", Source: "a/s1"},
			{Name: "s2", Description: "S2", Source: "a/s2"},
		},
	})
	createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
		Name: "org-b",
		Skills: []SkillEntry{
			{Name: "s3", Description: "S3", Source: "b/s3"},
		},
	})

	registries := []Registry{
		{Name: "org-a", Repo: repoA},
		{Name: "org-b", Repo: repoB},
	}

	skills := rm.ListSkills(registries)
	if len(skills) != 3 {
		t.Fatalf("len(skills) = %d, want 3", len(skills))
	}

	// Verify registry association
	if skills[0].RegistryName != "org-a" {
		t.Errorf("skills[0].RegistryName = %q, want %q", skills[0].RegistryName, "org-a")
	}
	if skills[0].RegistryRepo != repoA {
		t.Errorf("skills[0].RegistryRepo = %q, want %q", skills[0].RegistryRepo, repoA)
	}
	if skills[0].Skill.Name != "s1" {
		t.Errorf("skills[0].Skill.Name = %q, want %q", skills[0].Skill.Name, "s1")
	}
	if skills[2].RegistryName != "org-b" {
		t.Errorf("skills[2].RegistryName = %q, want %q", skills[2].RegistryName, "org-b")
	}
	if skills[2].RegistryRepo != repoB {
		t.Errorf("skills[2].RegistryRepo = %q, want %q", skills[2].RegistryRepo, repoB)
	}
}

func TestRegistryManager_ListSkills_SameNameDifferentRepos(t *testing.T) {
	registriesDir := t.TempDir()
	rm := NewRegistryManager(registriesDir)

	repoA := "git@github.com:org/registry-a.git"
	repoB := "git@github.com:org/registry-b.git"

	// Both registries have the same manifest name but different repos
	createTestRegistryClone(t, registriesDir, repoA, RegistryManifest{
		Name: "same-name",
		Skills: []SkillEntry{
			{Name: "skill-from-a", Description: "From A", Source: "a/skill"},
		},
	})
	createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
		Name: "same-name",
		Skills: []SkillEntry{
			{Name: "skill-from-b", Description: "From B", Source: "b/skill"},
		},
	})

	registries := []Registry{
		{Name: "same-name", Repo: repoA},
		{Name: "same-name", Repo: repoB},
	}

	skills := rm.ListSkills(registries)
	if len(skills) != 2 {
		t.Fatalf("len(skills) = %d, want 2 (one from each registry)", len(skills))
	}

	// Each skill should come from a different repo
	if skills[0].Skill.Name != "skill-from-a" {
		t.Errorf("skills[0].Skill.Name = %q, want %q", skills[0].Skill.Name, "skill-from-a")
	}
	if skills[0].RegistryRepo != repoA {
		t.Errorf("skills[0].RegistryRepo = %q, want %q", skills[0].RegistryRepo, repoA)
	}
	if skills[1].Skill.Name != "skill-from-b" {
		t.Errorf("skills[1].Skill.Name = %q, want %q", skills[1].Skill.Name, "skill-from-b")
	}
	if skills[1].RegistryRepo != repoB {
		t.Errorf("skills[1].RegistryRepo = %q, want %q", skills[1].RegistryRepo, repoB)
	}
}

func TestRegistryManager_FindSkill(t *testing.T) {
	registriesDir := t.TempDir()
	rm := NewRegistryManager(registriesDir)

	repoA := "git@example.com:org-a/skills.git"
	repoB := "git@example.com:org-b/skills.git"

	createTestRegistryClone(t, registriesDir, repoA, RegistryManifest{
		Name: "org-a",
		Skills: []SkillEntry{
			{Name: "go-review", Description: "Go review", Source: "org-a/go-review"},
			{Name: "shared-lint", Description: "Shared lint", Source: "org-a/shared-lint"},
		},
	})
	createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
		Name: "org-b",
		Skills: []SkillEntry{
			{Name: "py-review", Description: "Python review", Source: "org-b/py-review"},
			{Name: "shared-lint", Description: "Shared lint B", Source: "org-b/shared-lint"},
		},
	})

	registries := []Registry{
		{Name: "org-a", Repo: repoA},
		{Name: "org-b", Repo: repoB},
	}

	t.Run("finds unique skill", func(t *testing.T) {
		info, err := rm.FindSkill(registries, "go-review", "")
		if err != nil {
			t.Fatalf("FindSkill() error = %v", err)
		}
		if info.Skill.Name != "go-review" {
			t.Errorf("Skill.Name = %q, want %q", info.Skill.Name, "go-review")
		}
		if info.RegistryName != "org-a" {
			t.Errorf("RegistryName = %q, want %q", info.RegistryName, "org-a")
		}
		if info.Skill.Source != "org-a/go-review" {
			t.Errorf("Skill.Source = %q, want %q", info.Skill.Source, "org-a/go-review")
		}
	})

	t.Run("errors on ambiguous skill", func(t *testing.T) {
		_, err := rm.FindSkill(registries, "shared-lint", "")
		if err == nil {
			t.Fatal("expected error for ambiguous skill")
		}
		if !containsStr(err.Error(), "multiple registries") {
			t.Errorf("error = %q, want to contain 'multiple registries'", err.Error())
		}
		if !containsStr(err.Error(), "--registry") {
			t.Errorf("error = %q, want to contain '--registry'", err.Error())
		}
	})

	t.Run("disambiguates with registry filter by name", func(t *testing.T) {
		info, err := rm.FindSkill(registries, "shared-lint", "org-b")
		if err != nil {
			t.Fatalf("FindSkill() error = %v", err)
		}
		if info.RegistryName != "org-b" {
			t.Errorf("RegistryName = %q, want %q", info.RegistryName, "org-b")
		}
		if info.Skill.Source != "org-b/shared-lint" {
			t.Errorf("Skill.Source = %q, want %q", info.Skill.Source, "org-b/shared-lint")
		}
	})

	t.Run("disambiguates with registry filter by repo URL", func(t *testing.T) {
		info, err := rm.FindSkill(registries, "shared-lint", repoA)
		if err != nil {
			t.Fatalf("FindSkill() error = %v", err)
		}
		if info.RegistryName != "org-a" {
			t.Errorf("RegistryName = %q, want %q", info.RegistryName, "org-a")
		}
	})

	t.Run("errors on unknown skill", func(t *testing.T) {
		_, err := rm.FindSkill(registries, "nonexistent", "")
		if err == nil {
			t.Fatal("expected error for nonexistent skill")
		}
		if !containsStr(err.Error(), "not found") {
			t.Errorf("error = %q, want to contain 'not found'", err.Error())
		}
		if !containsStr(err.Error(), "Available") {
			t.Errorf("error = %q, want to contain 'Available'", err.Error())
		}
	})

	t.Run("errors on unknown registry filter", func(t *testing.T) {
		_, err := rm.FindSkill(registries, "go-review", "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent registry")
		}
		if !containsStr(err.Error(), "not found") {
			t.Errorf("error = %q, want to contain 'not found'", err.Error())
		}
	})

	t.Run("errors on empty skill name", func(t *testing.T) {
		_, err := rm.FindSkill(registries, "", "")
		if err == nil {
			t.Fatal("expected error for empty skill name")
		}
	})

	t.Run("errors with no registries configured", func(t *testing.T) {
		_, err := rm.FindSkill(nil, "go-review", "")
		if err == nil {
			t.Fatal("expected error when no registries")
		}
		if !containsStr(err.Error(), "no skills available") {
			t.Errorf("error = %q, want to contain 'no skills available'", err.Error())
		}
	})
}

// containsStr is a simple substring check for test assertions.
func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestRegistryManager_Remove(t *testing.T) {
	t.Run("removes registry clone", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoURL := "git@example.com:org/to-delete.git"
		regDir := createTestRegistryClone(t, registriesDir, repoURL, RegistryManifest{Name: "to-delete"})

		err := rm.Remove(repoURL)
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

		err := rm.Remove("git@example.com:nonexistent.git")
		if err == nil {
			t.Fatal("expected error for nonexistent registry")
		}
	})

	t.Run("error when repo URL is empty", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		err := rm.Remove("")
		if err == nil {
			t.Fatal("expected error for empty repo URL")
		}
	})

	t.Run("error when repo URL is whitespace only", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		err := rm.Remove("   ")
		if err == nil {
			t.Fatal("expected error for whitespace-only repo URL")
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

		// Verify clone exists at repo-key-derived location
		dirKey := RegistryDirKey(bareRepo)
		keyedDir := filepath.Join(registriesDir, dirKey)
		if !dirExists(keyedDir) {
			t.Error("registry not cloned to expected location")
		}

		// Verify manifest can be loaded by repo URL
		loaded, err := rm.LoadManifest(bareRepo)
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

	t.Run("error when URL is whitespace only", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		_, err := rm.Add("   ")
		if err == nil {
			t.Fatal("expected error for whitespace-only URL")
		}
	})

	t.Run("trims trailing whitespace from URL", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		bareRepo := t.TempDir()
		setupTestGitRepo(t, bareRepo)

		// Add with trailing spaces — should succeed.
		manifest, err := rm.Add(bareRepo + "  ")
		if err != nil {
			t.Fatalf("Add() with trailing spaces error = %v", err)
		}
		if manifest.Name != "test-org" {
			t.Errorf("manifest.Name = %q, want %q", manifest.Name, "test-org")
		}

		// Clone should be stored under the trimmed URL's key.
		dirKey := RegistryDirKey(bareRepo)
		if !dirExists(filepath.Join(registriesDir, dirKey)) {
			t.Error("registry should be stored under trimmed URL key")
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

		// Refresh should succeed (even though nothing changed) — use repo URL
		manifest, err := rm.Refresh(bareRepo)
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

		_, err := rm.Refresh("git@example.com:nonexistent.git")
		if err == nil {
			t.Fatal("expected error for nonexistent registry")
		}
	})

	t.Run("error when repo URL is empty", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		_, err := rm.Refresh("")
		if err == nil {
			t.Fatal("expected error for empty repo URL")
		}
	})

	t.Run("error when repo URL is whitespace only", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		_, err := rm.Refresh("   ")
		if err == nil {
			t.Fatal("expected error for whitespace-only repo URL")
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
		dirKey := RegistryDirKey(bareRepo)
		cloneDir := filepath.Join(registriesDir, dirKey)
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
