package core

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// canonicalSkillsDir is the project-relative path where skill assets are stored.
const canonicalSkillsDir = ".agents/skills"

const (
	cloneTimeout = 60 * time.Second
)

// excludedFiles are files/dirs excluded when copying skills.
var excludedFiles = map[string]bool{
	"README.md":     true,
	"metadata.json": true,
	".git":          true,
}

var sanitizeRegexp = regexp.MustCompile(`[^a-zA-Z0-9-]`)

// cloneRepo clones a git repository to a temp directory.
// When shallow is true, only the latest commit is fetched (faster but cannot
// resolve per-path commits). When shallow is false, the full history is cloned
// so that git log can accurately resolve per-path commits.
func cloneRepo(url string, ref string, shallow bool) (string, error) {
	tmpDir, err := os.MkdirTemp("", "duckrow-clone-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	args := []string{"clone"}
	if shallow {
		args = append(args, "--depth", "1")
	}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := runWithTimeout(cmd, cloneTimeout)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", ClassifyCloneError(url, FormatCommand(url, ref), output)
	}

	return tmpDir, nil
}

// cloneRepoAtCommit fetches a specific commit without full clone history.
// Uses git init + fetch --depth 1 + checkout FETCH_HEAD.
func cloneRepoAtCommit(url string, commit string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "duckrow-clone-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}

	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	// git init
	initCmd := exec.Command("git", "init", tmpDir)
	initCmd.Env = env
	if output, err := runWithTimeout(initCmd, cloneTimeout); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git init failed: %s", output)
	}

	// git remote add origin <url>
	remoteCmd := exec.Command("git", "-C", tmpDir, "remote", "add", "origin", url)
	remoteCmd.Env = env
	if output, err := runWithTimeout(remoteCmd, cloneTimeout); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git remote add failed: %s", output)
	}

	// git fetch --depth 1 origin <commit>
	fetchCmd := exec.Command("git", "-C", tmpDir, "fetch", "--depth", "1", "origin", commit)
	fetchCmd.Env = env
	if output, err := runWithTimeout(fetchCmd, cloneTimeout); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("commit %s not found in remote (may have been force-pushed away): %s", commit, output)
	}

	// git checkout FETCH_HEAD
	checkoutCmd := exec.Command("git", "-C", tmpDir, "checkout", "FETCH_HEAD")
	checkoutCmd.Env = env
	if output, err := runWithTimeout(checkoutCmd, cloneTimeout); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git checkout failed: %s", output)
	}

	return tmpDir, nil
}

// runWithTimeout runs a command with a timeout.
func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) (string, error) {
	done := make(chan struct{})
	var output []byte
	var cmdErr error

	go func() {
		output, cmdErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		return string(output), cmdErr
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return "", fmt.Errorf("command timed out after %s", timeout)
	}
}

// copyDirectory copies the contents of src to dst, excluding certain files.
func copyDirectory(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip excluded files
		baseName := filepath.Base(path)
		if excludedFiles[baseName] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files/dirs starting with _
		if strings.HasPrefix(baseName, "_") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// sanitizeName normalizes a name for use as a directory name.
func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = sanitizeRegexp.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-.")
	if len(name) > 255 {
		name = name[:255]
	}
	if name == "" {
		name = "unnamed-skill"
	}
	return name
}

// cleanupEmptyDir removes a directory if it is empty.
func cleanupEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		_ = os.Remove(dir)
	}
}

// dirExists returns true if the path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// expandPath expands ~ to home directory and $VAR / $XDG_CONFIG to env values.
func expandPath(p string) string {
	// Handle $XDG_CONFIG
	if strings.Contains(p, "$XDG_CONFIG") {
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			home, _ := os.UserHomeDir()
			xdgConfig = filepath.Join(home, ".config")
		}
		p = strings.ReplaceAll(p, "$XDG_CONFIG", xdgConfig)
	}

	// Handle other env vars like $CODEX_HOME
	if strings.Contains(p, "$") {
		p = os.Expand(p, func(key string) string {
			if key == "XDG_CONFIG" {
				// Already handled above
				return ""
			}
			return os.Getenv(key)
		})
	}

	// Handle ~
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[2:])
	} else if p == "~" {
		home, _ := os.UserHomeDir()
		p = home
	}

	return p
}
