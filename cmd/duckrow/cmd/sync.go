package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install everything from lock file",
	Long: `Install all assets declared in duckrow.lock.json at their pinned versions.

Skills whose directories already exist are skipped. MCP entries that already
exist in agent config files are skipped unless --force is used.

This command enforces the lock file and does not fetch upstream updates.
Use duckrow skill outdated and duckrow skill update to move the lock file forward.

This is equivalent to running duckrow skill sync and duckrow mcp sync in sequence.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stdout, "Syncing from duckrow.lock.json...")
		fmt.Fprintln(os.Stdout)

		// Sync skills.
		skillResult, skillErr := runSkillSync(cmd)
		if skillResult != nil {
			fmt.Fprintf(os.Stdout, "Skills: %d installed, %d skipped, %d errors\n",
				skillResult.installed, skillResult.skipped, skillResult.errors)
		} else if skillErr != nil {
			fmt.Fprintf(os.Stderr, "Skills: error: %v\n", skillErr)
		}

		// Sync MCPs.
		mcpResult, mcpErr := runMCPSync(cmd)
		if mcpResult != nil {
			fmt.Fprintf(os.Stdout, "MCPs:   %d installed, %d skipped, %d errors\n",
				mcpResult.installed, mcpResult.skipped, mcpResult.errors)
			printRequiredEnvSummary(mcpResult.requiredEnv)
		} else if mcpErr != nil {
			fmt.Fprintf(os.Stderr, "MCPs:   error: %v\n", mcpErr)
		}

		fmt.Fprintln(os.Stdout, "\nSynced successfully.")

		// Return the first error encountered.
		if skillErr != nil {
			return skillErr
		}
		if skillResult != nil && skillResult.errors > 0 {
			return fmt.Errorf("%d skill(s) failed to sync", skillResult.errors)
		}
		if mcpErr != nil {
			return mcpErr
		}
		if mcpResult != nil && mcpResult.errors > 0 {
			return fmt.Errorf("%d MCP(s) failed to sync", mcpResult.errors)
		}
		return nil
	},
}

func init() {
	syncCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	syncCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	syncCmd.Flags().Bool("force", false, "Overwrite existing MCP entries in agent config files")
	syncCmd.Flags().String("agents", "", "Comma-separated agent names to target (e.g. cursor,claude-code)")
	rootCmd.AddCommand(syncCmd)
}
