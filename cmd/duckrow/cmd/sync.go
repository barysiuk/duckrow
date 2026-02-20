package cmd

import (
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install everything from lock file",
	Long: `Install all assets declared in duckrow.lock.json at their pinned versions.

Skills whose directories already exist are skipped. To force a reinstall,
delete the skill directory and rerun duckrow sync.

This command enforces the lock file and does not fetch upstream updates.
Use duckrow skill outdated and duckrow skill update to move the lock file forward.

This is equivalent to running duckrow skill sync (and future asset-type
sync commands) in sequence.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// For now, sync only handles skills. As new asset types are added
		// (MCPs, hooks, rules), this will orchestrate all of them.
		return runSkillSync(cmd)
	},
}

func init() {
	syncCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	syncCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	syncCmd.Flags().String("agents", "", "Comma-separated agent names to also symlink into (e.g. cursor,claude-code)")
	rootCmd.AddCommand(syncCmd)
}
