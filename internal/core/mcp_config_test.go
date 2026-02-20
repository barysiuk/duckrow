package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testMCPAgents returns a minimal set of MCP-capable agents for tests.
func testMCPAgents() []AgentDef {
	return []AgentDef{
		{
			Name:          "cursor",
			DisplayName:   "Cursor",
			MCPConfigPath: ".cursor/mcp.json",
			MCPConfigKey:  "mcpServers",
		},
		{
			Name:          "claude-code",
			DisplayName:   "Claude Code",
			MCPConfigPath: ".mcp.json",
			MCPConfigKey:  "mcpServers",
		},
		{
			Name:          "github-copilot",
			DisplayName:   "GitHub Copilot",
			MCPConfigPath: ".vscode/mcp.json",
			MCPConfigKey:  "servers",
		},
		{
			Name:          "opencode",
			DisplayName:   "OpenCode",
			MCPConfigPath: "opencode.json",
			MCPConfigKey:  "mcp",
		},
	}
}

// ---------------------------------------------------------------------------
// buildAgentMCPConfig tests
// ---------------------------------------------------------------------------

func TestBuildStdioMCPConfig_Cursor(t *testing.T) {
	entry := MCPEntry{
		Name:    "internal-db",
		Command: "npx",
		Args:    []string{"-y", "@acme/mcp-db-server"},
		Env:     map[string]string{"DATABASE_URL": "$DATABASE_URL"},
	}
	agent := AgentDef{Name: "cursor", MCPConfigKey: "mcpServers"}

	got := buildAgentMCPConfig(entry, agent)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["command"] != "duckrow" {
		t.Errorf("command = %v, want \"duckrow\"", parsed["command"])
	}

	args, ok := parsed["args"].([]interface{})
	if !ok {
		t.Fatalf("args is not an array: %T", parsed["args"])
	}

	// Expected: ["env", "--mcp", "internal-db", "--", "npx", "-y", "@acme/mcp-db-server"]
	expectedArgs := []string{"env", "--mcp", "internal-db", "--", "npx", "-y", "@acme/mcp-db-server"}
	if len(args) != len(expectedArgs) {
		t.Fatalf("len(args) = %d, want %d", len(args), len(expectedArgs))
	}
	for i, ea := range expectedArgs {
		if args[i] != ea {
			t.Errorf("args[%d] = %v, want %q", i, args[i], ea)
		}
	}

	// Cursor should NOT have a "type" field for stdio.
	if _, hasType := parsed["type"]; hasType {
		t.Error("cursor stdio config should not have type field")
	}
}

func TestBuildStdioMCPConfig_GithubCopilot(t *testing.T) {
	entry := MCPEntry{
		Name:    "test-mcp",
		Command: "node",
		Args:    []string{"server.js"},
	}
	agent := AgentDef{Name: "github-copilot", MCPConfigKey: "servers"}

	got := buildAgentMCPConfig(entry, agent)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["type"] != "stdio" {
		t.Errorf("type = %v, want \"stdio\"", parsed["type"])
	}
	if parsed["command"] != "duckrow" {
		t.Errorf("command = %v, want \"duckrow\"", parsed["command"])
	}
}

func TestBuildStdioMCPConfig_OpenCode(t *testing.T) {
	entry := MCPEntry{
		Name:    "test-mcp",
		Command: "npx",
		Args:    []string{"-y", "@acme/server"},
	}
	agent := AgentDef{Name: "opencode", MCPConfigKey: "mcp"}

	got := buildAgentMCPConfig(entry, agent)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["type"] != "local" {
		t.Errorf("type = %v, want \"local\"", parsed["type"])
	}

	cmd, ok := parsed["command"].([]interface{})
	if !ok {
		t.Fatalf("command is not an array: %T", parsed["command"])
	}

	// First element should be "duckrow"
	if len(cmd) == 0 || cmd[0] != "duckrow" {
		t.Errorf("command[0] = %v, want \"duckrow\"", cmd[0])
	}

	// Should NOT have "args" field — command is the array.
	if _, hasArgs := parsed["args"]; hasArgs {
		t.Error("opencode config should not have args field")
	}
}

