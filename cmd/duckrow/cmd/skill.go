package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage skills",
	Long:  `Install, uninstall, update, and inspect skills.`,
}

// ---------------------------------------------------------------------------
// skill install
// ---------------------------------------------------------------------------

var skillInstallCmd = &cobra.Command{
	Use:   "install <source-or-name>",
	Short: "Install skill(s) from a source or registry",
	Long: `Install skill(s) from a git URL or from a configured registry.

If the argument is a URL (https:// or git@), it is treated as a direct
source install. Otherwise it is treated as a registry skill name lookup.

Examples:
  duckrow skill install https://github.com/acme/skills.git
  duckrow skill install git@github.com:acme/skills.git
  duckrow skill install go-review
  duckrow skill install go-review --registry my-org`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		registryFilter, _ := cmd.Flags().GetString("registry")
		noLock, _ := cmd.Flags().GetBool("no-lock")

		cfg, err := d.config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		arg := args[0]

		// Reject local paths explicitly with a clear error message.
		if strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "../") ||
			strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "~/") {
			return fmt.Errorf("local path installs are not supported: %q\nUse a git URL (https://... or git@...) or a registry skill name", arg)
		}

		isURL := strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "git@")

		var source *core.ParsedSource
		var registryCommit string
		var skillFilter string

		if isURL {
			// Direct source install — full URL required.
			if registryFilter != "" {
				return fmt.Errorf("--registry cannot be used with a direct URL source")
			}
			source, err = core.ParseSource(arg)
			if err != nil {
				return fmt.Errorf("invalid source: %w", err)
			}
		} else {
			// Registry name lookup.
			rm := core.NewRegistryManager(d.config.RegistriesDir())
			skillInfo, findErr := rm.FindSkill(cfg.Registries, arg, registryFilter)
			if findErr != nil {
				return findErr
			}

			source, err = core.ParseSource(skillInfo.Skill.Source)
			if err != nil {
				return fmt.Errorf("invalid skill source in registry: %w", err)
			}

			skillFilter = skillInfo.Skill.Name
			registryCommit = skillInfo.Skill.Commit
		}

		// Apply clone URL override if one exists for this repo.
		source.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

		targetDir, _ := cmd.Flags().GetString("dir")
		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
		}

		internal, _ := cmd.Flags().GetBool("internal")
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

		installer := core.NewInstaller(d.agents)
		result, err := installer.InstallFromSource(source, core.InstallOptions{
			TargetDir:       targetDir,
			SkillFilter:     skillFilter,
			IncludeInternal: internal,
			IsInternal:      !isURL, // registry installs enable internal
			TargetAgents:    targetAgents,
			Commit:          registryCommit,
		})
		if err != nil {
			return err
		}

		// Read existing lock file for source-change warnings.
		var existingLock *core.LockFile
		if !noLock {
			existingLock, _ = core.ReadLockFile(targetDir)
		}

		for _, s := range result.InstalledSkills {
			fmt.Fprintf(os.Stdout, "Installed: %s\n", s.Name)
			fmt.Fprintf(os.Stdout, "  Path: %s\n", s.Path)
			if len(s.Agents) > 0 {
				fmt.Fprintf(os.Stdout, "  Agents: %s\n", joinStrings(s.Agents))
			}

			// Update lock file unless --no-lock is set.
			if !noLock && s.Commit != "" {
				// Warn if source changed for an existing skill.
				if existingLock != nil {
					for _, existing := range existingLock.Skills {
						if existing.Name == s.Name && existing.Source != s.Source {
							fmt.Fprintf(os.Stderr, "Warning: skill %q source changed from %q to %q\n",
								s.Name, existing.Source, s.Source)
						}
					}
				}

				entry := core.LockedSkill{
					Name:   s.Name,
					Source: s.Source,
					Commit: s.Commit,
					Ref:    s.Ref,
				}
				if lockErr := core.AddOrUpdateLockEntry(targetDir, entry); lockErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
				}
			} else if !noLock && s.Commit == "" {
				fmt.Fprintf(os.Stderr, "Warning: could not determine commit for %q; skill not pinned in lock file\n", s.Name)
			}
		}
		return nil
	},
}

