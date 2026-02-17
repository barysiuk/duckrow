package core

import (
	"encoding/json"
	"os"
	"path/filepath"
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