func TestBuildRemoteMCPConfig_Cursor(t *testing.T) {
	entry := MCPEntry{
		Name: "docs-search",
		Type: "http",
		URL:  "https://mcp.acme.com/mcp",
	}
	agent := AgentDef{Name: "cursor", MCPConfigKey: "mcpServers"}

	got := buildAgentMCPConfig(entry, agent)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["type"] != "http" {
		t.Errorf("type = %v, want \"http\"", parsed["type"])
	}
	if parsed["url"] != "https://mcp.acme.com/mcp" {
		t.Errorf("url = %v, want \"https://mcp.acme.com/mcp\"", parsed["url"])
	}
}

func TestBuildRemoteMCPConfig_OpenCode(t *testing.T) {
	entry := MCPEntry{
		Name: "docs-search",
		Type: "http",
		URL:  "https://mcp.acme.com/mcp",
	}
	agent := AgentDef{Name: "opencode", MCPConfigKey: "mcp"}

	got := buildAgentMCPConfig(entry, agent)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// OpenCode uses "remote" instead of "http"/"sse".
	if parsed["type"] != "remote" {
		t.Errorf("type = %v, want \"remote\"", parsed["type"])
	}
}

// ---------------------------------------------------------------------------
// InstallMCPConfig tests
// ---------------------------------------------------------------------------

func TestInstallMCPConfig_StdioMCP(t *testing.T) {
	projectDir := t.TempDir()
	agents := testMCPAgents()

	entry := MCPEntry{
		Name:    "internal-db",
		Command: "npx",
		Args:    []string{"-y", "@acme/mcp-db-server"},
		Env:     map[string]string{"DATABASE_URL": "$DATABASE_URL"},
	}

	result, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatalf("InstallMCPConfig failed: %v", err)
	}

	if len(result.AgentResults) != 4 {
		t.Fatalf("len(AgentResults) = %d, want 4", len(result.AgentResults))
	}

	for _, ar := range result.AgentResults {
		if ar.Action != "wrote" {
			t.Errorf("agent %s: action = %q, want \"wrote\"", ar.Agent.Name, ar.Action)
		}

		// Verify the file was created.
		data, err := os.ReadFile(ar.FilePath)
		if err != nil {
			t.Fatalf("agent %s: reading config file: %v", ar.Agent.Name, err)
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("agent %s: invalid JSON: %v", ar.Agent.Name, err)
		}

		// Check that the MCP key exists.
		topKey := ar.Agent.MCPConfigKey
		servers, ok := parsed[topKey].(map[string]interface{})
		if !ok {
			t.Fatalf("agent %s: %s is not an object", ar.Agent.Name, topKey)
		}

		if _, exists := servers["internal-db"]; !exists {
			t.Errorf("agent %s: MCP entry \"internal-db\" not found in %s", ar.Agent.Name, topKey)
		}
	}
}

func TestInstallMCPConfig_SkipsExisting(t *testing.T) {
	projectDir := t.TempDir()
	agents := []AgentDef{testMCPAgents()[0]} // Cursor only

	entry := MCPEntry{
		Name:    "test-mcp",
		Command: "node",
		Args:    []string{"server.js"},
	}

	// First install.
	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Second install without force should skip.
	result, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	if len(result.AgentResults) != 1 {
		t.Fatalf("len(AgentResults) = %d, want 1", len(result.AgentResults))
	}
	if result.AgentResults[0].Action != "skipped" {
		t.Errorf("action = %q, want \"skipped\"", result.AgentResults[0].Action)
	}
}

func TestInstallMCPConfig_ForceOverwrites(t *testing.T) {
	projectDir := t.TempDir()
	agents := []AgentDef{testMCPAgents()[0]} // Cursor only

	entry := MCPEntry{
		Name:    "test-mcp",
		Command: "node",
		Args:    []string{"server.js"},
	}

	// First install.
	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Second install with force should overwrite.
	result, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
		Force:        true,
	})
	if err != nil {
		t.Fatalf("forced install failed: %v", err)
	}

	if result.AgentResults[0].Action != "wrote" {
		t.Errorf("action = %q, want \"wrote\"", result.AgentResults[0].Action)
	}
}

