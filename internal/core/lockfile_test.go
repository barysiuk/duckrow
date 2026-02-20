package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLockFile_NotExists(t *testing.T) {
	dir := t.TempDir()
	lf, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lf != nil {
		t.Fatalf("expected nil lock file, got %+v", lf)
	}
}

func TestReadLockFile_Valid(t *testing.T) {
	dir := t.TempDir()
	content := `{
  "lockVersion": 1,
  "skills": [
    {
      "name": "test-skill",
      "source": "github.com/owner/repo/skills/test-skill",
      "commit": "abc123def456",
      "ref": "main"
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, lockFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lf, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lf == nil {
		t.Fatal("expected non-nil lock file")
	}
	if lf.LockVersion != 1 {
		t.Errorf("lockVersion = %d, want 1", lf.LockVersion)
	}
	if len(lf.Skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(lf.Skills))
	}
	s := lf.Skills[0]
	if s.Name != "test-skill" {
		t.Errorf("name = %q, want %q", s.Name, "test-skill")
	}
	if s.Source != "github.com/owner/repo/skills/test-skill" {
		t.Errorf("source = %q, want %q", s.Source, "github.com/owner/repo/skills/test-skill")
	}
	if s.Commit != "abc123def456" {
		t.Errorf("commit = %q, want %q", s.Commit, "abc123def456")
	}
	if s.Ref != "main" {
		t.Errorf("ref = %q, want %q", s.Ref, "main")
	}
}

func TestReadLockFile_Invalid(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, lockFileName), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadLockFile(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestWriteLockFile_SortsSkills(t *testing.T) {
	dir := t.TempDir()
	lf := &LockFile{
		LockVersion: 1,
		Skills: []LockedSkill{
			{Name: "zeta", Source: "github.com/o/r/zeta", Commit: "aaa"},
			{Name: "alpha", Source: "github.com/o/r/alpha", Commit: "bbb"},
			{Name: "middle", Source: "github.com/o/r/middle", Commit: "ccc"},
		},
	}

	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back and verify order.
	got, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("read back error: %v", err)
	}
	if len(got.Skills) != 3 {
		t.Fatalf("len(skills) = %d, want 3", len(got.Skills))
	}
	if got.Skills[0].Name != "alpha" {
		t.Errorf("skills[0].Name = %q, want %q", got.Skills[0].Name, "alpha")
	}
	if got.Skills[1].Name != "middle" {
		t.Errorf("skills[1].Name = %q, want %q", got.Skills[1].Name, "middle")
	}
	if got.Skills[2].Name != "zeta" {
		t.Errorf("skills[2].Name = %q, want %q", got.Skills[2].Name, "zeta")
	}
}

func TestWriteLockFile_OmitsEmptyRef(t *testing.T) {
	dir := t.TempDir()
	lf := &LockFile{
		LockVersion: 1,
		Skills: []LockedSkill{
			{Name: "no-ref", Source: "github.com/o/r/skill", Commit: "aaa"},
		},
	}

	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, lockFileName))
	if err != nil {
		t.Fatal(err)
	}

	// The JSON should NOT contain a "ref" field when empty.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var skills []map[string]json.RawMessage
	if err := json.Unmarshal(raw["skills"], &skills); err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(skills))
	}
	if _, exists := skills[0]["ref"]; exists {
		t.Error("expected ref to be omitted when empty")
	}
}

func TestAddOrUpdateLockEntry_CreatesNew(t *testing.T) {
	dir := t.TempDir()

	entry := LockedSkill{
		Name:   "new-skill",
		Source: "github.com/owner/repo/skills/new-skill",
		Commit: "abc123",
		Ref:    "main",
	}

	if err := AddOrUpdateLockEntry(dir, entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lf, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if lf.LockVersion != 1 {
		t.Errorf("lockVersion = %d, want 1", lf.LockVersion)
	}
	if len(lf.Skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(lf.Skills))
	}
	if lf.Skills[0].Name != "new-skill" {
		t.Errorf("name = %q, want %q", lf.Skills[0].Name, "new-skill")
	}
}

func TestAddOrUpdateLockEntry_UpdatesExisting(t *testing.T) {
	dir := t.TempDir()

	// Add initial entry.
	entry1 := LockedSkill{Name: "skill-a", Source: "github.com/o/r/a", Commit: "111"}
	if err := AddOrUpdateLockEntry(dir, entry1); err != nil {
		t.Fatal(err)
	}

	// Update with new commit.
	entry2 := LockedSkill{Name: "skill-a", Source: "github.com/o/r/a", Commit: "222"}
	if err := AddOrUpdateLockEntry(dir, entry2); err != nil {
		t.Fatal(err)
	}

	lf, err := ReadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.Skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1 (should not duplicate)", len(lf.Skills))
	}
	if lf.Skills[0].Commit != "222" {
		t.Errorf("commit = %q, want %q", lf.Skills[0].Commit, "222")
	}
}

func TestAddOrUpdateLockEntry_MultipleSkills(t *testing.T) {
	dir := t.TempDir()

	if err := AddOrUpdateLockEntry(dir, LockedSkill{Name: "b-skill", Source: "github.com/o/r/b", Commit: "bbb"}); err != nil {
		t.Fatal(err)
	}
	if err := AddOrUpdateLockEntry(dir, LockedSkill{Name: "a-skill", Source: "github.com/o/r/a", Commit: "aaa"}); err != nil {
		t.Fatal(err)
	}

	lf, err := ReadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.Skills) != 2 {
		t.Fatalf("len(skills) = %d, want 2", len(lf.Skills))
	}
	// Should be sorted alphabetically.
	if lf.Skills[0].Name != "a-skill" {
		t.Errorf("skills[0].Name = %q, want %q", lf.Skills[0].Name, "a-skill")
	}
	if lf.Skills[1].Name != "b-skill" {
		t.Errorf("skills[1].Name = %q, want %q", lf.Skills[1].Name, "b-skill")
	}
}

func TestRemoveLockEntry_Exists(t *testing.T) {
	dir := t.TempDir()

	// Write lock file with two skills.
	lf := &LockFile{
		LockVersion: 1,
		Skills: []LockedSkill{
			{Name: "keep", Source: "github.com/o/r/keep", Commit: "aaa"},
			{Name: "remove", Source: "github.com/o/r/remove", Commit: "bbb"},
		},
	}
	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatal(err)
	}

	if err := RemoveLockEntry(dir, "remove"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := ReadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got.Skills))
	}
	if got.Skills[0].Name != "keep" {
		t.Errorf("remaining skill = %q, want %q", got.Skills[0].Name, "keep")
	}
}

func TestRemoveLockEntry_NoLockFile(t *testing.T) {
	dir := t.TempDir()
	// Should be a no-op, not an error.
	if err := RemoveLockEntry(dir, "anything"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveLockEntry_SkillNotFound(t *testing.T) {
	dir := t.TempDir()
	lf := &LockFile{
		LockVersion: 1,
		Skills: []LockedSkill{
			{Name: "keep", Source: "github.com/o/r/keep", Commit: "aaa"},
		},
	}
	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatal(err)
	}

	// Removing a skill that doesn't exist should not error.
	if err := RemoveLockEntry(dir, "nonexistent"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := ReadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(got.Skills))
	}
}

func TestNormalizeSource(t *testing.T) {
	tests := []struct {
		host, owner, repo, relPath string
		want                       string
	}{
		{"github.com", "acme", "skills", "tools/lint", "github.com/acme/skills/tools/lint"},
		{"gitlab.com", "org", "repo", "skill-a", "gitlab.com/org/repo/skill-a"},
		{"github.com", "owner", "repo", "", "github.com/owner/repo"},
		{"github.com", "owner", "repo", ".", "github.com/owner/repo"},
		{"git.internal.co", "team", "skills", "deep/nested/path", "git.internal.co/team/skills/deep/nested/path"},
	}

	for _, tt := range tests {
		got := NormalizeSource(tt.host, tt.owner, tt.repo, tt.relPath)
		if got != tt.want {
			t.Errorf("NormalizeSource(%q, %q, %q, %q) = %q, want %q",
				tt.host, tt.owner, tt.repo, tt.relPath, got, tt.want)
		}
	}
}

func TestIsCanonicalSource(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"github.com/owner/repo/skills/foo", true},
		{"gitlab.com/org/repo/skill", true},
		{"git.internal.co/team/skills/lint", true},
		{"github.com/owner/repo", true}, // 3 segments, first has dot
		{"owner/repo", false},           // 2 segments, no dot
		{"owner/repo/path", false},      // 3 segments but first has no dot
		{"owner/repo@skill", false},     // shorthand
		{"", false},
	}

	for _, tt := range tests {
		got := isCanonicalSource(tt.source)
		if got != tt.want {
			t.Errorf("isCanonicalSource(%q) = %v, want %v", tt.source, got, tt.want)
		}
	}
}

func TestGetSkillCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that requires git")
	}

	// Create a temp dir with a git repo and a skill file.
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMd := `---
name: test-skill
description: A test skill
---
# Test Skill`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0o644); err != nil {
		t.Fatal(err)
	}

	setupTestGitRepoInDir(t, dir)

	commit, err := GetSkillCommit(dir, "skills/test-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commit) != 40 {
		t.Errorf("commit length = %d, want 40 (full SHA); got %q", len(commit), commit)
	}
}

