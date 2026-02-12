package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/barysiuk/duckrow/cmd/duckrow/cmd"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"duckrow": func() {
			if err := cmd.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	})
}

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 filepath.Join("testdata", "script"),
		RequireExplicitExec: true,
		Setup: func(e *testscript.Env) error {
			// Set HOME to WORK so ~/.duckrow/ is created inside the temp dir
			e.Vars = append(e.Vars, "HOME="+e.WorkDir)
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			// is-symlink asserts that a path is (or is not) a symlink.
			// Usage: [!] is-symlink <path>
			"is-symlink": cmdIsSymlink,

			// file-contains asserts that a file contains (or doesn't contain) a substring.
			// Usage: [!] file-contains <path> <substring>
			"file-contains": cmdFileContains,

			// setup-git-repo creates a local git repo with a duckrow.json manifest.
			// Usage: setup-git-repo <dir> <registry-name> [skill-name...]
			// Creates a git repo at <dir> with a manifest containing the given skills.
			"setup-git-repo": cmdSetupGitRepo,

			// dir-not-exists asserts that a directory does not exist.
			// Usage: [!] dir-not-exists <path>
			"dir-not-exists": cmdDirNotExists,
		},
	})
}

// cmdIsSymlink checks if a path is a symlink.
func cmdIsSymlink(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: is-symlink <path>")
	}
	path := ts.MkAbs(args[0])
	fi, err := os.Lstat(path)
	isSymlink := err == nil && fi.Mode()&os.ModeSymlink != 0

	if neg {
		if isSymlink {
			ts.Fatalf("%s is a symlink (expected not to be)", args[0])
		}
	} else {
		if !isSymlink {
			if err != nil {
				ts.Fatalf("%s: %v", args[0], err)
			}
			ts.Fatalf("%s is not a symlink (mode: %s)", args[0], fi.Mode())
		}
	}
}

// cmdFileContains checks if a file contains a substring.
func cmdFileContains(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: file-contains <path> <substring>")
	}
	path := ts.MkAbs(args[0])
	substr := args[1]

	data, err := os.ReadFile(path)
	if err != nil {
		ts.Fatalf("reading %s: %v", args[0], err)
	}

	contains := containsString(string(data), substr)
	if neg {
		if contains {
			ts.Fatalf("file %s contains %q (expected not to)", args[0], substr)
		}
	} else {
		if !contains {
			ts.Fatalf("file %s does not contain %q\nContent:\n%s", args[0], substr, string(data))
		}
	}
}

// cmdDirNotExists checks that a directory does not exist.
func cmdDirNotExists(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: dir-not-exists <path>")
	}
	path := ts.MkAbs(args[0])
	_, err := os.Stat(path)
	doesNotExist := os.IsNotExist(err)

	if neg {
		// ! dir-not-exists == dir exists
		if doesNotExist {
			ts.Fatalf("%s does not exist (expected it to exist)", args[0])
		}
	} else {
		if !doesNotExist {
			ts.Fatalf("%s exists (expected it not to)", args[0])
		}
	}
}

// cmdSetupGitRepo creates a local git repo with a duckrow.json manifest.
func cmdSetupGitRepo(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("setup-git-repo does not support negation")
	}
	if len(args) < 2 {
		ts.Fatalf("usage: setup-git-repo <dir> <registry-name> [skill-name...]")
	}

	dir := ts.MkAbs(args[0])
	registryName := args[1]
	skillNames := args[2:]

	if err := os.MkdirAll(dir, 0o755); err != nil {
		ts.Fatalf("creating dir: %v", err)
	}

	// Build manifest
	type skillEntry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Source      string `json:"source"`
		Version     string `json:"version,omitempty"`
	}
	type manifest struct {
		Name        string       `json:"name"`
		Description string       `json:"description,omitempty"`
		Skills      []skillEntry `json:"skills"`
	}

	m := manifest{
		Name:        registryName,
		Description: registryName + " skills",
	}
	for _, name := range skillNames {
		m.Skills = append(m.Skills, skillEntry{
			Name:        name,
			Description: "Skill " + name,
			Source:      registryName + "/" + name,
			Version:     "1.0.0",
		})
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		ts.Fatalf("marshaling manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "duckrow.json"), data, 0o644); err != nil {
		ts.Fatalf("writing manifest: %v", err)
	}

	// Initialize git repo
	gitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)

	runGit := func(gitArgs ...string) {
		c := exec.Command("git", gitArgs...)
		c.Dir = dir
		c.Env = gitEnv
		out, err := c.CombinedOutput()
		if err != nil {
			ts.Fatalf("git %v: %v\n%s", gitArgs, err, out)
		}
	}

	runGit("init")
	runGit("checkout", "-b", "main")
	runGit("add", ".")
	runGit("commit", "-m", "initial")
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
