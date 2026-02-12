package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstaller_InstallFromLocalSource(t *testing.T) {
	// Create a source directory with a skill
	srcDir := t.TempDir()
	skillDir := filepath.Join(srcDir, "skills", "test-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test-skill
description: A test skill for installation
metadata:
  version: "1.0.0"
---

# Test Skill
Instructions here.
`), 0o644)

	// Create an additional rules file
	os.WriteFile(filepath.Join(skillDir, "rules.md"), []byte("# Rules"), 0o644)

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
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(`---
name: symlink-test
description: Test symlink creation
---
`), 0o644)

	targetDir := t.TempDir()

	// Create cursor and claude directories to trigger agent detection
	os.MkdirAll(filepath.Join(targetDir, ".cursor", "skills"), 0o755)
	os.MkdirAll(filepath.Join(targetDir, ".claude", "skills"), 0o755)

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
	target, _ := os.Readlink(cursorLink)
	expectedTarget := "../../.agents/skills/symlink-test"
	if target != expectedTarget {
		t.Errorf("symlink target = %q, want %q", target, expectedTarget)
	}
}

func TestInstaller_SkillFilter(t *testing.T) {
	srcDir := t.TempDir()

	// Create two skills
	for _, name := range []string{"skill-a", "skill-b"} {
		dir := filepath.Join(srcDir, "skills", name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(`---
name: `+name+`
description: Test skill
---
`), 0o644)
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
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(`---
name: real-skill
description: Test skill
---
`), 0o644)

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
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte(`---
name: exclusion-test
description: Test copy exclusions
---
`), 0o644)
	os.WriteFile(filepath.Join(srcDir, "rules.md"), []byte("# Keep"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# Exclude"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "_internal.md"), []byte("# Exclude"), 0o644)
	os.MkdirAll(filepath.Join(srcDir, ".git"), 0o755)
	os.WriteFile(filepath.Join(srcDir, ".git", "config"), []byte("[core]"), 0o644)

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
