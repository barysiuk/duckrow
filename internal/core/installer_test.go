package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstaller_InstallFromLocalSource(t *testing.T) {
	// Create a source directory with a skill
	srcDir := t.TempDir()
	skillDir := filepath.Join(srcDir, "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test-skill
description: A test skill for installation
metadata:
  version: "1.0.0"
---

# Test Skill
Instructions here.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error: %v", err)
	}

	// Create an additional rules file
	if err := os.WriteFile(filepath.Join(skillDir, "rules.md"), []byte("# Rules"), 0o644); err != nil {
		t.Fatalf("WriteFile(rules.md) error: %v", err)
	}

	// Create target directory
	targetDir := t.TempDir()

	agents, _ := LoadAgents()
	installer := NewInstaller(agents)

	source := &ParsedSource{
		Type:      SourceTypeLocal,
		LocalPath: srcDir,
	}

	result, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir: targetDir,
	})
	if err != nil {
		t.Fatalf("InstallFromSource() error: %v", err)
	}

	if len(result.InstalledSkills) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(result.InstalledSkills))
	}

	installed := result.InstalledSkills[0]
	if installed.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", installed.Name, "test-skill")
	}

	// Verify canonical directory exists
	canonicalPath := filepath.Join(targetDir, ".agents", "skills", "test-skill")
	if _, err := os.Stat(canonicalPath); err != nil {
		t.Errorf("canonical directory not created: %v", err)
	}

	// Verify SKILL.md was copied
	skillMdPath := filepath.Join(canonicalPath, "SKILL.md")
	if _, err := os.Stat(skillMdPath); err != nil {
		t.Errorf("SKILL.md not copied: %v", err)
	}

	// Verify rules.md was copied
	rulesPath := filepath.Join(canonicalPath, "rules.md")
	if _, err := os.Stat(rulesPath); err != nil {
		t.Errorf("rules.md not copied: %v", err)
	}
}

func TestInstaller_InstallWithAgentSymlinks(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(`---
name: symlink-test
description: Test symlink creation
---
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	targetDir := t.TempDir()

	agents, _ := LoadAgents()

	// Find the cursor and claude-code agents to explicitly target them.
	var targetAgents []AgentDef
	for _, a := range agents {
		if a.Name == "cursor" || a.Name == "claude-code" {
			targetAgents = append(targetAgents, a)
		}
		if a.Universal {
			targetAgents = append(targetAgents, a)
		}
	}

	installer := NewInstaller(agents)

	source := &ParsedSource{
		Type:      SourceTypeLocal,
		LocalPath: srcDir,
	}

	result, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir:    targetDir,
		TargetAgents: targetAgents,
	})
	if err != nil {
		t.Fatalf("InstallFromSource() error: %v", err)
	}

	if len(result.InstalledSkills) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(result.InstalledSkills))
	}

	// Check symlinks were created
	cursorLink := filepath.Join(targetDir, ".cursor", "skills", "symlink-test")
	info, err := os.Lstat(cursorLink)
	if err != nil {
		t.Fatalf("cursor symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected cursor path to be a symlink")
	}

	claudeLink := filepath.Join(targetDir, ".claude", "skills", "symlink-test")
	info, err = os.Lstat(claudeLink)
	if err != nil {
		t.Fatalf("claude symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected claude path to be a symlink")
	}

	// Verify symlinks point to correct target
	target, err := os.Readlink(cursorLink)
	if err != nil {
		t.Fatalf("Readlink() error: %v", err)
	}
	expectedTarget := "../../.agents/skills/symlink-test"
	if target != expectedTarget {
		t.Errorf("symlink target = %q, want %q", target, expectedTarget)
	}
}

func TestInstaller_DefaultUniversalOnly(t *testing.T) {
	// When no TargetAgents is provided, only the canonical .agents/skills/ dir
	// should be created (universal agents). No agent-specific directories.
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(`---
name: universal-test
description: Test default universal-only install
---
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	targetDir := t.TempDir()
	agents, _ := LoadAgents()
	installer := NewInstaller(agents)

	source := &ParsedSource{
		Type:      SourceTypeLocal,
		LocalPath: srcDir,
	}

	result, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir: targetDir,
	})
	if err != nil {
		t.Fatalf("InstallFromSource() error: %v", err)
	}

	if len(result.InstalledSkills) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(result.InstalledSkills))
	}

	// Canonical dir should exist.
	canonicalPath := filepath.Join(targetDir, ".agents", "skills", "universal-test")
	if _, err := os.Stat(canonicalPath); err != nil {
		t.Errorf("canonical directory not created: %v", err)
	}

	// No agent-specific directories should be created.
	for _, agent := range agents {
		if agent.Universal {
			continue
		}
		agentDir := filepath.Join(targetDir, agent.SkillsDir, "universal-test")
		if _, err := os.Stat(agentDir); err == nil {
			t.Errorf("agent dir %s should not exist when no TargetAgents specified", agent.SkillsDir)
		}
	}

	// Installed agents should only be universal ones.
	installed := result.InstalledSkills[0]
	for _, agentName := range installed.Agents {
		found := false
		for _, agent := range agents {
			if agent.DisplayName == agentName && agent.Universal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected non-universal agent %q in installed agents", agentName)
		}
	}
}

