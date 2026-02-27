package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/barysiuk/duckrow/internal/core/system"
)

// folderModel is the main folder view showing installed skills and MCPs
// for the active folder, organized into tabs.
type folderModel struct {
	width  int
	height int

	// Tabs.
	activeKind asset.Kind
	keyOrder   []asset.Kind
	tabs       tabsModel

	// Bubbles lists — one per kind.
	lists map[asset.Kind]*list.Model

	// Data (pushed from App).
	status     *core.FolderStatus
	isTracked  bool
	regAssets  []core.RegistryAssetInfo // All registry assets (unified)
	availCount int                      // Number of registry items NOT installed

	// Update info: skill name -> update info.
	updateInfo  map[string]core.UpdateInfo
	updateCount int // Number of skills with updates available

	// MCP data from lock file.
	mcps []assetItem
}

func newFolderModel() folderModel {
	kinds := asset.Kinds()
	if len(kinds) == 0 {
		kinds = []asset.Kind{asset.KindSkill, asset.KindMCP}
	}

	labels := make([]tabDef, 0, len(kinds))
	lists := make(map[asset.Kind]*list.Model, len(kinds))
	for _, kind := range kinds {
		handler, _ := asset.Get(kind)
		label := string(kind)
		if handler != nil {
			label = handler.DisplayName() + "s"
		}
		labels = append(labels, tabDef{label: label})
		l := newFolderList()
		lists[kind] = &l
	}

	activeKind := kinds[0]
	return folderModel{
		activeKind: activeKind,
		keyOrder:   kinds,
		tabs:       newTabsModel(labels),
		lists:      lists,
	}
}

