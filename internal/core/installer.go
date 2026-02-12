package core

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	cloneTimeout = 60 * time.Second
)

// excludedFiles are files/dirs excluded when copying skills.
var excludedFiles = map[string]bool{
	"README.md":     true,
	"metadata.json": true,
	".git":          true,
}

var sanitizeRegexp = regexp.MustCompile(`[^a-zA-Z0-9-]`)

// Installer handles skill installation: clone, discover, copy, and symlink.
type Installer struct {
	agents []AgentDef
}

// NewInstaller creates an Installer with the given agent definitions.
func NewInstaller(agents []AgentDef) *Installer {
	return &Installer{agents: agents}
}

// InstallOptions configures an installation.
type InstallOptions struct {
	TargetDir       string     // Directory to install skills into (project root)
	SkillFilter     string     // If set, only install this specific skill name
	IncludeInternal bool       // Include skills with metadata.internal: true
	IsInternal      bool       // If true, this is from an internal registry (disable telemetry)
	TargetAgents    []AgentDef // Explicit list of agents to install for; if nil, defaults to universal-only
}

// InstallResult represents the result of a skill installation.
type InstallResult struct {
	InstalledSkills []InstalledSkillResult
}

// InstalledSkillResult represents one installed skill.
type InstalledSkillResult struct {
	Name   string
	Path   string   // Canonical path where files were copied
	Agents []string // Agent names that received the skill
}

// InstallFromSource installs skill(s) from the given source into the target directory.
func (inst *Installer) InstallFromSource(source *ParsedSource, opts InstallOptions) (*InstallResult, error) {
	if opts.TargetDir == "" {
		return nil, fmt.Errorf("target directory is required")
	}

	var skillSourceDir string
	var cleanupFn func()

	switch source.Type {
	case SourceTypeLocal:
		skillSourceDir = source.LocalPath
	case SourceTypeGitHub, SourceTypeGitLab, SourceTypeGit:
		tmpDir, err := cloneRepo(source.CloneURL, source.Ref)
		if err != nil {
			return nil, fmt.Errorf("cloning repository: %w", err)
		}
		cleanupFn = func() { _ = os.RemoveAll(tmpDir) }
		skillSourceDir = tmpDir
	default:
		return nil, fmt.Errorf("unsupported source type: %s", source.Type)
	}

	if cleanupFn != nil {
		defer cleanupFn()
	}

	// Apply skill filter from source (@skill syntax)
	skillFilter := opts.SkillFilter
	if source.SkillName != "" && skillFilter == "" {
		skillFilter = source.SkillName
	}

	// If installing from internal registry, include internal skills
	includeInternal := opts.IncludeInternal || opts.IsInternal

	// Discover skills in the source
	discovered, err := DiscoverSkills(skillSourceDir, source.SubPath, includeInternal)
	if err != nil {
		return nil, fmt.Errorf("discovering skills: %w", err)
	}

	if len(discovered) == 0 {
		return nil, fmt.Errorf("no skills found in source")
	}

	// Apply skill filter
	if skillFilter != "" {
		var filtered []DiscoveredSkill
		for _, s := range discovered {
			if s.Metadata.Name == skillFilter {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			var available []string
			for _, s := range discovered {
				available = append(available, s.Metadata.Name)
			}
			return nil, fmt.Errorf("skill %q not found. Available: %s", skillFilter, strings.Join(available, ", "))
		}
		discovered = filtered
	}

	// Determine which agents to install for.
	// If the caller provided an explicit list, use it.
	// Otherwise default to universal agents only (they read .agents/skills/ directly).
	var targetAgents []AgentDef
	if len(opts.TargetAgents) > 0 {
		targetAgents = opts.TargetAgents
	} else {
		targetAgents = GetUniversalAgents(inst.agents)
	}

	// Install each discovered skill
	result := &InstallResult{}
	for _, skill := range discovered {
		installed, err := inst.installSkill(skill, targetAgents, opts)
		if err != nil {
			return nil, fmt.Errorf("installing skill %q: %w", skill.Metadata.Name, err)
		}
		result.InstalledSkills = append(result.InstalledSkills, *installed)
	}

	return result, nil
}

// installSkill installs a single skill to the canonical location and creates symlinks.
func (inst *Installer) installSkill(skill DiscoveredSkill, agents []AgentDef, opts InstallOptions) (*InstalledSkillResult, error) {
	sanitizedName := sanitizeName(skill.Metadata.Name)
	canonicalDir := filepath.Join(opts.TargetDir, canonicalSkillsDir, sanitizedName)

	// Create/clean canonical directory
	if err := os.RemoveAll(canonicalDir); err != nil {
		return nil, fmt.Errorf("cleaning canonical dir: %w", err)
	}
	if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating canonical dir: %w", err)
	}

	// Copy skill files to canonical location
	if err := copyDirectory(skill.Path, canonicalDir); err != nil {
		return nil, fmt.Errorf("copying skill files: %w", err)
	}

	// Create symlinks for non-universal agents
	var installedAgents []string
	for _, agent := range agents {
		if agent.Universal {
			// Universal agents use .agents/skills directly -- no symlink needed
			installedAgents = append(installedAgents, agent.DisplayName)
			continue
		}

		agentSkillDir := filepath.Join(opts.TargetDir, agent.SkillsDir)
		if err := os.MkdirAll(agentSkillDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating agent dir for %s: %w", agent.DisplayName, err)
		}

		linkPath := filepath.Join(agentSkillDir, sanitizedName)

		// Remove existing link/dir if present
		_ = os.RemoveAll(linkPath)

		// Create relative symlink
		rel, err := filepath.Rel(agentSkillDir, canonicalDir)
		if err != nil {
			return nil, fmt.Errorf("computing relative path for %s: %w", agent.DisplayName, err)
		}

		if err := os.Symlink(rel, linkPath); err != nil {
			// Fall back to copy if symlink fails
			if copyErr := copyDirectory(canonicalDir, linkPath); copyErr != nil {
				return nil, fmt.Errorf("symlink and copy both failed for %s: symlink: %w, copy: %v", agent.DisplayName, err, copyErr)
			}
		}

		installedAgents = append(installedAgents, agent.DisplayName)
	}

	return &InstalledSkillResult{
		Name:   skill.Metadata.Name,
		Path:   canonicalDir,
		Agents: installedAgents,
	}, nil
}

// cloneRepo clones a git repository to a temporary directory.
// On failure it returns a *CloneError with classified diagnostics.
func cloneRepo(url string, ref string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "duckrow-clone-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := runWithTimeout(cmd, cloneTimeout)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", ClassifyCloneError(url, FormatCommand(url, ref), output)
	}

	return tmpDir, nil
}

// runWithTimeout runs a command with a timeout.
func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) (string, error) {
	// exec.CommandContext would be cleaner, but we want the combined output
	done := make(chan struct{})
	var output []byte
	var cmdErr error

	go func() {
		output, cmdErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		return string(output), cmdErr
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return "", fmt.Errorf("command timed out after %s", timeout)
	}
}

// copyDirectory copies the contents of src to dst, excluding certain files.
func copyDirectory(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip excluded files
		baseName := filepath.Base(path)
		if excludedFiles[baseName] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files/dirs starting with _
		if strings.HasPrefix(baseName, "_") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// sanitizeName normalizes a skill name for use as a directory name.
func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = sanitizeRegexp.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-.")
	if len(name) > 255 {
		name = name[:255]
	}
	if name == "" {
		name = "unnamed-skill"
	}
	return name
}