func TestGetSkillCommit_NoCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that requires git")
	}

	// Create a git repo, then ask for a path that has no commits.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dummy.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	setupTestGitRepoInDir(t, dir)

	_, err := GetSkillCommit(dir, "nonexistent/path")
	if err == nil {
		t.Fatal("expected error for path with no commits")
	}
}

func TestLockFilePath(t *testing.T) {
	got := LockFilePath("/project/root")
	want := filepath.Join("/project/root", "duckrow.lock.json")
	if got != want {
		t.Errorf("LockFilePath = %q, want %q", got, want)
	}
}

func TestParseLockSource(t *testing.T) {
	tests := []struct {
		source                        string
		wantHost, wantOwner, wantRepo string
		wantSubPath                   string
		wantErr                       bool
	}{
		{"github.com/acme/skills/tools/lint", "github.com", "acme", "skills", "tools/lint", false},
		{"gitlab.com/org/repo/skill-a", "gitlab.com", "org", "repo", "skill-a", false},
		{"github.com/owner/repo", "github.com", "owner", "repo", "", false},
		{"git.internal.co/team/skills", "git.internal.co", "team", "skills", "", false},
		{"owner/repo", "", "", "", "", true}, // too few segments
		{"justname", "", "", "", "", true},   // single segment
	}

	for _, tt := range tests {
		host, owner, repo, subPath, err := ParseLockSource(tt.source)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseLockSource(%q) expected error", tt.source)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseLockSource(%q) unexpected error: %v", tt.source, err)
			continue
		}
		if host != tt.wantHost || owner != tt.wantOwner || repo != tt.wantRepo || subPath != tt.wantSubPath {
			t.Errorf("ParseLockSource(%q) = (%q, %q, %q, %q), want (%q, %q, %q, %q)",
				tt.source, host, owner, repo, subPath,
				tt.wantHost, tt.wantOwner, tt.wantRepo, tt.wantSubPath)
		}
	}
}

