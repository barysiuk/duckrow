package core

import (
	"os"
	"path/filepath"
	"strings"
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

func TestResolveAgentsByNames_SingleAgent(t *testing.T) {
	agents, _ := LoadAgents()

	resolved, err := ResolveAgentsByNames(agents, []string{"cursor"})
	if err != nil {
		t.Fatalf("ResolveAgentsByNames() error: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(resolved))
	}
	if resolved[0].Name != "cursor" {
		t.Errorf("expected cursor, got %s", resolved[0].Name)
	}
}

func TestResolveAgentsByNames_MultipleAgents(t *testing.T) {
	agents, _ := LoadAgents()

	resolved, err := ResolveAgentsByNames(agents, []string{"cursor", "claude-code", "goose"})
	if err != nil {
		t.Fatalf("ResolveAgentsByNames() error: %v", err)
	}
	if len(resolved) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(resolved))
	}

	names := make(map[string]bool)
	for _, a := range resolved {
		names[a.Name] = true
	}
	for _, want := range []string{"cursor", "claude-code", "goose"} {
		if !names[want] {
			t.Errorf("expected agent %q in result", want)
		}
	}
}

func TestResolveAgentsByNames_UnknownAgent(t *testing.T) {
	agents, _ := LoadAgents()

	_, err := ResolveAgentsByNames(agents, []string{"cursor", "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown agent, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the unknown agent name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "available:") {
		t.Errorf("error should list available agents, got: %v", err)
	}
}

func TestResolveAgentsByNames_EmptyList(t *testing.T) {
	agents, _ := LoadAgents()

	resolved, err := ResolveAgentsByNames(agents, []string{})
	if err != nil {
		t.Fatalf("ResolveAgentsByNames() error: %v", err)
	}
	if len(resolved) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(resolved))
	}
}

func TestResolveAgentsByNames_UniversalAgent(t *testing.T) {
	agents, _ := LoadAgents()

	// Universal agents should also be resolvable by name.
	resolved, err := ResolveAgentsByNames(agents, []string{"opencode"})
	if err != nil {
		t.Fatalf("ResolveAgentsByNames() error: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(resolved))
	}
	if !resolved[0].Universal {
		t.Error("expected opencode to be universal")
	}
}

func TestDetectAgents(t *testing.T) {
	// Create a temp dir to act as a fake home with agent config dirs.
	tmpDir := t.TempDir()

	agents := []AgentDef{
		{Name: "found", DisplayName: "Found Agent", DetectPaths: []string{filepath.Join(tmpDir, ".found-agent")}},
		{Name: "missing", DisplayName: "Missing Agent", DetectPaths: []string{filepath.Join(tmpDir, ".missing-agent")}},
	}

	// Create the detect dir for "found" only.
	if err := os.MkdirAll(filepath.Join(tmpDir, ".found-agent"), 0o755); err != nil {
		t.Fatal(err)
	}

	detected := DetectAgents(agents)
	if len(detected) != 1 {
		t.Fatalf("expected 1 detected agent, got %d", len(detected))
	}
	if detected[0].Name != "found" {
		t.Errorf("expected 'found', got %q", detected[0].Name)
	}
}

func TestDetectAgents_NoneDetected(t *testing.T) {
	agents := []AgentDef{
		{Name: "a", DetectPaths: []string{"/nonexistent/path/a"}},
		{Name: "b", DetectPaths: []string{"/nonexistent/path/b"}},
	}

	detected := DetectAgents(agents)
	if len(detected) != 0 {
		t.Fatalf("expected 0 detected agents, got %d", len(detected))
	}
}

func TestDetectAgentsInFolder(t *testing.T) {
	tmpDir := t.TempDir()

	agents := []AgentDef{
		{Name: "has-skills-dir", SkillsDir: ".agent-a/skills", DetectPaths: []string{"/nonexistent"}},
		{Name: "has-global-only", SkillsDir: ".agent-b/skills", DetectPaths: []string{filepath.Join(tmpDir, ".agent-b-global")}},
		{Name: "not-present", SkillsDir: ".agent-c/skills", DetectPaths: []string{"/nonexistent"}},
	}

	// Create project-local skills dir for first agent.
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agent-a/skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create global detect path for second agent.
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agent-b-global"), 0o755); err != nil {
		t.Fatal(err)
	}

	detected := DetectAgentsInFolder(agents, tmpDir)
	if len(detected) != 2 {
		t.Fatalf("expected 2 detected agents, got %d", len(detected))
	}

	names := make(map[string]bool)
	for _, a := range detected {
		names[a.Name] = true
	}
	if !names["has-skills-dir"] {
		t.Error("expected 'has-skills-dir' to be detected via project-local dir")
	}
	if !names["has-global-only"] {
		t.Error("expected 'has-global-only' to be detected via global path")
	}
	if names["not-present"] {
		t.Error("'not-present' should not be detected")
	}
}

