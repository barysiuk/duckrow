package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	lockFileName       = "duckrow.lock.json"
	currentLockVersion = 1
	mcpLockVersion     = 2 // lock version when MCPs are present
)

// LockFilePath returns the full path to the lock file in the given directory.
func LockFilePath(dir string) string {
	return filepath.Join(dir, lockFileName)
}

// ReadLockFile reads and parses the lock file from the given directory.
// Returns nil, nil if the file does not exist.
func ReadLockFile(dir string) (*LockFile, error) {
	path := LockFilePath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading lock file: %w", err)
	}

	var lf LockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing lock file: %w", err)
	}
	return &lf, nil
}

// WriteLockFile writes the lock file to the given directory atomically.
// Skills and MCPs are sorted by name for deterministic output.
// The lock version is bumped to 2 if MCPs are present.
func WriteLockFile(dir string, lf *LockFile) error {
	// Sort skills by name for deterministic output.
	sort.Slice(lf.Skills, func(i, j int) bool {
		return lf.Skills[i].Name < lf.Skills[j].Name
	})

	// Sort MCPs by name for deterministic output.
	sort.Slice(lf.MCPs, func(i, j int) bool {
		return lf.MCPs[i].Name < lf.MCPs[j].Name
	})

	// Bump lock version to 2 if MCPs are present.
	if len(lf.MCPs) > 0 && lf.LockVersion < mcpLockVersion {
		lf.LockVersion = mcpLockVersion
	}

	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling lock file: %w", err)
	}
	// Ensure trailing newline.
	data = append(data, '\n')

	path := LockFilePath(dir)

	// Atomic write: write to temp file, then rename.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("writing lock file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("saving lock file: %w", err)
	}

	return nil
}

// AddOrUpdateLockEntry upserts a skill entry in the lock file by name.
// Creates the lock file if it does not exist.
func AddOrUpdateLockEntry(dir string, entry LockedSkill) error {
	lf, err := ReadLockFile(dir)
	if err != nil {
		return err
	}
	if lf == nil {
		lf = &LockFile{
			LockVersion: currentLockVersion,
			Skills:      []LockedSkill{},
		}
	}

	// Upsert: replace existing entry with the same name, or append.
	found := false
	for i, s := range lf.Skills {
		if s.Name == entry.Name {
			lf.Skills[i] = entry
			found = true
			break
		}
	}
	if !found {
		lf.Skills = append(lf.Skills, entry)
	}

	return WriteLockFile(dir, lf)
}

// RemoveLockEntry removes a skill entry from the lock file by name.
// No-op if the lock file does not exist or the skill is not found.
func RemoveLockEntry(dir string, skillName string) error {
	lf, err := ReadLockFile(dir)
	if err != nil {
		return err
	}
	if lf == nil {
		return nil
	}

	filtered := lf.Skills[:0]
	for _, s := range lf.Skills {
		if s.Name != skillName {
			filtered = append(filtered, s)
		}
	}
	lf.Skills = filtered

	return WriteLockFile(dir, lf)
}

// AddOrUpdateMCPLockEntry upserts an MCP entry in the lock file by name.
// Creates the lock file if it does not exist.
func AddOrUpdateMCPLockEntry(dir string, entry LockedMCP) error {
	lf, err := ReadLockFile(dir)
	if err != nil {
		return err
	}
	if lf == nil {
		lf = &LockFile{
			LockVersion: mcpLockVersion,
			Skills:      []LockedSkill{},
			MCPs:        []LockedMCP{},
		}
	}

	// Upsert: replace existing entry with the same name, or append.
	found := false
	for i, m := range lf.MCPs {
		if m.Name == entry.Name {
			lf.MCPs[i] = entry
			found = true
			break
		}
	}
	if !found {
		lf.MCPs = append(lf.MCPs, entry)
	}

	return WriteLockFile(dir, lf)
}

// RemoveMCPLockEntry removes an MCP entry from the lock file by name.
// No-op if the lock file does not exist or the MCP is not found.
func RemoveMCPLockEntry(dir string, mcpName string) error {
	lf, err := ReadLockFile(dir)
	if err != nil {
		return err
	}
	if lf == nil {
		return nil
	}

	filtered := lf.MCPs[:0]
	for _, m := range lf.MCPs {
		if m.Name != mcpName {
			filtered = append(filtered, m)
		}
	}
	lf.MCPs = filtered

	return WriteLockFile(dir, lf)
}

// envVarRefPattern matches $VAR references in env values.
var envVarRefPattern = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)

