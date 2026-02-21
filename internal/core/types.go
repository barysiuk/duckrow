// Package core provides the business logic for DuckRow.
// It has zero UI dependencies and is independently testable.
package core

import "time"

// Config represents the DuckRow configuration stored at ~/.duckrow/config.json.
type Config struct {
	Folders    []TrackedFolder `json:"folders"`
	Registries []Registry      `json:"registries"`
	Settings   Settings        `json:"settings"`
}

// TrackedFolder is a directory registered with DuckRow for skill management.
type TrackedFolder struct {
	Path    string    `json:"path"`
	AddedAt time.Time `json:"addedAt,omitempty"`
}

// Settings holds user preferences.
type Settings struct {
	AutoAddCurrentDir   bool              `json:"autoAddCurrentDir"`
	DisableAllTelemetry bool              `json:"disableAllTelemetry"`
	CloneURLOverrides   map[string]string `json:"cloneURLOverrides,omitempty"`
}

// Registry is a private skill catalog backed by a git repository.
type Registry struct {
	Name string `json:"name"`
	Repo string `json:"repo"`
}

// RegistryManifest is the parsed duckrow.json from a registry repo.
type RegistryManifest struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Skills      []SkillEntry `json:"skills"`
	MCPs        []MCPEntry   `json:"mcps,omitempty"`
	Warnings    []string     `json:"-"` // validation warnings, not serialized
}

// SkillEntry is a skill listed in a registry manifest.
type SkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Commit      string `json:"commit,omitempty"`
}

// MCPEntry is an MCP server configuration listed in a registry manifest.
// An entry is either stdio (command present) or remote (url present), not both.
type MCPEntry struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Type        string            `json:"type,omitempty"` // "http" or "sse" for remote MCPs
}

// InstalledSkill represents a skill found on disk in a tracked folder.
type InstalledSkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author,omitempty"`
	Path        string   `json:"path"`   // Canonical path (.agents/skills/<name>)
	Agents      []string `json:"agents"` // Agent names that have this skill (via symlink or copy)
}

// SkillMetadata is the YAML frontmatter parsed from a SKILL.md file.
type SkillMetadata struct {
	Name        string               `yaml:"name"`
	Description string               `yaml:"description"`
	License     string               `yaml:"license,omitempty"`
	Metadata    SkillMetadataDetails `yaml:"metadata,omitempty"`
}

// SkillMetadataDetails holds optional metadata fields from SKILL.md frontmatter.
type SkillMetadataDetails struct {
	Author       string `yaml:"author,omitempty"`
	Version      string `yaml:"version,omitempty"`
	Internal     bool   `yaml:"internal,omitempty"`
	ArgumentHint string `yaml:"argument-hint,omitempty"`
}

// AgentDef defines an AI coding agent and its skill directory conventions.
type AgentDef struct {
	Name            string   `json:"name"`
	DisplayName     string   `json:"displayName"`
	SkillsDir       string   `json:"skillsDir"`                 // Project-relative skill directory (e.g. ".cursor/skills")
	AltSkillsDirs   []string `json:"altSkillsDirs,omitempty"`   // Additional native skill directories the agent reads from
	GlobalSkillsDir string   `json:"globalSkillsDir"`           // Global skill directory (e.g. "~/.cursor/skills")
	DetectPaths     []string `json:"detectPaths"`               // Paths to check for agent presence
	Universal       bool     `json:"universal"`                 // If true, uses .agents/skills as skillsDir
	MCPConfigPath   string   `json:"mcpConfigPath,omitempty"`   // Project-relative MCP config file (e.g. ".cursor/mcp.json")
	MCPConfigKey    string   `json:"mcpConfigKey,omitempty"`    // Top-level JSON key in the config file (e.g. "mcpServers")
	MCPConfigFormat string   `json:"mcpConfigFormat,omitempty"` // "jsonc" or "" (strict JSON); controls comment preservation
}

// ParsedSource represents a parsed skill source string.
type ParsedSource struct {
	Type      SourceType
	Host      string // Hostname (e.g. "github.com", "gitlab.com", "git.internal.co")
	Owner     string // Repository owner
	Repo      string // Repository name
	CloneURL  string // Full git clone URL
	Ref       string // Git ref (branch/tag) if specified
	SubPath   string // Path within repo to skill(s)
	SkillName string // Specific skill name filter (from @skill syntax)
}

// SourceType indicates the kind of skill source.
type SourceType string

const (
	SourceTypeGit SourceType = "git"
)

// FolderStatus aggregates information about a tracked folder.
type FolderStatus struct {
	Folder TrackedFolder
	Skills []InstalledSkill
	Agents []string // Detected agent names
	Error  error    // Non-nil if scanning failed
}

// LockFile represents the duckrow.lock.json file that pins installed skills
// and MCP server configurations.
type LockFile struct {
	LockVersion int           `json:"lockVersion"`
	Skills      []LockedSkill `json:"skills"`
	MCPs        []LockedMCP   `json:"mcps,omitempty"`
}

// LockedSkill is a single pinned skill entry in the lock file.
type LockedSkill struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Commit string `json:"commit"`
	Ref    string `json:"ref,omitempty"`
}

// LockedMCP is a single MCP server entry in the lock file.
// It records which MCP was installed, from which registry, and for which agents.
type LockedMCP struct {
	Name        string   `json:"name"`
	Registry    string   `json:"registry"`
	ConfigHash  string   `json:"configHash"`
	Agents      []string `json:"agents"`
	RequiredEnv []string `json:"requiredEnv,omitempty"`
}

// UpdateInfo holds update status for a single locked skill.
type UpdateInfo struct {
	Name            string `json:"name"`
	Source          string `json:"source"`
	InstalledCommit string `json:"installed"`
	AvailableCommit string `json:"available"`
	HasUpdate       bool   `json:"hasUpdate"`
}

// CachedCommits stores resolved commit SHAs for unpinned registry skills.
// Written to <registryDir>/duckrow.commits.json during hydration.
type CachedCommits struct {
	GeneratedAt time.Time         `json:"generatedAt"`
	Commits     map[string]string `json:"commits"` // source -> commit SHA
}
