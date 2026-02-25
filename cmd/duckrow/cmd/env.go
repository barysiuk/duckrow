package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env --mcp <name> -- <command> [args...]",
	Short: "Run a command with MCP environment variables",
	Long: `Internal runtime helper that injects environment variables into MCP server processes.

This command is written into agent MCP config files by duckrow during install/sync
and is not intended to be invoked directly by users.

It reads the requiredEnv list for the named MCP from duckrow.lock.json, resolves
values from the process environment, project .env.duckrow, and global
~/.duckrow/.env.duckrow, then exec's the given command with the filtered environment.`,
	DisableFlagParsing: true,
	SilenceUsage:       true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Manual flag parsing because cobra's flag parsing doesn't handle
		// the -- separator well with DisableFlagParsing.
		mcpName, targetDir, cmdArgs, err := parseEnvArgs(args)
		if err != nil {
			return err
		}

		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
		}

		// Read lock file.
		lf, err := core.ReadLockFile(targetDir)
		if err != nil {
			return fmt.Errorf("reading lock file: %w", err)
		}
		if lf == nil {
			return fmt.Errorf("duckrow.lock.json not found in %s", targetDir)
		}

		// Find the MCP entry.
		mcpEntry := core.FindLockedAsset(lf, asset.KindMCP, mcpName)
		if mcpEntry == nil {
			return fmt.Errorf("MCP %q not found in lock file", mcpName)
		}

		requiredEnv := lockedRequiredEnvVars(*mcpEntry)

		// Resolve environment variables.
		resolver := core.NewEnvResolver(targetDir, "")
		resolved, missing := resolver.ResolveEnv(requiredEnv)

		// Warn about missing vars.
		for _, name := range missing {
			fmt.Fprintf(os.Stderr, "Warning: env var %s required by MCP %q not found\n", name, mcpName)
		}

		// Build environment: start with current process env, add resolved vars.
		environ := os.Environ()
		for k, v := range resolved {
			environ = append(environ, k+"="+v)
		}

		// Find the command binary.
		binary, err := exec.LookPath(cmdArgs[0])
		if err != nil {
			return fmt.Errorf("command not found: %s", cmdArgs[0])
		}

		// Exec the command (replaces this process).
		return syscall.Exec(binary, cmdArgs, environ)
	},
}

func lockedRequiredEnvVars(locked asset.LockedAsset) []string {
	if locked.Data == nil {
		return nil
	}
	if envs, ok := locked.Data["requiredEnv"]; ok {
		switch v := envs.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

// parseEnvArgs manually parses the args for `duckrow env`.
// Expected format: --mcp <name> [-d <dir>] -- <command> [args...]
func parseEnvArgs(args []string) (mcpName, targetDir string, cmdArgs []string, err error) {
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--mcp":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("--mcp requires a value")
			}
			mcpName = args[i+1]
			i += 2
		case "-d", "--dir":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("%s requires a value", args[i])
			}
			targetDir = args[i+1]
			i += 2
		case "--":
			cmdArgs = args[i+1:]
			i = len(args) // exit loop
		default:
			return "", "", nil, fmt.Errorf("unexpected argument: %s", args[i])
		}
	}

	if mcpName == "" {
		return "", "", nil, fmt.Errorf("--mcp flag is required")
	}
	if len(cmdArgs) == 0 {
		return "", "", nil, fmt.Errorf("no command specified after --")
	}

	return mcpName, targetDir, cmdArgs, nil
}

func init() {
	rootCmd.AddCommand(envCmd)
}