// ExtractRequiredEnv extracts environment variable names from $VAR references
// in an MCP entry's env map. Returns a sorted, deduplicated list.
func ExtractRequiredEnv(env map[string]string) []string {
	seen := make(map[string]bool)
	for _, val := range env {
		matches := envVarRefPattern.FindAllStringSubmatch(val, -1)
		for _, match := range matches {
			seen[match[1]] = true
		}
	}

	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// ComputeConfigHash computes a SHA-256 hash of an MCP entry's config-relevant
// fields. The hash input is a deterministic JSON object containing only config
// fields (command, args, env, url, type). name and description are excluded.
// Keys are sorted alphabetically, with compact JSON (no whitespace).
// The returned hash has a "sha256:" prefix.
func ComputeConfigHash(entry MCPEntry) string {
	// Build a map with only config-relevant fields.
	m := make(map[string]interface{})
	if entry.Command != "" {
		m["command"] = entry.Command
	}
	if len(entry.Args) > 0 {
		m["args"] = entry.Args
	}
	if len(entry.Env) > 0 {
		// Sort env keys for determinism.
		sortedEnv := make(map[string]string, len(entry.Env))
		for k, v := range entry.Env {
			sortedEnv[k] = v
		}
		m["env"] = sortedEnv
	}
	if entry.URL != "" {
		m["url"] = entry.URL
	}
	if entry.Type != "" {
		m["type"] = entry.Type
	}

	// Marshal with sorted keys (Go's encoding/json sorts map keys by default).
	data, _ := json.Marshal(m)

	h := sha256.Sum256(data)
	return "sha256:" + fmt.Sprintf("%x", h)
}

// GetSkillCommit returns the git commit SHA that last modified the given sub-path
// within a repository directory. Uses `git log -1 --format=%H -- <subPath>`.
func GetSkillCommit(repoDir, subPath string) (string, error) {
	args := []string{"-C", repoDir, "log", "-1", "--format=%H"}
	if subPath != "" && subPath != "." {
		args = append(args, "--", subPath)
	}
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting skill commit: %w", err)
	}

	commit := strings.TrimSpace(string(output))
	if commit == "" {
		return "", fmt.Errorf("no commits found for path %q in %s", subPath, repoDir)
	}
	return commit, nil
}

// NormalizeSource builds a canonical lock file source string from its components.
// Format: host/owner/repo/skillRelPath (e.g. "github.com/acme/skills/tools/lint").
func NormalizeSource(host, owner, repo, skillRelPath string) string {
	base := host + "/" + owner + "/" + repo
	if skillRelPath == "" || skillRelPath == "." {
		return base
	}
	// Normalize path separators to forward slashes.
	skillRelPath = filepath.ToSlash(skillRelPath)
	return base + "/" + skillRelPath
}

// ParseLockSource splits a canonical lock source like "github.com/acme/skills/tools/lint"
// into its components (host, owner, repo, subPath). Returns an error if the source
// has fewer than 3 segments (host/owner/repo minimum).
func ParseLockSource(source string) (host, owner, repo, subPath string, err error) {
	parts := strings.Split(source, "/")
	if len(parts) < 3 {
		return "", "", "", "", fmt.Errorf("invalid lock source %q: expected at least host/owner/repo", source)
	}
	host = parts[0]
	owner = parts[1]
	repo = parts[2]
	if len(parts) > 3 {
		subPath = strings.Join(parts[3:], "/")
	}
	return host, owner, repo, subPath, nil
}

// isCanonicalSource checks whether a source string uses the canonical
// host/owner/repo/path format. Canonical means at least 3 slash-separated
// segments where the first segment contains a dot (hostname indicator).
func isCanonicalSource(source string) bool {
	parts := strings.Split(source, "/")
	if len(parts) < 3 {
		return false
	}
	return strings.Contains(parts[0], ".")
}

// repoKey extracts "host/owner/repo" from a canonical source string.
func repoKey(source string) string {
	parts := strings.Split(source, "/")
	if len(parts) < 3 {
		return source
	}
	return parts[0] + "/" + parts[1] + "/" + parts[2]
}

// SourcePathKey strips the host from a canonical source, returning "owner/repo/path".
// This allows matching sources across different host aliases (e.g. github.com vs
// github.com-work) that refer to the same repository and skill path.
func SourcePathKey(source string) string {
	idx := strings.Index(source, "/")
	if idx < 0 {
		return source
	}
	return source[idx+1:]
}

// TruncateCommit returns the first 7 characters of a commit hash,
// or the full string if it's shorter.
func TruncateCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// skillSubPath extracts the sub-path portion from a canonical source string.
// Returns "" if the source has exactly 3 segments (host/owner/repo).
func skillSubPath(source string) string {
	parts := strings.Split(source, "/")
	if len(parts) <= 3 {
		return ""
	}
	return strings.Join(parts[3:], "/")
}

