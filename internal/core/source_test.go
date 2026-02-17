package core

import (
	"strings"
	"testing"
)

func TestParseSource_OwnerRepo(t *testing.T) {
	src, err := ParseSource("vercel-labs/agent-skills")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "github.com" {
		t.Errorf("Host = %q, want %q", src.Host, "github.com")
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
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "github.com" {
		t.Errorf("Host = %q, want %q", src.Host, "github.com")
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
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "github.com" {
		t.Errorf("Host = %q, want %q", src.Host, "github.com")
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
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "github.com" {
		t.Errorf("Host = %q, want %q", src.Host, "github.com")
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
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "gitlab.com" {
		t.Errorf("Host = %q, want %q", src.Host, "gitlab.com")
	}
}

func TestParseSource_SSHSelfHosted(t *testing.T) {
	src, err := ParseSource("git@git.internal.co:team/repo.git")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "git.internal.co" {
		t.Errorf("Host = %q, want %q", src.Host, "git.internal.co")
	}
	if src.Owner != "team" {
		t.Errorf("Owner = %q, want %q", src.Owner, "team")
	}
	if src.Repo != "repo" {
		t.Errorf("Repo = %q, want %q", src.Repo, "repo")
	}
}

func TestParseSource_HTTPSGitHub(t *testing.T) {
	src, err := ParseSource("https://github.com/vercel-labs/agent-skills")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "github.com" {
		t.Errorf("Host = %q, want %q", src.Host, "github.com")
	}
	if src.Owner != "vercel-labs" {
		t.Errorf("Owner = %q, want %q", src.Owner, "vercel-labs")
	}
	if src.Repo != "agent-skills" {
		t.Errorf("Repo = %q, want %q", src.Repo, "agent-skills")
	}
}

func TestParseSource_HTTPSGitLab(t *testing.T) {
	src, err := ParseSource("https://gitlab.com/org/repo")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "gitlab.com" {
		t.Errorf("Host = %q, want %q", src.Host, "gitlab.com")
	}
}

func TestParseSource_HTTPSSelfHosted(t *testing.T) {
	src, err := ParseSource("https://git.internal.co/team/repo")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "git.internal.co" {
		t.Errorf("Host = %q, want %q", src.Host, "git.internal.co")
	}
	if src.Owner != "team" {
		t.Errorf("Owner = %q, want %q", src.Owner, "team")
	}
	if src.Repo != "repo" {
		t.Errorf("Repo = %q, want %q", src.Repo, "repo")
	}
}

func TestParseSource_HTTPSWithTree(t *testing.T) {
	src, err := ParseSource("https://github.com/owner/repo/tree/main/skills/my-skill")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "github.com" {
		t.Errorf("Host = %q, want %q", src.Host, "github.com")
	}
	if src.Ref != "main" {
		t.Errorf("Ref = %q, want %q", src.Ref, "main")
	}
	if src.SubPath != "skills/my-skill" {
		t.Errorf("SubPath = %q, want %q", src.SubPath, "skills/my-skill")
	}
}

func TestParseSource_LocalPathRejected(t *testing.T) {
	cases := []string{
		"./foo",
		"../bar",
		"/absolute/path",
		"~/home-path",
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			_, err := ParseSource(input)
			if err == nil {
				t.Fatalf("expected error for local path %q, got nil", input)
			}
			if !strings.Contains(err.Error(), "local path installs are not supported") {
				t.Errorf("error = %q, want it to contain %q", err.Error(), "local path installs are not supported")
			}
		})
	}
}

func TestParseSource_CanonicalGitHub(t *testing.T) {
	src, err := ParseSource("github.com/pandadoc-studio/skills/skills/communication/slack-digest")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Type != SourceTypeGit {
		t.Errorf("Type = %q, want %q", src.Type, SourceTypeGit)
	}
	if src.Host != "github.com" {
		t.Errorf("Host = %q, want %q", src.Host, "github.com")
	}
	if src.Owner != "pandadoc-studio" {
		t.Errorf("Owner = %q, want %q", src.Owner, "pandadoc-studio")
	}
	if src.Repo != "skills" {
		t.Errorf("Repo = %q, want %q", src.Repo, "skills")
	}
	if src.SubPath != "skills/communication/slack-digest" {
		t.Errorf("SubPath = %q, want %q", src.SubPath, "skills/communication/slack-digest")
	}
	if src.CloneURL != "https://github.com/pandadoc-studio/skills.git" {
		t.Errorf("CloneURL = %q, want %q", src.CloneURL, "https://github.com/pandadoc-studio/skills.git")
	}
}

func TestParseSource_CanonicalGitLab(t *testing.T) {
	src, err := ParseSource("gitlab.com/org/repo/path/to/skill")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Host != "gitlab.com" {
		t.Errorf("Host = %q, want %q", src.Host, "gitlab.com")
	}
	if src.Owner != "org" {
		t.Errorf("Owner = %q, want %q", src.Owner, "org")
	}
	if src.Repo != "repo" {
		t.Errorf("Repo = %q, want %q", src.Repo, "repo")
	}
	if src.SubPath != "path/to/skill" {
		t.Errorf("SubPath = %q, want %q", src.SubPath, "path/to/skill")
	}
	if src.CloneURL != "https://gitlab.com/org/repo.git" {
		t.Errorf("CloneURL = %q, want %q", src.CloneURL, "https://gitlab.com/org/repo.git")
	}
}

func TestParseSource_CanonicalSelfHosted(t *testing.T) {
	src, err := ParseSource("git.internal.co/team/repo/my-skill")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Host != "git.internal.co" {
		t.Errorf("Host = %q, want %q", src.Host, "git.internal.co")
	}
	if src.Owner != "team" {
		t.Errorf("Owner = %q, want %q", src.Owner, "team")
	}
	if src.Repo != "repo" {
		t.Errorf("Repo = %q, want %q", src.Repo, "repo")
	}
	if src.SubPath != "my-skill" {
		t.Errorf("SubPath = %q, want %q", src.SubPath, "my-skill")
	}
	if src.CloneURL != "https://git.internal.co/team/repo.git" {
		t.Errorf("CloneURL = %q, want %q", src.CloneURL, "https://git.internal.co/team/repo.git")
	}
}

func TestParseSource_CanonicalNoSubPath(t *testing.T) {
	src, err := ParseSource("github.com/owner/repo/skill")
	if err != nil {
		t.Fatalf("ParseSource() error: %v", err)
	}
	if src.Host != "github.com" {
		t.Errorf("Host = %q, want %q", src.Host, "github.com")
	}
	if src.Owner != "owner" {
		t.Errorf("Owner = %q, want %q", src.Owner, "owner")
	}
	if src.Repo != "repo" {
		t.Errorf("Repo = %q, want %q", src.Repo, "repo")
	}
	if src.SubPath != "skill" {
		t.Errorf("SubPath = %q, want %q", src.SubPath, "skill")
	}
}

func TestParseSource_CanonicalRepoKey(t *testing.T) {
	// Verify that canonical sources produce correct RepoKey (owner/repo, not host/owner).
	tests := []struct {
		input   string
		wantKey string
	}{
		{"github.com/pandadoc-studio/skills/skills/communication/slack-digest", "pandadoc-studio/skills"},
		{"gitlab.com/Org/Repo/path/to/skill", "org/repo"},
		{"git.internal.co/team/repo/my-skill", "team/repo"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			src, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource(%q) error: %v", tt.input, err)
			}
			got := src.RepoKey()
			if got != tt.wantKey {
				t.Errorf("RepoKey() = %q, want %q", got, tt.wantKey)
			}
		})
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

func TestRepoKey(t *testing.T) {
	tests := []struct {
		name  string
		input ParsedSource
		want  string
	}{
		{
			name:  "github shorthand",
			input: ParsedSource{Owner: "pandadoc-studio", Repo: "skills"},
			want:  "pandadoc-studio/skills",
		},
		{
			name:  "mixed case normalized",
			input: ParsedSource{Owner: "PandaDoc-Studio", Repo: "Skills"},
			want:  "pandadoc-studio/skills",
		},
		{
			name:  "no owner",
			input: ParsedSource{Repo: "skills"},
			want:  "",
		},
		{
			name:  "no repo",
			input: ParsedSource{Owner: "pandadoc"},
			want:  "",
		},
		{
			name:  "empty",
			input: ParsedSource{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.RepoKey()
			if got != tt.want {
				t.Errorf("RepoKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepoKey_FromParsedSource(t *testing.T) {
	// Verify RepoKey works on sources produced by ParseSource.
	tests := []struct {
		input string
		want  string
	}{
		{"pandadoc-studio/skills", "pandadoc-studio/skills"},
		{"pandadoc-studio/skills/skills/engineering/dokploy", "pandadoc-studio/skills"},
		{"git@github.com:pandadoc-studio/skills.git", "pandadoc-studio/skills"},
		{"git@github.com-work:pandadoc-studio/skills.git", "pandadoc-studio/skills"},
		{"https://github.com/PandaDoc/Skills", "pandadoc/skills"},
		{"github.com/pandadoc-studio/skills/skills/communication/slack-digest", "pandadoc-studio/skills"},
		{"gitlab.com/org/repo/path/to/skill", "org/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			src, err := ParseSource(tt.input)
			if err != nil {
				t.Fatalf("ParseSource(%q) error: %v", tt.input, err)
			}
			got := src.RepoKey()
			if got != tt.want {
				t.Errorf("RepoKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyCloneURLOverride(t *testing.T) {
	overrides := map[string]string{
		"pandadoc-studio/skills": "git@github.com-work:pandadoc-studio/skills.git",
	}

	t.Run("applies matching override", func(t *testing.T) {
		src, _ := ParseSource("pandadoc-studio/skills/skills/engineering/dokploy")
		if src.CloneURL != "https://github.com/pandadoc-studio/skills.git" {
			t.Fatalf("precondition: CloneURL = %q, expected HTTPS", src.CloneURL)
		}
		applied := src.ApplyCloneURLOverride(overrides)
		if !applied {
			t.Error("ApplyCloneURLOverride() returned false, want true")
		}
		if src.CloneURL != "git@github.com-work:pandadoc-studio/skills.git" {
			t.Errorf("CloneURL = %q, want SSH override", src.CloneURL)
		}
		// SubPath must be preserved.
		if src.SubPath != "skills/engineering/dokploy" {
			t.Errorf("SubPath = %q, want preserved", src.SubPath)
		}
	})

	t.Run("no match leaves CloneURL unchanged", func(t *testing.T) {
		src, _ := ParseSource("vercel-labs/agent-skills")
		origURL := src.CloneURL
		applied := src.ApplyCloneURLOverride(overrides)
		if applied {
			t.Error("ApplyCloneURLOverride() returned true, want false")
		}
		if src.CloneURL != origURL {
			t.Errorf("CloneURL changed to %q, expected unchanged %q", src.CloneURL, origURL)
		}
	})

	t.Run("nil overrides map", func(t *testing.T) {
		src, _ := ParseSource("pandadoc-studio/skills")
		applied := src.ApplyCloneURLOverride(nil)
		if applied {
			t.Error("ApplyCloneURLOverride(nil) returned true, want false")
		}
	})

	t.Run("empty overrides map", func(t *testing.T) {
		src, _ := ParseSource("pandadoc-studio/skills")
		applied := src.ApplyCloneURLOverride(map[string]string{})
		if applied {
			t.Error("ApplyCloneURLOverride({}) returned true, want false")
		}
	})

	t.Run("source without owner/repo has no repo key", func(t *testing.T) {
		src := &ParsedSource{Type: SourceTypeGit}
		applied := src.ApplyCloneURLOverride(overrides)
		if applied {
			t.Error("ApplyCloneURLOverride() returned true for source without owner/repo")
		}
	})
}
