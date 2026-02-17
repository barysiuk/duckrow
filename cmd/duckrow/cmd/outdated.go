package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Show skills with available updates",
	Long: `Show which installed skills differ from the available commit.

Reads duckrow.lock.json and checks each skill's source for newer commits.
Registry-pinned skills are checked against the registry commit first.
Other skills are checked by fetching the latest commit from the source repo.`,
	Args: cobra.NoArgs,
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

		jsonOutput, _ := cmd.Flags().GetBool("json")

		lf, err := core.ReadLockFile(targetDir)
		if err != nil {
			return fmt.Errorf("reading lock file: %w", err)
		}
		if lf == nil {
			return fmt.Errorf("no duckrow.lock.json found in %s", targetDir)
		}

		if len(lf.Skills) == 0 {
			fmt.Fprintln(os.Stdout, "Lock file has no skills.")
			return nil
		}

		cfg, err := d.config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Build registry commit lookup: map[lockSource] -> commit.
		rm := core.NewRegistryManager(d.config.RegistriesDir())
		rm.HydrateRegistryCommits(cfg.Registries, cfg.Settings.CloneURLOverrides)
		registryCommits := core.BuildRegistryCommitMap(cfg.Registries, rm)

		updates, err := core.CheckForUpdates(lf, cfg.Settings.CloneURLOverrides, registryCommits)
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		if jsonOutput {
			data, err := json.MarshalIndent(updates, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Fprintln(os.Stdout, string(data))
			return nil
		}

		// Table output.
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "Skill\tInstalled\tAvailable\tSource")

		for _, u := range updates {
			installed := core.TruncateCommit(u.InstalledCommit)
			available := "(up to date)"
			if u.HasUpdate {
				available = core.TruncateCommit(u.AvailableCommit)
			}
			// Truncate source to host/owner/repo for table output.
			source := truncateSource(u.Source)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.Name, installed, available, source)
		}

		_ = w.Flush()
		return nil
	},
}

// truncateSource returns the host/owner/repo portion of a canonical source.
func truncateSource(source string) string {
	parts := strings.Split(source, "/")
	if len(parts) > 3 {
		return parts[0] + "/" + parts[1] + "/" + parts[2]
	}
	return source
}

func init() {
	outdatedCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	outdatedCmd.Flags().Bool("json", false, "Output as JSON for scripting")
	rootCmd.AddCommand(outdatedCmd)
}
