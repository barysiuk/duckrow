package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install skills from lock file",
	Long: `Install all skills declared in duckrow.lock.json at their pinned commits.

Skills whose directories already exist are skipped. To force a reinstall,
delete the skill directory and rerun duckrow sync.

This command enforces the lock file and does not fetch upstream updates.
Use duckrow outdated and duckrow update to move the lock file forward.`,
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

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		agentsFlag, _ := cmd.Flags().GetString("agents")

		// Resolve target agents: universal-only unless --agents is provided.
		var targetAgents []core.AgentDef
		if agentsFlag != "" {
			names := strings.Split(agentsFlag, ",")
			for i := range names {
				names[i] = strings.TrimSpace(names[i])
			}
			specified, resolveErr := core.ResolveAgentsByNames(d.agents, names)
			if resolveErr != nil {
				return resolveErr
			}
			targetAgents = core.GetUniversalAgents(d.agents)
			targetAgents = append(targetAgents, specified...)
		}

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

		installer := core.NewInstaller(d.agents)

		var installed, skipped, errors int

		for _, skill := range lf.Skills {
			// Check if skill directory already exists.
			skillDir := filepath.Join(targetDir, ".agents", "skills", skill.Name)
			if _, statErr := os.Stat(skillDir); statErr == nil {
				skipped++
				if dryRun {
					fmt.Fprintf(os.Stdout, "skip: %s (already installed)\n", skill.Name)
				}
				continue
			}

			if dryRun {
				fmt.Fprintf(os.Stdout, "install: %s (commit %s)\n", skill.Name, truncateCommit(skill.Commit))
				installed++
				continue
			}

			// Parse canonical source to build a ParsedSource.
			host, owner, repo, subPath, parseErr := core.ParseLockSource(skill.Source)
			if parseErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %s: %v\n", skill.Name, parseErr)
				errors++
				continue
			}

			cloneURL := fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)
			source := &core.ParsedSource{
				Type:     core.SourceTypeGit,
				Host:     host,
				Owner:    owner,
				Repo:     repo,
				CloneURL: cloneURL,
				SubPath:  subPath,
			}

			// Apply clone URL override if one exists.
			source.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

			result, installErr := installer.InstallFromSource(source, core.InstallOptions{
				TargetDir:    targetDir,
				SkillFilter:  skill.Name,
				Commit:       skill.Commit,
				TargetAgents: targetAgents,
			})
			if installErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %s: %v\n", skill.Name, installErr)
				errors++
				continue
			}

			for _, s := range result.InstalledSkills {
				fmt.Fprintf(os.Stdout, "Installed: %s\n", s.Name)
			}
			installed++
		}

		fmt.Fprintf(os.Stdout, "\nSynced: %d installed, %d skipped, %d errors\n", installed, skipped, errors)

		if errors > 0 {
			return fmt.Errorf("%d skill(s) failed to sync", errors)
		}
		return nil
	},
}

func truncateCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

func init() {
	syncCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	syncCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	syncCmd.Flags().String("agents", "", "Comma-separated agent names to also symlink into (e.g. cursor,claude-code)")
	rootCmd.AddCommand(syncCmd)
}
