package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/barysiuk/duckrow/internal/core/asset"
)

const (
	lockFileName       = "duckrow.lock.json"
	currentLockVersion = 3
)

// LockFile represents duckrow.lock.json (v3).
// It uses a single assets array with a kind discriminator per entry.
//
// The Skills and MCPs fields are computed caches populated by ReadLockFile()
// and populateLegacyFields(). They are NOT serialized — the canonical data
// lives in Assets.
type LockFile struct {
	LockVersion int                 `json:"lockVersion"`
	Assets      []asset.LockedAsset `json:"assets"`

	// Computed compat fields — populated by ReadLockFile / populateLegacyFields.
	Skills []LockedSkill `json:"-"`
	MCPs   []LockedMCP   `json:"-"`
}

// LockFilePath returns the full path to the lock file in the given directory.
func LockFilePath(dir string) string {
	return filepath.Join(dir, lockFileName)
}

// populateLegacyFields rebuilds the Skills and MCPs computed fields from Assets.
// Called by ReadLockFile after parsing or migrating.
func (lf *LockFile) populateLegacyFields() {
	lf.Skills = lf.LockedSkills()
	lf.MCPs = lf.LockedMCPs()
}

// ReadLockFile reads and parses the lock file from the given directory.
// Returns nil, nil if the file does not exist.
// Handles v1/v2 formats by migrating them to v3 in memory.
func ReadLockFile(dir string) (*LockFile, error) {
	path := LockFilePath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading lock file: %w", err)
	}

	// Try v3 first.
	var lf LockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing lock file: %w", err)
	}

	// If the file has an Assets field populated, it's v3.
	if lf.LockVersion >= 3 && len(lf.Assets) > 0 {
		lf.populateLegacyFields()
		return &lf, nil
	}

	// Try to read as v1/v2 legacy format and migrate.
	var legacy legacyLockFile
	if err := json.Unmarshal(data, &legacy); err != nil {
		// Already parsed as v3 with empty assets — return as-is.
		return &lf, nil
	}

	migrated := migrateLegacyLockFile(&legacy)
	migrated.populateLegacyFields()
	return migrated, nil
}

// legacyLockFile represents the old v1/v2 lock file format.
type legacyLockFile struct {
	LockVersion int              `json:"lockVersion"`
	Skills      []legacyLocked   `json:"skills"`
	MCPs        []legacyLockedMC `json:"mcps,omitempty"`
}

type legacyLocked struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Commit string `json:"commit"`
	Ref    string `json:"ref,omitempty"`
}

type legacyLockedMC struct {
	Name        string   `json:"name"`
	Registry    string   `json:"registry"`
	ConfigHash  string   `json:"configHash"`
	Agents      []string `json:"agents"`
	RequiredEnv []string `json:"requiredEnv,omitempty"`
}

// migrateLegacyLockFile converts a v1/v2 lock file to v3 format.
func migrateLegacyLockFile(legacy *legacyLockFile) *LockFile {
	lf := &LockFile{
		LockVersion: currentLockVersion,
	}

	for _, s := range legacy.Skills {
		lf.Assets = append(lf.Assets, asset.LockedAsset{
			Kind:   asset.KindSkill,
			Name:   s.Name,
			Source: s.Source,
			Commit: s.Commit,
			Ref:    s.Ref,
		})
	}

	for _, m := range legacy.MCPs {
		data := map[string]any{
			"registry":   m.Registry,
			"configHash": m.ConfigHash,
		}
		if len(m.RequiredEnv) > 0 {
			data["requiredEnv"] = m.RequiredEnv
		}
		lf.Assets = append(lf.Assets, asset.LockedAsset{
			Kind: asset.KindMCP,
			Name: m.Name,
			Data: data,
		})
	}

	return lf
}