// newFolderList creates a bubbles list configured for the folder view.
func newFolderList() list.Model {
	d := newSkillDelegate()
	l := list.New(nil, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowPagination(false)

	// Override AcceptWhileFiltering to remove arrow keys — pressing up/down
	// while typing a filter should not silently accept the filter and hide
	// the search input. The user must press Enter to apply the filter.
	l.KeyMap.AcceptWhileFiltering.SetKeys("enter", "tab", "shift+tab")

	return l
}

func (m folderModel) setSize(width, height int) folderModel {
	m.width = width
	m.height = height
	// Actual list height is calculated dynamically in view().
	for _, l := range m.lists {
		l.SetSize(width, max(1, height))
	}
	return m
}

func (m folderModel) setData(status *core.FolderStatus, isTracked bool, regAssets []core.RegistryAssetInfo, updateInfo map[string]core.UpdateInfo, mcps []assetItem) folderModel {
	m.status = status
	m.isTracked = isTracked
	m.regAssets = regAssets
	m.availCount = m.countAvailable()
	m.updateInfo = updateInfo
	m.mcps = mcps

	// Count skills with updates.
	m.updateCount = 0
	for _, ui := range updateInfo {
		if ui.HasUpdate {
			m.updateCount++
		}
	}

	for _, kind := range m.keyOrder {
		list := m.lists[kind]
		if list == nil {
			continue
		}
		switch kind {
		case asset.KindMCP:
			list.SetItems(lockedAssetsToItems(kind, lockedFromAssetItems(mcps), descLookupFromAssetItems(mcps)))
		default:
			if status != nil {
				list.SetItems(installedAssetsToItems(kind, status.Assets[kind], updateInfo))
			} else {
				list.SetItems(nil)
			}
		}
	}

	// Update tab labels with counts.
	m.tabs = m.updateTabLabels()

	return m
}

// updateTabLabels builds tab labels with item counts and update indicators.
func (m folderModel) updateTabLabels() tabsModel {
	defs := make([]tabDef, 0, len(m.keyOrder))
	for _, kind := range m.keyOrder {
		handler, _ := asset.Get(kind)
		label := string(kind)
		if handler != nil {
			label = handler.DisplayName() + "s"
		}
		count := 0
		switch kind {
		case asset.KindMCP:
			count = len(m.mcps)
		default:
			if m.status != nil {
				count = len(m.status.Assets[kind])
			}
		}
		def := tabDef{label: fmt.Sprintf("%s (%d)", label, count)}
		if kind == asset.KindSkill && m.updateCount > 0 {
			def.extra = fmt.Sprintf(" ↓%d", m.updateCount)
		}
		defs = append(defs, def)
	}

	return m.tabs.setTabs(defs)
}

// activeList returns a pointer to the currently active list model.
func (m *folderModel) activeList() *list.Model {
	if list := m.lists[m.activeKind]; list != nil {
		return list
	}
	return nil
}

// isFiltering returns true if the active list is in filter mode.
func (m folderModel) isFiltering() bool {
	if list := m.lists[m.activeKind]; list != nil {
		return list.SettingFilter()
	}
	return false
}

func (m folderModel) update(msg tea.Msg, app *App) (folderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tabActiveMsg:
		idx := int(msg)
		if idx >= 0 && idx < len(m.keyOrder) {
			m.activeKind = m.keyOrder[idx]
		}
		return m, nil

	case tea.KeyMsg:
		// Try tab switching first (blocked during filter mode).
		tabs, cmd, consumed := m.tabs.update(msg, m.isFiltering())
		m.tabs = tabs
		if consumed {
			return m, cmd
		}

		// Don't intercept action keys while filtering.
		if m.isFiltering() {
			break
		}

		switch {
		case key.Matches(msg, keys.Delete):
			switch m.activeKind {
			case asset.KindMCP:
				return m, m.removeSelectedMCP(app)
			case asset.KindSkill:
				return m, m.removeSelectedSkill(app)
			default:
				return m, nil
			}

		case key.Matches(msg, keys.Update):
			if m.activeKind == asset.KindSkill {
				return m, m.updateSelectedSkill(app)
			}
			return m, nil

		case key.Matches(msg, keys.UpdateAll):
			return m, m.updateAllSkills(app)

		case key.Matches(msg, keys.Refresh):
			return m, m.refreshWithRegistries(app)

		case key.Matches(msg, keys.Enter):
			if m.activeKind == asset.KindSkill {
				return m, m.openPreview(app)
			}
			return m, nil
		}
	}

	// Forward to the active list.
	var cmd tea.Cmd
	al := m.activeList()
	if al == nil {
		return m, nil
	}
	*al, cmd = al.Update(msg)
	return m, cmd
}

// openPreview reads the selected skill's SKILL.md and triggers the preview overlay.
func (m folderModel) openPreview(app *App) tea.Cmd {
	list := m.lists[asset.KindSkill]
	if list == nil {
		return nil
	}
	item := list.SelectedItem()
	if item == nil {
		return nil
	}
	si, ok := item.(assetItem)
	if !ok {
		return nil
	}

	skillMdPath := filepath.Join(si.path, "SKILL.md")
	content, err := readSkillMdBody(skillMdPath)
	if err != nil {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("reading SKILL.md: %w", err)}
		}
	}

	title := si.name

	return func() tea.Msg {
		return openPreviewMsg{
			title:   title,
			content: content,
		}
	}
}

