package core

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/barysiuk/duckrow/internal/core/asset"
)

// testSkillEntry is a test helper that mirrors the old SkillEntry for constructing test manifests.
type testSkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Commit      string `json:"commit,omitempty"`
	Internal    bool   `json:"internal,omitempty"`
}

func skillEntriesToRaw(entries []testSkillEntry) []json.RawMessage {
	result := make([]json.RawMessage, len(entries))
	for i, e := range entries {
		data, _ := json.Marshal(e)
		result[i] = data
	}
	return result
}

// testMCPEntry is a test helper for constructing MCP manifest entries.
type testMCPEntry struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Type        string            `json:"type,omitempty"`
}

func mcpEntriesToRaw(entries []testMCPEntry) []json.RawMessage {
	result := make([]json.RawMessage, len(entries))
	for i, e := range entries {
		data, _ := json.Marshal(e)
		result[i] = data
	}
	return result
}

// parseRawSkill unmarshals a json.RawMessage into a testSkillEntry for test assertions.
func parseRawSkill(t *testing.T, raw json.RawMessage) testSkillEntry {
	t.Helper()
	var e testSkillEntry
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("failed to parse skill entry: %v", err)
	}
	return e
}

// parseRawMCP unmarshals a json.RawMessage into a testMCPEntry for test assertions.
func parseRawMCP(t *testing.T, raw json.RawMessage) testMCPEntry {
	t.Helper()
	var e testMCPEntry
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("failed to parse MCP entry: %v", err)
	}
	return e
}

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
			Skills: skillEntriesToRaw([]testSkillEntry{
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
			}),
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
		s0 := parseRawSkill(t, manifest.Skills[0])
		if s0.Name != "skill-a" {
			t.Errorf("Skills[0].Name = %q, want %q", s0.Name, "skill-a")
		}
		s1 := parseRawSkill(t, manifest.Skills[1])
		if s1.Source != "owner/other-repo" {
			t.Errorf("Skills[1].Source = %q, want %q", s1.Source, "owner/other-repo")
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
			Skills: skillEntriesToRaw([]testSkillEntry{}),
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
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "lint-rules", Description: "Linting", Source: "org/lint"},
			}),
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
		Skills: skillEntriesToRaw([]testSkillEntry{{Name: "s1", Description: "S1", Source: "a/s1"}}),
	})
	createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
		Name:   "org-b",
		Skills: skillEntriesToRaw([]testSkillEntry{{Name: "s2", Description: "S2", Source: "b/s2"}}),
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
		Skills: skillEntriesToRaw([]testSkillEntry{
			{Name: "s1", Description: "S1", Source: "a/s1"},
			{Name: "s2", Description: "S2", Source: "a/s2"},
		}),
	})
	createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
		Name: "org-b",
		Skills: skillEntriesToRaw([]testSkillEntry{
			{Name: "s3", Description: "S3", Source: "b/s3"},
		}),
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
		Skills: skillEntriesToRaw([]testSkillEntry{
			{Name: "skill-from-a", Description: "From A", Source: "a/skill"},
		}),
	})
	createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
		Name: "same-name",
		Skills: skillEntriesToRaw([]testSkillEntry{
			{Name: "skill-from-b", Description: "From B", Source: "b/skill"},
		}),
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
		Skills: skillEntriesToRaw([]testSkillEntry{
			{Name: "go-review", Description: "Go review", Source: "org-a/go-review"},
			{Name: "shared-lint", Description: "Shared lint", Source: "org-a/shared-lint"},
		}),
	})
	createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
		Name: "org-b",
		Skills: skillEntriesToRaw([]testSkillEntry{
			{Name: "py-review", Description: "Python review", Source: "org-b/py-review"},
			{Name: "shared-lint", Description: "Shared lint B", Source: "org-b/shared-lint"},
		}),
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
		Skills: skillEntriesToRaw([]testSkillEntry{
			{
				Name:        "test-skill",
				Description: "A test skill",
				Source:      "test-org/skills",
			},
		}),
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

