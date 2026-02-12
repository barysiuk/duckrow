package core

import (
	"testing"
)

func TestClassifyOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantKind CloneErrorKind
	}{
		// HTTPS auth errors.
		{
			name:     "https could not read username",
			output:   "fatal: could not read Username for 'https://github.com': terminal prompts disabled",
			wantKind: CloneErrAuth,
		},
		{
			name:     "https could not read password",
			output:   "fatal: could not read Password for 'https://github.com': terminal prompts disabled",
			wantKind: CloneErrAuth,
		},
		{
			name:     "https authentication failed",
			output:   "fatal: Authentication failed for 'https://github.com/owner/repo.git/'",
			wantKind: CloneErrAuth,
		},
		{
			name:     "https invalid credentials",
			output:   "remote: Invalid credentials\nfatal: Authentication failed",
			wantKind: CloneErrAuth,
		},
		{
			name:     "https 401",
			output:   "fatal: unable to access 'https://github.com/owner/repo.git/': The requested URL returned error: 401",
			wantKind: CloneErrAuth,
		},
		{
			name:     "https 403",
			output:   "fatal: unable to access 'https://github.com/owner/repo.git/': The requested URL returned error: 403",
			wantKind: CloneErrAuth,
		},
		{
			name:     "windows logon failed",
			output:   "Logon failed, use ctrl+c to cancel basic credential prompt.",
			wantKind: CloneErrAuth,
		},

		// SSH key errors.
		{
			name:     "ssh permission denied publickey",
			output:   "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.",
			wantKind: CloneErrSSHKey,
		},
		{
			name:     "ssh load key",
			output:   "Load key \"/home/user/.ssh/id_rsa\": No such file or directory\ngit@github.com: Permission denied (publickey).",
			wantKind: CloneErrSSHKey,
		},
		{
			name:     "ssh no such identity",
			output:   "no such identity: /home/user/.ssh/id_ed25519: No such file or directory",
			wantKind: CloneErrSSHKey,
		},

		// SSH host key errors.
		{
			name:     "ssh host key verification failed",
			output:   "Host key verification failed.\nfatal: Could not read from remote repository.",
			wantKind: CloneErrHostKey,
		},
		{
			name:     "ssh known_hosts issue",
			output:   "Warning: the ECDSA host key for 'github.com' differs from the key for the IP address\nOffending key for IP in /home/user/.ssh/known_hosts:5",
			wantKind: CloneErrHostKey,
		},

		// Repository not found.
		{
			name:     "github repo not found",
			output:   "remote: Repository not found.\nfatal: repository 'https://github.com/owner/repo.git/' not found",
			wantKind: CloneErrRepoNotFound,
		},
		{
			name:     "not a git repository",
			output:   "fatal: 'https://example.com/foo' does not appear to be a git repository\nfatal: Could not read from remote repository.",
			wantKind: CloneErrRepoNotFound,
		},
		{
			name:     "gitlab project not found",
			output:   "remote: The project you were looking for could not be found.\nfatal: repository 'https://gitlab.com/owner/repo.git/' not found",
			wantKind: CloneErrRepoNotFound,
		},

		// Network errors.
		{
			name:     "could not resolve host",
			output:   "fatal: unable to access 'https://github.com/owner/repo.git/': Could not resolve host: github.com",
			wantKind: CloneErrNetwork,
		},
		{
			name:     "connection refused",
			output:   "fatal: unable to access 'https://github.com/owner/repo.git/': Failed to connect to github.com port 443: Connection refused",
			wantKind: CloneErrNetwork,
		},
		{
			name:     "network unreachable",
			output:   "fatal: unable to access 'https://github.com/owner/repo.git/': Network is unreachable",
			wantKind: CloneErrNetwork,
		},

		// Timeout.
		{
			name:     "duckrow timeout",
			output:   "command timed out after 60s",
			wantKind: CloneErrTimeout,
		},

		// Unknown.
		{
			name:     "unknown error",
			output:   "fatal: something unexpected happened",
			wantKind: CloneErrUnknown,
		},
		{
			name:     "empty output",
			output:   "",
			wantKind: CloneErrUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOutput(tt.output)
			if got != tt.wantKind {
				t.Errorf("classifyOutput(%q) = %v, want %v", tt.output, got, tt.wantKind)
			}
		})
	}
}