func (m folderModel) view() string {
	if m.status == nil {
		return mutedStyle.Render("  Loading...")
	}

	if m.status.Error != nil {
		return errorStyle.Render(fmt.Sprintf("  Error scanning: %v", m.status.Error))
	}

	// --- Render-then-measure: render fixed chrome first, measure, size list. ---

	// 1. Render fixed chrome parts.
	tabBar := m.tabs.view() + "\n"

	// Build footer: optional update prefix + registry status.
	var parts []string
	if m.updateCount > 0 {
		parts = append(parts,
			warningStyle.Render(fmt.Sprintf("%d updates available", m.updateCount))+
				"  "+mutedStyle.Render("[u] Update"))
	}

	if m.availCount > 0 {
		parts = append(parts,
			mutedStyle.Render(fmt.Sprintf("%d available from registries", m.availCount))+
				"  "+mutedStyle.Render("[i] Install"))
	} else if len(m.regAssets) == 0 {
		parts = append(parts,
			mutedStyle.Render("No registries configured.")+
				"  "+mutedStyle.Render("[s] Settings to add"))
	} else {
		parts = append(parts,
			mutedStyle.Render("All registry items installed"))
	}

	footer := "  " + strings.Join(parts, "  |  ")
	footerBlock := "\n\n" + footer

	// 2. Measure chrome height.
	chromeH := lipgloss.Height(tabBar) + lipgloss.Height(footerBlock)

	// 3. Size and render the active list.
	listH := max(1, m.height-chromeH)

	var listView string
	list := m.activeList()
	if list == nil {
		listView = "\n" + mutedStyle.Render("  No items installed")
	} else {
		if len(list.Items()) == 0 {
			emptyLabel := "items"
			handler, _ := asset.Get(m.activeKind)
			if handler != nil {
				emptyLabel = strings.ToLower(handler.DisplayName()) + "s"
			}
			listView = "\n" + mutedStyle.Render("  No "+emptyLabel+" installed")
		} else {
			list.SetSize(m.width, listH)
			listView = list.View()
		}
	}

	// 4. Assemble: pin footer to the bottom of the content area.
	//    The list fills the middle; compute any gap between list + footer and total height.
	rendered := tabBar + listView + footerBlock
	renderedH := lipgloss.Height(rendered)
	if renderedH < m.height {
		// Insert blank lines between list and footer so footer sits at the bottom.
		gap := strings.Repeat("\n", m.height-renderedH)
		rendered = tabBar + listView + gap + footerBlock
	}

	return rendered
}

func (m folderModel) removeSelectedSkill(app *App) tea.Cmd {
	list := m.lists[asset.KindSkill]
	if list == nil {
		return nil
	}
	item := list.SelectedItem()
	if item == nil || m.status == nil {
		return nil
	}

	si, ok := item.(assetItem)
	if !ok {
		return nil
	}

	skill := *si.installed
	folderPath := app.activeFolder

	// Use the directory name (sanitized) for removal, not the display name.
	skillDirName := filepath.Base(skill.Path)

	deleteCmd := func() tea.Msg {
		orch := core.NewOrchestrator()
		if err := orch.RemoveAsset(asset.KindSkill, skillDirName, folderPath, system.All()); err != nil {
			return assetRemovedMsg{kind: asset.KindSkill, name: skill.Name, err: fmt.Errorf("removing %s: %w", skill.Name, err)}
		}
		// Remove lock entry (TUI always updates lock file).
		_ = core.RemoveAssetEntry(folderPath, asset.KindSkill, skillDirName)
		return assetRemovedMsg{kind: asset.KindSkill, name: skill.Name}
	}

	app.confirm = app.confirm.show(
		fmt.Sprintf("Remove skill %s?", skill.Name),
		deleteCmd,
	)
	return nil
}

// updateSelectedSkill updates the currently selected skill if it has an update available.
func (m folderModel) updateSelectedSkill(app *App) tea.Cmd {
	if m.updateCount == 0 {
		return nil
	}

	list := m.lists[asset.KindSkill]
	if list == nil {
		return nil
	}
	item := list.SelectedItem()
	if item == nil || m.status == nil {
		return nil
	}

	si, ok := item.(assetItem)
	if !ok {
		return nil
	}

	ui, hasUpdate := m.updateInfo[si.name]
	if !hasUpdate || !ui.HasUpdate {
		return nil
	}

	folderPath := app.activeFolder

	updateCmd := m.buildUpdateCmd(app, ui, folderPath)

	shortOld := core.TruncateCommit(ui.InstalledCommit)
	shortNew := core.TruncateCommit(ui.AvailableCommit)

	app.confirm = app.confirm.show(
		fmt.Sprintf("Update %s? (%s -> %s)", ui.Name, shortOld, shortNew),
		updateCmd,
	)
	return nil
}