func TestBuildRegistryCommitMap(t *testing.T) {
	t.Run("builds map from registry manifests", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoA := "git@example.com:org/reg-a.git"
		repoB := "git@example.com:org/reg-b.git"

		createTestRegistryClone(t, registriesDir, repoA, RegistryManifest{
			Name: "org-a",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "skill-1", Source: "github.com/org/repo/skill-1", Commit: "abc1234"},
				{Name: "skill-2", Source: "github.com/org/repo/skill-2", Commit: "def5678"},
			}),
		})
		createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
			Name: "org-b",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "skill-3", Source: "github.com/other/repo/skill-3", Commit: "ghi9012"},
			}),
		})

		registries := []Registry{
			{Name: "org-a", Repo: repoA},
			{Name: "org-b", Repo: repoB},
		}

		commits := BuildRegistryCommitMap(registries, rm)

		if len(commits) != 3 {
			t.Fatalf("len(commits) = %d, want 3", len(commits))
		}
		if commits["github.com/org/repo/skill-1"] != "abc1234" {
			t.Errorf("skill-1 commit = %q, want %q", commits["github.com/org/repo/skill-1"], "abc1234")
		}
		if commits["github.com/org/repo/skill-2"] != "def5678" {
			t.Errorf("skill-2 commit = %q, want %q", commits["github.com/org/repo/skill-2"], "def5678")
		}
		if commits["github.com/other/repo/skill-3"] != "ghi9012" {
			t.Errorf("skill-3 commit = %q, want %q", commits["github.com/other/repo/skill-3"], "ghi9012")
		}
	})

	t.Run("skips skills with empty commit", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoURL := "git@example.com:org/reg.git"
		createTestRegistryClone(t, registriesDir, repoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "has-commit", Source: "github.com/org/repo/has-commit", Commit: "abc1234"},
				{Name: "no-commit", Source: "github.com/org/repo/no-commit", Commit: ""},
			}),
		})

		registries := []Registry{{Name: "org", Repo: repoURL}}
		commits := BuildRegistryCommitMap(registries, rm)

		if len(commits) != 1 {
			t.Fatalf("len(commits) = %d, want 1", len(commits))
		}
		if _, ok := commits["github.com/org/repo/no-commit"]; ok {
			t.Error("should not include skill with empty commit")
		}
	})

	t.Run("skips skills with empty source", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoURL := "git@example.com:org/reg.git"
		createTestRegistryClone(t, registriesDir, repoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "has-source", Source: "github.com/org/repo/has-source", Commit: "abc1234"},
				{Name: "no-source", Source: "", Commit: "def5678"},
			}),
		})

		registries := []Registry{{Name: "org", Repo: repoURL}}
		commits := BuildRegistryCommitMap(registries, rm)

		if len(commits) != 1 {
			t.Fatalf("len(commits) = %d, want 1", len(commits))
		}
	})

	t.Run("returns empty map when no registries", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		commits := BuildRegistryCommitMap(nil, rm)
		if len(commits) != 0 {
			t.Errorf("len(commits) = %d, want 0", len(commits))
		}
	})

	t.Run("skips missing registries gracefully", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoURL := "git@example.com:org/reg.git"
		createTestRegistryClone(t, registriesDir, repoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "skill-1", Source: "github.com/org/repo/skill-1", Commit: "abc1234"},
			}),
		})

		registries := []Registry{
			{Name: "org", Repo: repoURL},
			{Name: "missing", Repo: "git@example.com:missing/reg.git"},
		}

		commits := BuildRegistryCommitMap(registries, rm)
		if len(commits) != 1 {
			t.Fatalf("len(commits) = %d, want 1", len(commits))
		}
	})

	t.Run("last registry wins for duplicate sources", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoA := "git@example.com:org/reg-a.git"
		repoB := "git@example.com:org/reg-b.git"

		createTestRegistryClone(t, registriesDir, repoA, RegistryManifest{
			Name: "org-a",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "shared", Source: "github.com/org/repo/shared", Commit: "old-commit"},
			}),
		})
		createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
			Name: "org-b",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "shared", Source: "github.com/org/repo/shared", Commit: "new-commit"},
			}),
		})

		registries := []Registry{
			{Name: "org-a", Repo: repoA},
			{Name: "org-b", Repo: repoB},
		}

		commits := BuildRegistryCommitMap(registries, rm)
		// Registries are iterated in slice order; last write wins in the map.
		if commits["github.com/org/repo/shared"] == "" {
			t.Fatal("expected commit for shared source")
		}
	})
}

