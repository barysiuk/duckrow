package core

import (
	"os"
	"path/filepath"
	"testing"
)

// testAgents returns a minimal set of agents for remover tests.
func removerTestAgents() []AgentDef {
	return []AgentDef{
		{
			Name:        "opencode",
			DisplayName: "OpenCode",
			SkillsDir:   ".agents/skills",
			Universal:   true,
		},
		{
			Name:        "claude-code",
			DisplayName: "Claude Code",
			SkillsDir:   ".claude/skills",
			Universal:   false,
		},
		{
			Name:        "cursor",
			DisplayName: "Cursor",
			SkillsDir:   ".cursor/skills",
			Universal:   false,
		},
	}
}

// setupInstalledSkill creates a canonical skill directory and agent symlinks,
// simulating what the Installer would have created.
func setupInstalledSkill(t *testing.T, projectDir, skillName string, agents []AgentDef) {
	t.Helper()

	// Create canonical directory with a SKILL.md
	canonicalDir := filepath.Join(projectDir, ".agents", "skills", skillName)
	if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMd := "---\nname: " + skillName + "\ndescription: Test skill\n---\n# " + skillName
	if err := os.WriteFile(filepath.Join(canonicalDir, "SKILL.md"), []byte(skillMd), 0o644); err != nil {
		t.Fatal(err)
	}
	// Add an extra file to be thorough
	if err := os.WriteFile(filepath.Join(canonicalDir, "extra.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create symlinks for non-universal agents
	for _, agent := range agents {
		if agent.Universal {
			continue
		}
		agentSkillDir := filepath.Join(projectDir, agent.SkillsDir)
		if err := os.MkdirAll(agentSkillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(agentSkillDir, skillName)
		rel, err := filepath.Rel(agentSkillDir, canonicalDir)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(rel, linkPath); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRemover_Remove(t *testing.T) {
	agents := removerTestAgents()
	remover := NewRemover(agents)

	t.Run("removes canonical dir and agent symlinks", func(t *testing.T) {
		projectDir := t.TempDir()
		setupInstalledSkill(t, projectDir, "test-skill", agents)

		// Verify setup
		canonicalPath := filepath.Join(projectDir, ".agents", "skills", "test-skill")
		if !dirExists(canonicalPath) {
			t.Fatal("setup failed: canonical dir does not exist")
		}
		claudeLink := filepath.Join(projectDir, ".claude", "skills", "test-skill")
		if !pathExists(claudeLink) {
			t.Fatal("setup failed: claude symlink does not exist")
		}

		result, err := remover.Remove("test-skill", RemoveOptions{TargetDir: projectDir})
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		// Verify result
		if result.Name != "test-skill" {
			t.Errorf("result.Name = %q, want %q", result.Name, "test-skill")
		}
		if result.CanonicalPath != canonicalPath {
			t.Errorf("result.CanonicalPath = %q, want %q", result.CanonicalPath, canonicalPath)
		}
		if len(result.RemovedSymlinks) != 2 {
			t.Errorf("len(result.RemovedSymlinks) = %d, want 2", len(result.RemovedSymlinks))
		}

		// Verify canonical dir is gone
		if dirExists(canonicalPath) {
			t.Error("canonical directory still exists after removal")
		}

		// Verify symlinks are gone
		if pathExists(claudeLink) {
			t.Error("claude symlink still exists after removal")
		}
		cursorLink := filepath.Join(projectDir, ".cursor", "skills", "test-skill")
		if pathExists(cursorLink) {
			t.Error("cursor symlink still exists after removal")
		}
	})

	t.Run("error when skill does not exist", func(t *testing.T) {
		projectDir := t.TempDir()

		_, err := remover.Remove("nonexistent", RemoveOptions{TargetDir: projectDir})
		if err == nil {
			t.Fatal("expected error for nonexistent skill")
		}
	})

	t.Run("error when target dir is empty", func(t *testing.T) {
		_, err := remover.Remove("test-skill", RemoveOptions{TargetDir: ""})
		if err == nil {
			t.Fatal("expected error for empty target dir")
		}
	})

	t.Run("error when skill name is empty", func(t *testing.T) {
		_, err := remover.Remove("", RemoveOptions{TargetDir: "/tmp"})
		if err == nil {
			t.Fatal("expected error for empty skill name")
		}
	})

	t.Run("removes skill with only canonical dir (no symlinks)", func(t *testing.T) {
		projectDir := t.TempDir()

		// Only create canonical dir, no symlinks
		canonicalDir := filepath.Join(projectDir, ".agents", "skills", "solo-skill")
		if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
			t.Fatal(err)
		}
		skillMd := "---\nname: solo-skill\ndescription: Test\n---\n"
		if err := os.WriteFile(filepath.Join(canonicalDir, "SKILL.md"), []byte(skillMd), 0o644); err != nil {
			t.Fatal(err)
		}

		result, err := remover.Remove("solo-skill", RemoveOptions{TargetDir: projectDir})
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		if len(result.RemovedSymlinks) != 0 {
			t.Errorf("expected 0 removed symlinks, got %d", len(result.RemovedSymlinks))
		}

		if dirExists(canonicalDir) {
			t.Error("canonical directory still exists")
		}
	})

	t.Run("removes only targeted skill, leaves others intact", func(t *testing.T) {
		projectDir := t.TempDir()
		setupInstalledSkill(t, projectDir, "skill-a", agents)
		setupInstalledSkill(t, projectDir, "skill-b", agents)

		_, err := remover.Remove("skill-a", RemoveOptions{TargetDir: projectDir})
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		// skill-a should be gone
		if dirExists(filepath.Join(projectDir, ".agents", "skills", "skill-a")) {
			t.Error("skill-a canonical dir still exists")
		}
		if pathExists(filepath.Join(projectDir, ".claude", "skills", "skill-a")) {
			t.Error("skill-a claude symlink still exists")
		}

		// skill-b should remain
		if !dirExists(filepath.Join(projectDir, ".agents", "skills", "skill-b")) {
			t.Error("skill-b canonical dir was incorrectly removed")
		}
		if !pathExists(filepath.Join(projectDir, ".claude", "skills", "skill-b")) {
			t.Error("skill-b claude symlink was incorrectly removed")
		}
	})

	t.Run("cleans up empty agent skill directories", func(t *testing.T) {
		projectDir := t.TempDir()
		setupInstalledSkill(t, projectDir, "only-skill", agents)

		_, err := remover.Remove("only-skill", RemoveOptions{TargetDir: projectDir})
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		// .claude/skills/ and .cursor/skills/ should be cleaned up since
		// they were the only skill
		claudeSkillsDir := filepath.Join(projectDir, ".claude", "skills")
		if dirExists(claudeSkillsDir) {
			t.Error(".claude/skills/ should have been cleaned up")
		}
		cursorSkillsDir := filepath.Join(projectDir, ".cursor", "skills")
		if dirExists(cursorSkillsDir) {
			t.Error(".cursor/skills/ should have been cleaned up")
		}

		// .agents/skills/ should also be cleaned up
		agentsSkillsDir := filepath.Join(projectDir, ".agents", "skills")
		if dirExists(agentsSkillsDir) {
			t.Error(".agents/skills/ should have been cleaned up")
		}
	})

	t.Run("handles copy fallback instead of symlink", func(t *testing.T) {
		projectDir := t.TempDir()

		// Create canonical directory
		canonicalDir := filepath.Join(projectDir, ".agents", "skills", "copied-skill")
		if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
			t.Fatal(err)
		}
		skillMd := "---\nname: copied-skill\ndescription: Test\n---\n"
		if err := os.WriteFile(filepath.Join(canonicalDir, "SKILL.md"), []byte(skillMd), 0o644); err != nil {
			t.Fatal(err)
		}

		// Simulate copy fallback: create a real dir instead of symlink
		claudeSkillDir := filepath.Join(projectDir, ".claude", "skills", "copied-skill")
		if err := os.MkdirAll(claudeSkillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(claudeSkillDir, "SKILL.md"), []byte(skillMd), 0o644); err != nil {
			t.Fatal(err)
		}

		result, err := remover.Remove("copied-skill", RemoveOptions{TargetDir: projectDir})
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}

		if len(result.RemovedSymlinks) != 1 {
			t.Errorf("expected 1 removed link/copy, got %d", len(result.RemovedSymlinks))
		}

		if dirExists(canonicalDir) {
			t.Error("canonical directory still exists")
		}
		if dirExists(claudeSkillDir) {
			t.Error("claude copied directory still exists")
		}
	})
}

func TestRemover_RemoveAll(t *testing.T) {
	agents := removerTestAgents()
	remover := NewRemover(agents)

	t.Run("removes all skills", func(t *testing.T) {
		projectDir := t.TempDir()
		setupInstalledSkill(t, projectDir, "skill-x", agents)
		setupInstalledSkill(t, projectDir, "skill-y", agents)
		setupInstalledSkill(t, projectDir, "skill-z", agents)

		results, err := remover.RemoveAll(RemoveOptions{TargetDir: projectDir})
		if err != nil {
			t.Fatalf("RemoveAll() error = %v", err)
		}

		if len(results) != 3 {
			t.Errorf("len(results) = %d, want 3", len(results))
		}

		// All canonical dirs should be gone
		for _, name := range []string{"skill-x", "skill-y", "skill-z"} {
			if dirExists(filepath.Join(projectDir, ".agents", "skills", name)) {
				t.Errorf("%s canonical dir still exists", name)
			}
		}
	})

	t.Run("returns nil for no skills", func(t *testing.T) {
		projectDir := t.TempDir()

		results, err := remover.RemoveAll(RemoveOptions{TargetDir: projectDir})
		if err != nil {
			t.Fatalf("RemoveAll() error = %v", err)
		}
		if results != nil {
			t.Errorf("expected nil results, got %v", results)
		}
	})

	t.Run("error when target dir is empty", func(t *testing.T) {
		_, err := remover.RemoveAll(RemoveOptions{TargetDir: ""})
		if err == nil {
			t.Fatal("expected error for empty target dir")
		}
	})
}

func TestRemover_ListRemovable(t *testing.T) {
	agents := removerTestAgents()
	remover := NewRemover(agents)

	t.Run("lists installed skills", func(t *testing.T) {
		projectDir := t.TempDir()
		setupInstalledSkill(t, projectDir, "alpha", agents)
		setupInstalledSkill(t, projectDir, "beta", agents)

		names, err := remover.ListRemovable(projectDir)
		if err != nil {
			t.Fatalf("ListRemovable() error = %v", err)
		}

		if len(names) != 2 {
			t.Fatalf("len(names) = %d, want 2", len(names))
		}

		// ReadDir returns sorted entries
		if names[0] != "alpha" {
			t.Errorf("names[0] = %q, want %q", names[0], "alpha")
		}
		if names[1] != "beta" {
			t.Errorf("names[1] = %q, want %q", names[1], "beta")
		}
	})

	t.Run("returns nil for no skills", func(t *testing.T) {
		projectDir := t.TempDir()

		names, err := remover.ListRemovable(projectDir)
		if err != nil {
			t.Fatalf("ListRemovable() error = %v", err)
		}
		if names != nil {
			t.Errorf("expected nil, got %v", names)
		}
	})
}
