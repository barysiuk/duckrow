package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/barysiuk/duckrow/internal/core/system"
	"github.com/spf13/cobra"
)

// registerAssetCommands dynamically creates CLI subcommands for each
// registered asset kind. Called from root.go init().
func registerAssetCommands() {
	for _, kind := range asset.Kinds() {
		handler, _ := asset.Get(kind)
		cmd := buildAssetCommand(kind, handler)
		rootCmd.AddCommand(cmd)
	}
}

// buildAssetCommand creates a Cobra command tree for one asset kind:
//
//	duckrow <kind> install <source-or-name>
//	duckrow <kind> uninstall <name>
//	duckrow <kind> list
//	duckrow <kind> sync
//	duckrow <kind> outdated  (file-based kinds only)
//	duckrow <kind> update    (file-based kinds only)
func buildAssetCommand(kind asset.Kind, handler asset.Handler) *cobra.Command {
	name := string(kind)
	display := handler.DisplayName()
	lower := strings.ToLower(display)

	parent := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Manage %ss", lower),
		Long:  fmt.Sprintf("Install, uninstall, and manage %ss.", lower),
	}

	// --- install ---
	installCmd := &cobra.Command{
		Use:   "install <source-or-name>",
		Short: fmt.Sprintf("Install %s(s) from a source or registry", lower),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssetInstall(cmd, args, kind)
		},
	}
	installCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	installCmd.Flags().StringP("registry", "r", "", "Limit to a specific registry")
	addSystemsFlag(installCmd)
	installCmd.Flags().Bool("no-lock", false, "Skip lock file update")
	installCmd.Flags().Bool("force", false, "Overwrite existing")
	// Skill-specific flag
	if kind == asset.KindSkill {
		installCmd.Flags().Bool("internal", false, "Include internal skills")
	}
	parent.AddCommand(installCmd)

	// --- uninstall ---
	uninstallCmd := &cobra.Command{
		Use:   "uninstall [name]",
		Short: fmt.Sprintf("Remove an installed %s", lower),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssetUninstall(cmd, args, kind)
		},
	}
	uninstallCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	uninstallCmd.Flags().Bool("no-lock", false, "Remove without updating the lock file")
	uninstallCmd.Flags().Bool("all", false, fmt.Sprintf("Remove all installed %ss", lower))
	parent.AddCommand(uninstallCmd)

	// --- list ---
	listCmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List installed %ss", lower),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssetList(cmd, kind)
		},
	}
	listCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	listCmd.Flags().Bool("json", false, "Output as JSON")
	parent.AddCommand(listCmd)

	// --- sync ---
	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: fmt.Sprintf("Install %ss from lock file", lower),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAssetSync(cmd, kind)
		},
	}
	syncCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
	syncCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	syncCmd.Flags().Bool("force", false, "Overwrite existing entries")
	addSystemsFlag(syncCmd)
	parent.AddCommand(syncCmd)

	// --- outdated (file-based kinds only) ---
	if kind == asset.KindSkill {
		outdatedCmd := &cobra.Command{
			Use:   "outdated",
			Short: fmt.Sprintf("Show %ss with available updates", lower),
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runAssetOutdated(cmd, kind)
			},
		}
		outdatedCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
		outdatedCmd.Flags().Bool("json", false, "Output as JSON for scripting")
		parent.AddCommand(outdatedCmd)

		updateCmd := &cobra.Command{
			Use:   "update [name]",
			Short: fmt.Sprintf("Update %s(s) to the available commit", lower),
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return runAssetUpdate(cmd, args, kind)
			},
		}
		updateCmd.Flags().StringP("dir", "d", "", "Target directory (default: current directory)")
		updateCmd.Flags().Bool("all", false, fmt.Sprintf("Update all %ss in the lock file", lower))
		updateCmd.Flags().Bool("dry-run", false, "Show what would be updated without making changes")
		addSystemsFlag(updateCmd)
		parent.AddCommand(updateCmd)
	}

	return parent
}

// ---------------------------------------------------------------------------
// runAssetInstall — shared install handler for all asset kinds
// ---------------------------------------------------------------------------

func runAssetInstall(cmd *cobra.Command, args []string, kind asset.Kind) error {
	d, err := newDeps()
	if err != nil {
		return err
	}

	registryFilter, _ := cmd.Flags().GetString("registry")
	noLock, _ := cmd.Flags().GetBool("no-lock")
	force, _ := cmd.Flags().GetBool("force")

	cfg, err := d.config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	arg := args[0]

	// Reject local paths explicitly.
	if strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "../") ||
		strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "~/") {
		return fmt.Errorf("local path installs are not supported: %q\nUse a git URL (https://... or git@...) or a registry name", arg)
	}

	isURL := strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "git@")

	targetDir, err := resolveTargetDir(cmd)
	if err != nil {
		return err
	}

	targetSystems, err := resolveTargetSystems(cmd)
	if err != nil {
		return err
	}

	// Resolve additional systems: for skills, add universal systems to any
	// explicitly requested non-universal systems. MCP defaults to detected.
	if targetSystems != nil && kind == asset.KindSkill {
		// User specified --systems: add universal systems.
		targetSystems = append(system.Universal(), targetSystems...)
		targetSystems = deduplicateSystems(targetSystems)
	}

	orch := core.NewOrchestrator()

	switch kind {
	case asset.KindSkill:
		return installSkill(cmd, orch, cfg, arg, isURL, registryFilter, targetDir, targetSystems, noLock, force, d)
	case asset.KindMCP:
		return installMCP(orch, cfg, arg, registryFilter, targetDir, targetSystems, noLock, force, d)
	case asset.KindAgent:
		return installAgent(orch, cfg, arg, isURL, registryFilter, targetDir, targetSystems, noLock, force, d)
	default:
		return fmt.Errorf("install not implemented for kind %q", kind)
	}
}