func TestGetMCPCapableAgents(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents() error: %v", err)
	}

	capable := GetMCPCapableAgents(agents)

	// Expect exactly 4 MCP-capable agents: opencode, claude-code, cursor, github-copilot.
	if len(capable) != 4 {
		t.Fatalf("len(capable) = %d, want 4", len(capable))
	}

	names := make(map[string]bool)
	for _, a := range capable {
		names[a.Name] = true
		if a.MCPConfigPath == "" {
			t.Errorf("MCP-capable agent %q has empty MCPConfigPath", a.Name)
		}
		if a.MCPConfigKey == "" {
			t.Errorf("MCP-capable agent %q has empty MCPConfigKey", a.Name)
		}
	}

	expected := []string{"opencode", "claude-code", "cursor", "github-copilot"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected MCP-capable agent %q not found", name)
		}
	}
}

func TestGetMCPCapableAgents_NonMCPAgentsExcluded(t *testing.T) {
	agents, _ := LoadAgents()
	capable := GetMCPCapableAgents(agents)

	capableNames := make(map[string]bool)
	for _, a := range capable {
		capableNames[a.Name] = true
	}

	// These agents should NOT be MCP-capable.
	nonMCP := []string{"codex", "gemini-cli", "goose", "windsurf", "cline"}
	for _, name := range nonMCP {
		if capableNames[name] {
			t.Errorf("agent %q should not be MCP-capable", name)
		}
	}
}

func TestResolveMCPConfigPath(t *testing.T) {
	agent := AgentDef{
		Name:          "cursor",
		MCPConfigPath: ".cursor/mcp.json",
	}

	got := ResolveMCPConfigPath(agent, "/projects/myapp")
	want := filepath.Join("/projects/myapp", ".cursor/mcp.json")
	if got != want {
		t.Errorf("ResolveMCPConfigPath = %q, want %q", got, want)
	}
}

func TestResolveMCPConfigPath_Empty(t *testing.T) {
	agent := AgentDef{
		Name: "codex",
		// No MCPConfigPath.
	}

	got := ResolveMCPConfigPath(agent, "/projects/myapp")
	if got != "" {
		t.Errorf("ResolveMCPConfigPath = %q, want empty", got)
	}
}

func TestResolveMCPConfigPath_AltExists(t *testing.T) {
	dir := t.TempDir()

	// Create the alt file (opencode.jsonc).
	altPath := filepath.Join(dir, "opencode.jsonc")
	if err := os.WriteFile(altPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	agent := AgentDef{
		Name:             "opencode",
		MCPConfigPath:    "opencode.json",
		MCPConfigPathAlt: "opencode.jsonc",
	}

	got := ResolveMCPConfigPath(agent, dir)
	if got != altPath {
		t.Errorf("ResolveMCPConfigPath = %q, want %q (alt path)", got, altPath)
	}
}

func TestResolveMCPConfigPath_AltNotExists(t *testing.T) {
	dir := t.TempDir()

	// No opencode.jsonc on disk — should fall back to primary.
	agent := AgentDef{
		Name:             "opencode",
		MCPConfigPath:    "opencode.json",
		MCPConfigPathAlt: "opencode.jsonc",
	}

	got := ResolveMCPConfigPath(agent, dir)
	want := filepath.Join(dir, "opencode.json")
	if got != want {
		t.Errorf("ResolveMCPConfigPath = %q, want %q (primary path)", got, want)
	}
}

func TestResolveMCPConfigPath_AltPreferredOverPrimary(t *testing.T) {
	dir := t.TempDir()

	// Both files exist — alt should win.
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode.jsonc"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	agent := AgentDef{
		Name:             "opencode",
		MCPConfigPath:    "opencode.json",
		MCPConfigPathAlt: "opencode.jsonc",
	}

	got := ResolveMCPConfigPath(agent, dir)
	want := filepath.Join(dir, "opencode.jsonc")
	if got != want {
		t.Errorf("ResolveMCPConfigPath = %q, want %q (alt preferred)", got, want)
	}
}

