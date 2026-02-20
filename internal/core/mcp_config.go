package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// MCPInstallOptions configures an MCP installation.
type MCPInstallOptions struct {
	ProjectDir   string     // Project root directory
	TargetAgents []AgentDef // Agents to write configs for; if nil, uses all MCP-capable detected agents
	Force        bool       // Overwrite existing MCP entry with the same name
}

// MCPInstallResult reports what happened for each agent during MCP install.
type MCPInstallResult struct {
	AgentResults []MCPAgentResult
}

// MCPAgentResult is the per-agent outcome of an MCP install or uninstall.
type MCPAgentResult struct {
	Agent    AgentDef
	FilePath string // Absolute path to the config file
	Action   string // "wrote", "skipped", "error"
	Message  string // Human-readable detail (e.g. "already exists, use --force")
}

// MCPUninstallOptions configures an MCP uninstallation.
type MCPUninstallOptions struct {
	ProjectDir string // Project root directory
}

// MCPUninstallResult reports what happened for each agent during MCP uninstall.
type MCPUninstallResult struct {
	AgentResults []MCPAgentResult
}

// InstallMCPConfig writes an MCP entry into agent config files.
// For stdio MCPs, the command is wrapped with `duckrow env --mcp <name>`.
// For remote MCPs, the config is written as-is.
func InstallMCPConfig(entry MCPEntry, opts MCPInstallOptions) (*MCPInstallResult, error) {
	if opts.ProjectDir == "" {
		return nil, fmt.Errorf("project directory is required")
	}
	if entry.Name == "" {
		return nil, fmt.Errorf("MCP name is required")
	}

	result := &MCPInstallResult{}

	for _, agent := range opts.TargetAgents {
		if agent.MCPConfigPath == "" {
			continue
		}

		agentResult := installMCPForAgent(entry, agent, opts)
		result.AgentResults = append(result.AgentResults, agentResult)
	}

	return result, nil
}

// UninstallMCPConfig removes an MCP entry from agent config files.
// It reads the lock file to determine which agents were targeted.
func UninstallMCPConfig(mcpName string, agents []AgentDef, opts MCPUninstallOptions) (*MCPUninstallResult, error) {
	if opts.ProjectDir == "" {
		return nil, fmt.Errorf("project directory is required")
	}
	if mcpName == "" {
		return nil, fmt.Errorf("MCP name is required")
	}

	result := &MCPUninstallResult{}

	for _, agent := range agents {
		if agent.MCPConfigPath == "" {
			continue
		}

		agentResult := uninstallMCPForAgent(mcpName, agent, opts)
		result.AgentResults = append(result.AgentResults, agentResult)
	}

	return result, nil
}

// installMCPForAgent writes a single MCP entry into one agent's config file.
func installMCPForAgent(entry MCPEntry, agent AgentDef, opts MCPInstallOptions) MCPAgentResult {
	configPath := ResolveMCPConfigPath(agent, opts.ProjectDir)
	ar := MCPAgentResult{
		Agent:    agent,
		FilePath: configPath,
	}

	// Read existing config file content (or start fresh).
	content, err := readConfigFile(configPath)
	if err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("reading config: %v", err)
		return ar
	}

	// Check if entry already exists.
	entryPath := agent.MCPConfigKey + "." + escapeJSONKey(entry.Name)
	if gjson.Get(content, entryPath).Exists() && !opts.Force {
		ar.Action = "skipped"
		ar.Message = "already exists, use --force"
		return ar
	}

	// Build the agent-specific MCP config value.
	mcpValue := buildAgentMCPConfig(entry, agent)

	// Set the entry in the config.
	newContent, err := sjson.SetRaw(content, entryPath, mcpValue)
	if err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("writing MCP entry: %v", err)
		return ar
	}

	// Write atomically.
	if err := writeConfigFile(configPath, newContent); err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("saving config: %v", err)
		return ar
	}

	ar.Action = "wrote"
	return ar
}

