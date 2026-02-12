package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <source>",
	Short: "Install skill(s) from a source",
	Long: `Install skill(s) from a git repository, GitHub shorthand, or local path.

Sources can be:
  owner/repo              GitHub shorthand
  owner/repo@skill-name   Specific skill from a repo
  ./local/path            Local directory
  https://github.com/...  Full URL
  git@host:owner/repo.git SSH clone URL

By default, skills are installed to .agents/skills/ which is read by
universal agents (OpenCode, Codex, Gemini CLI, GitHub Copilot).

To also create symlinks for non-universal agents, pass --agents:
  --agents cursor,claude-code   Symlink to Cursor and Claude Code`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := newDeps()
		if err != nil {
			return err
		}

		source, err := core.ParseSource(args[0])
		if err != nil {
			return fmt.Errorf("invalid source: %w", err)
		}

		// Apply clone URL override if one exists for this repo.
		cfg, cfgErr := d.config.Load()
		if cfgErr == nil {
			source.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)
		}

		targetDir, _ := cmd.Flags().GetString("dir")
		if targetDir == "" {
			targetDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
		}

		skillFilter, _ := cmd.Flags().GetString("skill")
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
			TargetAgents:    targetAgents,
		})
		if err != nil {
			return err
		}

		for _, s := range result.InstalledSkills {
			fmt.Fprintf(os.Stdout, "Installed: %s\n", s.Name)
			fmt.Fprintf(os.Stdout, "  Path: %s\n", s.Path)
			if len(s.Agents) > 0 {
				fmt.Fprintf(os.Stdout, "  Agents: %s\n", joinStrings(s.Agents))
			}
		}
		return nil
	},
}

func init() {
	installCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	installCmd.Flags().StringP("skill", "s", "", "Install only a specific skill by name")
	installCmd.Flags().Bool("internal", false, "Include internal skills")
	installCmd.Flags().String("agents", "", "Comma-separated agent names to also symlink into (e.g. cursor,claude-code)")
	rootCmd.AddCommand(installCmd)
}