// WriteLockFile writes the lock file to the given directory atomically.
// Assets are sorted by (kind, name) for deterministic output.
func WriteLockFile(dir string, lf *LockFile) error {
	lf.LockVersion = currentLockVersion

	// Ensure Assets is never nil to serialize as [] instead of null.
	if lf.Assets == nil {
		lf.Assets = []asset.LockedAsset{}
	}

	// Sort assets by (kind, name) for deterministic output.
	sort.Slice(lf.Assets, func(i, j int) bool {
		if lf.Assets[i].Kind != lf.Assets[j].Kind {
			return lf.Assets[i].Kind < lf.Assets[j].Kind
		}
		return lf.Assets[i].Name < lf.Assets[j].Name
	})

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

// --- Generic CRUD (never inspects the Data field) ---

// AddOrUpdateAsset upserts a locked asset by (kind, name).
func AddOrUpdateAsset(dir string, entry asset.LockedAsset) error {
	lf, err := ReadLockFile(dir)
	if err != nil {
		return err
	}
	if lf == nil {
		lf = &LockFile{LockVersion: currentLockVersion}
	}

	found := false
	for i, a := range lf.Assets {
		if a.Kind == entry.Kind && a.Name == entry.Name {
			lf.Assets[i] = entry
			found = true
			break
		}
	}
	if !found {
		lf.Assets = append(lf.Assets, entry)
	}

	return WriteLockFile(dir, lf)
}

// RemoveAssetEntry removes a locked asset by (kind, name).
// No-op if the lock file does not exist or the asset is not found.
func RemoveAssetEntry(dir string, kind asset.Kind, name string) error {
	lf, err := ReadLockFile(dir)
	if err != nil {
		return err
	}
	if lf == nil {
		return nil
	}

	filtered := lf.Assets[:0]
	for _, a := range lf.Assets {
		if a.Kind != kind || a.Name != name {
			filtered = append(filtered, a)
		}
	}
	lf.Assets = filtered

	return WriteLockFile(dir, lf)
}

// FindLockedAsset returns the locked entry for a (kind, name) pair, or nil.
func FindLockedAsset(lf *LockFile, kind asset.Kind, name string) *asset.LockedAsset {
	if lf == nil {
		return nil
	}
	for i := range lf.Assets {
		if lf.Assets[i].Kind == kind && lf.Assets[i].Name == name {
			return &lf.Assets[i]
		}
	}
	return nil
}

// AssetsByKind returns all locked assets of the given kind.
func AssetsByKind(lf *LockFile, kind asset.Kind) []asset.LockedAsset {
	if lf == nil {
		return nil
	}
	var result []asset.LockedAsset
	for _, a := range lf.Assets {
		if a.Kind == kind {
			result = append(result, a)
		}
	}
	return result
}

// --- Utility functions (kind-agnostic) ---

// ExtractRequiredEnv returns a sorted, deduplicated copy of the env var names.
func ExtractRequiredEnv(env []string) []string {
	if len(env) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(env))
	var result []string
	for _, name := range env {
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

// ComputeConfigHash computes a SHA-256 hash of an MCP entry's config-relevant
// fields. The hash input is a deterministic JSON object.
// The returned hash has a "sha256:" prefix.
func ComputeConfigHash(meta asset.MCPMeta) string {
	m := make(map[string]interface{})
	if meta.Command != "" {
		m["command"] = meta.Command
	}
	if len(meta.Args) > 0 {
		m["args"] = meta.Args
	}
	if len(meta.Env) > 0 {
		sorted := make([]string, len(meta.Env))
		copy(sorted, meta.Env)
		sort.Strings(sorted)
		m["env"] = sorted
	}
	if meta.URL != "" {
		m["url"] = meta.URL
	}
	if meta.Transport != "" {
		m["type"] = meta.Transport
	}

	data, _ := json.Marshal(m)
	h := sha256.Sum256(data)
	return "sha256:" + fmt.Sprintf("%x", h)
}

// GetSkillCommit returns the git commit SHA that last modified the given sub-path
// within a repository directory.
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
func NormalizeSource(host, owner, repo, skillRelPath string) string {
	base := host + "/" + owner + "/" + repo
	if skillRelPath == "" || skillRelPath == "." {
		return base
	}
	skillRelPath = filepath.ToSlash(skillRelPath)
	return base + "/" + skillRelPath
}

// ParseLockSource splits a canonical lock source into components.
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

// isCanonicalSource checks whether a source string uses the canonical format.
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

// SourcePathKey strips the host from a canonical source.
func SourcePathKey(source string) string {
	idx := strings.Index(source, "/")
	if idx < 0 {
		return source
	}
	return source[idx+1:]
}

// TruncateCommit returns the first 7 characters of a commit hash.
func TruncateCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// skillSubPath extracts the sub-path from a canonical source string.
func skillSubPath(source string) string {
	parts := strings.Split(source, "/")
	if len(parts) <= 3 {
		return ""
	}
	return strings.Join(parts[3:], "/")
}

// LookupRegistryCommit finds the registry commit for a given source string.
func LookupRegistryCommit(source string, registryCommits map[string]string, pathIndex map[string]string) string {
	if commit, ok := registryCommits[source]; ok && commit != "" {
		return commit
	}
	if commit, ok := pathIndex[SourcePathKey(source)]; ok && commit != "" {
		return commit
	}
	return ""
}

// BuildPathIndex builds a host-agnostic index from a registry commit map.
func BuildPathIndex(registryCommits map[string]string) map[string]string {
	index := make(map[string]string, len(registryCommits))
	for source, commit := range registryCommits {
		index[SourcePathKey(source)] = commit
	}
	return index
}

// CheckForUpdates checks each locked asset of the given kind for available
// updates. It works for any source-based kind (skills, agents) that uses
// commit-pinned lock entries.
func CheckForUpdates(lf *LockFile, kind asset.Kind, overrides map[string]string, registryCommits map[string]string) ([]UpdateInfo, error) {
	var results []UpdateInfo

	pathIndex := BuildPathIndex(registryCommits)

	// Get all assets of the requested kind from the lock file.
	assets := AssetsByKind(lf, kind)

	type pendingAsset struct {
		asset   asset.LockedAsset
		subPath string
	}

	type repoRefKey struct {
		repo string
		ref  string
	}
	repoGroups := make(map[repoRefKey][]pendingAsset)
	var repoGroupOrder []repoRefKey

	for _, a := range assets {
		if regCommit := LookupRegistryCommit(a.Source, registryCommits, pathIndex); regCommit != "" {
			results = append(results, UpdateInfo{
				Name:            a.Name,
				Source:          a.Source,
				InstalledCommit: a.Commit,
				AvailableCommit: regCommit,
				HasUpdate:       a.Commit != regCommit,
			})
			continue
		}

		key := repoRefKey{repo: repoKey(a.Source), ref: a.Ref}
		if _, exists := repoGroups[key]; !exists {
			repoGroupOrder = append(repoGroupOrder, key)
		}
		repoGroups[key] = append(repoGroups[key], pendingAsset{
			asset:   a,
			subPath: skillSubPath(a.Source),
		})
	}

	for _, key := range repoGroupOrder {
		pending := repoGroups[key]
		host, owner, repo, _, err := ParseLockSource(pending[0].asset.Source)
		if err != nil {
			for _, ps := range pending {
				results = append(results, UpdateInfo{
					Name:            ps.asset.Name,
					Source:          ps.asset.Source,
					InstalledCommit: ps.asset.Commit,
					AvailableCommit: ps.asset.Commit,
					HasUpdate:       false,
				})
			}
			continue
		}

		cloneURL := fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)
		repoKeyStr := strings.ToLower(owner) + "/" + strings.ToLower(repo)
		if override, ok := overrides[repoKeyStr]; ok && override != "" {
			cloneURL = override
		}

		tmpDir, cloneErr := cloneRepo(cloneURL, key.ref, false)
		if cloneErr != nil {
			for _, ps := range pending {
				results = append(results, UpdateInfo{
					Name:            ps.asset.Name,
					Source:          ps.asset.Source,
					InstalledCommit: ps.asset.Commit,
					AvailableCommit: ps.asset.Commit,
					HasUpdate:       false,
				})
			}
			continue
		}

		for _, ps := range pending {
			available, commitErr := GetSkillCommit(tmpDir, ps.subPath)
			if commitErr != nil {
				available = ps.asset.Commit
			}
			results = append(results, UpdateInfo{
				Name:            ps.asset.Name,
				Source:          ps.asset.Source,
				InstalledCommit: ps.asset.Commit,
				AvailableCommit: available,
				HasUpdate:       ps.asset.Commit != available,
			})
		}

		_ = os.RemoveAll(tmpDir)
	}

	return results, nil
}
