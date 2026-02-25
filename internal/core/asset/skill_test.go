package asset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillHandler_Kind(t *testing.T) {
	h := &SkillHandler{}
	if h.Kind() != KindSkill {
		t.Errorf("Kind() = %q, want %q", h.Kind(), KindSkill)
	}
	if h.DisplayName() != "Skill" {
		t.Errorf("DisplayName() = %q, want %q", h.DisplayName(), "Skill")
	}
}

func TestSkillMeta_AssetKind(t *testing.T) {
	m := SkillMeta{Author: "test"}
	if m.AssetKind() != KindSkill {
		t.Errorf("AssetKind() = %q, want %q", m.AssetKind(), KindSkill)
	}
}

func TestSkillHandler_Discover(t *testing.T) {
	dir := t.TempDir()

	// Create skills in a tree.
	skills := []struct {
		path    string
		name    string
		content string
	}{
		{
			path: "skills/my-skill",
			name: "my-skill",
			content: `---
name: my-skill
description: A test skill
metadata:
  author: testorg
  version: "1.0.0"
---
# Test Skill
`,
		},
		{
			path: "skills/another",
			name: "another",
			content: `---
name: another
description: Another skill
---
`,
		},
	}

	for _, s := range skills {
		skillDir := filepath.Join(dir, s.path)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(s.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h := &SkillHandler{}
	assets, err := h.Discover(dir, DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}

	names := make(map[string]bool)
	for _, a := range assets {
		names[a.Name] = true
		if a.Kind != KindSkill {
			t.Errorf("asset %q has Kind=%q, want %q", a.Name, a.Kind, KindSkill)
		}
		if _, ok := a.Meta.(SkillMeta); !ok {
			t.Errorf("asset %q Meta is %T, want SkillMeta", a.Name, a.Meta)
		}
	}

	if !names["my-skill"] {
		t.Error("expected to find my-skill")
	}
	if !names["another"] {
		t.Error("expected to find another")
	}
}

func TestSkillHandler_Discover_Internal(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "internal-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: internal-skill
description: Internal
metadata:
  internal: true
---
`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &SkillHandler{}

	// Without includeInternal.
	assets, _ := h.Discover(dir, DiscoverOptions{})
	if len(assets) != 0 {
		t.Errorf("expected 0 assets without includeInternal, got %d", len(assets))
	}

	// With includeInternal.
	assets, _ = h.Discover(dir, DiscoverOptions{IncludeInternal: true})
	if len(assets) != 1 {
		t.Errorf("expected 1 asset with includeInternal, got %d", len(assets))
	}
}

func TestSkillHandler_Discover_NameFilter(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"alpha", "beta"} {
		skillDir := filepath.Join(dir, name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf("---\nname: %s\ndescription: Test\n---\n", name)
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h := &SkillHandler{}
	assets, _ := h.Discover(dir, DiscoverOptions{NameFilter: "alpha"})
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].Name != "alpha" {
		t.Errorf("Name = %q, want %q", assets[0].Name, "alpha")
	}
}

func TestSkillHandler_Discover_SubPath(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "sub", "path", "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: my-skill
description: Test
---
`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &SkillHandler{}
	assets, _ := h.Discover(dir, DiscoverOptions{SubPath: "sub/path"})
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
}

func TestSkillHandler_Discover_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()

	// In .git — should be skipped.
	gitSkillDir := filepath.Join(dir, ".git", "hooks", "my-skill")
	if err := os.MkdirAll(gitSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitSkillDir, "SKILL.md"), []byte(`---
name: git-skill
description: Should not be found
---
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// In .agents — should be found.
	agentSkillDir := filepath.Join(dir, ".agents", "skills", "real-skill")
	if err := os.MkdirAll(agentSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentSkillDir, "SKILL.md"), []byte(`---
name: real-skill
description: Should be found
---
`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &SkillHandler{}
	assets, _ := h.Discover(dir, DiscoverOptions{})
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].Name != "real-skill" {
		t.Errorf("Name = %q, want %q", assets[0].Name, "real-skill")
	}
}

func TestSkillHandler_Parse(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: web-design
description: Web design guidelines
license: MIT
metadata:
  author: vercel
  version: "2.0.0"
  argument-hint: <file-or-pattern>
---
`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &SkillHandler{}
	meta, err := h.Parse(skillDir)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	sm, ok := meta.(SkillMeta)
	if !ok {
		t.Fatalf("expected SkillMeta, got %T", meta)
	}
	if sm.Author != "vercel" {
		t.Errorf("Author = %q, want %q", sm.Author, "vercel")
	}
	if sm.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", sm.Version, "2.0.0")
	}
	if sm.License != "MIT" {
		t.Errorf("License = %q, want %q", sm.License, "MIT")
	}
	if sm.ArgHint != "<file-or-pattern>" {
		t.Errorf("ArgHint = %q, want %q", sm.ArgHint, "<file-or-pattern>")
	}
}

func TestSkillHandler_Validate(t *testing.T) {
	h := &SkillHandler{}

	// Valid.
	err := h.Validate(Asset{Name: "test", Meta: SkillMeta{}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Missing name.
	err = h.Validate(Asset{Name: "", Meta: SkillMeta{}})
	if err == nil {
		t.Error("expected error for empty name")
	}

	// Wrong meta type.
	err = h.Validate(Asset{Name: "test", Meta: MCPMeta{}})
	if err == nil {
		t.Error("expected error for wrong meta type")
	}
}

func TestSkillHandler_ParseManifestEntries(t *testing.T) {
	h := &SkillHandler{}

	raw := json.RawMessage(`[
		{"name": "go-review", "description": "Go code review", "source": "github.com/acme/skills/go-review"},
		{"name": "python-lint", "description": "Python linting", "source": "github.com/acme/skills/python-lint", "commit": "abc123def456abc123def456abc123def456abc1"}
	]`)

	entries, err := h.ParseManifestEntries(raw)
	if err != nil {
		t.Fatalf("ParseManifestEntries() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "go-review" {
		t.Errorf("entries[0].Name = %q", entries[0].Name)
	}
	if entries[1].Commit != "abc123def456abc123def456abc123def456abc1" {
		t.Errorf("entries[1].Commit = %q", entries[1].Commit)
	}
}

func TestSkillHandler_LockData(t *testing.T) {
	h := &SkillHandler{}

	a := Asset{
		Kind:   KindSkill,
		Name:   "my-skill",
		Source: "github.com/acme/skills/my-skill",
	}
	info := InstallInfo{
		Commit: "abc123",
		Ref:    "main",
	}

	locked := h.LockData(a, info)
	if locked.Kind != KindSkill {
		t.Errorf("Kind = %q", locked.Kind)
	}
	if locked.Name != "my-skill" {
		t.Errorf("Name = %q", locked.Name)
	}
	if locked.Source != "github.com/acme/skills/my-skill" {
		t.Errorf("Source = %q", locked.Source)
	}
	if locked.Commit != "abc123" {
		t.Errorf("Commit = %q", locked.Commit)
	}
	if locked.Ref != "main" {
		t.Errorf("Ref = %q", locked.Ref)
	}
}
