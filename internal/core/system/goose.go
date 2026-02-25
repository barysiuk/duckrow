package system

import "github.com/barysiuk/duckrow/internal/core/asset"

// Goose implements the System interface for the Goose AI coding tool.
type Goose struct {
	BaseSystem
}

// NewGoose creates a configured Goose system.
func NewGoose() *Goose {
	return &Goose{BaseSystem{
		name:            "goose",
		displayName:     "Goose",
		universal:       false,
		skillsDir:       ".goose/skills",
		globalSkillsDir: "$XDG_CONFIG/goose/skills",
		detectPaths:     []string{"$XDG_CONFIG/goose"},
		configSignals:   []string{".goose"},
		supportedKinds:  []asset.Kind{asset.KindSkill},
	}}
}

// Goose is skills-only, non-universal. Uses default BaseSystem behavior
// (symlink from .goose/skills/ to .agents/skills/).

func init() { Register(NewGoose()) }
