package core

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ownerRepoPattern matches "owner/repo" format (2 segments, no protocol).
var ownerRepoPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)

// ownerRepoPathPattern matches "owner/repo/path/to/skill" format (3+ segments).
var ownerRepoPathPattern = regexp.MustCompile(`^([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)/(.+)$`)

// ParseSource parses a skill source string into a structured ParsedSource.
//
// Supported formats:
//   - "host/owner/repo/path/to/skill"     → Canonical source (host contains a dot)
//   - "owner/repo"                        → GitHub repo (shorthand)
//   - "owner/repo@skill-name"             → GitHub repo, specific skill
//   - "owner/repo/path/to/skill"          → GitHub repo with subpath
//   - "git@host:owner/repo.git"           → SSH git URL
//   - "https://github.com/owner/repo"     → HTTPS git URL
//   - "https://gitlab.com/owner/repo"     → GitLab HTTPS URL
//   - "https://git.example.com/owner/repo" → Any git host HTTPS URL
//
// Local paths (./foo, ../foo, /foo, ~/foo) are explicitly rejected.
func ParseSource(input string) (*ParsedSource, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty source")
	}

	// Local paths are not supported — reject explicitly.
	if strings.HasPrefix(input, "./") || strings.HasPrefix(input, "../") ||
		strings.HasPrefix(input, "/") || strings.HasPrefix(input, "~/") {
		return nil, fmt.Errorf("local path installs are not supported: %q (use a git URL or owner/repo shorthand)", input)
	}

	// SSH git URL: git@host:owner/repo.git
	if strings.HasPrefix(input, "git@") {
		return parseSSHSource(input)
	}

	// HTTPS URLs
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		return parseHTTPSource(input)
	}

	// owner/repo@skill-name
	if atIdx := strings.LastIndex(input, "@"); atIdx > 0 && strings.Contains(input[:atIdx], "/") {
		// Check if this is owner/repo@skill format (not an email-like pattern)
		parts := strings.SplitN(input, "@", 2)
		if len(parts) == 2 && ownerRepoPattern.MatchString(parts[0]) {
			segments := strings.SplitN(parts[0], "/", 2)
			return &ParsedSource{
				Type:      SourceTypeGit,
				Host:      "github.com",
				Owner:     segments[0],
				Repo:      segments[1],
				CloneURL:  fmt.Sprintf("https://github.com/%s/%s.git", segments[0], segments[1]),
				SkillName: parts[1],
			}, nil
		}
	}

	// Canonical source: host/owner/repo[/path/to/skill]
	// Detected when the first segment contains a dot (hostname indicator).
	if m := ownerRepoPathPattern.FindStringSubmatch(input); m != nil && strings.Contains(m[1], ".") {
		host := m[1]
		// Remaining is owner/repo[/subpath] — split further.
		rest := m[2] + "/" + m[3] // rejoin segments after host
		restParts := strings.SplitN(rest, "/", 3)
		if len(restParts) < 2 {
			return nil, fmt.Errorf("canonical source %q must have at least host/owner/repo", input)
		}
		owner := restParts[0]
		repo := restParts[1]
		var subPath string
		if len(restParts) == 3 {
			subPath = restParts[2]
		}
		return &ParsedSource{
			Type:     SourceTypeGit,
			Host:     host,
			Owner:    owner,
			Repo:     repo,
			CloneURL: fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo),
			SubPath:  subPath,
		}, nil
	}

	// owner/repo/path/to/skill (3+ path segments, no host)
	if m := ownerRepoPathPattern.FindStringSubmatch(input); m != nil {
		return &ParsedSource{
			Type:     SourceTypeGit,
			Host:     "github.com",
			Owner:    m[1],
			Repo:     m[2],
			CloneURL: fmt.Sprintf("https://github.com/%s/%s.git", m[1], m[2]),
			SubPath:  m[3],
		}, nil
	}

	// owner/repo (exactly 2 path segments)
	if ownerRepoPattern.MatchString(input) {
		segments := strings.SplitN(input, "/", 2)
		return &ParsedSource{
			Type:     SourceTypeGit,
			Host:     "github.com",
			Owner:    segments[0],
			Repo:     segments[1],
			CloneURL: fmt.Sprintf("https://github.com/%s/%s.git", segments[0], segments[1]),
		}, nil
	}

	return nil, fmt.Errorf("unrecognized source format: %q", input)
}

func parseSSHSource(input string) (*ParsedSource, error) {
	// git@github.com:owner/repo.git
	// git@gitlab.com:owner/repo.git
	// git@git.internal.co:owner/repo.git
	parts := strings.SplitN(input, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid SSH URL: %q", input)
	}

	host := strings.TrimPrefix(parts[0], "git@")
	repoPath := strings.TrimSuffix(parts[1], ".git")
	segments := strings.SplitN(repoPath, "/", 2)

	result := &ParsedSource{
		Type:     SourceTypeGit,
		Host:     host,
		CloneURL: input,
	}

	if len(segments) == 2 {
		result.Owner = segments[0]
		result.Repo = segments[1]
	}

	return result, nil
}

// RepoKey returns a normalized "owner/repo" key for this source.
// This key is used to look up clone URL overrides in the config.
// Returns empty string if Owner or Repo are not set.
func (ps *ParsedSource) RepoKey() string {
	if ps.Owner == "" || ps.Repo == "" {
		return ""
	}
	return strings.ToLower(ps.Owner) + "/" + strings.ToLower(ps.Repo)
}

// ApplyCloneURLOverride replaces CloneURL with the override value if one
// exists for this source's RepoKey. Returns true if an override was applied.
func (ps *ParsedSource) ApplyCloneURLOverride(overrides map[string]string) bool {
	if len(overrides) == 0 {
		return false
	}
	key := ps.RepoKey()
	if key == "" {
		return false
	}
	if override, ok := overrides[key]; ok && override != "" {
		ps.CloneURL = override
		return true
	}
	return false
}

func parseHTTPSource(input string) (*ParsedSource, error) {
	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Parse path segments: /owner/repo[/tree/branch/subpath]
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")

	result := &ParsedSource{
		Type: SourceTypeGit,
		Host: u.Host,
	}

	if len(pathParts) >= 2 {
		result.Owner = pathParts[0]
		result.Repo = strings.TrimSuffix(pathParts[1], ".git")

		cloneURL := fmt.Sprintf("https://%s/%s/%s.git", u.Host, result.Owner, result.Repo)
		result.CloneURL = cloneURL

		// Handle /tree/branch/subpath pattern
		if len(pathParts) >= 4 && pathParts[2] == "tree" {
			result.Ref = pathParts[3]
			if len(pathParts) > 4 {
				result.SubPath = strings.Join(pathParts[4:], "/")
			}
		}
	} else {
		// Not a recognizable owner/repo URL, use as-is
		result.CloneURL = input
	}

	return result, nil
}