func TestInstaller_SkillFilter(t *testing.T) {
	srcDir := t.TempDir()

	// Create two skills
	for _, name := range []string{"skill-a", "skill-b"} {
		dir := filepath.Join(srcDir, "skills", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(`---
name: `+name+`
description: Test skill
---
`), 0o644); err != nil {
			t.Fatalf("WriteFile(%s/SKILL.md) error: %v", name, err)
		}
	}

	targetDir := t.TempDir()
	agents, _ := LoadAgents()
	installer := NewInstaller(agents)

	source := &ParsedSource{
		Type:      SourceTypeLocal,
		LocalPath: srcDir,
	}

	result, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir:   targetDir,
		SkillFilter: "skill-a",
	})
	if err != nil {
		t.Fatalf("InstallFromSource() error: %v", err)
	}

	if len(result.InstalledSkills) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(result.InstalledSkills))
	}
	if result.InstalledSkills[0].Name != "skill-a" {
		t.Errorf("Name = %q, want %q", result.InstalledSkills[0].Name, "skill-a")
	}
}

func TestInstaller_SkillFilterNotFound(t *testing.T) {
	srcDir := t.TempDir()
	dir := filepath.Join(srcDir, "skills", "real-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(`---
name: real-skill
description: Test skill
---
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	targetDir := t.TempDir()
	agents, _ := LoadAgents()
	installer := NewInstaller(agents)

	source := &ParsedSource{
		Type:      SourceTypeLocal,
		LocalPath: srcDir,
	}

	_, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir:   targetDir,
		SkillFilter: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent skill filter")
	}
}

func TestInstaller_NoSkillsFound(t *testing.T) {
	srcDir := t.TempDir() // Empty directory
	targetDir := t.TempDir()

	agents, _ := LoadAgents()
	installer := NewInstaller(agents)

	source := &ParsedSource{
		Type:      SourceTypeLocal,
		LocalPath: srcDir,
	}

	_, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir: targetDir,
	})
	if err == nil {
		t.Error("expected error when no skills found")
	}
}

