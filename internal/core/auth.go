package core

import (
	"fmt"
	"strings"
)

// CloneErrorKind classifies why a git clone failed.
type CloneErrorKind int

const (
	// CloneErrUnknown is an unclassified clone failure.
	CloneErrUnknown CloneErrorKind = iota
	// CloneErrAuth means authentication failed (credentials missing or invalid).
	CloneErrAuth
	// CloneErrRepoNotFound means the repository URL is wrong or the user has no access.
	CloneErrRepoNotFound
	// CloneErrNetwork means the host could not be reached (DNS, connectivity).
	CloneErrNetwork
	// CloneErrSSHKey means the SSH key was rejected or not found.
	CloneErrSSHKey
	// CloneErrHostKey means SSH host key verification failed.
	CloneErrHostKey
	// CloneErrTimeout means the clone operation timed out.
	CloneErrTimeout
)

// String returns a human-readable label for the error kind.
func (k CloneErrorKind) String() string {
	switch k {
	case CloneErrAuth:
		return "Authentication Required"
	case CloneErrRepoNotFound:
		return "Repository Not Found"
	case CloneErrNetwork:
		return "Network Error"
	case CloneErrSSHKey:
		return "SSH Key Error"
	case CloneErrHostKey:
		return "SSH Host Key Error"
	case CloneErrTimeout:
		return "Timeout"
	default:
		return "Unknown Error"
	}
}

// CloneError is a structured error returned when git clone fails.
// It wraps the raw git output with classification and actionable hints.
type CloneError struct {
	Kind      CloneErrorKind
	Protocol  string   // "https" or "ssh"
	URL       string   // The clone URL that was attempted
	Command   string   // The full git command that was run (for display)
	RawOutput string   // Raw stderr/stdout from git
	Hints     []string // Actionable suggestions for the user
}

// Error implements the error interface.
func (e *CloneError) Error() string {
	return fmt.Sprintf("git clone failed (%s): %s", e.Kind, e.firstLine())
}

// firstLine returns the first non-empty line of raw output for a concise error message.
func (e *CloneError) firstLine() string {
	for _, line := range strings.Split(e.RawOutput, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "Cloning into") {
			return line
		}
	}
	if e.RawOutput != "" {
		return strings.TrimSpace(e.RawOutput)
	}
	return "clone failed"
}

// IsCloneError checks whether an error is a *CloneError and returns it.
func IsCloneError(err error) (*CloneError, bool) {
	if err == nil {
		return nil, false
	}
	// Unwrap fmt.Errorf wrappers to find the CloneError.
	for err != nil {
		if ce, ok := err.(*CloneError); ok {
			return ce, true
		}
		// Try to unwrap.
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = u.Unwrap()
	}
	return nil, false
}

// ClassifyCloneError examines git clone output and returns a structured CloneError.
func ClassifyCloneError(cloneURL, command, rawOutput string) *CloneError {
	protocol := detectProtocol(cloneURL)
	kind := classifyOutput(rawOutput)

	ce := &CloneError{
		Kind:      kind,
		Protocol:  protocol,
		URL:       cloneURL,
		Command:   command,
		RawOutput: strings.TrimSpace(rawOutput),
		Hints:     hintsForError(kind, protocol, cloneURL),
	}

	return ce
}

// detectProtocol returns "ssh" or "https" based on the clone URL format.
func detectProtocol(url string) string {
	if strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://") {
		return "ssh"
	}
	return "https"
}

// classifyOutput pattern-matches git stderr to determine the error kind.
func classifyOutput(output string) CloneErrorKind {
	lower := strings.ToLower(output)

	// Timeout (checked first since it's set by us, not git).
	if strings.Contains(lower, "timed out") {
		return CloneErrTimeout
	}

	// SSH key errors.
	if strings.Contains(lower, "permission denied (publickey)") ||
		strings.Contains(lower, "no such identity") ||
		strings.Contains(lower, "load key") ||
		strings.Contains(lower, "identity file") {
		return CloneErrSSHKey
	}

	// SSH host key verification.
	if strings.Contains(lower, "host key verification failed") ||
		strings.Contains(lower, "known_hosts") {
		return CloneErrHostKey
	}

	// HTTPS auth errors.
	if strings.Contains(lower, "could not read username") ||
		strings.Contains(lower, "could not read password") ||
		strings.Contains(lower, "invalid credentials") ||
		strings.Contains(lower, "authentication failed") ||
		strings.Contains(lower, "401") ||
		strings.Contains(lower, "403") ||
		strings.Contains(lower, "logon failed") {
		return CloneErrAuth
	}

	// Repository not found (GitHub/GitLab return this for private repos with no access too).
	if strings.Contains(lower, "repository not found") ||
		strings.Contains(lower, "does not appear to be a git repository") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "project not found") {
		return CloneErrRepoNotFound
	}

	// Network errors.
	if strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "connection timed out") ||
		strings.Contains(lower, "network is unreachable") ||
		strings.Contains(lower, "no route to host") ||
		strings.Contains(lower, "name or service not known") {
		return CloneErrNetwork
	}

	return CloneErrUnknown
}

