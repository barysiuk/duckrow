package cmd

import (
	"fmt"
	"os"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <skill-name>",
	Short: "Remove an installed skill",
	Long:  `Remove a skill from the current directory (or specified directory). Removes the canonical copy and all agent symlinks.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		targetDir, _ := cmd.Flags().GetString("dir")
		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
		}

		remover := core.NewRemover(d.agents)
		result, err := remover.Remove(args[0], core.RemoveOptions{TargetDir: targetDir})
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Removed: %s\n", result.Name)
		if len(result.RemovedSymlinks) > 0 {
			fmt.Fprintf(os.Stdout, "  Cleaned up agent links: %s\n", joinStrings(result.RemovedSymlinks))
		}
		return nil
	},
}

var uninstallAllCmd = &cobra.Command{
	Use:   "uninstall-all",
	Short: "Remove all installed skills",
	Long:  `Remove all skills from the current directory (or specified directory).`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		targetDir, _ := cmd.Flags().GetString("dir")
		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
		}

		remover := core.NewRemover(d.agents)
		results, err := remover.RemoveAll(core.RemoveOptions{TargetDir: targetDir})
		if err != nil {
			return err
		}

		if len(results) == 0 {
			fmt.Fprintln(os.Stdout, "No skills installed.")
			return nil
		}

		for _, r := range results {
			fmt.Fprintf(os.Stdout, "Removed: %s\n", r.Name)
		}
		fmt.Fprintf(os.Stdout, "\nRemoved %d skill(s).\n", len(results))
		return nil
	},
}

func init() {
	uninstallCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	uninstallAllCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(uninstallAllCmd)
}