func TestInstallMCPConfig_PreservesExistingKeys(t *testing.T) {
	projectDir := t.TempDir()
	agents := []AgentDef{{
		Name:          "opencode",
		DisplayName:   "OpenCode",
		MCPConfigPath: "opencode.json",
		MCPConfigKey:  "mcp",
	}}

	// Write an existing opencode.json with other keys.
	existing := `{"provider":"anthropic","model":"claude-3.5-sonnet"}`
	configPath := filepath.Join(projectDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := MCPEntry{
		Name:    "test-mcp",
		Command: "node",
		Args:    []string{"server.js"},
	}

	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatalf("InstallMCPConfig failed: %v", err)
	}

	// Read back and verify other keys are preserved.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["provider"] != "anthropic" {
		t.Errorf("provider = %v, want \"anthropic\"", parsed["provider"])
	}
	if parsed["model"] != "claude-3.5-sonnet" {
		t.Errorf("model = %v, want \"claude-3.5-sonnet\"", parsed["model"])
	}
	if _, exists := parsed["mcp"]; !exists {
		t.Error("mcp key not found after install")
	}
}

func TestInstallMCPConfig_RemoteMCP(t *testing.T) {
	projectDir := t.TempDir()
	agents := []AgentDef{testMCPAgents()[0]} // Cursor only

	entry := MCPEntry{
		Name: "docs-search",
		Type: "http",
		URL:  "https://mcp.acme.com/mcp",
	}

	result, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatalf("InstallMCPConfig failed: %v", err)
	}

	if result.AgentResults[0].Action != "wrote" {
		t.Errorf("action = %q, want \"wrote\"", result.AgentResults[0].Action)
	}

	// Verify the remote config was written correctly.
	data, err := os.ReadFile(result.AgentResults[0].FilePath)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers := parsed["mcpServers"].(map[string]interface{})
	mcpEntry := servers["docs-search"].(map[string]interface{})

	if mcpEntry["url"] != "https://mcp.acme.com/mcp" {
		t.Errorf("url = %v, want \"https://mcp.acme.com/mcp\"", mcpEntry["url"])
	}
	if mcpEntry["type"] != "http" {
		t.Errorf("type = %v, want \"http\"", mcpEntry["type"])
	}
}

func TestInstallMCPConfig_EmptyName(t *testing.T) {
	entry := MCPEntry{Command: "node"}
	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   t.TempDir(),
		TargetAgents: testMCPAgents(),
	})
	if err == nil {
		t.Fatal("expected error for empty MCP name")
	}
}

func TestInstallMCPConfig_EmptyProjectDir(t *testing.T) {
	entry := MCPEntry{Name: "test", Command: "node"}
	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		TargetAgents: testMCPAgents(),
	})
	if err == nil {
		t.Fatal("expected error for empty project dir")
	}
}

// ---------------------------------------------------------------------------
// UninstallMCPConfig tests
// ---------------------------------------------------------------------------

func TestUninstallMCPConfig_RemovesEntry(t *testing.T) {
	projectDir := t.TempDir()
	agents := testMCPAgents()

	// Install first.
	entry := MCPEntry{
		Name:    "internal-db",
		Command: "npx",
		Args:    []string{"-y", "@acme/mcp-db-server"},
	}
	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Uninstall.
	result, err := UninstallMCPConfig("internal-db", agents, MCPUninstallOptions{
		ProjectDir: projectDir,
	})
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}

	for _, ar := range result.AgentResults {
		if ar.Action != "removed" {
			t.Errorf("agent %s: action = %q, want \"removed\"", ar.Agent.Name, ar.Action)
		}

		// Verify entry was removed from file.
		data, err := os.ReadFile(ar.FilePath)
		if err != nil {
			t.Fatalf("agent %s: reading config: %v", ar.Agent.Name, err)
		}

		if strings.Contains(string(data), "internal-db") {
			t.Errorf("agent %s: config still contains \"internal-db\" after uninstall", ar.Agent.Name)
		}
	}
}