// hintsForError returns actionable suggestions based on the error kind and protocol.
func hintsForError(kind CloneErrorKind, protocol, cloneURL string) []string {
	switch kind {
	case CloneErrAuth:
		hints := []string{
			"Run `gh auth login` in your terminal to authenticate with GitHub",
			"Or configure a git credential helper: `git config --global credential.helper store`",
		}
		if protocol == "https" {
			sshURL := httpsToSSH(cloneURL)
			if sshURL != "" {
				hints = append(hints, fmt.Sprintf("Try SSH instead: %s", sshURL))
			}
		}
		return hints

	case CloneErrSSHKey:
		hints := []string{
			"Ensure your SSH key is loaded: `ssh-add -l`",
			"If no keys are listed, add one: `ssh-add ~/.ssh/id_ed25519`",
			"Check `~/.ssh/config` for the correct Host alias if using multiple accounts",
		}
		if protocol == "ssh" {
			httpsURL := sshToHTTPS(cloneURL)
			if httpsURL != "" {
				hints = append(hints, fmt.Sprintf("Try HTTPS instead: %s", httpsURL))
			}
		}
		return hints

	case CloneErrHostKey:
		return []string{
			"The SSH host key is not trusted. Run: `ssh-keyscan github.com >> ~/.ssh/known_hosts`",
			"Or connect once manually: `ssh -T git@github.com` and accept the host key",
		}

	case CloneErrRepoNotFound:
		return []string{
			"Verify the repository URL is correct",
			"Ensure you have access to this repository (it may be private)",
			"If using SSH, check that your key has access to this organization",
		}

	case CloneErrNetwork:
		return []string{
			"Check your internet connection",
			"Verify the hostname in the URL is correct",
			"If behind a proxy, ensure git is configured to use it",
		}

	case CloneErrTimeout:
		return []string{
			"The clone operation timed out after 60 seconds",
			"This may indicate a network issue or a very large repository",
			"Try again — the server may have been temporarily unavailable",
		}

	default:
		return []string{
			"Check the error message above for details",
			"Verify the repository URL is correct and accessible",
			"Try cloning manually: `git clone <url>` to diagnose the issue",
		}
	}
}

// httpsToSSH converts an HTTPS GitHub/GitLab URL to SSH format.
// Returns empty string if conversion is not possible.
func httpsToSSH(url string) string {
	// https://github.com/owner/repo.git -> git@github.com:owner/repo.git
	for _, host := range []string{"github.com", "gitlab.com"} {
		prefix := "https://" + host + "/"
		if strings.HasPrefix(url, prefix) {
			path := strings.TrimPrefix(url, prefix)
			if !strings.HasSuffix(path, ".git") {
				path += ".git"
			}
			return "git@" + host + ":" + path
		}
	}
	return ""
}

// sshToHTTPS converts an SSH git URL to HTTPS format.
// Returns empty string if conversion is not possible.
func sshToHTTPS(url string) string {
	// git@github.com:owner/repo.git -> https://github.com/owner/repo.git
	if !strings.HasPrefix(url, "git@") {
		return ""
	}
	// Split on ":"
	parts := strings.SplitN(strings.TrimPrefix(url, "git@"), ":", 2)
	if len(parts) != 2 {
		return ""
	}
	host := parts[0]
	path := parts[1]
	// Only convert known hosts.
	switch host {
	case "github.com", "gitlab.com":
		return "https://" + host + "/" + path
	default:
		return ""
	}
}

// FormatCommand builds the display string for a git clone command.
func FormatCommand(url, ref string) string {
	args := []string{"git", "clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url)
	return strings.Join(args, " ")
}
