package asset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Handler identity
// ---------------------------------------------------------------------------

func TestAgentHandler_Kind(t *testing.T) {
	h := &AgentHandler{}
	if h.Kind() != KindAgent {
		t.Errorf("Kind() = %q, want %q", h.Kind(), KindAgent)
	}
	if h.DisplayName() != "Agent" {
		t.Errorf("DisplayName() = %q, want %q", h.DisplayName(), "Agent")
	}
}

func TestAgentMeta_AssetKind(t *testing.T) {
	m := AgentMeta{}
	if m.AssetKind() != KindAgent {
		t.Errorf("AssetKind() = %q, want %q", m.AssetKind(), KindAgent)
	}
}

func TestAgentDataMeta_AssetKind(t *testing.T) {
	m := AgentDataMeta{Data: &AgentData{}}
	if m.AssetKind() != KindAgent {
		t.Errorf("AssetKind() = %q, want %q", m.AssetKind(), KindAgent)
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestAgentHandler_Registered(t *testing.T) {
	h, ok := Get(KindAgent)
	if !ok {
		t.Fatal("agent handler not registered")
	}
	if h.Kind() != KindAgent {
		t.Errorf("registered handler Kind() = %q", h.Kind())
	}
}

func TestKinds_IncludesAgent(t *testing.T) {
	kinds := Kinds()
	found := false
	for _, k := range kinds {
		if k == KindAgent {
			found = true
			break
		}
	}
	if !found {
		t.Error("KindAgent not found in Kinds()")
	}
}

// ---------------------------------------------------------------------------
// ParseAgentContent
// ---------------------------------------------------------------------------

func TestParseAgentContent_Basic(t *testing.T) {
	raw := []byte(`---
name: code-reviewer
description: Reviews code for quality
model: claude-sonnet
---

You are a code reviewer. Focus on clarity and correctness.
`)
	data, err := ParseAgentContent(raw, "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Frontmatter["name"] != "code-reviewer" {
		t.Errorf("name = %v", data.Frontmatter["name"])
	}
	if data.Frontmatter["description"] != "Reviews code for quality" {
		t.Errorf("description = %v", data.Frontmatter["description"])
	}
	if data.Frontmatter["model"] != "claude-sonnet" {
		t.Errorf("model = %v", data.Frontmatter["model"])
	}
	if !strings.Contains(data.Body, "You are a code reviewer") {
		t.Errorf("body = %q", data.Body)
	}
}

func TestParseAgentContent_NoFrontmatter(t *testing.T) {
	raw := []byte("Just plain markdown content")
	_, err := ParseAgentContent(raw, "test.md")
	if err == nil {
		t.Error("expected error for content without frontmatter")
	}
	if !strings.Contains(err.Error(), "no frontmatter") {
		t.Errorf("error = %v", err)
	}
}

func TestParseAgentContent_NoClosingDelimiter(t *testing.T) {
	raw := []byte(`---
name: broken
description: Missing closing delimiter
`)
	_, err := ParseAgentContent(raw, "test.md")
	if err == nil {
		t.Error("expected error for missing closing delimiter")
	}
	if !strings.Contains(err.Error(), "no closing frontmatter") {
		t.Errorf("error = %v", err)
	}
}

func TestParseAgentContent_MinimalFrontmatter(t *testing.T) {
	// A single empty line between delimiters produces a nil YAML map,
	// which ParseAgentContent converts to an empty map.
	raw := []byte("---\n\n---\n\nBody content here.\n")
	data, err := ParseAgentContent(raw, "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.Frontmatter) != 0 {
		t.Errorf("expected empty frontmatter, got %v", data.Frontmatter)
	}
	if !strings.Contains(data.Body, "Body content here") {
		t.Errorf("body = %q", data.Body)
	}
}

func TestParseAgentContent_AdjacentDelimiters(t *testing.T) {
	// ---\n--- with no content between them is rejected because the
	// parser looks for \n--- to find the closing delimiter.
	raw := []byte("---\n---\n\nBody.\n")
	_, err := ParseAgentContent(raw, "test.md")
	if err == nil {
		t.Error("expected error for adjacent delimiters")
	}
}

func TestParseAgentContent_SystemOverrides(t *testing.T) {
	raw := []byte(`---
name: test-agent
description: Test
model: default-model
claude-code:
  model: claude-sonnet-4-20250514
opencode:
  model: claude-sonnet-4-20250514
---

System prompt.
`)
	data, err := ParseAgentContent(raw, "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that system overrides are parsed as maps.
	cc, ok := data.Frontmatter["claude-code"].(map[string]any)
	if !ok {
		t.Fatalf("claude-code is %T, want map", data.Frontmatter["claude-code"])
	}
	if cc["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("claude-code.model = %v", cc["model"])
	}

	oc, ok := data.Frontmatter["opencode"].(map[string]any)
	if !ok {
		t.Fatalf("opencode is %T, want map", data.Frontmatter["opencode"])
	}
	if oc["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("opencode.model = %v", oc["model"])
	}
}

func TestParseAgentContent_ToolsList(t *testing.T) {
	raw := []byte(`---
name: test-agent
description: Test
tools:
  - Read
  - Write
  - Bash
---

Prompt.
`)
	data, err := ParseAgentContent(raw, "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tools, ok := data.Frontmatter["tools"].([]any)
	if !ok {
		t.Fatalf("tools is %T, want []any", data.Frontmatter["tools"])
	}
	if len(tools) != 3 {
		t.Errorf("tools len = %d, want 3", len(tools))
	}
}

func TestParseAgentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-agent.md")
	content := `---
name: file-agent
description: Test agent from file
---

File body.
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := ParseAgentFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Frontmatter["name"] != "file-agent" {
		t.Errorf("name = %v", data.Frontmatter["name"])
	}
}

func TestParseAgentFile_NotFound(t *testing.T) {
	_, err := ParseAgentFile("/nonexistent/path.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// ---------------------------------------------------------------------------
// RenderForSystem
// ---------------------------------------------------------------------------

func TestRenderForSystem_Basic(t *testing.T) {
	data := &AgentData{
		Frontmatter: map[string]any{
			"name":        "test-agent",
			"description": "A test agent",
			"model":       "claude-sonnet",
		},
		Body: "You are a helpful assistant.",
	}

	// For non-Gemini systems, "name" should be removed.
	out, err := RenderForSystem(data, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(out)
	if strings.Contains(content, "name:") {
		t.Errorf("output should not contain 'name:' for claude-code, got:\n%s", content)
	}
	if !strings.Contains(content, "description: A test agent") {
		t.Errorf("output should contain description, got:\n%s", content)
	}
	if !strings.Contains(content, "model: claude-sonnet") {
		t.Errorf("output should contain model, got:\n%s", content)
	}
	if !strings.Contains(content, "You are a helpful assistant.") {
		t.Errorf("output should contain body, got:\n%s", content)
	}
}

func TestRenderForSystem_GeminiKeepsName(t *testing.T) {
	data := &AgentData{
		Frontmatter: map[string]any{
			"name":        "test-agent",
			"description": "A test agent",
		},
		Body: "Prompt.",
	}

	out, err := RenderForSystem(data, "gemini-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(out)
	if !strings.Contains(content, "name: test-agent") {
		t.Errorf("gemini-cli output should contain 'name:', got:\n%s", content)
	}
}

func TestRenderForSystem_SystemOverrideApplied(t *testing.T) {
	data := &AgentData{
		Frontmatter: map[string]any{
			"name":        "test-agent",
			"description": "A test agent",
			"model":       "default-model",
			"claude-code": map[string]any{
				"model": "claude-opus-4-20250514",
			},
			"opencode": map[string]any{
				"model": "opencode-model",
			},
		},
		Body: "Prompt.",
	}

	// For claude-code, should use the override model.
	out, err := RenderForSystem(data, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(out)
	if !strings.Contains(content, "model: claude-opus-4-20250514") {
		t.Errorf("expected claude-code override model, got:\n%s", content)
	}

	// For opencode, should use opencode override.
	out2, err := RenderForSystem(data, "opencode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content2 := string(out2)
	if !strings.Contains(content2, "model: opencode-model") {
		t.Errorf("expected opencode override model, got:\n%s", content2)
	}

	// For github-copilot (no override), should use default model.
	out3, err := RenderForSystem(data, "github-copilot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content3 := string(out3)
	if !strings.Contains(content3, "model: default-model") {
		t.Errorf("expected default model for github-copilot, got:\n%s", content3)
	}
}

func TestRenderForSystem_OverrideBlocksRemoved(t *testing.T) {
	data := &AgentData{
		Frontmatter: map[string]any{
			"name":           "test-agent",
			"description":    "A test agent",
			"claude-code":    map[string]any{"model": "cc-model"},
			"opencode":       map[string]any{"model": "oc-model"},
			"github-copilot": map[string]any{"model": "gh-model"},
			"gemini-cli":     map[string]any{"model": "gem-model"},
		},
		Body: "Prompt.",
	}

	// Render for claude-code — should NOT contain other system keys.
	out, err := RenderForSystem(data, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content := string(out)

	for _, key := range []string{"opencode:", "github-copilot:", "gemini-cli:"} {
		if strings.Contains(content, key) {
			t.Errorf("claude-code output should not contain %q, got:\n%s", key, content)
		}
	}
	// claude-code block itself should also not appear as a block.
	if strings.Contains(content, "claude-code:") {
		t.Errorf("output should not contain 'claude-code:' block key, got:\n%s", content)
	}
}

func TestRenderForSystem_EmptyBody(t *testing.T) {
	data := &AgentData{
		Frontmatter: map[string]any{
			"description": "Test",
		},
		Body: "",
	}

	out, err := RenderForSystem(data, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(out)
	// Should have frontmatter but no body section.
	if !strings.HasPrefix(content, "---\n") {
		t.Errorf("expected frontmatter start, got:\n%s", content)
	}
	if !strings.Contains(content, "---\n") {
		t.Errorf("expected frontmatter closing, got:\n%s", content)
	}
}

func TestRenderForSystem_NilData(t *testing.T) {
	_, err := RenderForSystem(nil, "claude-code")
	if err == nil {
		t.Error("expected error for nil data")
	}
}

func TestRenderForSystem_FieldOrder(t *testing.T) {
	data := &AgentData{
		Frontmatter: map[string]any{
			"name":        "test",
			"zebra":       "last",
			"description": "A desc",
			"alpha":       "first",
			"model":       "a-model",
			"tools":       []string{"Read", "Write"},
		},
		Body: "Prompt.",
	}

	out, err := RenderForSystem(data, "gemini-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(out)
	lines := strings.Split(content, "\n")

	// Find the positions of key fields. Priority order: name, description, model, tools.
	positions := make(map[string]int)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, key := range []string{"name:", "description:", "model:", "tools:", "alpha:", "zebra:"} {
			if strings.HasPrefix(trimmed, key) {
				positions[key] = i
			}
		}
	}

	// name should come before description.
	if positions["name:"] >= positions["description:"] {
		t.Error("name should come before description")
	}
	// description should come before model.
	if positions["description:"] >= positions["model:"] {
		t.Error("description should come before model")
	}
	// model should come before tools.
	if positions["model:"] >= positions["tools:"] {
		t.Error("model should come before tools")
	}
	// alpha should come before zebra (alphabetical for non-priority fields).
	if positions["alpha:"] >= positions["zebra:"] {
		t.Error("alpha should come before zebra")
	}
	// tools should come before alpha (priority before non-priority).
	if positions["tools:"] >= positions["alpha:"] {
		t.Error("tools should come before alpha")
	}
}

// ---------------------------------------------------------------------------
// Discover
// ---------------------------------------------------------------------------

func TestAgentHandler_Discover_Basic(t *testing.T) {
	dir := t.TempDir()

	// Create a valid agent file.
	content := `---
name: code-reviewer
description: Reviews code quality
---

You review code for quality issues.
`
	if err := os.WriteFile(filepath.Join(dir, "code-reviewer.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &AgentHandler{}
	assets, err := h.Discover(dir, DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].Name != "code-reviewer" {
		t.Errorf("Name = %q", assets[0].Name)
	}
	if assets[0].Kind != KindAgent {
		t.Errorf("Kind = %q", assets[0].Kind)
	}
}

func TestAgentHandler_Discover_RequiresNameAndDescription(t *testing.T) {
	dir := t.TempDir()

	// File with name but no description — should NOT be discovered.
	content := `---
name: no-desc
---

Body.
`
	if err := os.WriteFile(filepath.Join(dir, "no-desc.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// File with description but no name — should NOT be discovered.
	content2 := `---
description: Has desc but no name
---

Body.
`
	if err := os.WriteFile(filepath.Join(dir, "no-name.md"), []byte(content2), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &AgentHandler{}
	assets, err := h.Discover(dir, DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(assets) != 0 {
		t.Errorf("expected 0 assets, got %d", len(assets))
	}
}

func TestAgentHandler_Discover_ExcludedFiles(t *testing.T) {
	dir := t.TempDir()

	for _, excluded := range []string{"SKILL.md", "AGENTS.md", "README.md", "CLAUDE.md", "GEMINI.md", "codex.md"} {
		content := `---
name: ` + excluded + `
description: Should be excluded
---

Body.
`
		if err := os.WriteFile(filepath.Join(dir, excluded), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h := &AgentHandler{}
	assets, err := h.Discover(dir, DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(assets) != 0 {
		t.Errorf("expected 0 assets (all excluded), got %d", len(assets))
	}
}

func TestAgentHandler_Discover_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()

	// Agent in .git — should be skipped.
	gitDir := filepath.Join(dir, ".git", "agents")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "hidden.md"), []byte(`---
name: hidden-agent
description: Should not be found
---

Body.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Agent in .claude — should be found (allowed directory).
	claudeDir := filepath.Join(dir, ".claude", "agents")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "visible.md"), []byte(`---
name: visible-agent
description: Should be found
---

Body.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &AgentHandler{}
	assets, err := h.Discover(dir, DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].Name != "visible-agent" {
		t.Errorf("Name = %q, want %q", assets[0].Name, "visible-agent")
	}
}

func TestAgentHandler_Discover_NameFilter(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"alpha", "beta"} {
		content := "---\nname: " + name + "\ndescription: Test\n---\n\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h := &AgentHandler{}
	assets, err := h.Discover(dir, DiscoverOptions{NameFilter: "alpha"})
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].Name != "alpha" {
		t.Errorf("Name = %q, want %q", assets[0].Name, "alpha")
	}
}

func TestAgentHandler_Discover_SubPath(t *testing.T) {
	dir := t.TempDir()

	subDir := filepath.Join(dir, "agents", "custom")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "deep.md"), []byte(`---
name: deep-agent
description: In a subdirectory
---

Body.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &AgentHandler{}
	assets, err := h.Discover(dir, DiscoverOptions{SubPath: "agents/custom"})
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].Name != "deep-agent" {
		t.Errorf("Name = %q", assets[0].Name)
	}
}

func TestAgentHandler_Discover_Deduplication(t *testing.T) {
	dir := t.TempDir()

	// Two files with the same agent name — should only get one.
	for _, fname := range []string{"first.md", "second.md"} {
		content := "---\nname: same-name\ndescription: Duplicate\n---\n\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, fname), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h := &AgentHandler{}
	assets, err := h.Discover(dir, DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(assets) != 1 {
		t.Errorf("expected 1 asset (deduplicated), got %d", len(assets))
	}
}

func TestAgentHandler_Discover_NonMdFilesIgnored(t *testing.T) {
	dir := t.TempDir()

	// .txt file should be ignored.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not an agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Directory should be ignored even if it ends with .md (unlikely but defensive).
	if err := os.MkdirAll(filepath.Join(dir, "dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	h := &AgentHandler{}
	assets, err := h.Discover(dir, DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if len(assets) != 0 {
		t.Errorf("expected 0 assets, got %d", len(assets))
	}
}

// ---------------------------------------------------------------------------
// Parse
// ---------------------------------------------------------------------------

func TestAgentHandler_Parse_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	if err := os.WriteFile(path, []byte(`---
name: parse-test
description: Test parse
---

Body.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &AgentHandler{}
	meta, err := h.Parse(path)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	adm, ok := meta.(AgentDataMeta)
	if !ok {
		t.Fatalf("expected AgentDataMeta, got %T", meta)
	}
	if adm.Data.Frontmatter["name"] != "parse-test" {
		t.Errorf("name = %v", adm.Data.Frontmatter["name"])
	}
}

func TestAgentHandler_Parse_Directory(t *testing.T) {
	dir := t.TempDir()

	h := &AgentHandler{}
	_, err := h.Parse(dir)
	if err == nil {
		t.Error("expected error when parsing a directory")
	}
	if !strings.Contains(err.Error(), "not directories") {
		t.Errorf("error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestAgentHandler_Validate_Valid(t *testing.T) {
	h := &AgentHandler{}
	err := h.Validate(Asset{
		Name: "test-agent",
		Meta: AgentDataMeta{
			Data: &AgentData{
				Frontmatter: map[string]any{
					"name":        "test-agent",
					"description": "A valid agent",
				},
				Body: "You are a helpful assistant.",
			},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAgentHandler_Validate_EmptyName(t *testing.T) {
	h := &AgentHandler{}
	err := h.Validate(Asset{
		Name: "",
		Meta: AgentMeta{},
	})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestAgentHandler_Validate_MissingDescription(t *testing.T) {
	h := &AgentHandler{}
	err := h.Validate(Asset{
		Name: "test",
		Meta: AgentDataMeta{
			Data: &AgentData{
				Frontmatter: map[string]any{},
				Body:        "Some body",
			},
		},
	})
	if err == nil {
		t.Error("expected error for missing description")
	}
}

func TestAgentHandler_Validate_EmptyBody(t *testing.T) {
	h := &AgentHandler{}
	err := h.Validate(Asset{
		Name: "test",
		Meta: AgentDataMeta{
			Data: &AgentData{
				Frontmatter: map[string]any{
					"description": "Has desc",
				},
				Body: "",
			},
		},
	})
	if err == nil {
		t.Error("expected error for empty body")
	}
}

func TestAgentHandler_Validate_PlainAgentMeta(t *testing.T) {
	h := &AgentHandler{}
	err := h.Validate(Asset{
		Name: "test",
		Meta: AgentMeta{},
	})
	if err != nil {
		t.Errorf("plain AgentMeta should be valid: %v", err)
	}
}

func TestAgentHandler_Validate_WrongMetaType(t *testing.T) {
	h := &AgentHandler{}
	err := h.Validate(Asset{
		Name: "test",
		Meta: SkillMeta{},
	})
	if err == nil {
		t.Error("expected error for wrong meta type")
	}
}

// ---------------------------------------------------------------------------
// ParseManifestEntries
// ---------------------------------------------------------------------------

func TestAgentHandler_ParseManifestEntries(t *testing.T) {
	h := &AgentHandler{}

	raw := json.RawMessage(`[
		{"name": "code-reviewer", "description": "Reviews code", "source": "github.com/acme/agents/code-reviewer"},
		{"name": "test-writer", "description": "Writes tests", "source": "github.com/acme/agents/test-writer", "commit": "abc123def456"}
	]`)

	entries, err := h.ParseManifestEntries(raw)
	if err != nil {
		t.Fatalf("ParseManifestEntries() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "code-reviewer" {
		t.Errorf("entries[0].Name = %q", entries[0].Name)
	}
	if entries[0].Description != "Reviews code" {
		t.Errorf("entries[0].Description = %q", entries[0].Description)
	}
	if entries[0].Source != "github.com/acme/agents/code-reviewer" {
		t.Errorf("entries[0].Source = %q", entries[0].Source)
	}
	if _, ok := entries[0].Meta.(AgentMeta); !ok {
		t.Errorf("entries[0].Meta is %T, want AgentMeta", entries[0].Meta)
	}
	if entries[1].Commit != "abc123def456" {
		t.Errorf("entries[1].Commit = %q", entries[1].Commit)
	}
}

func TestAgentHandler_ParseManifestEntries_Invalid(t *testing.T) {
	h := &AgentHandler{}

	raw := json.RawMessage(`not json`)
	_, err := h.ParseManifestEntries(raw)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// LockData
// ---------------------------------------------------------------------------

func TestAgentHandler_LockData(t *testing.T) {
	h := &AgentHandler{}

	a := Asset{
		Kind:   KindAgent,
		Name:   "code-reviewer",
		Source: "github.com/acme/agents/code-reviewer",
	}
	info := InstallInfo{
		Commit: "abc123",
		Ref:    "main",
	}

	locked := h.LockData(a, info)
	if locked.Kind != KindAgent {
		t.Errorf("Kind = %q", locked.Kind)
	}
	if locked.Name != "code-reviewer" {
		t.Errorf("Name = %q", locked.Name)
	}
	if locked.Source != "github.com/acme/agents/code-reviewer" {
		t.Errorf("Source = %q", locked.Source)
	}
	if locked.Commit != "abc123" {
		t.Errorf("Commit = %q", locked.Commit)
	}
	if locked.Ref != "main" {
		t.Errorf("Ref = %q", locked.Ref)
	}
	// Agent lock data should NOT have a data map (thin format).
	if locked.Data != nil {
		t.Errorf("Data should be nil, got %v", locked.Data)
	}
}

// ---------------------------------------------------------------------------
// marshalOrderedYAML
// ---------------------------------------------------------------------------

func TestMarshalOrderedYAML_EmptyMap(t *testing.T) {
	out, err := marshalOrderedYAML(map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil for empty map, got %q", out)
	}
}

func TestMarshalOrderedYAML_PriorityOrder(t *testing.T) {
	m := map[string]any{
		"zebra":       "z",
		"tools":       []string{"Read"},
		"description": "desc",
		"model":       "m",
		"alpha":       "a",
		"name":        "n",
	}

	out, err := marshalOrderedYAML(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(out)
	nameIdx := strings.Index(content, "name:")
	descIdx := strings.Index(content, "description:")
	modelIdx := strings.Index(content, "model:")
	toolsIdx := strings.Index(content, "tools:")
	alphaIdx := strings.Index(content, "alpha:")
	zebraIdx := strings.Index(content, "zebra:")

	if nameIdx >= descIdx {
		t.Error("name should come before description")
	}
	if descIdx >= modelIdx {
		t.Error("description should come before model")
	}
	if modelIdx >= toolsIdx {
		t.Error("model should come before tools")
	}
	if toolsIdx >= alphaIdx {
		t.Error("tools should come before alpha")
	}
	if alphaIdx >= zebraIdx {
		t.Error("alpha should come before zebra")
	}
}
