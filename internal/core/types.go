// Package core provides the business logic for DuckRow.
// It has zero UI dependencies and is independently testable.
package core

import (
	"time"

	"github.com/barysiuk/duckrow/internal/core/asset"
)

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
	Assets map[asset.Kind][]asset.InstalledAsset
	Error  error // Non-nil if scanning failed
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
