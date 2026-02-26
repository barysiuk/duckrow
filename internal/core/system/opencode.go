package system

import (
	"encoding/json"

	"github.com/barysiuk/duckrow/internal/core/asset"
)

// OpenCode implements the System interface for the OpenCode AI coding tool.
type OpenCode struct {
	BaseSystem
}

// NewOpenCode creates a configured OpenCode system.
func NewOpenCode() *OpenCode {
	return &OpenCode{BaseSystem{
		name:             "opencode",
		displayName:      "OpenCode",
		universal:        true,
		skillsDir:        ".agents/skills",
		altSkillsDirs:    []string{".opencode/skills"},
		agentsDir:        ".opencode/agents",
		globalSkillsDir:  "$XDG_CONFIG/opencode/skills",
		detectPaths:      []string{"$XDG_CONFIG/opencode"},
		configSignals:    []string{"opencode.json", "opencode.jsonc"},
		supportedKinds:   []asset.Kind{asset.KindSkill, asset.KindMCP, asset.KindAgent},
		mcpConfigPath:    "opencode.json",
		mcpConfigPathAlt: "opencode.jsonc",
		mcpConfigKey:     "mcp",
		mcpConfigFormat:  "jsonc",
	}}
}

// Install overrides BaseSystem to produce OpenCode-specific MCP format.
func (o *OpenCode) Install(a asset.Asset, projectDir string, opts InstallOptions) error {
	if a.Kind == asset.KindMCP {
		return o.installMCPOpenCode(a, projectDir, opts)
	}
	return o.BaseSystem.Install(a, projectDir, opts)
}

func (o *OpenCode) installMCPOpenCode(a asset.Asset, projectDir string, opts InstallOptions) error {
	meta, ok := a.Meta.(asset.MCPMeta)
	if !ok {
		return nil
	}

	configPath := o.resolveMCPConfigPath(projectDir)

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

	entryPtr := "/" + jsonPointerEscape(o.mcpConfigKey) + "/" + jsonPointerEscape(a.Name)
	if root.Find(entryPtr) != nil && !opts.Force {
		return ErrAlreadyExists
	}

	var mcpValueJSON string
	if meta.IsStdio() {
		// OpenCode: { "type": "local", "command": ["duckrow", "env", "--mcp", ...] }
		wrapperArgs := []string{"env", "--mcp", a.Name, "--"}
		wrapperArgs = append(wrapperArgs, meta.Command)
		wrapperArgs = append(wrapperArgs, meta.Args...)
		cmdArray := append([]string{"duckrow"}, wrapperArgs...)

		m := map[string]interface{}{
			"type":    "local",
			"command": cmdArray,
		}
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		mcpValueJSON = string(data)
	} else {
		// OpenCode: { "type": "remote", "url": "..." }
		m := map[string]interface{}{
			"type": "remote",
			"url":  meta.URL,
		}
		data, _ := json.MarshalIndent(m, "\t\t", "\t")
		mcpValueJSON = string(data)
	}

	return o.patchAndWrite(root, entryPtr, mcpValueJSON, configPath)
}

func init() { Register(NewOpenCode()) }