// installSkill handles skill-specific install logic.
func installSkill(
	cmd *cobra.Command,
	orch *core.Orchestrator,
	cfg *core.Config,
	arg string,
	isURL bool,
	registryFilter string,
	targetDir string,
	targetSystems []system.System,
	noLock, force bool,
	d *deps,
) error {
	internal, _ := cmd.Flags().GetBool("internal")

	var source *core.ParsedSource
	var registryCommit string
	var skillFilter string
	var err error

	if isURL {
		if registryFilter != "" {
			return fmt.Errorf("--registry cannot be used with a direct URL source")
		}
		source, err = core.ParseSource(arg)
		if err != nil {
			return fmt.Errorf("invalid source: %w", err)
		}
	} else {
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

	source.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

	results, err := orch.InstallFromSource(source, asset.KindSkill, core.OrchestratorInstallOptions{
		TargetDir:       targetDir,
		TargetSystems:   targetSystems,
		IncludeInternal: internal,
		NameFilter:      skillFilter,
		Commit:          registryCommit,
		Force:           force,
	})
	if err != nil {
		return err
	}

	// Read existing lock for source-change warnings.
	var existingLock *core.LockFile
	if !noLock {
		existingLock, _ = core.ReadLockFile(targetDir)
	}

	for _, r := range results {
		fmt.Fprintf(os.Stdout, "Installed: %s\n", r.Asset.Name)
		if r.Asset.PreparedPath != "" {
			fmt.Fprintf(os.Stdout, "  Path: %s\n", r.Asset.PreparedPath)
		}
		if len(r.Systems) > 0 {
			fmt.Fprintf(os.Stdout, "  Systems: %s\n", joinStrings(r.Systems))
		}

		if !noLock && r.Commit != "" {
			src := r.Asset.Source
			if src == "" {
				src = core.NormalizeSource(source.Host, source.Owner, source.Repo, "")
			}

			// Warn if source changed.
			if existingLock != nil {
				for _, existing := range core.AssetsByKind(existingLock, asset.KindSkill) {
					if existing.Name == r.Asset.Name && existing.Source != src {
						fmt.Fprintf(os.Stderr, "Warning: skill %q source changed from %q to %q\n",
							r.Asset.Name, existing.Source, src)
					}
				}
			}

			entry := asset.LockedAsset{
				Kind:   asset.KindSkill,
				Name:   r.Asset.Name,
				Source: src,
				Commit: r.Commit,
				Ref:    r.Ref,
			}
			if lockErr := core.AddOrUpdateAsset(targetDir, entry); lockErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
			}
		} else if !noLock && r.Commit == "" {
			fmt.Fprintf(os.Stderr, "Warning: could not determine commit for %q; not pinned in lock file\n", r.Asset.Name)
		}
	}
	return nil
}

// installMCP handles MCP-specific install logic.
func installMCP(
	orch *core.Orchestrator,
	cfg *core.Config,
	name string,
	registryFilter string,
	targetDir string,
	targetSystems []system.System,
	noLock, force bool,
	d *deps,
) error {
	rm := core.NewRegistryManager(d.config.RegistriesDir())
	mcpInfo, findErr := rm.FindMCP(cfg.Registries, name, registryFilter)
	if findErr != nil {
		return findErr
	}

	// Resolve target systems for MCP.
	if targetSystems == nil {
		// Default: all MCP-capable systems detected in the folder.
		detected := system.DetectInFolder(targetDir)
		targetSystems = filterMCPCapable(detected)
		if len(targetSystems) == 0 {
			// Fall back to all MCP-capable systems.
			targetSystems = filterMCPCapable(system.All())
		}
	} else {
		targetSystems = filterMCPCapable(targetSystems)
		if len(targetSystems) == 0 {
			return fmt.Errorf("none of the specified systems support MCP configurations")
		}
	}

	fmt.Fprintf(os.Stdout, "Installing MCP %q from registry %q...\n\n", name, mcpInfo.RegistryName)

	// Build asset from MCP entry.
	meta, ok := mcpInfo.MCP.Meta.(asset.MCPMeta)
	if !ok {
		return fmt.Errorf("invalid MCP metadata")
	}
	a := asset.Asset{
		Kind:        asset.KindMCP,
		Name:        mcpInfo.MCP.Name,
		Description: mcpInfo.MCP.Description,
		Meta:        meta,
	}

	// Install into each target system.
	fmt.Fprintln(os.Stdout, "Wrote MCP config to:")
	for _, sys := range targetSystems {
		configPath := resolveMCPConfigPathFromSystem(sys, targetDir)

		err := sys.Install(a, targetDir, system.InstallOptions{Force: force})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				fmt.Fprintf(os.Stdout, "  ! %-24s %q already exists\n", configPath, name)
				continue
			}
			fmt.Fprintf(os.Stderr, "  x %-24s error: %s\n", configPath, err.Error())
			continue
		}

		fmt.Fprintf(os.Stdout, "  + %-24s (%s)\n", configPath, sys.DisplayName())
	}

	// Update lock file.
	if !noLock {
		requiredEnv := core.ExtractRequiredEnv(meta.Env)
		data := map[string]any{
			"registry":   mcpInfo.RegistryName,
			"configHash": core.ComputeConfigHash(meta),
		}
		if len(requiredEnv) > 0 {
			data["requiredEnv"] = requiredEnv
		}
		entry := asset.LockedAsset{
			Kind: asset.KindMCP,
			Name: name,
			Data: data,
		}
		if lockErr := core.AddOrUpdateAsset(targetDir, entry); lockErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
		} else {
			fmt.Fprintln(os.Stdout, "\nUpdated duckrow.lock.json")
		}

		if len(requiredEnv) > 0 {
			fmt.Fprintln(os.Stdout, "\n! The following environment variables are required:")
			for _, v := range requiredEnv {
				fmt.Fprintf(os.Stdout, "  %s  (used by %s)\n", v, name)
			}
			fmt.Fprintln(os.Stdout, "\n  Add values to .env.duckrow or ~/.duckrow/.env.duckrow")
		}
	}

	fmt.Fprintf(os.Stdout, "\nMCP %q installed successfully.\n", name)
	return nil
}

