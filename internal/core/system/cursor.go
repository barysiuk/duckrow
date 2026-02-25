package system

import "github.com/barysiuk/duckrow/internal/core/asset"

// Cursor implements the System interface for the Cursor editor.
type Cursor struct {
	BaseSystem
}

// NewCursor creates a configured Cursor system.
func NewCursor() *Cursor {
	return &Cursor{BaseSystem{
		name:            "cursor",
		displayName:     "Cursor",
		universal:       false,
		skillsDir:       ".cursor/skills",
		globalSkillsDir: "~/.cursor/skills",
		detectPaths:     []string{"~/.cursor"},
		configSignals:   []string{".cursor"},
		supportedKinds:  []asset.Kind{asset.KindSkill, asset.KindMCP},
		mcpConfigPath:   ".cursor/mcp.json",
		mcpConfigKey:    "mcpServers",
		mcpConfigFormat: "jsonc",
	}}
}

// Cursor uses the default BaseSystem behavior for both skills (symlink)
// and MCPs (standard { "command": "...", "args": [...] } format).

func init() { Register(NewCursor()) }