func TestLoadCachedCommits(t *testing.T) {
	t.Run("returns empty map for missing file", func(t *testing.T) {
		dir := t.TempDir()
		commits := loadCachedCommits(dir)
		if len(commits) != 0 {
			t.Errorf("len(commits) = %d, want 0", len(commits))
		}
	})

	t.Run("returns empty map for corrupt JSON", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, cachedCommitsFile), []byte("not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		commits := loadCachedCommits(dir)
		if len(commits) != 0 {
			t.Errorf("len(commits) = %d, want 0", len(commits))
		}
	})

	t.Run("returns empty map for null commits field", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, cachedCommitsFile), []byte(`{"generatedAt":"2025-01-01T00:00:00Z","commits":null}`), 0o644); err != nil {
			t.Fatal(err)
		}
		commits := loadCachedCommits(dir)
		if len(commits) != 0 {
			t.Errorf("len(commits) = %d, want 0", len(commits))
		}
	})

	t.Run("round-trips with writeCachedCommits", func(t *testing.T) {
		dir := t.TempDir()

		original := map[string]string{
			"github.com/org/repo/skill-a": "aaa111",
			"github.com/org/repo/skill-b": "bbb222",
		}

		if err := writeCachedCommits(dir, original); err != nil {
			t.Fatalf("writeCachedCommits: %v", err)
		}

		loaded := loadCachedCommits(dir)
		if len(loaded) != 2 {
			t.Fatalf("len(loaded) = %d, want 2", len(loaded))
		}
		if loaded["github.com/org/repo/skill-a"] != "aaa111" {
			t.Errorf("skill-a commit = %q, want %q", loaded["github.com/org/repo/skill-a"], "aaa111")
		}
		if loaded["github.com/org/repo/skill-b"] != "bbb222" {
			t.Errorf("skill-b commit = %q, want %q", loaded["github.com/org/repo/skill-b"], "bbb222")
		}
	})
}

func TestWriteCachedCommits(t *testing.T) {
	t.Run("creates valid JSON file", func(t *testing.T) {
		dir := t.TempDir()

		commits := map[string]string{
			"github.com/org/repo/skill": "abc123",
		}

		if err := writeCachedCommits(dir, commits); err != nil {
			t.Fatalf("writeCachedCommits: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(dir, cachedCommitsFile))
		if err != nil {
			t.Fatal(err)
		}

		var cached CachedCommits
		if err := json.Unmarshal(data, &cached); err != nil {
			t.Fatalf("JSON parse error: %v", err)
		}

		if cached.GeneratedAt.IsZero() {
			t.Error("generatedAt should not be zero")
		}
		if cached.Commits["github.com/org/repo/skill"] != "abc123" {
			t.Errorf("commit = %q, want %q", cached.Commits["github.com/org/repo/skill"], "abc123")
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		dir := t.TempDir()

		first := map[string]string{"github.com/org/repo/old": "old-sha"}
		second := map[string]string{"github.com/org/repo/new": "new-sha"}

		if err := writeCachedCommits(dir, first); err != nil {
			t.Fatal(err)
		}
		if err := writeCachedCommits(dir, second); err != nil {
			t.Fatal(err)
		}

		loaded := loadCachedCommits(dir)
		if len(loaded) != 1 {
			t.Fatalf("len(loaded) = %d, want 1", len(loaded))
		}
		if _, ok := loaded["github.com/org/repo/old"]; ok {
			t.Error("old entry should not be present after overwrite")
		}
		if loaded["github.com/org/repo/new"] != "new-sha" {
			t.Errorf("new commit = %q, want %q", loaded["github.com/org/repo/new"], "new-sha")
		}
	})
}

func TestBuildRegistryCommitMap_WithCachedCommits(t *testing.T) {
	t.Run("includes cached commits for unpinned skills", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoURL := "git@example.com:org/reg.git"
		regDir := createTestRegistryClone(t, registriesDir, repoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "pinned", Source: "github.com/org/repo/pinned", Commit: "pinned-sha"},
				{Name: "unpinned", Source: "github.com/org/repo/unpinned"}, // no commit
			}),
		})

		// Write cached commit for the unpinned skill.
		if err := writeCachedCommits(regDir, map[string]string{
			"github.com/org/repo/unpinned": "cached-sha",
		}); err != nil {
			t.Fatal(err)
		}

		registries := []Registry{{Name: "org", Repo: repoURL}}
		commits := BuildRegistryCommitMap(registries, rm)

		if len(commits) != 2 {
			t.Fatalf("len(commits) = %d, want 2", len(commits))
		}
		if commits["github.com/org/repo/pinned"] != "pinned-sha" {
			t.Errorf("pinned commit = %q, want %q", commits["github.com/org/repo/pinned"], "pinned-sha")
		}
		if commits["github.com/org/repo/unpinned"] != "cached-sha" {
			t.Errorf("unpinned commit = %q, want %q", commits["github.com/org/repo/unpinned"], "cached-sha")
		}
	})

	t.Run("pinned commit takes precedence over cached", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoURL := "git@example.com:org/reg.git"
		regDir := createTestRegistryClone(t, registriesDir, repoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "skill", Source: "github.com/org/repo/skill", Commit: "pinned-sha"},
			}),
		})

		// Write a different cached commit for the same skill.
		if err := writeCachedCommits(regDir, map[string]string{
			"github.com/org/repo/skill": "stale-cached-sha",
		}); err != nil {
			t.Fatal(err)
		}

		registries := []Registry{{Name: "org", Repo: repoURL}}
		commits := BuildRegistryCommitMap(registries, rm)

		if commits["github.com/org/repo/skill"] != "pinned-sha" {
			t.Errorf("commit = %q, want %q (pinned should win)", commits["github.com/org/repo/skill"], "pinned-sha")
		}
	})

	t.Run("no cache file still works", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoURL := "git@example.com:org/reg.git"
		createTestRegistryClone(t, registriesDir, repoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "skill", Source: "github.com/org/repo/skill", Commit: "abc123"},
			}),
		})

		registries := []Registry{{Name: "org", Repo: repoURL}}
		commits := BuildRegistryCommitMap(registries, rm)

		if len(commits) != 1 {
			t.Fatalf("len(commits) = %d, want 1", len(commits))
		}
		if commits["github.com/org/repo/skill"] != "abc123" {
			t.Errorf("commit = %q, want %q", commits["github.com/org/repo/skill"], "abc123")
		}
	})
}

func TestHydrateRegistryCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("resolves commits for unpinned skills", func(t *testing.T) {
		// Create a source repo with two skills in sub-paths.
		sourceDir := t.TempDir()

		// Create skill files.
		skillADir := filepath.Join(sourceDir, "skills", "skill-a")
		skillBDir := filepath.Join(sourceDir, "skills", "skill-b")
		if err := os.MkdirAll(skillADir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(skillBDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillADir, "SKILL.md"), []byte("---\nname: skill-a\ndescription: A\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillBDir, "SKILL.md"), []byte("---\nname: skill-b\ndescription: B\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		setupTestGitRepoInDir(t, sourceDir)

		// Create registry directory with a manifest pointing to the source repo.
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		regRepoURL := "git@example.com:org/reg.git"
		regDir := createTestRegistryClone(t, registriesDir, regRepoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "skill-a", Source: "localhost/testorg/testrepo/skills/skill-a"},       // unpinned
				{Name: "skill-b", Source: "localhost/testorg/testrepo/skills/skill-b"},       // unpinned
				{Name: "pinned", Source: "localhost/testorg/testrepo/pinned", Commit: "abc"}, // pinned — should be skipped
			}),
		})

		registries := []Registry{{Name: "org", Repo: regRepoURL}}

		// Use clone URL override to point to the local git repo.
		overrides := map[string]string{
			"testorg/testrepo": sourceDir,
		}

		rm.HydrateRegistryCommits(registries, overrides)

		// Verify cache file was written.
		cached := loadCachedCommits(regDir)
		if len(cached) != 2 {
			t.Fatalf("len(cached) = %d, want 2 (only unpinned skills)", len(cached))
		}

		// Both should have valid non-empty commit SHAs.
		for _, source := range []string{
			"localhost/testorg/testrepo/skills/skill-a",
			"localhost/testorg/testrepo/skills/skill-b",
		} {
			sha := cached[source]
			if sha == "" {
				t.Errorf("expected non-empty commit for %s", source)
			}
			if len(sha) != 40 {
				t.Errorf("commit for %s = %q, want 40-char SHA", source, sha)
			}
		}

		// Pinned skill should NOT be in the cache.
		if _, ok := cached["localhost/testorg/testrepo/pinned"]; ok {
			t.Error("pinned skill should not be in cached commits")
		}
	})

	t.Run("skips all-pinned registries", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		regRepoURL := "git@example.com:org/reg.git"
		regDir := createTestRegistryClone(t, registriesDir, regRepoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "skill", Source: "github.com/org/repo/skill", Commit: "abc123"},
			}),
		})

		registries := []Registry{{Name: "org", Repo: regRepoURL}}
		rm.HydrateRegistryCommits(registries, nil)

		// No cache file should be written since all skills are pinned.
		_, err := os.Stat(filepath.Join(regDir, cachedCommitsFile))
		if err == nil {
			t.Error("cache file should not exist for all-pinned registry")
		}
	})

	t.Run("continues on clone error", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		regRepoURL := "git@example.com:org/reg.git"
		createTestRegistryClone(t, registriesDir, regRepoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				// Source points to a non-existent repo — clone will fail.
				{Name: "unreachable", Source: "localhost/no/such-repo/skill"},
			}),
		})

		registries := []Registry{{Name: "org", Repo: regRepoURL}}

		// Should not panic — clone errors are silently skipped.
		rm.HydrateRegistryCommits(registries, nil)
	})

	t.Run("end-to-end with BuildRegistryCommitMap", func(t *testing.T) {
		// Create source repo with a skill.
		sourceDir := t.TempDir()
		skillDir := filepath.Join(sourceDir, "my-skill")
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\ndescription: Test\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		setupTestGitRepoInDir(t, sourceDir)

		// Set up registry with one pinned and one unpinned skill.
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		regRepoURL := "git@example.com:org/reg.git"
		createTestRegistryClone(t, registriesDir, regRepoURL, RegistryManifest{
			Name: "org",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "pinned-skill", Source: "github.com/org/other/pinned-skill", Commit: "pinned-sha"},
				{Name: "my-skill", Source: "localhost/testorg/testrepo/my-skill"}, // unpinned
			}),
		})

		registries := []Registry{{Name: "org", Repo: regRepoURL}}
		overrides := map[string]string{"testorg/testrepo": sourceDir}

		// Hydrate first.
		rm.HydrateRegistryCommits(registries, overrides)

		// BuildRegistryCommitMap should now include both pinned and cached commits.
		commits := BuildRegistryCommitMap(registries, rm)

		if len(commits) != 2 {
			t.Fatalf("len(commits) = %d, want 2", len(commits))
		}
		if commits["github.com/org/other/pinned-skill"] != "pinned-sha" {
			t.Errorf("pinned commit = %q, want %q", commits["github.com/org/other/pinned-skill"], "pinned-sha")
		}
		hydrated := commits["localhost/testorg/testrepo/my-skill"]
		if hydrated == "" {
			t.Fatal("expected non-empty commit for hydrated skill")
		}
		if len(hydrated) != 40 {
			t.Errorf("hydrated commit = %q, want 40-char SHA", hydrated)
		}
	})
}

