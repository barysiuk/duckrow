package system

import "github.com/barysiuk/duckrow/internal/core/asset"

// ClaudeCode implements the System interface for Claude Code.
type ClaudeCode struct {
	BaseSystem
}

// NewClaudeCode creates a configured Claude Code system.
func NewClaudeCode() *ClaudeCode {
	return &ClaudeCode{BaseSystem{
		name:            "claude-code",
		displayName:     "Claude Code",
		universal:       false,
		skillsDir:       ".claude/skills",
		agentsDir:       ".claude/agents",
		globalSkillsDir: "~/.claude/skills",
		detectPaths:     []string{"~/.claude"},
		configSignals:   []string{"CLAUDE.md", ".claude", ".mcp.json"},
		supportedKinds:  []asset.Kind{asset.KindSkill, asset.KindMCP, asset.KindAgent},
		mcpConfigPath:   ".mcp.json",
		mcpConfigKey:    "mcpServers",
	}}
}

// Claude Code uses the default BaseSystem behavior for both skills (symlink)
// and MCPs (standard { "command": "...", "args": [...] } format).

func init() { Register(NewClaudeCode()) }