// updateAllSkills updates all skills that have updates available.
func (m folderModel) updateAllSkills(app *App) tea.Cmd {
	if m.updateCount == 0 {
		return nil
	}

	folderPath := app.activeFolder

	bulkCmd := func() tea.Msg {
		var updated, errors int
		cfg, cfgErr := app.config.Load()

		for _, ui := range m.updateInfo {
			if !ui.HasUpdate {
				continue
			}

			err := executeSkillUpdate(app, ui, folderPath, cfg, cfgErr)
			if err != nil {
				errors++
				continue
			}
			updated++
		}

		return bulkUpdateDoneMsg{
			updated: updated,
			errors:  errors,
		}
	}

	app.confirm = app.confirm.show(
		fmt.Sprintf("Update all skills? (%d updates available)", m.updateCount),
		bulkCmd,
	)
	return nil
}

// refreshWithRegistries triggers an async registry refresh + data reload.
func (m folderModel) refreshWithRegistries(app *App) tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return startRegistryRefreshMsg{} },
		app.loadDataCmd,
	)
}

// buildUpdateCmd creates a tea.Cmd that updates a single skill.
func (m folderModel) buildUpdateCmd(app *App, ui core.UpdateInfo, folderPath string) tea.Cmd {
	return func() tea.Msg {
		cfg, cfgErr := app.config.Load()

		err := executeSkillUpdate(app, ui, folderPath, cfg, cfgErr)
		if err != nil {
			return updateDoneMsg{
				skillName: ui.Name,
				err:       err,
			}
		}

		return updateDoneMsg{
			skillName: ui.Name,
		}
	}
}

// executeSkillUpdate performs the actual update: remove old skill, reinstall at new commit,
// update lock entry. Returns an error if any step fails.
func executeSkillUpdate(app *App, ui core.UpdateInfo, folderPath string, cfg *core.Config, cfgErr error) error {
	// Read lock file to get the ref.
	lf, err := core.ReadLockFile(folderPath)
	if err != nil {
		return fmt.Errorf("reading lock file: %w", err)
	}
	if lf == nil {
		return fmt.Errorf("no lock file found")
	}

	// Find the lock entry for this skill.
	lockEntry := core.FindLockedAsset(lf, asset.KindSkill, ui.Name)
	if lockEntry == nil {
		return fmt.Errorf("skill %s not found in lock file", ui.Name)
	}

	// Parse lock source to build a ParsedSource.
	host, owner, repo, subPath, parseErr := core.ParseLockSource(ui.Source)
	if parseErr != nil {
		return fmt.Errorf("parsing source: %w", parseErr)
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
	if cfgErr == nil && cfg != nil {
		source.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)
	}

	// Remove existing skill.
	remover := core.NewOrchestrator()
	removeErr := remover.RemoveAsset(asset.KindSkill, ui.Name, folderPath, system.All())
	if removeErr != nil {
		return fmt.Errorf("removing: %w", removeErr)
	}

	// Reinstall at the available commit.
	installer := core.NewOrchestrator()
	result, installErr := installer.InstallFromSource(source, asset.KindSkill, core.OrchestratorInstallOptions{
		TargetDir:       folderPath,
		NameFilter:      ui.Name,
		Commit:          ui.AvailableCommit,
		IncludeInternal: true,
	})
	if installErr != nil {
		return fmt.Errorf("installing: %w", installErr)
	}

	// Update lock file with new commit.
	for _, r := range result {
		entry := asset.LockedAsset{
			Kind:   asset.KindSkill,
			Name:   r.Asset.Name,
			Source: r.Asset.Source,
			Commit: r.Commit,
			Ref:    r.Ref,
		}
		if lockErr := core.AddOrUpdateAsset(folderPath, entry); lockErr != nil {
			return fmt.Errorf("updating lock file: %w", lockErr)
		}
	}

	return nil
}