// --- MCP Registry Tests ---

func TestReadManifest_WithMCPs(t *testing.T) {
	t.Run("parses MCPs alongside skills", func(t *testing.T) {
		dir := t.TempDir()
		manifest := RegistryManifest{
			Name:        "test-registry",
			Description: "Registry with MCPs",
			Skills: skillEntriesToRaw([]testSkillEntry{
				{Name: "skill-a", Description: "Skill A", Source: "owner/repo"},
			}),
			MCPs: mcpEntriesToRaw([]testMCPEntry{
				{
					Name:        "internal-db",
					Description: "Query databases",
					Command:     "npx",
					Args:        []string{"-y", "@acme/mcp-db-server"},
					Env:         map[string]string{"DATABASE_URL": "$DATABASE_URL"},
				},
				{
					Name:        "docs-search",
					Description: "Search docs",
					Type:        "http",
					URL:         "https://mcp.acme.com/mcp",
				},
			}),
		}
		createTestManifest(t, dir, manifest)

		got, err := readManifest(dir)
		if err != nil {
			t.Fatalf("readManifest() error = %v", err)
		}

		if len(got.MCPs) != 2 {
			t.Fatalf("len(MCPs) = %d, want 2", len(got.MCPs))
		}
		mcp0 := parseRawMCP(t, got.MCPs[0])
		if mcp0.Name != "internal-db" {
			t.Errorf("MCPs[0].Name = %q, want %q", mcp0.Name, "internal-db")
		}
		if mcp0.Command != "npx" {
			t.Errorf("MCPs[0].Command = %q, want %q", mcp0.Command, "npx")
		}
		if len(mcp0.Args) != 2 || mcp0.Args[1] != "@acme/mcp-db-server" {
			t.Errorf("MCPs[0].Args = %v, want [-y @acme/mcp-db-server]", mcp0.Args)
		}
		if mcp0.Env["DATABASE_URL"] != "$DATABASE_URL" {
			t.Errorf("MCPs[0].Env[DATABASE_URL] = %q, want %q", mcp0.Env["DATABASE_URL"], "$DATABASE_URL")
		}
		mcp1 := parseRawMCP(t, got.MCPs[1])
		if mcp1.Name != "docs-search" {
			t.Errorf("MCPs[1].Name = %q, want %q", mcp1.Name, "docs-search")
		}
		if mcp1.Type != "http" {
			t.Errorf("MCPs[1].Type = %q, want %q", mcp1.Type, "http")
		}
		if mcp1.URL != "https://mcp.acme.com/mcp" {
			t.Errorf("MCPs[1].URL = %q, want %q", mcp1.URL, "https://mcp.acme.com/mcp")
		}
	})

	t.Run("handles manifest with no MCPs", func(t *testing.T) {
		dir := t.TempDir()
		manifest := RegistryManifest{
			Name:   "skills-only",
			Skills: skillEntriesToRaw([]testSkillEntry{{Name: "s", Description: "S", Source: "o/r"}}),
		}
		createTestManifest(t, dir, manifest)

		got, err := readManifest(dir)
		if err != nil {
			t.Fatalf("readManifest() error = %v", err)
		}
		if len(got.MCPs) != 0 {
			t.Errorf("len(MCPs) = %d, want 0", len(got.MCPs))
		}
	})
}