// LookupRegistryCommit finds the registry commit for a given source string.
// It first tries an exact match, then falls back to host-agnostic matching
// using SourcePathKey. This handles SSH host aliases (e.g. github.com-work)
// that may differ from the registry's canonical host.
//
// The pathIndex maps SourcePathKey(source) -> commit and must be pre-built
// by the caller for efficiency.
func LookupRegistryCommit(source string, registryCommits map[string]string, pathIndex map[string]string) string {
	// Exact match first.
	if commit, ok := registryCommits[source]; ok && commit != "" {
		return commit
	}
	// Fallback: match by path (host-agnostic).
	if commit, ok := pathIndex[SourcePathKey(source)]; ok && commit != "" {
		return commit
	}
	return ""
}

// BuildPathIndex builds a host-agnostic index from a registry commit map.
// The returned map keys are SourcePathKey values (owner/repo/path).
func BuildPathIndex(registryCommits map[string]string) map[string]string {
	index := make(map[string]string, len(registryCommits))
	for source, commit := range registryCommits {
		index[SourcePathKey(source)] = commit
	}
	return index
}

// CheckForUpdates checks each locked skill for available updates.
// registryCommits maps lock file source strings to the commit from the registry.
// overrides maps repo keys (owner/repo) to clone URL overrides.
func CheckForUpdates(lf *LockFile, overrides map[string]string, registryCommits map[string]string) ([]UpdateInfo, error) {
	var results []UpdateInfo

	// Build a path-based index for host-agnostic source matching.
	// This allows matching sources across SSH host aliases (e.g. github.com
	// vs github.com-work) that refer to the same repository and skill path.
	pathIndex := BuildPathIndex(registryCommits)

	// Separate skills into registry-resolved and network-fetch groups.
	type pendingSkill struct {
		skill   LockedSkill
		subPath string
	}

	// Group skills needing network fetch by (repoKey, ref) to avoid duplicate clones.
	type repoRefKey struct {
		repo string
		ref  string
	}
	repoGroups := make(map[repoRefKey][]pendingSkill)
	var repoGroupOrder []repoRefKey

	for _, skill := range lf.Skills {
		// Check registry commit first (exact match, then host-agnostic fallback).
		if regCommit := LookupRegistryCommit(skill.Source, registryCommits, pathIndex); regCommit != "" {
			results = append(results, UpdateInfo{
				Name:            skill.Name,
				Source:          skill.Source,
				InstalledCommit: skill.Commit,
				AvailableCommit: regCommit,
				HasUpdate:       skill.Commit != regCommit,
			})
			continue
		}

		// Need network fetch. Group by repo + ref.
		key := repoRefKey{repo: repoKey(skill.Source), ref: skill.Ref}
		if _, exists := repoGroups[key]; !exists {
			repoGroupOrder = append(repoGroupOrder, key)
		}
		repoGroups[key] = append(repoGroups[key], pendingSkill{
			skill:   skill,
			subPath: skillSubPath(skill.Source),
		})
	}

	// Process each repo group: clone once, check all skills.
	for _, key := range repoGroupOrder {
		skills := repoGroups[key]
		// Parse the source to build a clone URL.
		host, owner, repo, _, err := ParseLockSource(skills[0].skill.Source)
		if err != nil {
			// Add error results for all skills in this group.
			for _, ps := range skills {
				results = append(results, UpdateInfo{
					Name:            ps.skill.Name,
					Source:          ps.skill.Source,
					InstalledCommit: ps.skill.Commit,
					AvailableCommit: ps.skill.Commit,
					HasUpdate:       false,
				})
			}
			continue
		}

		cloneURL := fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)

		// Apply clone URL override.
		repoKeyStr := strings.ToLower(owner) + "/" + strings.ToLower(repo)
		if override, ok := overrides[repoKeyStr]; ok && override != "" {
			cloneURL = override
		}

		tmpDir, cloneErr := cloneRepo(cloneURL, key.ref)
		if cloneErr != nil {
			// Can't clone — mark all skills as no-update.
			for _, ps := range skills {
				results = append(results, UpdateInfo{
					Name:            ps.skill.Name,
					Source:          ps.skill.Source,
					InstalledCommit: ps.skill.Commit,
					AvailableCommit: ps.skill.Commit,
					HasUpdate:       false,
				})
			}
			continue
		}

		for _, ps := range skills {
			available, commitErr := GetSkillCommit(tmpDir, ps.subPath)
			if commitErr != nil {
				available = ps.skill.Commit
			}
			results = append(results, UpdateInfo{
				Name:            ps.skill.Name,
				Source:          ps.skill.Source,
				InstalledCommit: ps.skill.Commit,
				AvailableCommit: available,
				HasUpdate:       ps.skill.Commit != available,
			})
		}

		_ = os.RemoveAll(tmpDir)
	}

	return results, nil
}