func TestSourcePathKey(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{"github.com/org/repo/skill", "org/repo/skill"},
		{"github.com-work/org/repo/skill", "org/repo/skill"},
		{"github.com/org/repo", "org/repo"},
		{"noseparator", "noseparator"},
		{"host/a", "a"},
	}
	for _, tt := range tests {
		got := SourcePathKey(tt.source)
		if got != tt.want {
			t.Errorf("SourcePathKey(%q) = %q, want %q", tt.source, got, tt.want)
		}
	}
}

func TestLookupRegistryCommit(t *testing.T) {
	registryCommits := map[string]string{
		"github.com/org/repo/skill-a": "aaa",
		"github.com/org/repo/skill-b": "bbb",
	}
	pathIndex := BuildPathIndex(registryCommits)

	t.Run("exact match", func(t *testing.T) {
		got := LookupRegistryCommit("github.com/org/repo/skill-a", registryCommits, pathIndex)
		if got != "aaa" {
			t.Errorf("got %q, want %q", got, "aaa")
		}
	})

	t.Run("host-agnostic fallback", func(t *testing.T) {
		got := LookupRegistryCommit("github.com-work/org/repo/skill-a", registryCommits, pathIndex)
		if got != "aaa" {
			t.Errorf("got %q, want %q", got, "aaa")
		}
	})

	t.Run("no match", func(t *testing.T) {
		got := LookupRegistryCommit("github.com/other/repo/skill-x", registryCommits, pathIndex)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("exact match wins over empty commit", func(t *testing.T) {
		commits := map[string]string{
			"github.com/org/repo/skill": "",
		}
		idx := BuildPathIndex(commits)
		got := LookupRegistryCommit("github.com/org/repo/skill", commits, idx)
		if got != "" {
			t.Errorf("got %q, want empty (commit is empty string)", got)
		}
	})
}

func TestBuildPathIndex(t *testing.T) {
	commits := map[string]string{
		"github.com/org/repo/skill-a": "aaa",
		"gitlab.com/org/repo/skill-b": "bbb",
	}
	index := BuildPathIndex(commits)

	if len(index) != 2 {
		t.Fatalf("len(index) = %d, want 2", len(index))
	}
	if index["org/repo/skill-a"] != "aaa" {
		t.Errorf("skill-a = %q, want %q", index["org/repo/skill-a"], "aaa")
	}
	if index["org/repo/skill-b"] != "bbb" {
		t.Errorf("skill-b = %q, want %q", index["org/repo/skill-b"], "bbb")
	}
}

// --- MCP Lock File Tests ---

func TestReadLockFile_V2WithMCPs(t *testing.T) {
	dir := t.TempDir()
	content := `{
  "lockVersion": 2,
  "skills": [
    {
      "name": "test-skill",
      "source": "github.com/owner/repo/skills/test-skill",
      "commit": "abc123def456",
      "ref": "main"
    }
  ],
  "mcps": [
    {
      "name": "internal-db",
      "registry": "acme-internal",
      "configHash": "sha256:a1b2c3d4",
      "agents": ["cursor", "claude-code"],
      "requiredEnv": ["DATABASE_URL"]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, lockFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lf, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lf == nil {
		t.Fatal("expected non-nil lock file")
	}
	if lf.LockVersion != 2 {
		t.Errorf("lockVersion = %d, want 2", lf.LockVersion)
	}
	if len(lf.Skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(lf.Skills))
	}
	if len(lf.MCPs) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(lf.MCPs))
	}
	m := lf.MCPs[0]
	if m.Name != "internal-db" {
		t.Errorf("mcp name = %q, want %q", m.Name, "internal-db")
	}
	if m.Registry != "acme-internal" {
		t.Errorf("mcp registry = %q, want %q", m.Registry, "acme-internal")
	}
	if m.ConfigHash != "sha256:a1b2c3d4" {
		t.Errorf("mcp configHash = %q, want %q", m.ConfigHash, "sha256:a1b2c3d4")
	}
	if len(m.Agents) != 2 || m.Agents[0] != "cursor" || m.Agents[1] != "claude-code" {
		t.Errorf("mcp agents = %v, want [cursor claude-code]", m.Agents)
	}
	if len(m.RequiredEnv) != 1 || m.RequiredEnv[0] != "DATABASE_URL" {
		t.Errorf("mcp requiredEnv = %v, want [DATABASE_URL]", m.RequiredEnv)
	}
}

func TestReadLockFile_V1IgnoresMCPs(t *testing.T) {
	dir := t.TempDir()
	content := `{
  "lockVersion": 1,
  "skills": [
    {
      "name": "skill-a",
      "source": "github.com/o/r/a",
      "commit": "abc"
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, lockFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lf, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lf.LockVersion != 1 {
		t.Errorf("lockVersion = %d, want 1", lf.LockVersion)
	}
	if len(lf.MCPs) != 0 {
		t.Errorf("len(mcps) = %d, want 0", len(lf.MCPs))
	}
}

func TestWriteLockFile_SortsMCPs(t *testing.T) {
	dir := t.TempDir()
	lf := &LockFile{
		LockVersion: 2,
		Skills:      []LockedSkill{},
		MCPs: []LockedMCP{
			{Name: "zeta-mcp", Registry: "reg", ConfigHash: "sha256:aaa", Agents: []string{"cursor"}},
			{Name: "alpha-mcp", Registry: "reg", ConfigHash: "sha256:bbb", Agents: []string{"cursor"}},
		},
	}

	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("read back error: %v", err)
	}
	if len(got.MCPs) != 2 {
		t.Fatalf("len(mcps) = %d, want 2", len(got.MCPs))
	}
	if got.MCPs[0].Name != "alpha-mcp" {
		t.Errorf("mcps[0].Name = %q, want %q", got.MCPs[0].Name, "alpha-mcp")
	}
	if got.MCPs[1].Name != "zeta-mcp" {
		t.Errorf("mcps[1].Name = %q, want %q", got.MCPs[1].Name, "zeta-mcp")
	}
}

func TestWriteLockFile_BumpsVersionForMCPs(t *testing.T) {
	dir := t.TempDir()
	lf := &LockFile{
		LockVersion: 1,
		Skills:      []LockedSkill{{Name: "s", Source: "github.com/o/r/s", Commit: "aaa"}},
		MCPs:        []LockedMCP{{Name: "m", Registry: "r", ConfigHash: "sha256:bbb", Agents: []string{"cursor"}}},
	}

	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("read back error: %v", err)
	}
	if got.LockVersion != 2 {
		t.Errorf("lockVersion = %d, want 2 (should bump for MCPs)", got.LockVersion)
	}
}

func TestWriteLockFile_NoMCPsKeepsV1(t *testing.T) {
	dir := t.TempDir()
	lf := &LockFile{
		LockVersion: 1,
		Skills:      []LockedSkill{{Name: "s", Source: "github.com/o/r/s", Commit: "aaa"}},
	}

	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("read back error: %v", err)
	}
	if got.LockVersion != 1 {
		t.Errorf("lockVersion = %d, want 1 (no MCPs)", got.LockVersion)
	}
}

func TestWriteLockFile_OmitsMCPsWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	lf := &LockFile{
		LockVersion: 1,
		Skills:      []LockedSkill{{Name: "s", Source: "github.com/o/r/s", Commit: "aaa"}},
	}

	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, lockFileName))
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, exists := raw["mcps"]; exists {
		t.Error("expected mcps to be omitted when empty")
	}
}