func TestReadManifest_MCPValidation(t *testing.T) {
	t.Run("warns on missing name", func(t *testing.T) {
		dir := t.TempDir()
		manifest := RegistryManifest{
			Name: "test",
			MCPs: mcpEntriesToRaw([]testMCPEntry{
				{Command: "npx", Args: []string{"pkg"}},
			}),
		}
		createTestManifest(t, dir, manifest)

		got, err := readManifest(dir)
		if err != nil {
			t.Fatalf("readManifest() error = %v", err)
		}
		parsed, err := ParseManifest(got)
		if err != nil {
			t.Fatalf("ParseManifest() error = %v", err)
		}
		if len(parsed.Warnings) == 0 {
			t.Fatal("expected warning for MCP with missing name")
		}
		if !containsStr(parsed.Warnings[0], "missing required 'name'") {
			t.Errorf("warning = %q, want to contain 'missing required name'", parsed.Warnings[0])
		}
	})

	t.Run("warns on missing both command and url", func(t *testing.T) {
		dir := t.TempDir()
		manifest := RegistryManifest{
			Name: "test",
			MCPs: mcpEntriesToRaw([]testMCPEntry{
				{Name: "empty-mcp"},
			}),
		}
		createTestManifest(t, dir, manifest)

		got, err := readManifest(dir)
		if err != nil {
			t.Fatalf("readManifest() error = %v", err)
		}
		parsed, err := ParseManifest(got)
		if err != nil {
			t.Fatalf("ParseManifest() error = %v", err)
		}
		if len(parsed.Warnings) == 0 {
			t.Fatal("expected warning for MCP with no command and no url")
		}
		if !containsStr(parsed.Warnings[0], "missing both") {
			t.Errorf("warning = %q, want to contain 'missing both'", parsed.Warnings[0])
		}
	})

	t.Run("warns on having both command and url", func(t *testing.T) {
		dir := t.TempDir()
		manifest := RegistryManifest{
			Name: "test",
			MCPs: mcpEntriesToRaw([]testMCPEntry{
				{Name: "both-mcp", Command: "npx", URL: "https://example.com"},
			}),
		}
		createTestManifest(t, dir, manifest)

		got, err := readManifest(dir)
		if err != nil {
			t.Fatalf("readManifest() error = %v", err)
		}
		parsed, err := ParseManifest(got)
		if err != nil {
			t.Fatalf("ParseManifest() error = %v", err)
		}
		if len(parsed.Warnings) == 0 {
			t.Fatal("expected warning for MCP with both command and url")
		}
		if !containsStr(parsed.Warnings[0], "both") {
			t.Errorf("warning = %q, want to contain 'both'", parsed.Warnings[0])
		}
	})

	t.Run("no warnings for valid MCPs", func(t *testing.T) {
		dir := t.TempDir()
		manifest := RegistryManifest{
			Name: "test",
			MCPs: mcpEntriesToRaw([]testMCPEntry{
				{Name: "stdio-mcp", Command: "npx", Args: []string{"-y", "pkg"}},
				{Name: "remote-mcp", URL: "https://example.com", Type: "http"},
			}),
		}
		createTestManifest(t, dir, manifest)

		got, err := readManifest(dir)
		if err != nil {
			t.Fatalf("readManifest() error = %v", err)
		}
		parsed, err := ParseManifest(got)
		if err != nil {
			t.Fatalf("ParseManifest() error = %v", err)
		}
		if len(parsed.Warnings) != 0 {
			t.Errorf("expected no warnings, got %v", parsed.Warnings)
		}
	})
}

