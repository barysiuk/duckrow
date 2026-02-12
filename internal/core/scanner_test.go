package core

import (
	"os"
	"path/filepath"
	"testing"
)

// createTestSkill creates a SKILL.md in a test directory.
func createTestSkill(t *testing.T, baseDir, skillName, content string) string {
	t.Helper()
	skillDir := filepath.Join(baseDir, ".agents", "skills", skillName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMd := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMd, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return skillDir
}

// createAgentSymlink creates a symlink for a skill in an agent's skill directory.
func createAgentSymlink(t *testing.T, baseDir, agentSkillsDir, skillName, canonicalPath string) {
	t.Helper()
	agentDir := filepath.Join(baseDir, agentSkillsDir)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(agentDir, skillName)
	// Create relative symlink like the real installer does
	rel, err := filepath.Rel(agentDir, canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, linkPath); err != nil {
		t.Fatal(err)
	}
}

func TestScanner_ScanFolder(t *testing.T) {
	dir := t.TempDir()

	skillContent := `---
name: code-review
description: Review code for best practices
metadata:
  author: testorg
  version: "1.0.0"
---

# Code Review Skill
`

	canonicalPath := createTestSkill(t, dir, "code-review", skillContent)

	// Create symlinks for cursor and claude
	createAgentSymlink(t, dir, ".cursor/skills", "code-review", canonicalPath)
	createAgentSymlink(t, dir, ".claude/skills", "code-review", canonicalPath)

	agents, _ := LoadAgents()
	scanner := NewScanner(agents)

	skills, err := scanner.ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder() error: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	skill := skills[0]
	if skill.Name != "code-review" {
		t.Errorf("Name = %q, want %q", skill.Name, "code-review")
	}
	if skill.Description != "Review code for best practices" {
		t.Errorf("Description = %q, want %q", skill.Description, "Review code for best practices")
	}
	if skill.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", skill.Version, "1.0.0")
	}
	if skill.Author != "testorg" {
		t.Errorf("Author = %q, want %q", skill.Author, "testorg")
	}

	// Should detect agents that have symlinks
	hasAgent := func(name string) bool {
		for _, a := range skill.Agents {
			if a == name {
				return true
			}
		}
		return false
	}

	if !hasAgent("Cursor") {
		t.Error("expected Cursor in agents list")
	}
	if !hasAgent("Claude Code") {
		t.Error("expected Claude Code in agents list")
	}
}

func TestScanner_ScanEmptyFolder(t *testing.T) {
	dir := t.TempDir()

	agents, _ := LoadAgents()
	scanner := NewScanner(agents)

	skills, err := scanner.ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder() error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestScanner_MultipleSkills(t *testing.T) {
	dir := t.TempDir()

	skill1 := `---
name: skill-one
description: First skill
metadata:
  version: "1.0.0"
---
`
	skill2 := `---
name: skill-two
description: Second skill
metadata:
  version: "2.0.0"
---
`

	createTestSkill(t, dir, "skill-one", skill1)
	createTestSkill(t, dir, "skill-two", skill2)

	agents, _ := LoadAgents()
	scanner := NewScanner(agents)

	skills, err := scanner.ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder() error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

func TestParseSkillMd(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "SKILL.md")

	content := `---
name: web-design-guidelines
description: Review UI code for Web Interface Guidelines compliance.
license: MIT
metadata:
  author: vercel
  version: "1.0.0"
  argument-hint: <file-or-pattern>
---

# Web Interface Guidelines
`

	if err := os.WriteFile(mdPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	metadata, err := ParseSkillMd(mdPath)
	if err != nil {
		t.Fatalf("ParseSkillMd() error: %v", err)
	}
	if metadata.Name != "web-design-guidelines" {
		t.Errorf("Name = %q", metadata.Name)
	}
	if metadata.License != "MIT" {
		t.Errorf("License = %q", metadata.License)
	}
	if metadata.Metadata.Author != "vercel" {
		t.Errorf("Author = %q", metadata.Metadata.Author)
	}
	if metadata.Metadata.Version != "1.0.0" {
		t.Errorf("Version = %q", metadata.Metadata.Version)
	}
}

func TestParseSkillMd_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(mdPath, []byte("# Just markdown"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := ParseSkillMd(mdPath)
	if err == nil {
		t.Error("expected error for file without frontmatter")
	}
}

func TestParseSkillMd_MissingName(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "SKILL.md")

	content := `---
description: No name field
---
`
	if err := os.WriteFile(mdPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := ParseSkillMd(mdPath)
	if err == nil {
		t.Error("expected error for SKILL.md without name")
	}
}

func TestDiscoverSkills(t *testing.T) {
	dir := t.TempDir()

	// Create skills in a skills/ subdirectory (like a real repo)
	skillDir := filepath.Join(dir, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: my-skill
description: A test skill
---
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	skills, err := DiscoverSkills(dir, "", false)
	if err != nil {
		t.Fatalf("DiscoverSkills() error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Metadata.Name != "my-skill" {
		t.Errorf("Name = %q", skills[0].Metadata.Name)
	}
}

func TestDiscoverSkills_Internal(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "skills", "internal-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: internal-skill
description: An internal skill
metadata:
  internal: true
---
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Without includeInternal
	skills, _ := DiscoverSkills(dir, "", false)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills without includeInternal, got %d", len(skills))
	}

	// With includeInternal
	skills, _ = DiscoverSkills(dir, "", true)
	if len(skills) != 1 {
		t.Errorf("expected 1 skill with includeInternal, got %d", len(skills))
	}
}

func TestDiscoverSkills_WithSubpath(t *testing.T) {
	dir := t.TempDir()

	// Create a skill at a specific subpath
	skillDir := filepath.Join(dir, "specific", "path")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: specific-skill
description: A skill at a specific path
---
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	skills, err := DiscoverSkills(dir, "specific/path", false)
	if err != nil {
		t.Fatalf("DiscoverSkills() error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
}

func TestScanner_DetectAgents(t *testing.T) {
	dir := t.TempDir()

	// Create agent skill directories
	if err := os.MkdirAll(filepath.Join(dir, ".cursor", "skills"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.cursor) error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".agents", "skills"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.agents) error: %v", err)
	}

	agents, _ := LoadAgents()
	scanner := NewScanner(agents)

	detected := scanner.DetectAgents(dir)
	if len(detected) == 0 {
		t.Error("expected at least one agent detected")
	}

	hasCursor := false
	for _, a := range detected {
		if a == "Cursor" {
			hasCursor = true
		}
	}
	if !hasCursor {
		t.Error("expected Cursor to be detected")
	}
}
