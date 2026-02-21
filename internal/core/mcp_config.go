package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tailscale/hujson"
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
	Agent      AgentDef
	FilePath   string // Absolute path to the config file
	ConfigPath string // Project-relative config path (for display, e.g. "opencode.jsonc")
	Action     string // "wrote", "skipped", "error"
	Message    string // Human-readable detail (e.g. "already exists, use --force")
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
// Comments and user formatting are preserved in JSONC-capable agent configs.
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
// Comments and user formatting are preserved in JSONC-capable agent configs.
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
	relPath, _ := filepath.Rel(opts.ProjectDir, configPath)
	ar := MCPAgentResult{
		Agent:      agent,
		FilePath:   configPath,
		ConfigPath: relPath,
	}

	// Read existing config file content (or start with empty object).
	content, err := readConfigFile(configPath)
	if err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("reading config: %v", err)
		return ar
	}
	if content == "" {
		content = "{}"
	}

	// Parse as JSONC/JWCC AST — preserves comments and whitespace.
	root, err := hujson.Parse([]byte(content))
	if err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("parsing config: %v", err)
		return ar
	}

	// Check if entry already exists using JSON Pointer.
	entryPtr := "/" + jsonPointerEscape(agent.MCPConfigKey) + "/" + jsonPointerEscape(entry.Name)
	if root.Find(entryPtr) != nil && !opts.Force {
		ar.Action = "skipped"
		ar.Message = "already exists, use --force"
		return ar
	}

	// Build the agent-specific MCP config value as JSON.
	mcpValueJSON := buildAgentMCPConfig(entry, agent)

	// Determine the patch operation: "add" for new, "replace" for force-update.
	op := "add"
	if root.Find(entryPtr) != nil {
		op = "replace"
	}

	// Ensure the top-level config key object exists.
	topKeyPtr := "/" + jsonPointerEscape(agent.MCPConfigKey)
	if root.Find(topKeyPtr) == nil {
		topKeyPatch := fmt.Sprintf(`[{"op":"add","path":%q,"value":{}}]`, topKeyPtr)
		if err := root.Patch([]byte(topKeyPatch)); err != nil {
			ar.Action = "error"
			ar.Message = fmt.Sprintf("creating config key %q: %v", agent.MCPConfigKey, err)
			return ar
		}
	}

	// Apply the patch to add/replace the MCP entry.
	patch := fmt.Sprintf(`[{"op":%q,"path":%q,"value":%s}]`, op, entryPtr, mcpValueJSON)
	if err := root.Patch([]byte(patch)); err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("writing MCP entry: %v", err)
		return ar
	}

	// Format and finalize the output.
	output := finalizeConfig(&root, agent)

	// Write atomically.
	if err := writeConfigFile(configPath, string(output)); err != nil {
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
	relPath, _ := filepath.Rel(opts.ProjectDir, configPath)
	ar := MCPAgentResult{
		Agent:      agent,
		FilePath:   configPath,
		ConfigPath: relPath,
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

	// Parse as JSONC/JWCC AST.
	root, err := hujson.Parse([]byte(content))
	if err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("parsing config: %v", err)
		return ar
	}

	// Check if entry exists.
	entryPtr := "/" + jsonPointerEscape(agent.MCPConfigKey) + "/" + jsonPointerEscape(mcpName)
	if root.Find(entryPtr) == nil {
		ar.Action = "skipped"
		ar.Message = "entry not found"
		return ar
	}

	// Remove the entry.
	patch := fmt.Sprintf(`[{"op":"remove","path":%q}]`, entryPtr)
	if err := root.Patch([]byte(patch)); err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("removing MCP entry: %v", err)
		return ar
	}

	// Format and finalize the output.
	output := finalizeConfig(&root, agent)

	// Write back.
	if err := writeConfigFile(configPath, string(output)); err != nil {
		ar.Action = "error"
		ar.Message = fmt.Sprintf("saving config: %v", err)
		return ar
	}

	ar.Action = "removed"
	return ar
}

// finalizeConfig formats the JSONC AST and produces the final output bytes.
// For JSONC agents: preserves comments, removes trailing commas.
// For strict-JSON agents: removes comments and trailing commas.
func finalizeConfig(root *hujson.Value, agent AgentDef) []byte {
	root.Format()
	removeTrailingCommas(root)

	if agent.MCPConfigFormat != "jsonc" {
		// Strict JSON: also remove comments.
		root.Standardize()
	}

	return root.Pack()
}

// removeTrailingCommas walks the JSONC AST and removes trailing commas
// from objects and arrays. This is necessary because hujson.Format() adds
// trailing commas (JWCC style), but not all JSONC parsers support them.
func removeTrailingCommas(v *hujson.Value) {
	switch vv := v.Value.(type) {
	case *hujson.Object:
		for i := range vv.Members {
			removeTrailingCommas(&vv.Members[i].Name)
			removeTrailingCommas(&vv.Members[i].Value)
		}
		if len(vv.Members) > 0 {
			vv.Members[len(vv.Members)-1].Value.AfterExtra = nil
		}
	case *hujson.Array:
		for i := range vv.Elements {
			removeTrailingCommas(&vv.Elements[i])
		}
		if len(vv.Elements) > 0 {
			vv.Elements[len(vv.Elements)-1].AfterExtra = nil
		}
	}
}

// buildAgentMCPConfig translates a registry MCPEntry to the agent-specific JSON
// format. Returns a raw JSON string suitable for use in a JSON Patch value.
func buildAgentMCPConfig(entry MCPEntry, agent AgentDef) string {
	isStdio := entry.Command != ""

	if isStdio {
		return buildStdioMCPConfig(entry, agent)
	}
	return buildRemoteMCPConfig(entry, agent)
}

// buildStdioMCPConfig builds the agent-specific config for a stdio MCP.
// The command is wrapped with `duckrow env --mcp <name> -- <original-command> [args...]`.
// Returns indented JSON so that hujson inherits the formatting in the patch.
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
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		return string(data)

	case "github-copilot":
		// GitHub Copilot uses "type": "stdio" explicitly.
		m := map[string]interface{}{
			"type":    "stdio",
			"command": "duckrow",
			"args":    wrapperArgs,
		}
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		return string(data)

	default:
		// Cursor, Claude Code: no explicit type field for stdio.
		m := map[string]interface{}{
			"command": "duckrow",
			"args":    wrapperArgs,
		}
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		return string(data)
	}
}

// buildRemoteMCPConfig builds the agent-specific config for a remote MCP.
// Returns indented JSON so that hujson inherits the formatting in the patch.
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
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		return string(data)

	default:
		// Cursor, Claude Code, GitHub Copilot: use the type from the registry.
		m := map[string]interface{}{
			"type": mcpType,
			"url":  entry.URL,
		}
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		return string(data)
	}
}

// readConfigFile reads a config file and returns its content as a string.
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

// writeConfigFile writes content to a config file atomically.
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

// jsonPointerEscape escapes a string for use as a JSON Pointer token (RFC 6901).
// The characters '~' and '/' have special meaning and must be escaped.
func jsonPointerEscape(s string) string {
	// Per RFC 6901: '~' is escaped as '~0', '/' is escaped as '~1'.
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '~':
			result = append(result, '~', '0')
		case '/':
			result = append(result, '~', '1')
		default:
			result = append(result, s[i])
		}
	}
	return string(result)
}