// ---------------------------------------------------------------------------
// skill uninstall
// ---------------------------------------------------------------------------

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall [name]",
	Short: "Remove an installed skill",
	Long: `Remove a skill from the current directory (or specified directory).
Removes the canonical copy and all agent symlinks.

Use --all to remove all installed skills.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")

		if len(args) == 0 && !all {
			return fmt.Errorf("specify a skill name or use --all")
		}
		if len(args) > 0 && all {
			return fmt.Errorf("cannot specify a skill name with --all")
		}

		d, err := newDeps()
		if err != nil {
			return err
		}

		noLock, _ := cmd.Flags().GetBool("no-lock")

		targetDir, _ := cmd.Flags().GetString("dir")
		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
		}

		remover := core.NewRemover(d.agents)

		if all {
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

			// Write empty lock file unless --no-lock is set.
			if !noLock {
				emptyLock := &core.LockFile{
					Assets: []asset.LockedAsset{},
				}
				if lockErr := core.WriteLockFile(targetDir, emptyLock); lockErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
				}
			}

			return nil
		}

		// Single skill uninstall.
		result, err := remover.Remove(args[0], core.RemoveOptions{TargetDir: targetDir})
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Removed: %s\n", result.Name)
		if len(result.RemovedSymlinks) > 0 {
			fmt.Fprintf(os.Stdout, "  Cleaned up agent links: %s\n", joinStrings(result.RemovedSymlinks))
		}

		// Remove lock entry unless --no-lock is set.
		if !noLock {
			if lockErr := core.RemoveLockEntry(targetDir, args[0]); lockErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
			}
		}

		return nil
	},
}

// ---------------------------------------------------------------------------
// skill outdated
// ---------------------------------------------------------------------------

var skillOutdatedCmd = &cobra.Command{
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
			source := truncateSource(u.Source)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.Name, installed, available, source)
		}

		_ = w.Flush()
		return nil
	},
}

// ---------------------------------------------------------------------------
// skill update
// ---------------------------------------------------------------------------

var skillUpdateCmd = &cobra.Command{
	Use:   "update [skill-name]",
	Short: "Update skill(s) to the available commit",
	Long: `Update one or all skills to the available commit and update the lock file.

Determines the available commit using this precedence:
  1. Registry commit (if the skill is in a configured registry)
  2. Latest commit on the lock entry's ref (branch/tag)
  3. Latest commit on the repository's default branch

If the available commit differs from the installed commit, the skill
is reinstalled with the new content and the lock file is updated.

Either a skill name or --all is required.`,
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
			return fmt.Errorf("specify a skill name or use --all\n\nUsage:\n  duckrow skill update <skill-name>\n  duckrow skill update --all")
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
		rm := core.NewRegistryManager(d.config.RegistriesDir())
		rm.HydrateRegistryCommits(cfg.Registries, cfg.Settings.CloneURLOverrides)
		registryCommits := core.BuildRegistryCommitMap(cfg.Registries, rm)

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
			skillsToCheck = core.LockFileForSkills(lf.LockVersion, []core.LockedSkill{*found})
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
					core.TruncateCommit(u.InstalledCommit), core.TruncateCommit(u.AvailableCommit))
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
			psource := &core.ParsedSource{
				Type:     core.SourceTypeGit,
				Host:     host,
				Owner:    owner,
				Repo:     repo,
				CloneURL: cloneURL,
				SubPath:  subPath,
				Ref:      lockEntry.Ref,
			}

			// Apply clone URL override.
			psource.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

			// Remove existing skill.
			_, removeErr := remover.Remove(u.Name, core.RemoveOptions{TargetDir: targetDir})
			if removeErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %s: removing: %v\n", u.Name, removeErr)
				errors++
				continue
			}

			// Reinstall at the available commit.
			installOpts := core.InstallOptions{
				TargetDir:    targetDir,
				SkillFilter:  u.Name,
				TargetAgents: targetAgents,
			}
			// If the available commit came from a registry, pin to it.
			if regCommit, ok := registryCommits[u.Source]; ok && regCommit == u.AvailableCommit {
				installOpts.Commit = u.AvailableCommit
			}

			result, installErr := installer.InstallFromSource(psource, installOpts)
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
					core.TruncateCommit(u.InstalledCommit), core.TruncateCommit(s.Commit))
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

// ---------------------------------------------------------------------------
// skill sync
// ---------------------------------------------------------------------------

var skillSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install skills from lock file",
	Long: `Install all skills declared in duckrow.lock.json at their pinned commits.

Skills whose directories already exist are skipped. To force a reinstall,
delete the skill directory and rerun duckrow skill sync.

This command enforces the lock file and does not fetch upstream updates.
Use duckrow skill outdated and duckrow skill update to move forward.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := runSkillSync(cmd)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "\nSynced: %d installed, %d skipped, %d errors\n",
			result.installed, result.skipped, result.errors)

		if result.errors > 0 {
			return fmt.Errorf("%d skill(s) failed to sync", result.errors)
		}
		return nil
	},
}