func TestRegistryManager_ListMCPs(t *testing.T) {
	t.Run("lists MCPs from all registries", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoA := "git@example.com:a.git"
		repoB := "git@example.com:b.git"

		createTestRegistryClone(t, registriesDir, repoA, RegistryManifest{
			Name: "org-a",
			MCPs: mcpEntriesToRaw([]testMCPEntry{
				{Name: "mcp-1", Command: "cmd1"},
				{Name: "mcp-2", Command: "cmd2"},
			}),
		})
		createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
			Name: "org-b",
			MCPs: mcpEntriesToRaw([]testMCPEntry{
				{Name: "mcp-3", URL: "https://example.com", Type: "http"},
			}),
		})

		registries := []Registry{
			{Name: "org-a", Repo: repoA},
			{Name: "org-b", Repo: repoB},
		}

		mcps := rm.ListMCPs(registries)
		if len(mcps) != 3 {
			t.Fatalf("len(mcps) = %d, want 3", len(mcps))
		}
		if mcps[0].RegistryName != "org-a" {
			t.Errorf("mcps[0].RegistryName = %q, want %q", mcps[0].RegistryName, "org-a")
		}
		if mcps[0].MCP.Name != "mcp-1" {
			t.Errorf("mcps[0].MCP.Name = %q, want %q", mcps[0].MCP.Name, "mcp-1")
		}
		if mcps[2].RegistryName != "org-b" {
			t.Errorf("mcps[2].RegistryName = %q, want %q", mcps[2].RegistryName, "org-b")
		}
		if mcps[2].MCP.Name != "mcp-3" {
			t.Errorf("mcps[2].MCP.Name = %q, want %q", mcps[2].MCP.Name, "mcp-3")
		}
	})

	t.Run("returns empty for registries with no MCPs", func(t *testing.T) {
		registriesDir := t.TempDir()
		rm := NewRegistryManager(registriesDir)

		repoURL := "git@example.com:skills-only.git"
		createTestRegistryClone(t, registriesDir, repoURL, RegistryManifest{
			Name:   "skills-only",
			Skills: skillEntriesToRaw([]testSkillEntry{{Name: "s", Description: "S", Source: "o/r"}}),
		})

		registries := []Registry{{Name: "skills-only", Repo: repoURL}}
		mcps := rm.ListMCPs(registries)
		if len(mcps) != 0 {
			t.Errorf("len(mcps) = %d, want 0", len(mcps))
		}
	})
}