func TestResolveMCPConfigPathRel(t *testing.T) {
	dir := t.TempDir()

	agent := AgentDef{
		Name:             "opencode",
		MCPConfigPath:    "opencode.json",
		MCPConfigPathAlt: "opencode.jsonc",
	}

	// Alt not on disk — returns primary.
	got := ResolveMCPConfigPathRel(agent, dir)
	if got != "opencode.json" {
		t.Errorf("ResolveMCPConfigPathRel = %q, want %q", got, "opencode.json")
	}

	// Create alt file — returns alt.
	if err := os.WriteFile(filepath.Join(dir, "opencode.jsonc"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = ResolveMCPConfigPathRel(agent, dir)
	if got != "opencode.jsonc" {
		t.Errorf("ResolveMCPConfigPathRel = %q, want %q", got, "opencode.jsonc")
	}
}

func TestResolveMCPConfigPathRel_NoAlt(t *testing.T) {
	agent := AgentDef{
		Name:          "cursor",
		MCPConfigPath: ".cursor/mcp.json",
	}

	got := ResolveMCPConfigPathRel(agent, "/projects/myapp")
	if got != ".cursor/mcp.json" {
		t.Errorf("ResolveMCPConfigPathRel = %q, want %q", got, ".cursor/mcp.json")
	}
}

func TestResolveMCPConfigPathRel_Empty(t *testing.T) {
	agent := AgentDef{Name: "codex"}
	got := ResolveMCPConfigPathRel(agent, "/projects/myapp")
	if got != "" {
		t.Errorf("ResolveMCPConfigPathRel = %q, want empty", got)
	}
}

func TestLoadAgents_MCPFields(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents() error: %v", err)
	}

	// Verify specific agents have the expected MCP fields.
	mcpExpected := map[string]struct {
		configPath    string
		configPathAlt string
		configKey     string
	}{
		"opencode":       {configPath: "opencode.json", configPathAlt: "opencode.jsonc", configKey: "mcp"},
		"claude-code":    {configPath: ".mcp.json", configKey: "mcpServers"},
		"cursor":         {configPath: ".cursor/mcp.json", configKey: "mcpServers"},
		"github-copilot": {configPath: ".vscode/mcp.json", configKey: "servers"},
	}

	for _, a := range agents {
		if expected, ok := mcpExpected[a.Name]; ok {
			if a.MCPConfigPath != expected.configPath {
				t.Errorf("agent %q: MCPConfigPath = %q, want %q", a.Name, a.MCPConfigPath, expected.configPath)
			}
			if a.MCPConfigPathAlt != expected.configPathAlt {
				t.Errorf("agent %q: MCPConfigPathAlt = %q, want %q", a.Name, a.MCPConfigPathAlt, expected.configPathAlt)
			}
			if a.MCPConfigKey != expected.configKey {
				t.Errorf("agent %q: MCPConfigKey = %q, want %q", a.Name, a.MCPConfigKey, expected.configKey)
			}
		} else {
			// Non-MCP agents should have empty MCP fields.
			if a.MCPConfigPath != "" {
				t.Errorf("agent %q: MCPConfigPath = %q, want empty", a.Name, a.MCPConfigPath)
			}
			if a.MCPConfigKey != "" {
				t.Errorf("agent %q: MCPConfigKey = %q, want empty", a.Name, a.MCPConfigKey)
			}
		}
	}
}
