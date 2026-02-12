package core

import (
	"testing"
)

func TestLoadAgents(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents() error: %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("LoadAgents() returned empty list")
	}

	// Verify we have the expected agents
	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}

	expected := []string{"opencode", "claude-code", "cursor", "codex", "goose", "gemini-cli"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected agent %q not found", name)
		}
	}
}

func TestLoadAgents_Fields(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents() error: %v", err)
	}

	for _, a := range agents {
		if a.Name == "" {
			t.Error("agent has empty name")
		}
		if a.DisplayName == "" {
			t.Errorf("agent %q has empty displayName", a.Name)
		}
		if a.SkillsDir == "" {
			t.Errorf("agent %q has empty skillsDir", a.Name)
		}
		if a.GlobalSkillsDir == "" {
			t.Errorf("agent %q has empty globalSkillsDir", a.Name)
		}
		if len(a.DetectPaths) == 0 {
			t.Errorf("agent %q has no detectPaths", a.Name)
		}
	}
}

func TestGetUniversalAgents(t *testing.T) {
	agents, _ := LoadAgents()
	universal := GetUniversalAgents(agents)

	if len(universal) == 0 {
		t.Fatal("no universal agents found")
	}

	for _, a := range universal {
		if !a.Universal {
			t.Errorf("agent %q returned by GetUniversalAgents but Universal=false", a.Name)
		}
		if a.SkillsDir != ".agents/skills" {
			t.Errorf("universal agent %q has unexpected skillsDir: %s", a.Name, a.SkillsDir)
		}
	}
}

func TestGetNonUniversalAgents(t *testing.T) {
	agents, _ := LoadAgents()
	nonUniversal := GetNonUniversalAgents(agents)

	if len(nonUniversal) == 0 {
		t.Fatal("no non-universal agents found")
	}

	for _, a := range nonUniversal {
		if a.Universal {
			t.Errorf("agent %q returned by GetNonUniversalAgents but Universal=true", a.Name)
		}
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHome bool // if true, expect path to start with home dir
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