func TestAddOrUpdateMCPLockEntry_CreatesNew(t *testing.T) {
	dir := t.TempDir()

	entry := LockedMCP{
		Name:        "new-mcp",
		Registry:    "acme",
		ConfigHash:  "sha256:abc123",
		Agents:      []string{"cursor", "claude-code"},
		RequiredEnv: []string{"DATABASE_URL"},
	}

	if err := AddOrUpdateMCPLockEntry(dir, entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lf, err := ReadLockFile(dir)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if lf.LockVersion != 2 {
		t.Errorf("lockVersion = %d, want 2", lf.LockVersion)
	}
	if len(lf.MCPs) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(lf.MCPs))
	}
	if lf.MCPs[0].Name != "new-mcp" {
		t.Errorf("name = %q, want %q", lf.MCPs[0].Name, "new-mcp")
	}
}

func TestAddOrUpdateMCPLockEntry_UpdatesExisting(t *testing.T) {
	dir := t.TempDir()

	entry1 := LockedMCP{Name: "mcp-a", Registry: "reg", ConfigHash: "sha256:111", Agents: []string{"cursor"}}
	if err := AddOrUpdateMCPLockEntry(dir, entry1); err != nil {
		t.Fatal(err)
	}

	entry2 := LockedMCP{Name: "mcp-a", Registry: "reg", ConfigHash: "sha256:222", Agents: []string{"cursor", "claude-code"}}
	if err := AddOrUpdateMCPLockEntry(dir, entry2); err != nil {
		t.Fatal(err)
	}

	lf, err := ReadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lf.MCPs) != 1 {
		t.Fatalf("len(mcps) = %d, want 1 (should not duplicate)", len(lf.MCPs))
	}
	if lf.MCPs[0].ConfigHash != "sha256:222" {
		t.Errorf("configHash = %q, want %q", lf.MCPs[0].ConfigHash, "sha256:222")
	}
	if len(lf.MCPs[0].Agents) != 2 {
		t.Errorf("agents = %v, want [cursor claude-code]", lf.MCPs[0].Agents)
	}
}

