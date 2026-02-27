package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/barysiuk/duckrow/internal/core/system"
)

// Orchestrator coordinates asset handlers and systems for install, remove,
// scan, and sync operations. It lives in the core package so it can import
// both the asset and system sub-packages without circular dependencies.
type Orchestrator struct{}

// NewOrchestrator creates an Orchestrator.
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{}
}

// OrchestratorInstallResult is the outcome of an asset installation.
type OrchestratorInstallResult struct {
	Asset   asset.Asset
	Systems []string // system names that received the asset
	Commit  string
	Ref     string
}

// OrchestratorInstallOptions configures an installation.
type OrchestratorInstallOptions struct {
	TargetDir       string
	TargetSystems   []system.System // explicit list; nil = auto-detect
	IncludeInternal bool
	NameFilter      string // install only this specific asset
	Commit          string // pin to a specific commit (for sync)
	Force           bool
}

// InstallFromSource is the main install entry point.
// 1. Clone the source repo
// 2. Ask the asset handler to discover assets
// 3. For each discovered asset, iterate over target systems and call Install
// 4. Return results for lock file updates
func (o *Orchestrator) InstallFromSource(
	source *ParsedSource,
	kind asset.Kind,
	opts OrchestratorInstallOptions,
) ([]OrchestratorInstallResult, error) {
	handler, ok := asset.Get(kind)
	if !ok {
		return nil, fmt.Errorf("unknown asset kind: %s", kind)
	}

	// 1. Clone
	tmpDir, err := cloneSource(source, opts.Commit)
	if err != nil {
		return nil, fmt.Errorf("cloning: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 2. Discover
	discovered, err := handler.Discover(tmpDir, asset.DiscoverOptions{
		SubPath:         source.SubPath,
		IncludeInternal: opts.IncludeInternal,
		NameFilter:      opts.NameFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("discovering %s assets: %w", handler.DisplayName(), err)
	}
	if len(discovered) == 0 {
		return nil, fmt.Errorf("no %s assets found in source", handler.DisplayName())
	}

	// 3. Validate
	for _, a := range discovered {
		if err := handler.Validate(a); err != nil {
			return nil, fmt.Errorf("invalid %s %q: %w", handler.DisplayName(), a.Name, err)
		}
	}

	// 4. Resolve target systems
	targets := opts.TargetSystems
	if len(targets) == 0 {
		// Default: universal systems only. Non-universal systems require
		// explicit targeting (e.g. via --agents flag).
		targets = system.Universal()
	}
	// Filter to systems that support this kind
	var compatible []system.System
	for _, s := range targets {
		if s.Supports(kind) {
			compatible = append(compatible, s)
		}
	}

	// 5. Install each asset into each compatible system
	var results []OrchestratorInstallResult
	for _, a := range discovered {
		// For file-based assets (skills), copy to canonical location first.
		if kind == asset.KindSkill {
			if err := copyToCanonical(a, opts.TargetDir); err != nil {
				return nil, fmt.Errorf("copying %q to canonical location: %w", a.Name, err)
			}
		}

		var installedFor []string
		for _, sys := range compatible {
			if err := sys.Install(a, opts.TargetDir, system.InstallOptions{
				Force: opts.Force,
			}); err != nil {
				return nil, fmt.Errorf("installing %q for %s: %w",
					a.Name, sys.DisplayName(), err)
			}
			installedFor = append(installedFor, sys.Name())
		}

		// Populate source if the handler didn't set it (e.g. skill/agent
		// discovery doesn't know the origin URL). This ensures lock file
		// entries always contain a valid source for sync.
		if a.Source == "" {
			relPath := ""
			if a.PreparedPath != "" {
				if rel, err := filepath.Rel(tmpDir, a.PreparedPath); err == nil && rel != "." {
					relPath = filepath.ToSlash(rel)
				}
			}
			a.Source = NormalizeSource(source.Host, source.Owner, source.Repo, relPath)
		}

		// Resolve commit for lock file
		commit := opts.Commit
		if commit == "" {
			commit, _ = getAssetCommit(tmpDir, a)
		}

		results = append(results, OrchestratorInstallResult{
			Asset:   a,
			Systems: installedFor,
			Commit:  commit,
			Ref:     source.Ref,
		})
	}

	return results, nil
}

// InstallFromRegistry installs an asset by name from a configured registry.
func (o *Orchestrator) InstallFromRegistry(
	name string,
	kind asset.Kind,
	registries []Registry,
	registriesDir string,
	opts OrchestratorInstallOptions,
) ([]OrchestratorInstallResult, error) {
	handler, ok := asset.Get(kind)
	if !ok {
		return nil, fmt.Errorf("unknown asset kind: %s", kind)
	}

	rm := NewRegistryManager(registriesDir)
	entry, _, err := rm.FindAsset(registries, kind, name)
	if err != nil {
		return nil, err
	}

	source, err := ParseSource(entry.Source)
	if err != nil {
		return nil, fmt.Errorf("invalid source for %s %q: %w",
			handler.DisplayName(), name, err)
	}

	opts.NameFilter = name
	results, err := o.InstallFromSource(source, kind, opts)
	if err != nil {
		return nil, err
	}

	// Tag results with registry info for lock file
	for i := range results {
		results[i].Asset.Source = entry.Source
	}
	return results, nil
}

// RemoveAsset removes an asset from all target systems and cleans up files.
func (o *Orchestrator) RemoveAsset(
	kind asset.Kind,
	name string,
	projectDir string,
	targetSystems []system.System,
) error {
	// For file-based assets with canonical copies, verify they exist before removing.
	if kind == asset.KindSkill {
		canonicalPath := filepath.Join(projectDir, canonicalSkillsDir, name)
		if _, err := os.Stat(canonicalPath); os.IsNotExist(err) {
			return fmt.Errorf("skill %q not found in %s", name, projectDir)
		}
	}
	// Agents don't have a canonical copy — each system has its own rendered file.
	// No pre-check needed; the per-system Remove() calls handle missing files.

	if len(targetSystems) == 0 {
		// Use all systems to ensure cleanup even if a system was
		// uninstalled globally after the skill was installed.
		targetSystems = system.All()
	}

	for _, sys := range targetSystems {
		if !sys.Supports(kind) {
			continue
		}
		if err := sys.Remove(kind, name, projectDir); err != nil {
			return fmt.Errorf("removing %q from %s: %w", name, sys.DisplayName(), err)
		}
	}

	// For file-based assets with canonical copies, remove the canonical copy.
	if kind == asset.KindSkill {
		_ = removeCanonical(name, projectDir)
	}

	return nil
}

// ScanFolder discovers all installed assets of all kinds in a project folder.
func (o *Orchestrator) ScanFolder(
	projectDir string,
) (map[asset.Kind][]asset.InstalledAsset, error) {
	result := make(map[asset.Kind][]asset.InstalledAsset)

	systems := system.DetectInFolder(projectDir)
	for _, kind := range asset.Kinds() {
		for _, sys := range systems {
			if !sys.Supports(kind) {
				continue
			}
			installed, err := sys.Scan(kind, projectDir)
			if err != nil {
				return nil, fmt.Errorf("scanning %s for %s: %w",
					sys.DisplayName(), kind, err)
			}
			result[kind] = deduplicateInstalled(result[kind], installed)
		}
	}
	return result, nil
}

// SyncFromLock installs everything declared in the lock file at pinned versions.
func (o *Orchestrator) SyncFromLock(
	lockFile *LockFile,
	opts OrchestratorInstallOptions,
) (*SyncResult, error) {
	result := &SyncResult{}

	for _, locked := range lockFile.Assets {
		handler, ok := asset.Get(locked.Kind)
		if !ok {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("skipping unknown kind %q for %q", locked.Kind, locked.Name))
			continue
		}

		// Check if already installed
		if !opts.Force && isAssetPresent(locked, opts.TargetDir) {
			result.Skipped++
			continue
		}

		source, err := ParseSource(locked.Source)
		if err != nil {
			result.Errors = append(result.Errors,
				fmt.Errorf("%s %q: invalid source: %w", handler.DisplayName(), locked.Name, err))
			continue
		}

		installOpts := opts
		installOpts.Commit = locked.Commit
		installOpts.NameFilter = locked.Name

		_, err = o.InstallFromSource(source, locked.Kind, installOpts)
		if err != nil {
			result.Errors = append(result.Errors,
				fmt.Errorf("%s %q: %w", handler.DisplayName(), locked.Name, err))
			continue
		}
		result.Installed++
	}
	return result, nil
}

// SyncResult summarizes a sync operation.
type SyncResult struct {
	Installed int
	Skipped   int
	Errors    []error
	Warnings  []string
}

// --- Helper functions ---

// cloneSource clones a parsed source, optionally at a specific commit.
func cloneSource(source *ParsedSource, commit string) (string, error) {
	if commit != "" {
		return cloneRepoAtCommit(source.CloneURL, commit)
	}
	return cloneRepo(source.CloneURL, source.Ref, false)
}

// copyToCanonical copies a discovered asset's files to the canonical location.
func copyToCanonical(a asset.Asset, targetDir string) error {
	sanitized := sanitizeName(a.Name)
	canonicalDir := filepath.Join(targetDir, canonicalSkillsDir, sanitized)

	// Clean and recreate.
	if err := os.RemoveAll(canonicalDir); err != nil {
		return fmt.Errorf("cleaning canonical dir: %w", err)
	}
	if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
		return fmt.Errorf("creating canonical dir: %w", err)
	}

	return copyDirectory(a.PreparedPath, canonicalDir)
}

// removeCanonical removes the canonical copy of a skill.
func removeCanonical(name string, projectDir string) error {
	canonicalPath := filepath.Join(projectDir, canonicalSkillsDir, name)
	if err := os.RemoveAll(canonicalPath); err != nil {
		return fmt.Errorf("removing canonical skill directory: %w", err)
	}
	// Clean up empty parent directory.
	cleanupEmptyDir(filepath.Join(projectDir, canonicalSkillsDir))
	return nil
}

// getAssetCommit resolves the git commit for an asset in a cloned repo.
func getAssetCommit(repoDir string, a asset.Asset) (string, error) {
	if a.PreparedPath == "" {
		return "", fmt.Errorf("no prepared path")
	}
	relPath, err := filepath.Rel(repoDir, a.PreparedPath)
	if err != nil {
		return "", err
	}
	return GetSkillCommit(repoDir, relPath)
}

// isAssetPresent checks if an asset from the lock file is already installed.
func isAssetPresent(locked asset.LockedAsset, targetDir string) bool {
	switch locked.Kind {
	case asset.KindSkill:
		// Check if canonical directory exists.
		canonical := filepath.Join(targetDir, canonicalSkillsDir, locked.Name)
		info, err := os.Stat(canonical)
		return err == nil && info.IsDir()
	case asset.KindAgent:
		// Check if any system has the rendered agent file.
		filename := sanitizeName(locked.Name) + ".md"
		for _, sys := range system.Supporting(asset.KindAgent) {
			agentDir := sys.AssetDir(asset.KindAgent, targetDir)
			if agentDir == "" {
				continue
			}
			agentPath := filepath.Join(agentDir, filename)
			if _, err := os.Stat(agentPath); err == nil {
				return true
			}
		}
		return false
	default:
		// For other kinds (MCP), always re-evaluate.
		return false
	}
}

// deduplicateInstalled merges new assets into existing, deduplicating by name.
func deduplicateInstalled(existing, new []asset.InstalledAsset) []asset.InstalledAsset {
	seen := make(map[string]bool)
	for _, a := range existing {
		seen[a.Name] = true
	}

	result := make([]asset.InstalledAsset, len(existing))
	copy(result, existing)

	for _, a := range new {
		if !seen[a.Name] {
			result = append(result, a)
			seen[a.Name] = true
		}
	}
	return result
}
