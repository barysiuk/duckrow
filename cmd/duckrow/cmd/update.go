package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update [skill-name]",
	Short: "Update skill(s) to the available commit",
	Long: `Update one or all skills to the available commit and update the lock file.

Determines the available commit using this precedence:
  1. Registry commit (if the skill is in a configured registry)
  2. Latest commit on the lock entry's ref (branch/tag)
  3. Latest commit on the repository's default branch

If the available commit differs from the installed commit, the skill
is reinstalled with the new content and the lock file is updated.

Either a skill name or --all is required. Running without arguments
returns an error with a usage hint.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		all, _ := cmd.Flags().GetBool("all")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		agentsFlag, _ := cmd.Flags().GetString("agents")

		if len(args) == 0 && !all {
			return fmt.Errorf("specify a skill name or use --all\n\nUsage:\n  duckrow update <skill-name>\n  duckrow update --all")
		}

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

		targetDir, _ := cmd.Flags().GetString("dir")
		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
		}

		lf, err := core.ReadLockFile(targetDir)
		if err != nil {
			return fmt.Errorf("reading lock file: %w", err)
		}
		if lf == nil {
			return fmt.Errorf("no duckrow.lock.json found in %s", targetDir)
		}

		cfg, err := d.config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Build registry commit lookup.
		registryCommits := buildRegistryCommitLookup(d.config, cfg)

		// Determine which skills to check.
		var skillsToCheck *core.LockFile
		if all {
			skillsToCheck = lf
		} else {
			skillName := args[0]
			var found *core.LockedSkill
			for i := range lf.Skills {
				if lf.Skills[i].Name == skillName {
					found = &lf.Skills[i]
					break
				}
			}
			if found == nil {
				return fmt.Errorf("skill %q not found in lock file", skillName)
			}
			skillsToCheck = &core.LockFile{
				LockVersion: lf.LockVersion,
				Skills:      []core.LockedSkill{*found},
			}
		}

		// Check for updates.
		updates, err := core.CheckForUpdates(skillsToCheck, cfg.Settings.CloneURLOverrides, registryCommits)
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		installer := core.NewInstaller(d.agents)
		remover := core.NewRemover(d.agents)

		var updated, skipped, errors int

		for _, u := range updates {
			if !u.HasUpdate {
				skipped++
				if dryRun {
					fmt.Fprintf(os.Stdout, "skip: %s (up to date)\n", u.Name)
				}
				continue
			}

			if dryRun {
				fmt.Fprintf(os.Stdout, "update: %s %s -> %s\n", u.Name,
					truncateCommit(u.InstalledCommit), truncateCommit(u.AvailableCommit))
				updated++
				continue
			}

			// Find the lock entry for this skill (need the ref).
			var lockEntry *core.LockedSkill
			for i := range lf.Skills {
				if lf.Skills[i].Name == u.Name {
					lockEntry = &lf.Skills[i]
					break
				}
			}
			if lockEntry == nil {
				fmt.Fprintf(os.Stderr, "Error: %s: lock entry not found\n", u.Name)
				errors++
				continue
			}

			// Parse lock source.
			host, owner, repo, subPath, parseErr := core.ParseLockSource(u.Source)
			if parseErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %s: %v\n", u.Name, parseErr)
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
				Ref:      lockEntry.Ref,
			}

			// Apply clone URL override.
			source.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

			// Remove existing skill.
			_, removeErr := remover.Remove(u.Name, core.RemoveOptions{TargetDir: targetDir})
			if removeErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %s: removing: %v\n", u.Name, removeErr)
				errors++
				continue
			}

			// Reinstall at the available commit.
			// Only pass Commit if we have a registry commit (pinning).
			// Otherwise, the fresh clone already has the latest.
			installOpts := core.InstallOptions{
				TargetDir:    targetDir,
				SkillFilter:  u.Name,
				TargetAgents: targetAgents,
			}
			// If the available commit came from a registry, pin to it.
			if regCommit, ok := registryCommits[u.Source]; ok && regCommit == u.AvailableCommit {
				installOpts.Commit = u.AvailableCommit
			}

			result, installErr := installer.InstallFromSource(source, installOpts)
			if installErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %s: installing: %v\n", u.Name, installErr)
				errors++
				continue
			}

			// Update lock file with new commit.
			for _, s := range result.InstalledSkills {
				entry := core.LockedSkill{
					Name:   s.Name,
					Source: s.Source,
					Commit: s.Commit,
					Ref:    s.Ref,
				}
				if lockErr := core.AddOrUpdateLockEntry(targetDir, entry); lockErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
				}
				fmt.Fprintf(os.Stdout, "Updated: %s %s -> %s\n", s.Name,
					truncateCommit(u.InstalledCommit), truncateCommit(s.Commit))
			}
			updated++
		}

		fmt.Fprintf(os.Stdout, "\nUpdate: %d updated, %d up-to-date, %d errors\n", updated, skipped, errors)

		if errors > 0 {
			return fmt.Errorf("%d skill(s) failed to update", errors)
		}
		return nil
	},
}

func init() {
	updateCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	updateCmd.Flags().Bool("all", false, "Update all skills in the lock file")
	updateCmd.Flags().Bool("dry-run", false, "Show what would be updated without making changes")
	updateCmd.Flags().String("agents", "", "Comma-separated agent names to also symlink into (e.g. cursor,claude-code)")
	rootCmd.AddCommand(updateCmd)
}