func TestAddOrUpdateMCPLockEntry_PreservesSkills(t *testing.T) {
	dir := t.TempDir()

	// Write a v1 lock file with skills.
	lf := &LockFile{
		LockVersion: 1,
		Skills:      []LockedSkill{{Name: "skill-a", Source: "github.com/o/r/a", Commit: "aaa"}},
	}
	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatal(err)
	}

	// Add an MCP.
	entry := LockedMCP{Name: "mcp-b", Registry: "reg", ConfigHash: "sha256:bbb", Agents: []string{"cursor"}}
	if err := AddOrUpdateMCPLockEntry(dir, entry); err != nil {
		t.Fatal(err)
	}

	got, err := ReadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.LockVersion != 2 {
		t.Errorf("lockVersion = %d, want 2 after adding MCP", got.LockVersion)
	}
	if len(got.Skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1 (skills should be preserved)", len(got.Skills))
	}
	if len(got.MCPs) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(got.MCPs))
	}
}

func TestRemoveMCPLockEntry_Exists(t *testing.T) {
	dir := t.TempDir()

	lf := &LockFile{
		LockVersion: 2,
		Skills:      []LockedSkill{},
		MCPs: []LockedMCP{
			{Name: "keep", Registry: "reg", ConfigHash: "sha256:aaa", Agents: []string{"cursor"}},
			{Name: "remove", Registry: "reg", ConfigHash: "sha256:bbb", Agents: []string{"cursor"}},
		},
	}
	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatal(err)
	}

	if err := RemoveMCPLockEntry(dir, "remove"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := ReadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.MCPs) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(got.MCPs))
	}
	if got.MCPs[0].Name != "keep" {
		t.Errorf("remaining mcp = %q, want %q", got.MCPs[0].Name, "keep")
	}
}