func TestClassifyCloneError(t *testing.T) {
	ce := ClassifyCloneError(
		"https://github.com/owner/repo.git",
		"git clone --depth 1 https://github.com/owner/repo.git /tmp/foo",
		"fatal: could not read Username for 'https://github.com': terminal prompts disabled",
	)

	if ce.Kind != CloneErrAuth {
		t.Errorf("Kind = %v, want CloneErrAuth", ce.Kind)
	}
	if ce.Protocol != "https" {
		t.Errorf("Protocol = %q, want %q", ce.Protocol, "https")
	}
	if ce.URL != "https://github.com/owner/repo.git" {
		t.Errorf("URL = %q, want %q", ce.URL, "https://github.com/owner/repo.git")
	}
	if ce.Command != "git clone --depth 1 https://github.com/owner/repo.git /tmp/foo" {
		t.Errorf("Command = %q, unexpected", ce.Command)
	}
	if len(ce.Hints) == 0 {
		t.Error("expected non-empty hints")
	}
}

func TestCloneErrorKindString(t *testing.T) {
	tests := []struct {
		kind CloneErrorKind
		want string
	}{
		{CloneErrAuth, "Authentication Required"},
		{CloneErrRepoNotFound, "Repository Not Found"},
		{CloneErrNetwork, "Network Error"},
		{CloneErrSSHKey, "SSH Key Error"},
		{CloneErrHostKey, "SSH Host Key Error"},
		{CloneErrTimeout, "Timeout"},
		{CloneErrUnknown, "Unknown Error"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectProtocol(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/owner/repo.git", "https"},
		{"http://github.com/owner/repo.git", "https"},
		{"git@github.com:owner/repo.git", "ssh"},
		{"ssh://git@github.com/owner/repo.git", "ssh"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := detectProtocol(tt.url); got != tt.want {
				t.Errorf("detectProtocol(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestHTTPSToSSH(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/owner/repo.git", "git@github.com:owner/repo.git"},
		{"https://github.com/owner/repo", "git@github.com:owner/repo.git"},
		{"https://gitlab.com/owner/repo.git", "git@gitlab.com:owner/repo.git"},
		{"https://example.com/owner/repo.git", ""}, // unknown host
		{"git@github.com:owner/repo.git", ""},      // already SSH
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := httpsToSSH(tt.url); got != tt.want {
				t.Errorf("httpsToSSH(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestSSHToHTTPS(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:owner/repo.git", "https://github.com/owner/repo.git"},
		{"git@gitlab.com:owner/repo.git", "https://gitlab.com/owner/repo.git"},
		{"git@example.com:owner/repo.git", ""},    // unknown host
		{"https://github.com/owner/repo.git", ""}, // not SSH
		{"git@github.com-without-colon", ""},      // malformed
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := sshToHTTPS(tt.url); got != tt.want {
				t.Errorf("sshToHTTPS(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestHintsForError(t *testing.T) {
	// HTTPS auth should suggest SSH alternative.
	hints := hintsForError(CloneErrAuth, "https", "https://github.com/owner/repo.git")
	found := false
	for _, h := range hints {
		if contains(h, "git@github.com:owner/repo.git") {
			found = true
			break
		}
	}
	if !found {
		t.Error("HTTPS auth hints should suggest SSH alternative")
	}

	// SSH key error should suggest HTTPS alternative.
	hints = hintsForError(CloneErrSSHKey, "ssh", "git@github.com:owner/repo.git")
	found = false
	for _, h := range hints {
		if contains(h, "https://github.com/owner/repo.git") {
			found = true
			break
		}
	}
	if !found {
		t.Error("SSH key hints should suggest HTTPS alternative")
	}

	// Non-convertible URL should not produce empty hints.
	hints = hintsForError(CloneErrAuth, "https", "https://example.com/owner/repo.git")
	if len(hints) == 0 {
		t.Error("expected non-empty hints even for non-convertible URLs")
	}
}

func TestIsCloneError(t *testing.T) {
	ce := &CloneError{Kind: CloneErrAuth, URL: "https://github.com/owner/repo.git"}

	// Direct.
	got, ok := IsCloneError(ce)
	if !ok || got != ce {
		t.Error("IsCloneError should find direct CloneError")
	}

	// Nil.
	got, ok = IsCloneError(nil)
	if ok || got != nil {
		t.Error("IsCloneError(nil) should return false")
	}
}

func TestFormatCommand(t *testing.T) {
	got := FormatCommand("https://github.com/owner/repo.git", "")
	want := "git clone --depth 1 https://github.com/owner/repo.git"
	if got != want {
		t.Errorf("FormatCommand() = %q, want %q", got, want)
	}

	got = FormatCommand("https://github.com/owner/repo.git", "main")
	want = "git clone --depth 1 --branch main https://github.com/owner/repo.git"
	if got != want {
		t.Errorf("FormatCommand(with ref) = %q, want %q", got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