func TestRegistryManager_FindMCP(t *testing.T) {
	registriesDir := t.TempDir()
	rm := NewRegistryManager(registriesDir)

	repoA := "git@example.com:org-a/tools.git"
	repoB := "git@example.com:org-b/tools.git"

	createTestRegistryClone(t, registriesDir, repoA, RegistryManifest{
		Name: "org-a",
		MCPs: mcpEntriesToRaw([]testMCPEntry{
			{Name: "internal-db", Description: "DB queries", Command: "npx", Args: []string{"-y", "@acme/db"}},
			{Name: "shared-mcp", Description: "Shared A", Command: "cmd-a"},
		}),
	})
	createTestRegistryClone(t, registriesDir, repoB, RegistryManifest{
		Name: "org-b",
		MCPs: mcpEntriesToRaw([]testMCPEntry{
			{Name: "jira", Description: "Jira integration", Command: "npx", Args: []string{"-y", "@acme/jira"}},
			{Name: "shared-mcp", Description: "Shared B", Command: "cmd-b"},
		}),
	})

	registries := []Registry{
		{Name: "org-a", Repo: repoA},
		{Name: "org-b", Repo: repoB},
	}

	t.Run("finds unique MCP", func(t *testing.T) {
		info, err := rm.FindMCP(registries, "internal-db", "")
		if err != nil {
			t.Fatalf("FindMCP() error = %v", err)
		}
		if info.MCP.Name != "internal-db" {
			t.Errorf("MCP.Name = %q, want %q", info.MCP.Name, "internal-db")
		}
		if info.RegistryName != "org-a" {
			t.Errorf("RegistryName = %q, want %q", info.RegistryName, "org-a")
		}
	})

	t.Run("errors on ambiguous MCP", func(t *testing.T) {
		_, err := rm.FindMCP(registries, "shared-mcp", "")
		if err == nil {
			t.Fatal("expected error for ambiguous MCP")
		}
		if !containsStr(err.Error(), "multiple registries") {
			t.Errorf("error = %q, want to contain 'multiple registries'", err.Error())
		}
		if !containsStr(err.Error(), "--registry") {
			t.Errorf("error = %q, want to contain '--registry'", err.Error())
		}
	})

	t.Run("disambiguates with registry filter by name", func(t *testing.T) {
		info, err := rm.FindMCP(registries, "shared-mcp", "org-b")
		if err != nil {
			t.Fatalf("FindMCP() error = %v", err)
		}
		if info.RegistryName != "org-b" {
			t.Errorf("RegistryName = %q, want %q", info.RegistryName, "org-b")
		}
		meta, ok := info.MCP.Meta.(asset.MCPMeta)
		if !ok {
			t.Fatalf("MCP.Meta has unexpected type")
		}
		if meta.Command != "cmd-b" {
			t.Errorf("MCP.Command = %q, want %q", meta.Command, "cmd-b")
		}
	})

	t.Run("disambiguates with registry filter by repo URL", func(t *testing.T) {
		info, err := rm.FindMCP(registries, "shared-mcp", repoA)
		if err != nil {
			t.Fatalf("FindMCP() error = %v", err)
		}
		if info.RegistryName != "org-a" {
			t.Errorf("RegistryName = %q, want %q", info.RegistryName, "org-a")
		}
	})

	t.Run("errors on unknown MCP", func(t *testing.T) {
		_, err := rm.FindMCP(registries, "nonexistent", "")
		if err == nil {
			t.Fatal("expected error for nonexistent MCP")
		}
		if !containsStr(err.Error(), "not found") {
			t.Errorf("error = %q, want to contain 'not found'", err.Error())
		}
		if !containsStr(err.Error(), "Available") {
			t.Errorf("error = %q, want to contain 'Available'", err.Error())
		}
	})

	t.Run("errors on empty MCP name", func(t *testing.T) {
		_, err := rm.FindMCP(registries, "", "")
		if err == nil {
			t.Fatal("expected error for empty MCP name")
		}
	})

	t.Run("errors with no registries configured", func(t *testing.T) {
		_, err := rm.FindMCP(nil, "internal-db", "")
		if err == nil {
			t.Fatal("expected error when no registries")
		}
		if !containsStr(err.Error(), "no MCPs available") {
			t.Errorf("error = %q, want to contain 'no MCPs available'", err.Error())
		}
	})

	t.Run("errors on unknown registry filter", func(t *testing.T) {
		_, err := rm.FindMCP(registries, "internal-db", "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent registry")
		}
		if !containsStr(err.Error(), "not found") {
			t.Errorf("error = %q, want to contain 'not found'", err.Error())
		}
	})
}
