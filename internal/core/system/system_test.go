package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/barysiuk/duckrow/internal/core/asset"
)

func TestSystemRegistry(t *testing.T) {
	// All 7 systems should be registered via init().
	all := All()
	if len(all) != 7 {
		t.Fatalf("expected 7 systems, got %d", len(all))
	}

	expected := []string{"opencode", "claude-code", "cursor", "codex", "gemini-cli", "github-copilot", "goose"}
	names := make(map[string]bool)
	for _, s := range all {
		names[s.Name()] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected system %q not found in registry", name)
		}
	}
}

func TestByName(t *testing.T) {
	s, ok := ByName("cursor")
	if !ok {
		t.Fatal("ByName(cursor) not found")
	}
	if s.Name() != "cursor" {
		t.Errorf("Name() = %q", s.Name())
	}
	if s.DisplayName() != "Cursor" {
		t.Errorf("DisplayName() = %q", s.DisplayName())
	}
}

func TestByName_Unknown(t *testing.T) {
	_, ok := ByName("nonexistent")
	if ok {
		t.Error("expected ByName for unknown to return false")
	}
}

func TestByNames(t *testing.T) {
	systems, err := ByNames([]string{"opencode", "cursor"})
	if err != nil {
		t.Fatalf("ByNames() error: %v", err)
	}
	if len(systems) != 2 {
		t.Fatalf("expected 2, got %d", len(systems))
	}
}

func TestByNames_Unknown(t *testing.T) {
	_, err := ByNames([]string{"cursor", "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown system name")
	}
}

func TestUniversalSystems(t *testing.T) {
	uni := Universal()
	if len(uni) == 0 {
		t.Fatal("no universal systems found")
	}
	for _, s := range uni {
		if !s.IsUniversal() {
			t.Errorf("system %q is not universal", s.Name())
		}
	}
}

func TestNonUniversalSystems(t *testing.T) {
	nonUni := NonUniversal()
	if len(nonUni) == 0 {
		t.Fatal("no non-universal systems found")
	}
	for _, s := range nonUni {
		if s.IsUniversal() {
			t.Errorf("system %q is universal", s.Name())
		}
	}
}

func TestSupporting(t *testing.T) {
	skillSystems := Supporting(asset.KindSkill)
	if len(skillSystems) != 7 {
		t.Errorf("expected 7 systems supporting skills, got %d", len(skillSystems))
	}

	mcpSystems := Supporting(asset.KindMCP)
	// OpenCode, Claude Code, Cursor, GitHub Copilot = 4.
	if len(mcpSystems) != 4 {
		t.Errorf("expected 4 systems supporting MCP, got %d", len(mcpSystems))
	}
}

func TestSystemFields(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		universal   bool
		skillsDir   string
		supportsMCP bool
	}{
		{"opencode", "OpenCode", true, ".agents/skills", true},
		{"claude-code", "Claude Code", false, ".claude/skills", true},
		{"cursor", "Cursor", false, ".cursor/skills", true},
		{"codex", "Codex", true, ".agents/skills", false},
		{"gemini-cli", "Gemini CLI", true, ".agents/skills", false},
		{"github-copilot", "GitHub Copilot", true, ".agents/skills", true},
		{"goose", "Goose", false, ".goose/skills", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ok := ByName(tt.name)
			if !ok {
				t.Fatalf("system %q not found", tt.name)
			}

			if s.DisplayName() != tt.displayName {
				t.Errorf("DisplayName() = %q, want %q", s.DisplayName(), tt.displayName)
			}
			if s.IsUniversal() != tt.universal {
				t.Errorf("IsUniversal() = %v, want %v", s.IsUniversal(), tt.universal)
			}
			if s.Supports(asset.KindSkill) != true {
				t.Error("expected all systems to support skills")
			}
			if s.Supports(asset.KindMCP) != tt.supportsMCP {
				t.Errorf("Supports(MCP) = %v, want %v", s.Supports(asset.KindMCP), tt.supportsMCP)
			}

			// Check AssetDir for skills.
			dir := s.AssetDir(asset.KindSkill, "/projects/myapp")
			wantDir := filepath.Join("/projects/myapp", tt.skillsDir)
			if dir != wantDir {
				t.Errorf("AssetDir(skill) = %q, want %q", dir, wantDir)
			}
		})
	}
}

func TestIsActiveInFolder(t *testing.T) {
	dir := t.TempDir()

	// Create Cursor config signal.
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}

	cursor, _ := ByName("cursor")
	if !cursor.IsActiveInFolder(dir) {
		t.Error("expected Cursor to be active in folder with .cursor dir")
	}

	// OpenCode should not be active without config file.
	opencode, _ := ByName("opencode")
	if opencode.IsActiveInFolder(dir) {
		t.Error("expected OpenCode not to be active without config")
	}

	// Create OpenCode config.
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !opencode.IsActiveInFolder(dir) {
		t.Error("expected OpenCode to be active with opencode.json")
	}
}

func TestDetectionSignals(t *testing.T) {
	opencode, _ := ByName("opencode")
	signals := opencode.DetectionSignals()
	if len(signals) == 0 {
		t.Error("expected detection signals for OpenCode")
	}

	found := false
	for _, s := range signals {
		if s == "opencode.json" {
			found = true
		}
	}
	if !found {
		t.Error("expected opencode.json in detection signals")
	}
}

func TestNames(t *testing.T) {
	all := All()
	names := Names(all)
	if len(names) != len(all) {
		t.Errorf("Names() returned %d, expected %d", len(names), len(all))
	}
}

func TestDisplayNames(t *testing.T) {
	all := All()
	names := DisplayNames(all)
	if len(names) != len(all) {
		t.Errorf("DisplayNames() returned %d, expected %d", len(names), len(all))
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHome bool
	}{
		{"tilde expansion", "~/.cursor", true},
		{"plain path", "/usr/local/bin", false},
		{"relative path", ".agents/skills", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if tt.wantHome && result == tt.input {
				t.Errorf("expandPath(%q) = %q, expected ~ to be expanded", tt.input, result)
			}
			if tt.wantHome && result[0] != '/' {
				t.Errorf("expandPath(%q) = %q, expected absolute path", tt.input, result)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Skill", "my-skill"},
		{"skill@v2", "skill-v2"},
		{"---test---", "test"},
		{"UPPER", "upper"},
		{"a-b-c", "a-b-c"},
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
