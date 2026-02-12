package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSource_OwnerRepo(t *testing.T) {
	src, err := ParseSource("vercel-labs/agent-skills")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGitHub {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGitHub)
	}
	if src.Owner != "vercel-labs" {
		t.Errorf("Owner = %q, want %q", src.Owner, "vercel-labs")
	}
	if src.Repo != "agent-skills" {
		t.Errorf("Repo = %q, want %q", src.Repo, "agent-skills")
	}
	if src.CloneURL != "https://github.com/vercel-labs/agent-skills.git" {
		t.Errorf("CloneURL = %q, want %q", src.CloneURL, "https://github.com/vercel-labs/agent-skills.git")
	}
	if src.SkillName != "" {
		t.Errorf("SkillName = %q, want empty", src.SkillName)
	}
}

func TestParseSource_OwnerRepoAtSkill(t *testing.T) {
	src, err := ParseSource("vercel-labs/agent-skills@web-design-guidelines")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGitHub {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGitHub)
	}
	if src.Owner != "vercel-labs" {
		t.Errorf("Owner = %q, want %q", src.Owner, "vercel-labs")
	}
	if src.Repo != "agent-skills" {
		t.Errorf("Repo = %q, want %q", src.Repo, "agent-skills")
	}
	if src.SkillName != "web-design-guidelines" {
		t.Errorf("SkillName = %q, want %q", src.SkillName, "web-design-guidelines")
	}
}

func TestParseSource_OwnerRepoSubpath(t *testing.T) {
	src, err := ParseSource("pandadoc/skills/contract-review")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGitHub {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGitHub)
	}
	if src.Owner != "pandadoc" {
		t.Errorf("Owner = %q, want %q", src.Owner, "pandadoc")
	}
	if src.Repo != "skills" {
		t.Errorf("Repo = %q, want %q", src.Repo, "skills")
	}
	if src.SubPath != "contract-review" {
		t.Errorf("SubPath = %q, want %q", src.SubPath, "contract-review")
	}
}

func TestParseSource_SSHUrl(t *testing.T) {
	src, err := ParseSource("git@github.com:pandadoc/skill-registry.git")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGitHub {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGitHub)
	}
	if src.Owner != "pandadoc" {
		t.Errorf("Owner = %q, want %q", src.Owner, "pandadoc")
	}
	if src.Repo != "skill-registry" {
		t.Errorf("Repo = %q, want %q", src.Repo, "skill-registry")
	}
	if src.CloneURL != "git@github.com:pandadoc/skill-registry.git" {
		t.Errorf("CloneURL = %q, want original SSH URL", src.CloneURL)
	}
}

func TestParseSource_SSHGitLab(t *testing.T) {
	src, err := ParseSource("git@gitlab.com:org/repo.git")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGitLab {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGitLab)
	}
}

func TestParseSource_HTTPSGitHub(t *testing.T) {
	src, err := ParseSource("https://github.com/vercel-labs/agent-skills")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGitHub {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGitHub)
	}
	if src.Owner != "vercel-labs" {
		t.Errorf("Owner = %q, want %q", src.Owner, "vercel-labs")
	}
	if src.Repo != "agent-skills" {
		t.Errorf("Repo = %q, want %q", src.Repo, "agent-skills")
	}
}

func TestParseSource_HTTPSWithTree(t *testing.T) {
	src, err := ParseSource("https://github.com/owner/repo/tree/main/skills/my-skill")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGitHub {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGitHub)
	}
	if src.Ref != "main" {
		t.Errorf("Ref = %q, want %q", src.Ref, "main")
	}
	if src.SubPath != "skills/my-skill" {
		t.Errorf("SubPath = %q, want %q", src.SubPath, "skills/my-skill")
	}
}

func TestParseSource_LocalPath(t *testing.T) {
	dir := t.TempDir()

	src, err := ParseSource(dir)
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeLocal {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeLocal)
	}
	if src.LocalPath != dir {
		t.Errorf("LocalPath = %q, want %q", src.LocalPath, dir)
	}
}

func TestParseSource_LocalRelativePath(t *testing.T) {
	// Create a temp subdirectory relative to cwd
	dir := t.TempDir()
	// Resolve symlinks (macOS /tmp -> /private/var)
	dir, _ = filepath.EvalSymlinks(dir)
	subDir := filepath.Join(dir, "skills")
	os.MkdirAll(subDir, 0o755)

	// Change to parent dir
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	src, err := ParseSource("./skills")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeLocal {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeLocal)
	}
	if src.LocalPath != subDir {
		t.Errorf("LocalPath = %q, want %q", src.LocalPath, subDir)
	}
}

func TestParseSource_Empty(t *testing.T) {
	_, err := ParseSource("")
	if err == nil {
		t.Error("expected error for empty source")
	}
}

func TestParseSource_Invalid(t *testing.T) {
	_, err := ParseSource("just-a-word")
	if err == nil {
		t.Error("expected error for unrecognized format")
	}
}