// removeSelectedMCP shows a confirmation dialog for the selected MCP.
// The confirmation message lists the agent config files that will be modified.
func (m folderModel) removeSelectedMCP(app *App) tea.Cmd {
	list := m.lists[asset.KindMCP]
	if list == nil {
		return nil
	}
	item := list.SelectedItem()
	if item == nil || m.status == nil {
		return nil
	}

	mcp, ok := item.(assetItem)
	if !ok || mcp.locked == nil {
		return nil
	}

	folderPath := app.activeFolder

	// Build list of system config files that will be modified.
	var configFiles []string
	for _, sysName := range lockedSystems(*mcp.locked) {
		if sys, ok := system.ByName(sysName); ok {
			if resolver, ok := sys.(interface{ ResolveMCPConfigPathRel(string) string }); ok {
				configFiles = append(configFiles, resolver.ResolveMCPConfigPathRel(folderPath))
			}
		}
	}

	var confirmMsg string
	if len(configFiles) > 0 {
		confirmMsg = fmt.Sprintf("Remove MCP %s?\nConfig files: %s",
			mcp.locked.Name, strings.Join(configFiles, ", "))
	} else {
		confirmMsg = fmt.Sprintf("Remove MCP %s?", mcp.locked.Name)
	}

	deleteCmd := func() tea.Msg {
		// Remove from system config files.
		for _, sysName := range lockedSystems(*mcp.locked) {
			if sys, ok := system.ByName(sysName); ok {
				if err := sys.Remove(asset.KindMCP, mcp.locked.Name, folderPath); err != nil {
					return assetRemovedMsg{kind: asset.KindMCP, name: mcp.locked.Name, err: fmt.Errorf("removing MCP %s: %w", mcp.locked.Name, err)}
				}
			}
		}
		// Remove lock entry.
		_ = core.RemoveAssetEntry(folderPath, asset.KindMCP, mcp.locked.Name)
		return assetRemovedMsg{kind: asset.KindMCP, name: mcp.locked.Name}
	}

	app.confirm = app.confirm.show(confirmMsg, deleteCmd)
	return nil
}

// countAvailable counts registry items NOT already installed in the active folder.
func (m folderModel) countAvailable() int {
	if m.status == nil {
		return len(m.regAssets)
	}

	// Build installed sets per kind.
	installedByKind := make(map[asset.Kind]map[string]bool)
	for _, kind := range m.keyOrder {
		set := make(map[string]bool)
		if kind == asset.KindMCP {
			for _, mcp := range m.mcps {
				if mcp.locked != nil {
					set[mcp.locked.Name] = true
				}
			}
		} else {
			for _, a := range m.status.Assets[kind] {
				set[a.Name] = true
			}
		}
		installedByKind[kind] = set
	}

	count := 0
	for _, ra := range m.regAssets {
		if installed, ok := installedByKind[ra.Kind]; ok {
			if !installed[ra.Entry.Name] {
				count++
			}
		} else {
			count++ // Unknown kind — not installed
		}
	}
	return count
}

// shortenPath returns a display-friendly folder path using ~ for home dir.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Base(path)
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// readSkillMdBody reads a SKILL.md file and returns the content after the
// YAML frontmatter (everything after the closing ---).
func readSkillMdBody(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)

	// Look for opening ---
	if !scanner.Scan() {
		return "", fmt.Errorf("empty file: %s", path)
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		// No frontmatter, return entire file.
		var b strings.Builder
		b.WriteString(scanner.Text())
		b.WriteString("\n")
		for scanner.Scan() {
			b.WriteString(scanner.Text())
			b.WriteString("\n")
		}
		return b.String(), scanner.Err()
	}

	// Skip frontmatter until closing ---
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "---" {
			break
		}
	}

	// Read the rest as body content.
	var body strings.Builder
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteString("\n")
	}

	return strings.TrimLeft(body.String(), "\n"), scanner.Err()
}
