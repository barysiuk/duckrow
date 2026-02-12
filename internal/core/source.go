package core

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
//   - "owner/repo"                  → GitHub repo
//   - "owner/repo@skill-name"       → GitHub repo, specific skill
//   - "owner/repo/path/to/skill"    → GitHub repo with subpath
//   - "./local/path" or "/abs/path" → Local directory
//   - "git@host:owner/repo.git"     → SSH git URL
//   - "https://github.com/owner/repo" → HTTPS git URL
//   - "https://gitlab.com/owner/repo" → GitLab HTTPS URL
func ParseSource(input string) (*ParsedSource, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty source")
	}

	// Local paths: starts with ./ ../ / or ~
	if isLocalPath(input) {
		return parseLocalSource(input)
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
	if atIdx := strings.LastIndex(input, "@"); atIdx > 0 && !strings.Contains(input[:atIdx], "/") == false {
		// Check if this is owner/repo@skill format (not an email-like pattern)
		parts := strings.SplitN(input, "@", 2)
		if len(parts) == 2 && ownerRepoPattern.MatchString(parts[0]) {
			segments := strings.SplitN(parts[0], "/", 2)
			return &ParsedSource{
				Type:      SourceTypeGitHub,
				Owner:     segments[0],
				Repo:      segments[1],
				CloneURL:  fmt.Sprintf("https://github.com/%s/%s.git", segments[0], segments[1]),
				SkillName: parts[1],
			}, nil
		}
	}

	// owner/repo/path/to/skill (3+ path segments)
	if m := ownerRepoPathPattern.FindStringSubmatch(input); m != nil {
		return &ParsedSource{
			Type:     SourceTypeGitHub,
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
			Type:     SourceTypeGitHub,
			Owner:    segments[0],
			Repo:     segments[1],
			CloneURL: fmt.Sprintf("https://github.com/%s/%s.git", segments[0], segments[1]),
		}, nil
	}

	return nil, fmt.Errorf("unrecognized source format: %q", input)
}

func isLocalPath(input string) bool {
	return strings.HasPrefix(input, "./") ||
		strings.HasPrefix(input, "../") ||
		strings.HasPrefix(input, "/") ||
		strings.HasPrefix(input, "~/")
}

func parseLocalSource(input string) (*ParsedSource, error) {
	expanded := expandPath(input)
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return nil, fmt.Errorf("resolving local path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("local path not found: %s", absPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local path is not a directory: %s", absPath)
	}

	return &ParsedSource{
		Type:      SourceTypeLocal,
		LocalPath: absPath,
	}, nil
}

func parseSSHSource(input string) (*ParsedSource, error) {
	// git@github.com:owner/repo.git
	// git@gitlab.com:owner/repo.git
	parts := strings.SplitN(input, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid SSH URL: %q", input)
	}

	host := strings.TrimPrefix(parts[0], "git@")
	repoPath := strings.TrimSuffix(parts[1], ".git")
	segments := strings.SplitN(repoPath, "/", 2)

	sourceType := SourceTypeGit
	if strings.Contains(host, "github.com") {
		sourceType = SourceTypeGitHub
	} else if strings.Contains(host, "gitlab.com") {
		sourceType = SourceTypeGitLab
	}

	result := &ParsedSource{
		Type:     sourceType,
		CloneURL: input,
	}

	if len(segments) == 2 {
		result.Owner = segments[0]
		result.Repo = segments[1]
	}

	return result, nil
}

func parseHTTPSource(input string) (*ParsedSource, error) {
	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	sourceType := SourceTypeGit
	if strings.Contains(u.Host, "github.com") {
		sourceType = SourceTypeGitHub
	} else if strings.Contains(u.Host, "gitlab.com") {
		sourceType = SourceTypeGitLab
	}

	// Parse path segments: /owner/repo[/tree/branch/subpath]
	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")

	result := &ParsedSource{
		Type: sourceType,
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
