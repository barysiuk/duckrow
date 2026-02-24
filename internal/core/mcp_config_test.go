package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

// testMCPAgents returns a minimal set of MCP-capable agents for tests.
func testMCPAgents() []AgentDef {
	return []AgentDef{
		{
			Name:            "cursor",
			DisplayName:     "Cursor",
			MCPConfigPath:   ".cursor/mcp.json",
			MCPConfigKey:    "mcpServers",
			MCPConfigFormat: "jsonc",
		},
		{
			Name:          "claude-code",
			DisplayName:   "Claude Code",
			MCPConfigPath: ".mcp.json",
			MCPConfigKey:  "mcpServers",
		},
		{
			Name:            "github-copilot",
			DisplayName:     "GitHub Copilot",
			MCPConfigPath:   ".vscode/mcp.json",
			MCPConfigKey:    "servers",
			MCPConfigFormat: "jsonc",
		},
		{
			Name:            "opencode",
			DisplayName:     "OpenCode",
			MCPConfigPath:   "opencode.json",
			MCPConfigKey:    "mcp",
			MCPConfigFormat: "jsonc",
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
// jsonPointerEscape tests
// ---------------------------------------------------------------------------

func TestJSONPointerEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple-name", "simple-name"},
		{"name/with/slashes", "name~1with~1slashes"},
		{"name~tilde", "name~0tilde"},
		{"normal", "normal"},
		{"a~b/c", "a~0b~1c"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := jsonPointerEscape(tt.input)
			if got != tt.want {
				t.Errorf("jsonPointerEscape(%q) = %q, want %q", tt.input, got, tt.want)
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

// ---------------------------------------------------------------------------
// JSONC comment preservation tests
// ---------------------------------------------------------------------------

func TestInstallMCPConfig_PreservesComments_JSONC(t *testing.T) {
	projectDir := t.TempDir()
	agent := AgentDef{
		Name:            "opencode",
		DisplayName:     "OpenCode",
		MCPConfigPath:   "opencode.json",
		MCPConfigKey:    "mcp",
		MCPConfigFormat: "jsonc",
	}

	// Write a JSONC config with comments.
	existing := `{
  // Provider configuration
  "provider": "anthropic",
  "model": "claude-3.5-sonnet" // fast model
}`
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
		TargetAgents: []AgentDef{agent},
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// Comments should be preserved.
	if !strings.Contains(content, "// Provider configuration") {
		t.Error("line comment was not preserved")
	}
	if !strings.Contains(content, "// fast model") {
		t.Error("inline comment was not preserved")
	}

	// MCP entry should have been added.
	if !strings.Contains(content, "test-mcp") {
		t.Error("MCP entry was not added")
	}

	// Other keys should be preserved.
	if !strings.Contains(content, "anthropic") {
		t.Error("existing key 'provider' was lost")
	}
}

func TestInstallMCPConfig_StripsComments_StrictJSON(t *testing.T) {
	projectDir := t.TempDir()
	agent := AgentDef{
		Name:          "claude-code",
		DisplayName:   "Claude Code",
		MCPConfigPath: ".mcp.json",
		MCPConfigKey:  "mcpServers",
		// No MCPConfigFormat — strict JSON.
	}

	// Write a file with comments (user might have accidentally added them).
	existing := `{
  // This comment should be stripped
  "mcpServers": {}
}`
	configPath := filepath.Join(projectDir, ".mcp.json")
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
		TargetAgents: []AgentDef{agent},
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// Comments should be stripped for strict JSON.
	if strings.Contains(content, "//") {
		t.Error("comment was not stripped for strict JSON agent")
	}

	// Must be valid standard JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestInstallMCPConfig_NoTrailingCommas(t *testing.T) {
	projectDir := t.TempDir()
	agent := AgentDef{
		Name:            "opencode",
		DisplayName:     "OpenCode",
		MCPConfigPath:   "opencode.json",
		MCPConfigKey:    "mcp",
		MCPConfigFormat: "jsonc",
	}

	entry := MCPEntry{
		Name:    "test-mcp",
		Command: "node",
		Args:    []string{"server.js"},
	}

	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: []AgentDef{agent},
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// Should not have trailing commas (e.g. "},}" or "],}").
	if strings.Contains(content, ",\n}") || strings.Contains(content, ",\n\t}") || strings.Contains(content, ",\n\t\t}") {
		t.Errorf("output contains trailing commas:\n%s", content)
	}
}

func TestInstallMCPConfig_FormattedOutput(t *testing.T) {
	projectDir := t.TempDir()
	agent := AgentDef{
		Name:          "claude-code",
		DisplayName:   "Claude Code",
		MCPConfigPath: ".mcp.json",
		MCPConfigKey:  "mcpServers",
	}

	entry := MCPEntry{
		Name:    "test-mcp",
		Command: "node",
		Args:    []string{"server.js"},
	}

	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: []AgentDef{agent},
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(projectDir, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// Output should be formatted (contain newlines and indentation).
	if !strings.Contains(content, "\n") {
		t.Error("output is not formatted (no newlines)")
	}
	if !strings.Contains(content, "\t") {
		t.Error("output is not indented")
	}
}

func TestUninstallMCPConfig_PreservesComments(t *testing.T) {
	projectDir := t.TempDir()
	agent := AgentDef{
		Name:            "cursor",
		DisplayName:     "Cursor",
		MCPConfigPath:   ".cursor/mcp.json",
		MCPConfigKey:    "mcpServers",
		MCPConfigFormat: "jsonc",
	}

	// Write a JSONC config with comments and two MCP entries.
	existing := `{
	// Cursor MCP configuration
	"mcpServers": {
		// Database server for local dev
		"db-server": {
			"command": "node",
			"args": ["db.js"]
		},
		"other-server": {
			"command": "python",
			"args": ["serve.py"]
		}
	}
}`
	configPath := filepath.Join(projectDir, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	// Remove db-server.
	_, err := UninstallMCPConfig("db-server", []AgentDef{agent}, MCPUninstallOptions{
		ProjectDir: projectDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// Top-level comment should be preserved.
	if !strings.Contains(content, "// Cursor MCP configuration") {
		t.Error("top-level comment was not preserved after uninstall")
	}

	// The removed entry should be gone.
	if strings.Contains(content, "db-server") {
		t.Error("removed entry still present")
	}

	// The other entry should remain.
	if !strings.Contains(content, "other-server") {
		t.Error("other entry was lost after uninstall")
	}
}

func TestInstallMCPConfig_PreservesBlockComments(t *testing.T) {
	projectDir := t.TempDir()
	agent := AgentDef{
		Name:            "github-copilot",
		DisplayName:     "GitHub Copilot",
		MCPConfigPath:   ".vscode/mcp.json",
		MCPConfigKey:    "servers",
		MCPConfigFormat: "jsonc",
	}

	// Write a JSONC config with block comments.
	existing := `{
	/*
	 * GitHub Copilot MCP servers
	 * Managed by the team
	 */
	"servers": {}
}`
	configPath := filepath.Join(projectDir, ".vscode", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := MCPEntry{
		Name: "docs-search",
		Type: "http",
		URL:  "https://mcp.example.com/docs",
	}

	_, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: []AgentDef{agent},
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// Block comment should be preserved.
	if !strings.Contains(content, "Managed by the team") {
		t.Error("block comment was not preserved")
	}

	// MCP entry should have been added.
	if !strings.Contains(content, "docs-search") {
		t.Error("MCP entry was not added")
	}
}

// ---------------------------------------------------------------------------
// removeTrailingCommas tests
// ---------------------------------------------------------------------------

func TestRemoveTrailingCommas(t *testing.T) {
	input := []byte(`{
	"a": [1, 2, 3,],
	"b": {"x": 1, "y": 2,},
}`)

	v, err := hujson.Parse(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	removeTrailingCommas(&v)
	output := string(v.Pack())

	// Should not have trailing commas.
	if strings.Contains(output, ",]") || strings.Contains(output, ",\n]") {
		t.Error("trailing comma in array not removed")
	}
	if strings.Contains(output, ",}") || strings.Contains(output, ",\n}") {
		t.Error("trailing comma in object not removed")
	}

	// Content should be preserved.
	if !strings.Contains(output, `"a"`) || !strings.Contains(output, `"b"`) {
		t.Error("content was lost")
	}
}

// ---------------------------------------------------------------------------
// Alternative config path (.jsonc) tests
// ---------------------------------------------------------------------------

func TestInstallMCPConfig_UsesJsoncWhenPresent(t *testing.T) {
	projectDir := t.TempDir()

	agent := AgentDef{
		Name:             "opencode",
		DisplayName:      "OpenCode",
		MCPConfigPath:    "opencode.json",
		MCPConfigPathAlt: "opencode.jsonc",
		MCPConfigKey:     "mcp",
		MCPConfigFormat:  "jsonc",
	}

	// Create opencode.jsonc with existing content.
	jsoncPath := filepath.Join(projectDir, "opencode.jsonc")
	existing := `{
	// My OpenCode config
	"provider": "anthropic"
}`
	if err := os.WriteFile(jsoncPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := MCPEntry{
		Name:    "test-mcp",
		Command: "node",
		Args:    []string{"server.js"},
	}

	result, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: []AgentDef{agent},
	})
	if err != nil {
		t.Fatalf("InstallMCPConfig failed: %v", err)
	}

	// Should have written to the .jsonc file.
	if len(result.AgentResults) != 1 {
		t.Fatalf("expected 1 agent result, got %d", len(result.AgentResults))
	}
	ar := result.AgentResults[0]
	if ar.Action != "wrote" {
		t.Fatalf("expected action 'wrote', got %q: %s", ar.Action, ar.Message)
	}
	if ar.FilePath != jsoncPath {
		t.Errorf("FilePath = %q, want %q", ar.FilePath, jsoncPath)
	}
	if ar.ConfigPath != "opencode.jsonc" {
		t.Errorf("ConfigPath = %q, want %q", ar.ConfigPath, "opencode.jsonc")
	}

	// Verify the .jsonc file was updated (not a new .json created).
	data, err := os.ReadFile(jsoncPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "My OpenCode config") {
		t.Error("comment was lost from .jsonc file")
	}
	if !strings.Contains(content, "test-mcp") {
		t.Error("MCP entry not written to .jsonc file")
	}

	// Verify opencode.json was NOT created.
	jsonPath := filepath.Join(projectDir, "opencode.json")
	if _, err := os.Stat(jsonPath); err == nil {
		t.Error("opencode.json was created — should use .jsonc when it exists")
	}
}

func TestInstallMCPConfig_FallsBackToJson(t *testing.T) {
	projectDir := t.TempDir()

	agent := AgentDef{
		Name:             "opencode",
		DisplayName:      "OpenCode",
		MCPConfigPath:    "opencode.json",
		MCPConfigPathAlt: "opencode.jsonc",
		MCPConfigKey:     "mcp",
		MCPConfigFormat:  "jsonc",
	}

	// No .jsonc file exists — should create opencode.json.
	entry := MCPEntry{
		Name:    "test-mcp",
		Command: "node",
		Args:    []string{"server.js"},
	}

	result, err := InstallMCPConfig(entry, MCPInstallOptions{
		ProjectDir:   projectDir,
		TargetAgents: []AgentDef{agent},
	})
	if err != nil {
		t.Fatalf("InstallMCPConfig failed: %v", err)
	}

	ar := result.AgentResults[0]
	if ar.Action != "wrote" {
		t.Fatalf("expected action 'wrote', got %q: %s", ar.Action, ar.Message)
	}
	if ar.ConfigPath != "opencode.json" {
		t.Errorf("ConfigPath = %q, want %q", ar.ConfigPath, "opencode.json")
	}

	// Verify opencode.json was created.
	jsonPath := filepath.Join(projectDir, "opencode.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("opencode.json not created: %v", err)
	}
	if !strings.Contains(string(data), "test-mcp") {
		t.Error("MCP entry not written to opencode.json")
	}
}

func TestUninstallMCPConfig_UsesJsoncWhenPresent(t *testing.T) {
	projectDir := t.TempDir()

	agent := AgentDef{
		Name:             "opencode",
		DisplayName:      "OpenCode",
		MCPConfigPath:    "opencode.json",
		MCPConfigPathAlt: "opencode.jsonc",
		MCPConfigKey:     "mcp",
		MCPConfigFormat:  "jsonc",
	}

	// Create opencode.jsonc with an existing MCP entry.
	jsoncPath := filepath.Join(projectDir, "opencode.jsonc")
	existing := `{
	// My config
	"mcp": {
		"test-mcp": {
			"command": "duckrow",
			"args": ["env", "--mcp", "test-mcp", "--", "node", "server.js"]
		}
	}
}`
	if err := os.WriteFile(jsoncPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := UninstallMCPConfig("test-mcp", []AgentDef{agent}, MCPUninstallOptions{
		ProjectDir: projectDir,
	})
	if err != nil {
		t.Fatalf("UninstallMCPConfig failed: %v", err)
	}

	ar := result.AgentResults[0]
	if ar.Action != "removed" {
		t.Fatalf("expected action 'removed', got %q: %s", ar.Action, ar.Message)
	}
	if ar.FilePath != jsoncPath {
		t.Errorf("FilePath = %q, want %q", ar.FilePath, jsoncPath)
	}

	// Verify comment was preserved.
	data, err := os.ReadFile(jsoncPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "My config") {
		t.Error("comment was lost from .jsonc file after uninstall")
	}
	if strings.Contains(content, "test-mcp") {
		t.Error("MCP entry was not removed from .jsonc file")
	}
}
