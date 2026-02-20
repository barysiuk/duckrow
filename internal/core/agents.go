package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed agents.json
var embeddedAgentsJSON []byte

// LoadAgents parses the embedded agent definitions.
func LoadAgents() ([]AgentDef, error) {
	var agents []AgentDef
	if err := json.Unmarshal(embeddedAgentsJSON, &agents); err != nil {
		return nil, fmt.Errorf("parsing agent definitions: %w", err)
	}
	return agents, nil
}

// DetectAgents returns the list of agents detected on this system.
// Detection checks whether agent-specific config directories exist.
func DetectAgents(agents []AgentDef) []AgentDef {
	var detected []AgentDef
	for _, agent := range agents {
		if isAgentDetected(agent) {
			detected = append(detected, agent)
		}
	}
	return detected
}

// DetectAgentsInFolder returns agents detected for a specific project folder.
// It checks both global detection paths and project-local skill directories.
func DetectAgentsInFolder(agents []AgentDef, folderPath string) []AgentDef {
	var detected []AgentDef
	for _, agent := range agents {
		// Check if the agent's project skill directory exists in this folder
		skillDir := filepath.Join(folderPath, agent.SkillsDir)
		if dirExists(skillDir) {
			detected = append(detected, agent)
			continue
		}
		// Fall back to global detection
		if isAgentDetected(agent) {
			detected = append(detected, agent)
		}
	}
	return detected
}

// GetUniversalAgents returns agents that use .agents/skills as their project skill directory.
func GetUniversalAgents(agents []AgentDef) []AgentDef {
	var universal []AgentDef
	for _, a := range agents {
		if a.Universal {
			universal = append(universal, a)
		}
	}
	return universal
}

// GetNonUniversalAgents returns agents with their own skill directories.
func GetNonUniversalAgents(agents []AgentDef) []AgentDef {
	var nonUniversal []AgentDef
	for _, a := range agents {
		if !a.Universal {
			nonUniversal = append(nonUniversal, a)
		}
	}
	return nonUniversal
}

// GetMCPCapableAgents returns agents that have MCP config paths defined.
func GetMCPCapableAgents(agents []AgentDef) []AgentDef {
	var capable []AgentDef
	for _, a := range agents {
		if a.MCPConfigPath != "" {
			capable = append(capable, a)
		}
	}
	return capable
}

// ResolveMCPConfigPath resolves the full path to an agent's MCP config file
// relative to the given project directory.
func ResolveMCPConfigPath(agent AgentDef, projectDir string) string {
	if agent.MCPConfigPath == "" {
		return ""
	}
	return filepath.Join(projectDir, agent.MCPConfigPath)
}

// ResolveAgentSkillsDir resolves the project-level skill directory for an agent,
// relative to the given base directory.
func ResolveAgentSkillsDir(agent AgentDef, baseDir string) string {
	return filepath.Join(baseDir, agent.SkillsDir)
}

// ResolveAgentsByNames returns agents matching the given names.
// Returns an error if any name doesn't match a known agent.
func ResolveAgentsByNames(agents []AgentDef, names []string) ([]AgentDef, error) {
	agentMap := make(map[string]AgentDef, len(agents))
	for _, a := range agents {
		agentMap[a.Name] = a
	}

	var resolved []AgentDef
	for _, name := range names {
		agent, ok := agentMap[name]
		if !ok {
			var valid []string
			for _, a := range agents {
				if !a.Universal {
					valid = append(valid, a.Name)
				}
			}
			return nil, fmt.Errorf("unknown agent %q; available: %s", name, strings.Join(valid, ", "))
		}
		resolved = append(resolved, agent)
	}
	return resolved, nil
}

// ResolveAgentGlobalSkillsDir resolves the global skill directory for an agent,
// expanding ~ and environment variables.
func ResolveAgentGlobalSkillsDir(agent AgentDef) string {
	return expandPath(agent.GlobalSkillsDir)
}

func isAgentDetected(agent AgentDef) bool {
	for _, p := range agent.DetectPaths {
		expanded := expandPath(p)
		if dirExists(expanded) {
			return true
		}
	}
	return false
}

// expandPath expands ~ to home directory and $VAR / $XDG_CONFIG to env values.
func expandPath(p string) string {
	// Handle $XDG_CONFIG
	if strings.Contains(p, "$XDG_CONFIG") {
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			home, _ := os.UserHomeDir()
			xdgConfig = filepath.Join(home, ".config")
		}
		p = strings.ReplaceAll(p, "$XDG_CONFIG", xdgConfig)
	}

	// Handle other env vars like $CODEX_HOME
	if strings.Contains(p, "$") {
		p = os.Expand(p, func(key string) string {
			if key == "XDG_CONFIG" {
				// Already handled above
				return ""
			}
			return os.Getenv(key)
		})
	}

	// Handle ~
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[2:])
	} else if p == "~" {
		home, _ := os.UserHomeDir()
		p = home
	}

	return p
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