// uninstallMCPForAgent removes a single MCP entry from one agent's config file.
func uninstallMCPForAgent(mcpName string, agent AgentDef, opts MCPUninstallOptions) MCPAgentResult {
	configPath := ResolveMCPConfigPath(agent, opts.ProjectDir)
	ar := MCPAgentResult{
		Agent:    agent,
		FilePath: configPath,
	}

	// Read existing config file.
	content, err := readConfigFile(configPath)
	if err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("reading config: %v", err)
		return ar
	}
	if content == "" {
		ar.Action = "skipped"
		ar.Message = "config file does not exist"
		return ar
	}

	// Check if entry exists.
	entryPath := agent.MCPConfigKey + "." + escapeJSONKey(mcpName)
	if !gjson.Get(content, entryPath).Exists() {
		ar.Action = "skipped"
		ar.Message = "entry not found"
		return ar
	}

	// Delete the entry.
	newContent, err := sjson.Delete(content, entryPath)
	if err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("removing MCP entry: %v", err)
		return ar
	}

	// Write back.
	if err := writeConfigFile(configPath, newContent); err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("saving config: %v", err)
		return ar
	}

	ar.Action = "removed"
	return ar
}

// buildAgentMCPConfig translates a registry MCPEntry to the agent-specific JSON
// format. Returns a raw JSON string suitable for sjson.SetRaw.
func buildAgentMCPConfig(entry MCPEntry, agent AgentDef) string {
	isStdio := entry.Command != ""

	if isStdio {
		return buildStdioMCPConfig(entry, agent)
	}
	return buildRemoteMCPConfig(entry, agent)
}

// buildStdioMCPConfig builds the agent-specific config for a stdio MCP.
// The command is wrapped with `duckrow env --mcp <name> -- <original-command> [args...]`.
func buildStdioMCPConfig(entry MCPEntry, agent AgentDef) string {
	// Build the duckrow env wrapper args.
	wrapperArgs := []string{"env", "--mcp", entry.Name, "--"}
	wrapperArgs = append(wrapperArgs, entry.Command)
	wrapperArgs = append(wrapperArgs, entry.Args...)

	switch agent.Name {
	case "opencode":
		// OpenCode uses "type": "local" and "command" as an array.
		cmdArray := append([]string{"duckrow"}, wrapperArgs...)
		m := map[string]interface{}{
			"type":    "local",
			"command": cmdArray,
		}
		data, _ := json.Marshal(m)
		return string(data)

	case "github-copilot":
		// GitHub Copilot uses "type": "stdio" explicitly.
		m := map[string]interface{}{
			"type":    "stdio",
			"command": "duckrow",
			"args":    wrapperArgs,
		}
		data, _ := json.Marshal(m)
		return string(data)

	default:
		// Cursor, Claude Code: no explicit type field for stdio.
		m := map[string]interface{}{
			"command": "duckrow",
			"args":    wrapperArgs,
		}
		data, _ := json.Marshal(m)
		return string(data)
	}
}

// buildRemoteMCPConfig builds the agent-specific config for a remote MCP.
func buildRemoteMCPConfig(entry MCPEntry, agent AgentDef) string {
	mcpType := entry.Type
	if mcpType == "" {
		mcpType = "http"
	}

	switch agent.Name {
	case "opencode":
		// OpenCode uses "type": "remote" for remote MCPs.
		m := map[string]interface{}{
			"type": "remote",
			"url":  entry.URL,
		}
		data, _ := json.Marshal(m)
		return string(data)

	default:
		// Cursor, Claude Code, GitHub Copilot: use the type from the registry.
		m := map[string]interface{}{
			"type": mcpType,
			"url":  entry.URL,
		}
		data, _ := json.Marshal(m)
		return string(data)
	}
}

// readConfigFile reads a JSON config file and returns its content as a string.
// Returns empty string if the file does not exist.
func readConfigFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// writeConfigFile writes content to a JSON config file atomically.
// Creates parent directories if needed.
func writeConfigFile(path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Atomic write: temp file + rename.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// escapeJSONKey escapes a key for use with gjson/sjson path syntax.
// Keys containing dots or special characters need to be escaped.
func escapeJSONKey(key string) string {
	// sjson/gjson use dots as path separators. If the key contains dots,
	// we need to escape them. The sjson convention is to wrap in literal syntax.
	needsEscape := false
	for _, c := range key {
		if c == '.' || c == '*' || c == '?' || c == '#' {
			needsEscape = true
			break
		}
	}
	if needsEscape {
		return `\` + key
	}
	return key
}
