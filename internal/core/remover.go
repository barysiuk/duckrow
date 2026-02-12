package core

import (
	"fmt"
	"os"
	"path/filepath"
)

// Remover handles skill uninstallation: removes canonical files and agent symlinks.
type Remover struct {
	agents []AgentDef
}

// NewRemover creates a Remover with the given agent definitions.
func NewRemover(agents []AgentDef) *Remover {
	return &Remover{agents: agents}
}

// RemoveOptions configures a removal.
type RemoveOptions struct {
	TargetDir string // Project root directory
}

// RemoveResult represents the result of a skill removal.
type RemoveResult struct {
	Name            string   // Skill name (sanitized dir name)
	CanonicalPath   string   // Path that was removed
	RemovedSymlinks []string // Agent display names whose symlinks were removed
}

// Remove removes a skill by its directory name from the canonical location
// and all agent symlink directories.
func (r *Remover) Remove(skillDirName string, opts RemoveOptions) (*RemoveResult, error) {
	if opts.TargetDir == "" {
		return nil, fmt.Errorf("target directory is required")
	}
	if skillDirName == "" {
		return nil, fmt.Errorf("skill name is required")
	}

	canonicalPath := filepath.Join(opts.TargetDir, canonicalSkillsDir, skillDirName)

	// Verify the skill exists in the canonical location
	if !dirExists(canonicalPath) {
		return nil, fmt.Errorf("skill %q not found at %s", skillDirName, canonicalPath)
	}

	// Remove symlinks/copies from non-universal agent directories first
	var removedSymlinks []string
	for _, agent := range r.agents {
		if agent.Universal {
			// Universal agents use .agents/skills directly — removing
			// the canonical dir handles them.
			continue
		}

		linkPath := filepath.Join(opts.TargetDir, agent.SkillsDir, skillDirName)
		if !pathExists(linkPath) {
			continue
		}

		if err := os.RemoveAll(linkPath); err != nil {
			return nil, fmt.Errorf("removing %s skill link for %s: %w", agent.DisplayName, skillDirName, err)
		}
		removedSymlinks = append(removedSymlinks, agent.DisplayName)

		// Clean up empty agent skills directory
		cleanupEmptyDir(filepath.Join(opts.TargetDir, agent.SkillsDir))
	}

	// Remove the canonical directory
	if err := os.RemoveAll(canonicalPath); err != nil {
		return nil, fmt.Errorf("removing canonical skill directory: %w", err)
	}

	// Clean up empty .agents/skills directory
	cleanupEmptyDir(filepath.Join(opts.TargetDir, canonicalSkillsDir))

	return &RemoveResult{
		Name:            skillDirName,
		CanonicalPath:   canonicalPath,
		RemovedSymlinks: removedSymlinks,
	}, nil
}

// RemoveAll removes all skills from a folder: all canonical skills
// and all agent symlinks.
func (r *Remover) RemoveAll(opts RemoveOptions) ([]RemoveResult, error) {
	if opts.TargetDir == "" {
		return nil, fmt.Errorf("target directory is required")
	}

	canonicalDir := filepath.Join(opts.TargetDir, canonicalSkillsDir)
	entries, err := os.ReadDir(canonicalDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Nothing to remove
		}
		return nil, fmt.Errorf("reading skills directory: %w", err)
	}

	var results []RemoveResult
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		result, err := r.Remove(entry.Name(), opts)
		if err != nil {
			return nil, fmt.Errorf("removing skill %q: %w", entry.Name(), err)
		}
		results = append(results, *result)
	}

	return results, nil
}

// ListRemovable returns the names of skills that can be removed from a folder.
func (r *Remover) ListRemovable(targetDir string) ([]string, error) {
	canonicalDir := filepath.Join(targetDir, canonicalSkillsDir)
	entries, err := os.ReadDir(canonicalDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading skills directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// cleanupEmptyDir removes a directory if it is empty.
func cleanupEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		_ = os.Remove(dir)
	}
}
