package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install [source]",
	Short: "Install skill(s) from a source",
	Long: `Install skill(s) from a git repository or GitHub shorthand,
or directly from a configured registry.

Sources can be:
  owner/repo              GitHub shorthand
  owner/repo@skill-name   Specific skill from a repo
  https://github.com/...  Full URL
  git@host:owner/repo.git SSH clone URL

To install a skill from a configured registry, use --skill without a source:
  duckrow install --skill go-review
  duckrow install --skill go-review --registry my-org

By default, skills are installed to .agents/skills/ which is read by
universal agents (OpenCode, Codex, Gemini CLI, GitHub Copilot).

To also create symlinks for non-universal agents, pass --agents:
  --agents cursor,claude-code   Symlink to Cursor and Claude Code`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		skillFilter, _ := cmd.Flags().GetString("skill")
		registryFilter, _ := cmd.Flags().GetString("registry")
		noLock, _ := cmd.Flags().GetBool("no-lock")

		// Validate flag combinations
		if registryFilter != "" && len(args) > 0 {
			return fmt.Errorf("--registry can only be used with --skill (without a source argument)")
		}
		if registryFilter != "" && skillFilter == "" {
			return fmt.Errorf("--registry requires --skill")
		}
		if len(args) == 0 && skillFilter == "" {
			return fmt.Errorf("provide a source argument or use --skill to install from a registry")
		}

		cfg, err := d.config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		var source *core.ParsedSource
		var registryCommit string

		if len(args) == 0 {
			// Registry-based install: resolve skill from registries
			rm := core.NewRegistryManager(d.config.RegistriesDir())
			skillInfo, findErr := rm.FindSkill(cfg.Registries, skillFilter, registryFilter)
			if findErr != nil {
				return findErr
			}

			source, err = core.ParseSource(skillInfo.Skill.Source)
			if err != nil {
				return fmt.Errorf("invalid skill source in registry: %w", err)
			}

			// Registry skills set IsInternal and use the skill name as filter
			// so only the target skill is installed from the source repo
			skillFilter = skillInfo.Skill.Name
			registryCommit = skillInfo.Skill.Commit
		} else {
			source, err = core.ParseSource(args[0])
			if err != nil {
				return fmt.Errorf("invalid source: %w", err)
			}
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
			IsInternal:      len(args) == 0, // registry installs enable internal
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

func init() {
	installCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	installCmd.Flags().StringP("skill", "s", "", "Install only a specific skill by name")
	installCmd.Flags().StringP("registry", "r", "", "Registry to search when using --skill without a source")
	installCmd.Flags().Bool("internal", false, "Include internal skills")
	installCmd.Flags().String("agents", "", "Comma-separated agent names to also symlink into (e.g. cursor,claude-code)")
	installCmd.Flags().Bool("no-lock", false, "Install without updating the lock file")
	rootCmd.AddCommand(installCmd)
}