// ---------------------------------------------------------------------------
// runAssetUninstall — shared uninstall handler for all asset kinds
// ---------------------------------------------------------------------------

func runAssetUninstall(cmd *cobra.Command, args []string, kind asset.Kind) error {
	all, _ := cmd.Flags().GetBool("all")

	if len(args) == 0 && !all {
		return fmt.Errorf("specify a name or use --all")
	}
	if len(args) > 0 && all {
		return fmt.Errorf("cannot specify a name with --all")
	}

	noLock, _ := cmd.Flags().GetBool("no-lock")

	targetDir, err := resolveTargetDir(cmd)
	if err != nil {
		return err
	}

	orch := core.NewOrchestrator()

	switch kind {
	case asset.KindSkill:
		return uninstallSkill(orch, targetDir, args, all, noLock)
	case asset.KindMCP:
		return uninstallMCP(targetDir, args, all, noLock)
	case asset.KindAgent:
		return uninstallAgent(orch, targetDir, args, all, noLock)
	default:
		return fmt.Errorf("uninstall not implemented for kind %q", kind)
	}
}

func uninstallSkill(orch *core.Orchestrator, targetDir string, args []string, all, noLock bool) error {
	if all {
		// Scan to find all installed skills, then remove each.
		allInstalled, err := orch.ScanFolder(targetDir)
		if err != nil {
			return fmt.Errorf("scanning folder: %w", err)
		}
		skills := allInstalled[asset.KindSkill]
		if len(skills) == 0 {
			fmt.Fprintln(os.Stdout, "No skills installed.")
			return nil
		}

		for _, s := range skills {
			if err := orch.RemoveAsset(asset.KindSkill, s.Name, targetDir, nil); err != nil {
				return fmt.Errorf("removing %q: %w", s.Name, err)
			}
			fmt.Fprintf(os.Stdout, "Removed: %s\n", s.Name)
		}
		fmt.Fprintf(os.Stdout, "\nRemoved %d skill(s).\n", len(skills))

		if !noLock {
			emptyLock := &core.LockFile{Assets: []asset.LockedAsset{}}
			if lockErr := core.WriteLockFile(targetDir, emptyLock); lockErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
			}
		}
		return nil
	}

	// Single skill uninstall.
	name := args[0]
	if err := orch.RemoveAsset(asset.KindSkill, name, targetDir, nil); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Removed: %s\n", name)

	if !noLock {
		if lockErr := core.RemoveAssetEntry(targetDir, asset.KindSkill, name); lockErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
		}
	}
	return nil
}

func uninstallMCP(targetDir string, args []string, all, noLock bool) error {
	lf, err := core.ReadLockFile(targetDir)
	if err != nil {
		return fmt.Errorf("reading lock file: %w", err)
	}
	if lf == nil {
		return fmt.Errorf("no duckrow.lock.json found in %s", targetDir)
	}

	if all {
		lockedMCPs := core.AssetsByKind(lf, asset.KindMCP)
		if len(lockedMCPs) == 0 {
			fmt.Fprintln(os.Stdout, "No MCPs installed.")
			return nil
		}

		for _, m := range lockedMCPs {
			if err := removeMCPFromSystems(m.Name, nil, targetDir); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Removed: %s\n", m.Name)
		}
		fmt.Fprintf(os.Stdout, "\nRemoved %d MCP(s).\n", len(lockedMCPs))

		// Remove all MCP entries from lock file.
		if !noLock {
			for _, m := range lockedMCPs {
				if lockErr := core.RemoveAssetEntry(targetDir, asset.KindMCP, m.Name); lockErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
				}
			}
		}
		return nil
	}

	// Single MCP uninstall.
	name := args[0]

	lockedMCP := core.FindLockedAsset(lf, asset.KindMCP, name)
	if lockedMCP == nil {
		return fmt.Errorf("MCP %q not found in lock file", name)
	}

	fmt.Fprintf(os.Stdout, "Removing MCP %q...\n\n", name)

	if err := removeMCPFromSystems(name, nil, targetDir); err != nil {
		return err
	}

	if !noLock {
		if lockErr := core.RemoveAssetEntry(targetDir, asset.KindMCP, name); lockErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
		} else {
			fmt.Fprintln(os.Stdout, "\nUpdated duckrow.lock.json")
		}
	}

	fmt.Fprintf(os.Stdout, "\nMCP %q removed.\n", name)
	return nil
}