func TestUninstallMCPConfig_PreservesOtherEntries(t *testing.T) {
	projectDir := t.TempDir()
	agents := []AgentDef{testMCPAgents()[0]} // Cursor only

	// Install two MCPs.
	entry1 := MCPEntry{Name: "mcp-one", Command: "cmd1"}
	entry2 := MCPEntry{Name: "mcp-two", Command: "cmd2"}

	_, err := InstallMCPConfig(entry1, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = InstallMCPConfig(entry2, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Remove only mcp-one.
	_, err = UninstallMCPConfig("mcp-one", agents, MCPUninstallOptions{
		ProjectDir: projectDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify mcp-two still exists.
	configPath := filepath.Join(projectDir, ".cursor", "mcp.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(data), "mcp-one") {
		t.Error("config still contains \"mcp-one\" after removal")
	}
	if !strings.Contains(string(data), "mcp-two") {
		t.Error("config lost \"mcp-two\" — should have been preserved")
	}
}

func TestUninstallMCPConfig_FileNotExists(t *testing.T) {
	projectDir := t.TempDir()
	agents := testMCPAgents()

	result, err := UninstallMCPConfig("nonexistent", agents, MCPUninstallOptions{
		ProjectDir: projectDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, ar := range result.AgentResults {
		if ar.Action != "skipped" {
			t.Errorf("agent %s: action = %q, want \"skipped\"", ar.Agent.Name, ar.Action)
		}
	}
}

func TestUninstallMCPConfig_EntryNotInFile(t *testing.T) {
	projectDir := t.TempDir()
	agents := []AgentDef{testMCPAgents()[0]}

	// Install one MCP, then try to uninstall a different name.
	entry := MCPEntry{Name: "mcp-one", Command: "cmd1"}
	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := UninstallMCPConfig("mcp-nonexistent", agents, MCPUninstallOptions{
		ProjectDir: projectDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.AgentResults[0].Action != "skipped" {
		t.Errorf("action = %q, want \"skipped\"", result.AgentResults[0].Action)
	}
}

// ---------------------------------------------------------------------------
// escapeJSONKey tests
// ---------------------------------------------------------------------------

func TestEscapeJSONKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple-name", "simple-name"},
		{"name.with.dots", `\name.with.dots`},
		{"name*star", `\name*star`},
		{"normal", "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeJSONKey(tt.input)
			if got != tt.want {
				t.Errorf("escapeJSONKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Multiple MCPs installed in sequence
// ---------------------------------------------------------------------------

func TestInstallMCPConfig_MultipleMCPs(t *testing.T) {
	projectDir := t.TempDir()
	agents := []AgentDef{testMCPAgents()[0]} // Cursor only

	entries := []MCPEntry{
		{Name: "mcp-a", Command: "cmd-a"},
		{Name: "mcp-b", Command: "cmd-b"},
		{Name: "mcp-c", Type: "http", URL: "https://example.com/mcp"},
	}

	for _, entry := range entries {
		_, err := InstallMCPConfig(entry, MCPInstallOptions{
			ProjectDir:   projectDir,
			TargetAgents: agents,
		})
		if err != nil {
			t.Fatalf("installing %q: %v", entry.Name, err)
		}
	}

	// Verify all three exist.
	configPath := filepath.Join(projectDir, ".cursor", "mcp.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers := parsed["mcpServers"].(map[string]interface{})
	for _, entry := range entries {
		if _, exists := servers[entry.Name]; !exists {
			t.Errorf("MCP %q not found in config", entry.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Creates parent directories
// ---------------------------------------------------------------------------

func TestInstallMCPConfig_CreatesParentDirs(t *testing.T) {
	projectDir := t.TempDir()
	agents := []AgentDef{{
		Name:          "cursor",
		DisplayName:   "Cursor",
		MCPConfigPath: ".cursor/mcp.json",
		MCPConfigKey:  "mcpServers",
	}}

	entry := MCPEntry{Name: "test-mcp", Command: "node"}

	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: agents,
	})
	if err != nil {
		t.Fatal(err)
	}

	// .cursor/ directory should have been created.
	cursorDir := filepath.Join(projectDir, ".cursor")
	if !dirExists(cursorDir) {
		t.Error(".cursor directory was not created")
	}
}
