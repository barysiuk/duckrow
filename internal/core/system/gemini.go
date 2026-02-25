package system

import "github.com/barysiuk/duckrow/internal/core/asset"

// GeminiCLI implements the System interface for the Gemini CLI.
type GeminiCLI struct {
	BaseSystem
}

// NewGeminiCLI creates a configured Gemini CLI system.
func NewGeminiCLI() *GeminiCLI {
	return &GeminiCLI{BaseSystem{
		name:            "gemini-cli",
		displayName:     "Gemini CLI",
		universal:       true,
		skillsDir:       ".agents/skills",
		globalSkillsDir: "~/.gemini/skills",
		detectPaths:     []string{"~/.gemini"},
		configSignals:   []string{"GEMINI.md"},
		supportedKinds:  []asset.Kind{asset.KindSkill},
	}}
}

// Gemini CLI is skills-only and uses the default universal BaseSystem behavior.

func init() { Register(NewGeminiCLI()) }
