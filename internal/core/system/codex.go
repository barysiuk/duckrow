package system

import "github.com/barysiuk/duckrow/internal/core/asset"

// Codex implements the System interface for the Codex CLI.
type Codex struct {
	BaseSystem
}

// NewCodex creates a configured Codex system.
func NewCodex() *Codex {
	return &Codex{BaseSystem{
		name:            "codex",
		displayName:     "Codex",
		universal:       true,
		skillsDir:       ".agents/skills",
		globalSkillsDir: "$CODEX_HOME/skills",
		detectPaths:     []string{"$CODEX_HOME", "/etc/codex"},
		configSignals:   []string{"codex.md"},
		supportedKinds:  []asset.Kind{asset.KindSkill},
	}}
}

// Codex is skills-only and uses the default universal BaseSystem behavior.

func init() { Register(NewCodex()) }
