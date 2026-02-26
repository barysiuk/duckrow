package cmd

import (
	"fmt"
	"os"

	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install everything from lock file",
	Long: `Install all assets declared in duckrow.lock.json at their pinned versions.

Skills whose directories already exist are skipped. Agent files that already
exist in system agent directories are skipped. MCP entries that already
exist in agent config files are skipped unless --force is used.

This command enforces the lock file and does not fetch upstream updates.
Use duckrow skill outdated and duckrow skill update to move the lock file forward.

This is equivalent to running duckrow skill sync, duckrow mcp sync, and duckrow agent sync in sequence.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stdout, "Syncing from duckrow.lock.json...")
		fmt.Fprintln(os.Stdout)

		var firstErr error

		for _, kind := range asset.Kinds() {
			result, err := runAssetSyncInner(cmd, kind)

			handler, _ := asset.Get(kind)
			display := handler.DisplayName()

			if result != nil {
				fmt.Fprintf(os.Stdout, "%ss: %d installed, %d skipped, %d errors\n",
					display, result.installed, result.skipped, result.errors)
				if kind == asset.KindMCP {
					printRequiredEnvSummary(result.requiredEnv)
				}
				if firstErr == nil && result.errors > 0 {
					firstErr = fmt.Errorf("%d %s(s) failed to sync", result.errors, display)
				}
			} else if err != nil {
				fmt.Fprintf(os.Stderr, "%ss: error: %v\n", display, err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}

		if firstErr == nil {
			fmt.Fprintln(os.Stdout, "\nSynced successfully.")
		}

		return firstErr
	},
}

func init() {
	syncCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	syncCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	syncCmd.Flags().Bool("force", false, "Overwrite existing MCP entries in agent config files")
	addSystemsFlag(syncCmd)
	rootCmd.AddCommand(syncCmd)
}