// skillSyncResult holds the summary of a skill sync operation.
type skillSyncResult struct {
	installed int
	skipped   int
	errors    int
}

// runSkillSync contains the shared sync logic used by both
// `duckrow skill sync` and the root-level `duckrow sync`.
func runSkillSync(cmd *cobra.Command) (*skillSyncResult, error) {
	d, err := newDeps()
	if err != nil {
		return nil, err
	}

	targetDir, _ := cmd.Flags().GetString("dir")
	if targetDir == "" {
		targetDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current directory: %w", err)
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
			return nil, resolveErr
		}
		targetAgents = core.GetUniversalAgents(d.agents)
		targetAgents = append(targetAgents, specified...)
	}

	lf, err := core.ReadLockFile(targetDir)
	if err != nil {
		return nil, fmt.Errorf("reading lock file: %w", err)
	}
	if lf == nil {
		return nil, fmt.Errorf("no duckrow.lock.json found in %s", targetDir)
	}

	res := &skillSyncResult{}

	if len(lf.Skills) == 0 {
		return res, nil
	}

	cfg, err := d.config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	installer := core.NewInstaller(d.agents)

	for _, skill := range lf.Skills {
		// Check if skill directory already exists.
		skillDir := filepath.Join(targetDir, ".agents", "skills", skill.Name)
		if _, statErr := os.Stat(skillDir); statErr == nil {
			res.skipped++
			if dryRun {
				fmt.Fprintf(os.Stdout, "skip: %s (already installed)\n", skill.Name)
			}
			continue
		}

		if dryRun {
			fmt.Fprintf(os.Stdout, "install: %s (commit %s)\n", skill.Name, core.TruncateCommit(skill.Commit))
			res.installed++
			continue
		}

		// Parse canonical source to build a ParsedSource.
		host, owner, repo, subPath, parseErr := core.ParseLockSource(skill.Source)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %v\n", skill.Name, parseErr)
			res.errors++
			continue
		}

		cloneURL := fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)
		psource := &core.ParsedSource{
			Type:     core.SourceTypeGit,
			Host:     host,
			Owner:    owner,
			Repo:     repo,
			CloneURL: cloneURL,
			SubPath:  subPath,
		}

		// Apply clone URL override if one exists.
		psource.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

		result, installErr := installer.InstallFromSource(psource, core.InstallOptions{
			TargetDir:    targetDir,
			SkillFilter:  skill.Name,
			Commit:       skill.Commit,
			TargetAgents: targetAgents,
		})
		if installErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %v\n", skill.Name, installErr)
			res.errors++
			continue
		}

		for _, s := range result.InstalledSkills {
			fmt.Fprintf(os.Stdout, "Installed: %s\n", s.Name)
		}
		res.installed++
	}

	return res, nil
}

func init() {
	// skill install flags
	skillInstallCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	skillInstallCmd.Flags().StringP("registry", "r", "", "Registry to search (disambiguates duplicates)")
	skillInstallCmd.Flags().Bool("internal", false, "Include internal skills")
	skillInstallCmd.Flags().String("agents", "", "Comma-separated agent names to also symlink into (e.g. cursor,claude-code)")
	skillInstallCmd.Flags().Bool("no-lock", false, "Install without updating the lock file")

	// skill uninstall flags
	skillUninstallCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	skillUninstallCmd.Flags().Bool("no-lock", false, "Remove skill without updating the lock file")
	skillUninstallCmd.Flags().Bool("all", false, "Remove all installed skills")

	// skill outdated flags
	skillOutdatedCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	skillOutdatedCmd.Flags().Bool("json", false, "Output as JSON for scripting")

	// skill update flags
	skillUpdateCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	skillUpdateCmd.Flags().Bool("all", false, "Update all skills in the lock file")
	skillUpdateCmd.Flags().Bool("dry-run", false, "Show what would be updated without making changes")
	skillUpdateCmd.Flags().String("agents", "", "Comma-separated agent names to also symlink into (e.g. cursor,claude-code)")

	// skill sync flags
	skillSyncCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	skillSyncCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	skillSyncCmd.Flags().String("agents", "", "Comma-separated agent names to also symlink into (e.g. cursor,claude-code)")

	// Wire up subcommands.
	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillUninstallCmd)
	skillCmd.AddCommand(skillOutdatedCmd)
	skillCmd.AddCommand(skillUpdateCmd)
	skillCmd.AddCommand(skillSyncCmd)
	rootCmd.AddCommand(skillCmd)
}