// removeMCPFromSystems removes an MCP entry from agent config files.
func removeMCPFromSystems(name string, agentNames []string, targetDir string) error {
	var targetSystems []system.System
	if len(agentNames) > 0 {
		var err error
		targetSystems, err = system.ByNames(agentNames)
		if err != nil {
			// Some systems may have been removed; fall back to all MCP-capable.
			targetSystems = filterMCPCapable(system.All())
		}
	} else {
		// No specific systems requested; remove from all MCP-capable systems.
		targetSystems = filterMCPCapable(system.All())
	}

	fmt.Fprintln(os.Stdout, "Removed from:")
	for _, sys := range targetSystems {
		if !sys.Supports(asset.KindMCP) {
			continue
		}
		configPath := resolveMCPConfigPathFromSystem(sys, targetDir)
		err := sys.Remove(asset.KindMCP, name, targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  x %-24s error: %s\n", configPath, err.Error())
			continue
		}
		fmt.Fprintf(os.Stdout, "  - %-24s (%s)\n", configPath, sys.DisplayName())
	}
	return nil
}

func lockedRequiredEnv(locked asset.LockedAsset) []string {
	if locked.Data == nil {
		return nil
	}
	if envs, ok := locked.Data["requiredEnv"]; ok {
		switch v := envs.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// runAssetList — shared list handler for all asset kinds
// ---------------------------------------------------------------------------

func runAssetList(cmd *cobra.Command, kind asset.Kind) error {
	targetDir, err := resolveTargetDir(cmd)
	if err != nil {
		return err
	}
	jsonOutput, _ := cmd.Flags().GetBool("json")

	orch := core.NewOrchestrator()
	allInstalled, err := orch.ScanFolder(targetDir)
	if err != nil {
		return fmt.Errorf("scanning folder: %w", err)
	}

	items := allInstalled[kind]

	if kind == asset.KindMCP {
		// MCPs are config-only; list from lock file.
		lf, _ := core.ReadLockFile(targetDir)
		lockedMCPs := core.AssetsByKind(lf, asset.KindMCP)
		if len(lockedMCPs) == 0 {
			fmt.Fprintln(os.Stdout, "No MCPs installed.")
			return nil
		}
		if jsonOutput {
			data, err := json.MarshalIndent(lockedMCPs, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			fmt.Fprintln(os.Stdout, string(data))
			return nil
		}
		for _, m := range lockedMCPs {
			fmt.Fprintf(os.Stdout, "%s\n", m.Name)
		}
		return nil
	}

	if kind == asset.KindAgent {
		// Agents are rendered per-system; scan each system to build system lists.
		return listAgents(targetDir, jsonOutput)
	}

	// File-based assets (skills).
	if len(items) == 0 {
		handler, _ := asset.Get(kind)
		fmt.Fprintf(os.Stdout, "No %ss installed.\n", strings.ToLower(handler.DisplayName()))
		return nil
	}

	if jsonOutput {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	for _, item := range items {
		fmt.Fprintf(os.Stdout, "%s\n", item.Name)
		if item.Description != "" {
			fmt.Fprintf(os.Stdout, "  %s\n", item.Description)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// runAssetSync — shared per-kind sync handler
// ---------------------------------------------------------------------------

// assetSyncResult holds the summary of an asset sync operation.
type assetSyncResult struct {
	installed   int
	skipped     int
	errors      int
	requiredEnv map[string][]string // envVar -> []mcpName (MCP-specific)
}

func runAssetSync(cmd *cobra.Command, kind asset.Kind) error {
	result, err := runAssetSyncInner(cmd, kind)
	if err != nil {
		return err
	}

	handler, _ := asset.Get(kind)
	display := handler.DisplayName()

	switch kind {
	case asset.KindMCP:
		fmt.Fprintf(os.Stdout, "\nMCPs: %d installed, %d skipped, %d errors\n",
			result.installed, result.skipped, result.errors)
		printRequiredEnvSummary(result.requiredEnv)
	case asset.KindAgent:
		fmt.Fprintf(os.Stdout, "\nAgents: %d installed, %d skipped, %d errors\n",
			result.installed, result.skipped, result.errors)
	default:
		fmt.Fprintf(os.Stdout, "\nSynced: %d installed, %d skipped, %d errors\n",
			result.installed, result.skipped, result.errors)
	}

	if result.errors > 0 {
		return fmt.Errorf("%d %s(s) failed to sync", result.errors, strings.ToLower(display))
	}
	return nil
}

func runAssetSyncInner(cmd *cobra.Command, kind asset.Kind) (*assetSyncResult, error) {
	d, err := newDeps()
	if err != nil {
		return nil, err
	}

	targetDir, err := resolveTargetDir(cmd)
	if err != nil {
		return nil, err
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")

	targetSystems, err := resolveTargetSystems(cmd)
	if err != nil {
		return nil, err
	}

	lf, err := core.ReadLockFile(targetDir)
	if err != nil {
		return nil, fmt.Errorf("reading lock file: %w", err)
	}
	if lf == nil {
		return nil, fmt.Errorf("no duckrow.lock.json found in %s", targetDir)
	}

	cfg, err := d.config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	switch kind {
	case asset.KindSkill:
		return syncSkills(lf, cfg, targetDir, targetSystems, dryRun, force)
	case asset.KindMCP:
		return syncMCPs(lf, cfg, targetDir, targetSystems, dryRun, force, d)
	case asset.KindAgent:
		return syncAgents(lf, cfg, targetDir, targetSystems, dryRun, force)
	default:
		return &assetSyncResult{}, nil
	}
}

func syncSkills(
	lf *core.LockFile,
	cfg *core.Config,
	targetDir string,
	targetSystems []system.System,
	dryRun, force bool,
) (*assetSyncResult, error) {
	res := &assetSyncResult{}

	lockedSkills := core.AssetsByKind(lf, asset.KindSkill)
	if len(lockedSkills) == 0 {
		return res, nil
	}

	orch := core.NewOrchestrator()

	for _, skill := range lockedSkills {
		// Check if skill directory already exists.
		skillDir := filepath.Join(targetDir, ".agents", "skills", skill.Name)
		if !force {
			if _, statErr := os.Stat(skillDir); statErr == nil {
				res.skipped++
				if dryRun {
					fmt.Fprintf(os.Stdout, "skip: %s (already installed)\n", skill.Name)
				}
				continue
			}
		}

		if dryRun {
			fmt.Fprintf(os.Stdout, "install: %s (commit %s)\n", skill.Name, core.TruncateCommit(skill.Commit))
			res.installed++
			continue
		}

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
		psource.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

		_, installErr := orch.InstallFromSource(psource, asset.KindSkill, core.OrchestratorInstallOptions{
			TargetDir:     targetDir,
			TargetSystems: targetSystems,
			NameFilter:    skill.Name,
			Commit:        skill.Commit,
		})
		if installErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %v\n", skill.Name, installErr)
			res.errors++
			continue
		}

		fmt.Fprintf(os.Stdout, "Installed: %s\n", skill.Name)
		res.installed++
	}

	return res, nil
}

func syncMCPs(
	lf *core.LockFile,
	cfg *core.Config,
	targetDir string,
	targetSystems []system.System,
	dryRun, force bool,
	d *deps,
) (*assetSyncResult, error) {
	result := &assetSyncResult{
		requiredEnv: make(map[string][]string),
	}

	lockedMCPs := core.AssetsByKind(lf, asset.KindMCP)
	if len(lockedMCPs) == 0 {
		return result, nil
	}

	rm := core.NewRegistryManager(d.config.RegistriesDir())

	for _, lockedMCP := range lockedMCPs {
		mcpInfo, findErr := rm.FindMCP(cfg.Registries, lockedMCP.Name, "")
		if findErr != nil {
			fmt.Fprintf(os.Stderr, "! MCP %q: registry %q not configured\n", lockedMCP.Name, lockedMCP.Data["registry"])
			fmt.Fprintf(os.Stderr, "  Run: duckrow registry add <url>\n")
			result.errors++
			continue
		}

		if dryRun {
			fmt.Fprintf(os.Stdout, "install: %s (from %s)\n", lockedMCP.Name, mcpInfo.RegistryName)
			result.installed++
			for _, v := range lockedRequiredEnv(lockedMCP) {
				result.requiredEnv[v] = append(result.requiredEnv[v], lockedMCP.Name)
			}
			continue
		}

		// Determine systems for this MCP.
		systems := targetSystems
		if len(systems) == 0 {
			// Fall back to all MCP-capable systems.
			systems = filterMCPCapable(system.All())
		}

		// Build asset and install.
		meta, ok := mcpInfo.MCP.Meta.(asset.MCPMeta)
		if !ok {
			result.errors++
			continue
		}
		a := asset.Asset{
			Kind:        asset.KindMCP,
			Name:        mcpInfo.MCP.Name,
			Description: mcpInfo.MCP.Description,
			Meta:        meta,
		}

		wrote := false
		for _, sys := range systems {
			if !sys.Supports(asset.KindMCP) {
				continue
			}
			err := sys.Install(a, targetDir, system.InstallOptions{Force: force})
			if err != nil {
				// Skip silently for already-exists.
				continue
			}
			wrote = true
		}

		if wrote {
			fmt.Fprintf(os.Stdout, "Installed: %s\n", lockedMCP.Name)
			result.installed++
		} else {
			result.skipped++
		}

		for _, v := range lockedRequiredEnv(lockedMCP) {
			result.requiredEnv[v] = append(result.requiredEnv[v], lockedMCP.Name)
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// runAssetOutdated — show assets with available updates (skill-specific)
// ---------------------------------------------------------------------------

func runAssetOutdated(cmd *cobra.Command, _ asset.Kind) error {
	d, err := newDeps()
	if err != nil {
		return err
	}

	targetDir, err := resolveTargetDir(cmd)
	if err != nil {
		return err
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")

	lf, err := core.ReadLockFile(targetDir)
	if err != nil {
		return fmt.Errorf("reading lock file: %w", err)
	}
	if lf == nil {
		return fmt.Errorf("no duckrow.lock.json found in %s", targetDir)
	}

	if len(core.AssetsByKind(lf, asset.KindSkill)) == 0 {
		fmt.Fprintln(os.Stdout, "Lock file has no skills.")
		return nil
	}

	cfg, err := d.config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

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
}

// ---------------------------------------------------------------------------
// runAssetUpdate — update assets to the available commit (skill-specific)
// ---------------------------------------------------------------------------

func runAssetUpdate(cmd *cobra.Command, args []string, _ asset.Kind) error {
	d, err := newDeps()
	if err != nil {
		return err
	}

	all, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if len(args) == 0 && !all {
		return fmt.Errorf("specify a skill name or use --all\n\nUsage:\n  duckrow skill update <skill-name>\n  duckrow skill update --all")
	}

	targetSystems, err := resolveTargetSystems(cmd)
	if err != nil {
		return err
	}

	targetDir, err := resolveTargetDir(cmd)
	if err != nil {
		return err
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

	rm := core.NewRegistryManager(d.config.RegistriesDir())
	rm.HydrateRegistryCommits(cfg.Registries, cfg.Settings.CloneURLOverrides)
	registryCommits := core.BuildRegistryCommitMap(cfg.Registries, rm)

	// Determine which skills to check.
	var skillsToCheck *core.LockFile
	if all {
		skillsToCheck = lf
	} else {
		skillName := args[0]
		found := core.FindLockedAsset(lf, asset.KindSkill, skillName)
		if found == nil {
			return fmt.Errorf("skill %q not found in lock file", skillName)
		}
		skillsToCheck = &core.LockFile{Assets: []asset.LockedAsset{*found}}
	}

	updates, err := core.CheckForUpdates(skillsToCheck, cfg.Settings.CloneURLOverrides, registryCommits)
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	orch := core.NewOrchestrator()
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

		// Find the lock entry for ref.
		lockEntry := core.FindLockedAsset(lf, asset.KindSkill, u.Name)
		if lockEntry == nil {
			fmt.Fprintf(os.Stderr, "Error: %s: lock entry not found\n", u.Name)
			errors++
			continue
		}

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
		psource.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

		// Remove existing.
		if err := orch.RemoveAsset(asset.KindSkill, u.Name, targetDir, nil); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: removing: %v\n", u.Name, err)
			errors++
			continue
		}

		// Reinstall at available commit.
		installOpts := core.OrchestratorInstallOptions{
			TargetDir:     targetDir,
			TargetSystems: targetSystems,
			NameFilter:    u.Name,
		}
		if regCommit, ok := registryCommits[u.Source]; ok && regCommit == u.AvailableCommit {
			installOpts.Commit = u.AvailableCommit
		}

		results, installErr := orch.InstallFromSource(psource, asset.KindSkill, installOpts)
		if installErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: installing: %v\n", u.Name, installErr)
			errors++
			continue
		}

		for _, r := range results {
			src := r.Asset.Source
			if src == "" {
				src = core.NormalizeSource(psource.Host, psource.Owner, psource.Repo, "")
			}
			entry := asset.LockedAsset{
				Kind:   asset.KindSkill,
				Name:   r.Asset.Name,
				Source: src,
				Commit: r.Commit,
				Ref:    r.Ref,
			}
			if lockErr := core.AddOrUpdateAsset(targetDir, entry); lockErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
			}
			fmt.Fprintf(os.Stdout, "Updated: %s %s -> %s\n", r.Asset.Name,
				core.TruncateCommit(u.InstalledCommit), core.TruncateCommit(r.Commit))
		}
		updated++
	}

	fmt.Fprintf(os.Stdout, "\nUpdate: %d updated, %d up-to-date, %d errors\n", updated, skipped, errors)

	if errors > 0 {
		return fmt.Errorf("%d skill(s) failed to update", errors)
	}
	return nil
}

// ---------------------------------------------------------------------------
// printRequiredEnvSummary — shared MCP env warning
// ---------------------------------------------------------------------------

func printRequiredEnvSummary(envMap map[string][]string) {
	if len(envMap) == 0 {
		return
	}

	var vars []string
	for v := range envMap {
		vars = append(vars, v)
	}
	sort.Strings(vars)

	fmt.Fprintln(os.Stdout, "\n! The following environment variables are required:")
	for _, v := range vars {
		mcps := envMap[v]
		fmt.Fprintf(os.Stdout, "  %s  (used by %s)\n", v, strings.Join(mcps, ", "))
	}
	fmt.Fprintln(os.Stdout, "\n  Add values to .env.duckrow or ~/.duckrow/.env.duckrow")
}

// ---------------------------------------------------------------------------
// Agent install / uninstall / list / sync
// ---------------------------------------------------------------------------

// installAgent handles agent-specific install logic.
// Agents can be installed from a direct git URL or by name from a registry.
func installAgent(
	orch *core.Orchestrator,
	cfg *core.Config,
	arg string,
	isURL bool,
	registryFilter string,
	targetDir string,
	targetSystems []system.System,
	noLock, force bool,
	d *deps,
) error {
	var source *core.ParsedSource
	var registryCommit string
	var agentFilter string
	var registryName string
	var err error

	if isURL {
		if registryFilter != "" {
			return fmt.Errorf("--registry cannot be used with a direct URL source")
		}
		source, err = core.ParseSource(arg)
		if err != nil {
			return fmt.Errorf("invalid source: %w", err)
		}
	} else {
		rm := core.NewRegistryManager(d.config.RegistriesDir())
		entry, regName, findErr := rm.FindAsset(cfg.Registries, asset.KindAgent, arg)
		if findErr != nil {
			return findErr
		}
		source, err = core.ParseSource(entry.Source)
		if err != nil {
			return fmt.Errorf("invalid agent source in registry: %w", err)
		}
		agentFilter = entry.Name
		registryCommit = entry.Commit
		registryName = regName
	}

	source.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

	// Resolve target systems for agents.
	if targetSystems == nil {
		// Default: all agent-capable systems detected in the folder.
		detected := system.DetectInFolder(targetDir)
		targetSystems = filterAgentCapable(detected)
		if len(targetSystems) == 0 {
			// Fall back to all agent-capable systems.
			targetSystems = filterAgentCapable(system.All())
		}
	} else {
		targetSystems = filterAgentCapable(targetSystems)
		if len(targetSystems) == 0 {
			return fmt.Errorf("none of the specified systems support agents")
		}
	}

	if registryName != "" {
		fmt.Fprintf(os.Stdout, "Installing agent %q from registry %q...\n\n", arg, registryName)
	}

	results, err := orch.InstallFromSource(source, asset.KindAgent, core.OrchestratorInstallOptions{
		TargetDir:     targetDir,
		TargetSystems: targetSystems,
		NameFilter:    agentFilter,
		Commit:        registryCommit,
		Force:         force,
	})
	if err != nil {
		return err
	}

	// Read existing lock for source-change warnings.
	var existingLock *core.LockFile
	if !noLock {
		existingLock, _ = core.ReadLockFile(targetDir)
	}

	fmt.Fprintln(os.Stdout, "Wrote agent files to:")
	for _, r := range results {
		for _, sysName := range r.Systems {
			sys, ok := system.ByName(sysName)
			if !ok {
				continue
			}
			agentDir := sys.AssetDir(asset.KindAgent, targetDir)
			relPath := filepath.Join(agentDir, r.Asset.Name+".md")
			fmt.Fprintf(os.Stdout, "  + %-40s (%s)\n", relPath, sys.DisplayName())
		}

		if !noLock && r.Commit != "" {
			src := r.Asset.Source
			if src == "" {
				src = core.NormalizeSource(source.Host, source.Owner, source.Repo, "")
			}

			// Warn if source changed.
			if existingLock != nil {
				for _, existing := range core.AssetsByKind(existingLock, asset.KindAgent) {
					if existing.Name == r.Asset.Name && existing.Source != src {
						fmt.Fprintf(os.Stderr, "Warning: agent %q source changed from %q to %q\n",
							r.Asset.Name, existing.Source, src)
					}
				}
			}

			entry := asset.LockedAsset{
				Kind:   asset.KindAgent,
				Name:   r.Asset.Name,
				Source: src,
				Commit: r.Commit,
				Ref:    r.Ref,
			}
			if lockErr := core.AddOrUpdateAsset(targetDir, entry); lockErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
			} else {
				fmt.Fprintln(os.Stdout, "\nUpdated duckrow.lock.json")
			}
		} else if !noLock && r.Commit == "" {
			fmt.Fprintf(os.Stderr, "Warning: could not determine commit for %q; not pinned in lock file\n", r.Asset.Name)
		}
	}

	if len(results) == 1 {
		fmt.Fprintf(os.Stdout, "\nAgent %q installed successfully.\n", results[0].Asset.Name)
	}
	return nil
}

// uninstallAgent handles agent-specific uninstall logic.
func uninstallAgent(orch *core.Orchestrator, targetDir string, args []string, all, noLock bool) error {
	if all {
		// Scan to find all installed agents, then remove each.
		allInstalled, err := orch.ScanFolder(targetDir)
		if err != nil {
			return fmt.Errorf("scanning folder: %w", err)
		}
		agents := allInstalled[asset.KindAgent]
		if len(agents) == 0 {
			fmt.Fprintln(os.Stdout, "No agents installed.")
			return nil
		}

		// Deduplicate by name (agents appear per-system).
		seen := make(map[string]bool)
		var uniqueNames []string
		for _, a := range agents {
			if !seen[a.Name] {
				seen[a.Name] = true
				uniqueNames = append(uniqueNames, a.Name)
			}
		}

		for _, name := range uniqueNames {
			if err := orch.RemoveAsset(asset.KindAgent, name, targetDir, nil); err != nil {
				return fmt.Errorf("removing %q: %w", name, err)
			}
			fmt.Fprintf(os.Stdout, "Removed: %s\n", name)
		}
		fmt.Fprintf(os.Stdout, "\nRemoved %d agent(s).\n", len(uniqueNames))

		if !noLock {
			for _, name := range uniqueNames {
				if lockErr := core.RemoveAssetEntry(targetDir, asset.KindAgent, name); lockErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
				}
			}
		}
		return nil
	}

	// Single agent uninstall.
	name := args[0]

	// Verify the agent exists in at least one system before removing.
	filename := name + ".md"
	found := false
	for _, sys := range system.Supporting(asset.KindAgent) {
		agentDir := sys.AssetDir(asset.KindAgent, targetDir)
		if agentDir == "" {
			continue
		}
		agentPath := filepath.Join(agentDir, filename)
		if _, statErr := os.Stat(agentPath); statErr == nil {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("agent %q not found in %s", name, targetDir)
	}

	fmt.Fprintf(os.Stdout, "Removing agent %q...\n\n", name)

	if err := orch.RemoveAsset(asset.KindAgent, name, targetDir, nil); err != nil {
		return err
	}

	// Show which systems the agent was removed from.
	fmt.Fprintln(os.Stdout, "Removed from:")
	for _, sys := range system.Supporting(asset.KindAgent) {
		agentDir := sys.AssetDir(asset.KindAgent, targetDir)
		relPath := filepath.Join(agentDir, name+".md")
		fmt.Fprintf(os.Stdout, "  - %-40s (%s)\n", relPath, sys.DisplayName())
	}

	if !noLock {
		if lockErr := core.RemoveAssetEntry(targetDir, asset.KindAgent, name); lockErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update lock file: %v\n", lockErr)
		} else {
			fmt.Fprintln(os.Stdout, "\nUpdated duckrow.lock.json")
		}
	}

	fmt.Fprintf(os.Stdout, "\nAgent %q removed.\n", name)
	return nil
}

// listAgents lists installed agents with their system associations.
func listAgents(targetDir string, jsonOutput bool) error {
	// Scan each agent-capable system individually to build system lists.
	type agentInfo struct {
		Name        string   `json:"name"`
		Description string   `json:"description,omitempty"`
		Systems     []string `json:"systems"`
	}

	agentMap := make(map[string]*agentInfo) // name -> info
	var order []string

	for _, sys := range system.Supporting(asset.KindAgent) {
		installed, err := sys.Scan(asset.KindAgent, targetDir)
		if err != nil {
			continue
		}
		for _, a := range installed {
			info, ok := agentMap[a.Name]
			if !ok {
				info = &agentInfo{
					Name:        a.Name,
					Description: a.Description,
				}
				agentMap[a.Name] = info
				order = append(order, a.Name)
			}
			info.Systems = append(info.Systems, sys.DisplayName())
		}
	}

	if len(agentMap) == 0 {
		fmt.Fprintln(os.Stdout, "No agents installed.")
		return nil
	}

	// Build sorted list.
	agents := make([]agentInfo, 0, len(order))
	for _, name := range order {
		agents = append(agents, *agentMap[name])
	}

	if jsonOutput {
		data, err := json.MarshalIndent(agents, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}

	for _, a := range agents {
		fmt.Fprintf(os.Stdout, "%-20s %-35s [%s]\n", a.Name, a.Description, joinStrings(a.Systems))
	}
	return nil
}

// syncAgents restores agent files from the lock file.
func syncAgents(
	lf *core.LockFile,
	cfg *core.Config,
	targetDir string,
	targetSystems []system.System,
	dryRun, force bool,
) (*assetSyncResult, error) {
	res := &assetSyncResult{}

	lockedAgents := core.AssetsByKind(lf, asset.KindAgent)
	if len(lockedAgents) == 0 {
		return res, nil
	}

	orch := core.NewOrchestrator()

	// Resolve target systems for agents.
	// Unlike skills, agents don't have a canonical location — they're rendered
	// per-system. During sync we always target all agent-capable systems so
	// that files are restored for every system, regardless of which system
	// directories currently exist on disk.
	if targetSystems == nil {
		targetSystems = filterAgentCapable(system.All())
	} else {
		targetSystems = filterAgentCapable(targetSystems)
	}

	for _, agent := range lockedAgents {
		// Check if agent file already exists in any target system.
		if !force {
			filename := agent.Name + ".md"
			exists := false
			for _, sys := range targetSystems {
				agentDir := sys.AssetDir(asset.KindAgent, targetDir)
				if agentDir == "" {
					continue
				}
				agentPath := filepath.Join(agentDir, filename)
				if _, statErr := os.Stat(agentPath); statErr == nil {
					exists = true
					break
				}
			}
			if exists {
				res.skipped++
				if dryRun {
					fmt.Fprintf(os.Stdout, "skip: %s (already installed)\n", agent.Name)
				}
				continue
			}
		}

		if dryRun {
			fmt.Fprintf(os.Stdout, "install: %s (commit %s)\n", agent.Name, core.TruncateCommit(agent.Commit))
			res.installed++
			continue
		}

		host, owner, repo, subPath, parseErr := core.ParseLockSource(agent.Source)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %v\n", agent.Name, parseErr)
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
		psource.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)

		_, installErr := orch.InstallFromSource(psource, asset.KindAgent, core.OrchestratorInstallOptions{
			TargetDir:     targetDir,
			TargetSystems: targetSystems,
			NameFilter:    agent.Name,
			Commit:        agent.Commit,
		})
		if installErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %s: %v\n", agent.Name, installErr)
			res.errors++
			continue
		}

		fmt.Fprintf(os.Stdout, "Installed: %s\n", agent.Name)
		res.installed++
	}

	return res, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// filterAgentCapable returns only systems that support agents.
func filterAgentCapable(systems []system.System) []system.System {
	var result []system.System
	for _, s := range systems {
		if s.Supports(asset.KindAgent) {
			result = append(result, s)
		}
	}
	return result
}

// filterMCPCapable returns only systems that support MCP.
func filterMCPCapable(systems []system.System) []system.System {
	var result []system.System
	for _, s := range systems {
		if s.Supports(asset.KindMCP) {
			result = append(result, s)
		}
	}
	return result
}

// deduplicateSystems removes duplicate systems by name.
func deduplicateSystems(systems []system.System) []system.System {
	seen := make(map[string]bool)
	var result []system.System
	for _, s := range systems {
		if !seen[s.Name()] {
			result = append(result, s)
			seen[s.Name()] = true
		}
	}
	return result
}

// resolveMCPConfigPathFromSystem returns the project-relative MCP config path
// for a system, checking the alternative path first.
func resolveMCPConfigPathFromSystem(sys system.System, projectDir string) string {
	type configPathResolver interface {
		ResolveMCPConfigPathRel(projectDir string) string
	}
	if r, ok := sys.(configPathResolver); ok {
		return r.ResolveMCPConfigPathRel(projectDir)
	}
	return ""
}