func TestInstaller_CopyExclusions(t *testing.T) {
	srcDir := t.TempDir()

	// Create a skill with files that should be excluded
	if err := os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(`---
name: exclusion-test
description: Test copy exclusions
---
`), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "rules.md"), []byte("# Keep"), 0o644); err != nil {
		t.Fatalf("WriteFile(rules.md) error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# Exclude"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "_internal.md"), []byte("# Exclude"), 0o644); err != nil {
		t.Fatalf("WriteFile(_internal.md) error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".git", "config"), []byte("[core]"), 0o644); err != nil {
		t.Fatalf("WriteFile(.git/config) error: %v", err)
	}

	targetDir := t.TempDir()
	agents, _ := LoadAgents()
	installer := NewInstaller(agents)

	source := &ParsedSource{
		Type:      SourceTypeLocal,
		LocalPath: srcDir,
	}

	_, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir: targetDir,
	})
	if err != nil {
		t.Fatalf("InstallFromSource() error: %v", err)
	}

	canonicalDir := filepath.Join(targetDir, ".agents", "skills", "exclusion-test")

	// Should exist
	if !fileExists(filepath.Join(canonicalDir, "SKILL.md")) {
		t.Error("SKILL.md should be copied")
	}
	if !fileExists(filepath.Join(canonicalDir, "rules.md")) {
		t.Error("rules.md should be copied")
	}

	// Should NOT exist
	if fileExists(filepath.Join(canonicalDir, "README.md")) {
		t.Error("README.md should be excluded")
	}
	if fileExists(filepath.Join(canonicalDir, "_internal.md")) {
		t.Error("_internal.md should be excluded")
	}
	if dirExists(filepath.Join(canonicalDir, ".git")) {
		t.Error(".git should be excluded")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"code-review", "code-review"},
		{"Code Review", "code-review"},
		{"my_skill", "my-skill"},
		{"My.Skill.v2", "my-skill-v2"},
		{"---leading-trailing---", "leading-trailing"},
		{"", "unnamed-skill"},
		{"UPPERCASE", "uppercase"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInstaller_CloneWithOverride(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a local git repo with a SKILL.md to serve as the "real" clone target.
	repoDir := t.TempDir()
	skillDir := filepath.Join(repoDir, "skills", "override-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: override-skill
description: Skill installed via clone URL override
metadata:
  version: "1.0.0"
---

# Override Skill
`), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error: %v", err)
	}

	setupTestGitRepoInDir(t, repoDir)

	// Parse a source as "fake-owner/fake-repo" — this would normally try GitHub.
	source, err := ParseSource("fake-owner/fake-repo")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}

	// The default CloneURL points to https://github.com/fake-owner/fake-repo.git
	// which doesn't exist. Apply an override to redirect to the local git repo.
	overrides := map[string]string{
		"fake-owner/fake-repo": repoDir,
	}
	applied := source.ApplyCloneURLOverride(overrides)
	if !applied {
		t.Fatal("expected override to be applied")
	}
	if source.CloneURL != repoDir {
		t.Fatalf("CloneURL = %q, want %q", source.CloneURL, repoDir)
	}

	targetDir := t.TempDir()
	agents, _ := LoadAgents()
	installer := NewInstaller(agents)

	result, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir: targetDir,
	})
	if err != nil {
		t.Fatalf("InstallFromSource() error: %v", err)
	}

	if len(result.InstalledSkills) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(result.InstalledSkills))
	}
	if result.InstalledSkills[0].Name != "override-skill" {
		t.Errorf("Name = %q, want %q", result.InstalledSkills[0].Name, "override-skill")
	}

	// Verify the SKILL.md is on disk
	skillMd := filepath.Join(targetDir, ".agents", "skills", "override-skill", "SKILL.md")
	if !fileExists(skillMd) {
		t.Error("SKILL.md not found after install with override")
	}
}

func TestInstaller_CloneFailureReturnsCloneError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Try to install from a nonexistent repository.
	source := &ParsedSource{
		Type:     SourceTypeGitHub,
		Owner:    "nonexistent-owner-xyz",
		Repo:     "nonexistent-repo-xyz",
		CloneURL: "https://github.com/nonexistent-owner-xyz/nonexistent-repo-xyz.git",
	}

	targetDir := t.TempDir()
	agents, _ := LoadAgents()
	installer := NewInstaller(agents)

	_, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir: targetDir,
	})
	if err == nil {
		t.Fatal("expected error when cloning nonexistent repo")
	}

	ce, ok := IsCloneError(err)
	if !ok {
		t.Fatalf("expected *CloneError, got %T: %v", err, err)
	}

	if ce.URL != source.CloneURL {
		t.Errorf("CloneError.URL = %q, want %q", ce.URL, source.CloneURL)
	}
	if ce.Kind == CloneErrUnknown {
		// It should be classified as something meaningful (auth, not found, etc.)
		// but this depends on network — just verify it's a real CloneError.
		t.Logf("CloneError.Kind = %s (acceptable for network-dependent test)", ce.Kind)
	}
}

// setupTestGitRepoInDir initializes a git repo in an existing directory.
// Unlike setupTestGitRepo (in registry_test.go) which also writes a duckrow.json,
// this only runs git init/add/commit on whatever files are already in the dir.
func setupTestGitRepoInDir(t *testing.T, dir string) {
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
	runGit("add", ".")
	runGit("commit", "-m", "initial")
}

func TestInstaller_CloneAndInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	targetDir := t.TempDir()
	agents, _ := LoadAgents()
	installer := NewInstaller(agents)

	source, err := ParseSource("vercel-labs/agent-skills")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}

	result, err := installer.InstallFromSource(source, InstallOptions{
		TargetDir:   targetDir,
		SkillFilter: "web-design-guidelines",
	})
	if err != nil {
		t.Fatalf("InstallFromSource() error: %v", err)
	}

	if len(result.InstalledSkills) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(result.InstalledSkills))
	}
	if result.InstalledSkills[0].Name != "web-design-guidelines" {
		t.Errorf("Name = %q", result.InstalledSkills[0].Name)
	}

	// Verify the SKILL.md is on disk
	skillMd := filepath.Join(targetDir, ".agents", "skills", "web-design-guidelines", "SKILL.md")
	if !fileExists(skillMd) {
		t.Error("SKILL.md not found after install")
	}
}