func TestRemoveMCPLockEntry_NoLockFile(t *testing.T) {
	dir := t.TempDir()
	if err := RemoveMCPLockEntry(dir, "anything"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveMCPLockEntry_MCPNotFound(t *testing.T) {
	dir := t.TempDir()
	lf := &LockFile{
		LockVersion: 2,
		Skills:      []LockedSkill{},
		MCPs:        []LockedMCP{{Name: "keep", Registry: "reg", ConfigHash: "sha256:aaa", Agents: []string{"cursor"}}},
	}
	if err := WriteLockFile(dir, lf); err != nil {
		t.Fatal(err)
	}

	if err := RemoveMCPLockEntry(dir, "nonexistent"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := ReadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.MCPs) != 1 {
		t.Fatalf("len(mcps) = %d, want 1", len(got.MCPs))
	}
}

func TestExtractRequiredEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "single var",
			env:  map[string]string{"DATABASE_URL": "$DATABASE_URL"},
			want: []string{"DATABASE_URL"},
		},
		{
			name: "multiple vars",
			env:  map[string]string{"DB": "$DATABASE_URL", "TOKEN": "$JIRA_TOKEN"},
			want: []string{"DATABASE_URL", "JIRA_TOKEN"},
		},
		{
			name: "duplicate var reference",
			env:  map[string]string{"A": "$MY_VAR", "B": "$MY_VAR"},
			want: []string{"MY_VAR"},
		},
		{
			name: "no env vars",
			env:  map[string]string{"FIXED": "literal-value"},
			want: []string{},
		},
		{
			name: "empty env map",
			env:  map[string]string{},
			want: []string{},
		},
		{
			name: "nil env map",
			env:  nil,
			want: []string{},
		},
		{
			name: "mixed literal and var",
			env:  map[string]string{"URL": "postgres://$DB_USER:$DB_PASS@localhost/db"},
			want: []string{"DB_PASS", "DB_USER"},
		},
		{
			name: "var with underscore and digits",
			env:  map[string]string{"K": "$API_KEY_2"},
			want: []string{"API_KEY_2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRequiredEnv(tt.env)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractRequiredEnv() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractRequiredEnv()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestComputeConfigHash(t *testing.T) {
	t.Run("stdio MCP", func(t *testing.T) {
		entry := MCPEntry{
			Name:        "internal-db",
			Description: "Query databases",
			Command:     "npx",
			Args:        []string{"-y", "@acme/mcp-db-server"},
			Env:         map[string]string{"DATABASE_URL": "$DATABASE_URL"},
		}
		hash := ComputeConfigHash(entry)
		if !strings.HasPrefix(hash, "sha256:") {
			t.Errorf("hash = %q, want sha256: prefix", hash)
		}
		if len(hash) < 71 { // "sha256:" + 64 hex chars
			t.Errorf("hash too short: %q", hash)
		}
	})

	t.Run("remote MCP", func(t *testing.T) {
		entry := MCPEntry{
			Name:        "docs-search",
			Description: "Search docs",
			Type:        "http",
			URL:         "https://mcp.acme.com/mcp",
		}
		hash := ComputeConfigHash(entry)
		if !strings.HasPrefix(hash, "sha256:") {
			t.Errorf("hash = %q, want sha256: prefix", hash)
		}
	})

	t.Run("excludes name and description", func(t *testing.T) {
		entry1 := MCPEntry{
			Name:        "name-a",
			Description: "desc-a",
			Command:     "npx",
			Args:        []string{"-y", "pkg"},
		}
		entry2 := MCPEntry{
			Name:        "name-b",
			Description: "desc-b",
			Command:     "npx",
			Args:        []string{"-y", "pkg"},
		}
		if ComputeConfigHash(entry1) != ComputeConfigHash(entry2) {
			t.Error("hash should be identical when only name/description differ")
		}
	})

	t.Run("different configs produce different hashes", func(t *testing.T) {
		entry1 := MCPEntry{Command: "npx", Args: []string{"-y", "pkg-a"}}
		entry2 := MCPEntry{Command: "npx", Args: []string{"-y", "pkg-b"}}
		if ComputeConfigHash(entry1) == ComputeConfigHash(entry2) {
			t.Error("different configs should produce different hashes")
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		entry := MCPEntry{
			Command: "npx",
			Args:    []string{"-y", "@acme/mcp-db-server"},
			Env:     map[string]string{"B": "$B_VAR", "A": "$A_VAR"},
		}
		hash1 := ComputeConfigHash(entry)
		hash2 := ComputeConfigHash(entry)
		if hash1 != hash2 {
			t.Errorf("hash not deterministic: %q != %q", hash1, hash2)
		}
	})

	t.Run("empty entry", func(t *testing.T) {
		entry := MCPEntry{}
		hash := ComputeConfigHash(entry)
		if !strings.HasPrefix(hash, "sha256:") {
			t.Errorf("hash = %q, want sha256: prefix", hash)
		}
	})
}
