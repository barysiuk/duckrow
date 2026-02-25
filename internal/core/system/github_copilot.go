package system

import (
	"encoding/json"

	"github.com/barysiuk/duckrow/internal/core/asset"
)

// GitHubCopilot implements the System interface for GitHub Copilot.
type GitHubCopilot struct {
	BaseSystem
}

// NewGitHubCopilot creates a configured GitHub Copilot system.
func NewGitHubCopilot() *GitHubCopilot {
	return &GitHubCopilot{BaseSystem{
		name:            "github-copilot",
		displayName:     "GitHub Copilot",
		universal:       true,
		skillsDir:       ".agents/skills",
		altSkillsDirs:   []string{".github/skills"},
		globalSkillsDir: "~/.copilot/skills",
		detectPaths:     []string{"~/.copilot"},
		configSignals:   []string{".github/copilot-instructions.md"},
		supportedKinds:  []asset.Kind{asset.KindSkill, asset.KindMCP},
		mcpConfigPath:   ".vscode/mcp.json",
		mcpConfigKey:    "servers",
		mcpConfigFormat: "jsonc",
	}}
}

// Install overrides BaseSystem to produce GitHub Copilot-specific MCP format.
func (g *GitHubCopilot) Install(a asset.Asset, projectDir string, opts InstallOptions) error {
	if a.Kind == asset.KindMCP {
		return g.installMCPCopilot(a, projectDir, opts)
	}
	return g.BaseSystem.Install(a, projectDir, opts)
}

func (g *GitHubCopilot) installMCPCopilot(a asset.Asset, projectDir string, opts InstallOptions) error {
	meta, ok := a.Meta.(asset.MCPMeta)
	if !ok {
		return nil
	}

	configPath := g.resolveMCPConfigPath(projectDir)

	content, err := readConfigFile(configPath)
	if err != nil {
		return err
	}
	if content == "" {
		content = "{}"
	}

	root, err := parseJSONC(content)
	if err != nil {
		return err
	}

	entryPtr := "/" + jsonPointerEscape(g.mcpConfigKey) + "/" + jsonPointerEscape(a.Name)
	if root.Find(entryPtr) != nil && !opts.Force {
		return ErrAlreadyExists
	}

	var mcpValueJSON string
	if meta.IsStdio() {
		// GitHub Copilot: { "type": "stdio", "command": "duckrow", "args": [...] }
		wrapperArgs := []string{"env", "--mcp", a.Name, "--"}
		wrapperArgs = append(wrapperArgs, meta.Command)
		wrapperArgs = append(wrapperArgs, meta.Args...)

		m := map[string]interface{}{
			"type":    "stdio",
			"command": "duckrow",
			"args":    wrapperArgs,
		}
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		mcpValueJSON = string(data)
	} else {
		mcpType := meta.Transport
		if mcpType == "" {
			mcpType = "http"
		}
		m := map[string]interface{}{
			"type": mcpType,
			"url":  meta.URL,
		}
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		mcpValueJSON = string(data)
	}

	return g.patchAndWrite(root, entryPtr, mcpValueJSON, configPath)
}

func init() { Register(NewGitHubCopilot()) }
