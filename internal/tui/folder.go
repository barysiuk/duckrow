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
)

// folderTab identifies which tab is active in the folder view.
type folderTab int

const (
	folderTabSkills folderTab = iota
	folderTabMCPs
)

// folderModel is the main folder view showing installed skills and MCPs
// for the active folder, organized into tabs.
type folderModel struct {
	width  int
	height int

	// Tabs.
	activeTab folderTab
	tabs      tabsModel

	// Bubbles lists — one per tab.
	skillsList list.Model
	mcpsList   list.Model

	// Data (pushed from App).
	status     *core.FolderStatus
	isTracked  bool
	regSkills  []core.RegistrySkillInfo
	regMCPs    []core.RegistryMCPInfo
	availCount int // Number of registry items (skills + MCPs) NOT installed

	// Update info: skill name -> update info.
	updateInfo  map[string]core.UpdateInfo
	updateCount int // Number of skills with updates available

	// MCP data from lock file.
	mcps []mcpItem
}

func newFolderModel() folderModel {
	return folderModel{
		tabs:       newTabsModel([]string{"Skills", "MCPs"}),
		skillsList: newFolderList(),
		mcpsList:   newFolderList(),
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
	return l
}

func (m folderModel) setSize(width, height int) folderModel {
	m.width = width
	m.height = height
	// Actual list height is calculated dynamically in view().
	m.skillsList.SetSize(width, max(1, height))
	m.mcpsList.SetSize(width, max(1, height))
	return m
}

func (m folderModel) setData(status *core.FolderStatus, isTracked bool, regSkills []core.RegistrySkillInfo, regMCPs []core.RegistryMCPInfo, updateInfo map[string]core.UpdateInfo, mcps []mcpItem) folderModel {
	m.status = status
	m.isTracked = isTracked
	m.regSkills = regSkills
	m.regMCPs = regMCPs
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

	// Populate skills list.
	if status != nil {
		m.skillsList.SetItems(skillsToItems(status.Skills, updateInfo))
	} else {
		m.skillsList.SetItems(nil)
	}

	// Populate MCPs list.
	m.mcpsList.SetItems(mcpsToItems(mcps))

	// Update tab labels with counts.
	m.tabs = m.updateTabLabels()

	return m
}

// updateTabLabels builds tab labels with item counts and update indicators.
func (m folderModel) updateTabLabels() tabsModel {
	skillCount := 0
	if m.status != nil {
		skillCount = len(m.status.Skills)
	}

	skillLabel := fmt.Sprintf("Skills (%d)", skillCount)
	if m.updateCount > 0 {
		skillLabel = fmt.Sprintf("Skills (%d ↑%d)", skillCount, m.updateCount)
	}

	mcpLabel := fmt.Sprintf("MCPs (%d)", len(m.mcps))

	return m.tabs.setLabels([]string{skillLabel, mcpLabel})
}

// activeList returns a pointer to the currently active list model.
func (m *folderModel) activeList() *list.Model {
	switch m.activeTab {
	case folderTabMCPs:
		return &m.mcpsList
	default:
		return &m.skillsList
	}
}

// isFiltering returns true if the active list is in filter mode.
func (m folderModel) isFiltering() bool {
	switch m.activeTab {
	case folderTabMCPs:
		return m.mcpsList.SettingFilter()
	default:
		return m.skillsList.SettingFilter()
	}
}

func (m folderModel) update(msg tea.Msg, app *App) (folderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tabActiveMsg:
		m.activeTab = folderTab(msg)
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
			if m.activeTab == folderTabMCPs {
				return m, m.removeSelectedMCP(app)
			}
			return m, m.removeSelectedSkill(app)

		case key.Matches(msg, keys.Update):
			if m.activeTab == folderTabSkills {
				return m, m.updateSelectedSkill(app)
			}
			return m, nil

		case key.Matches(msg, keys.UpdateAll):
			return m, m.updateAllSkills(app)

		case key.Matches(msg, keys.Refresh):
			return m, m.refreshWithRegistries(app)

		case key.Matches(msg, keys.Enter):
			if m.activeTab == folderTabSkills {
				return m, m.openPreview(app)
			}
			return m, nil
		}
	}

	// Forward to the active list.
	var cmd tea.Cmd
	al := m.activeList()
	*al, cmd = al.Update(msg)
	return m, cmd
}

// openPreview reads the selected skill's SKILL.md and triggers the preview overlay.
func (m folderModel) openPreview(app *App) tea.Cmd {
	item := m.skillsList.SelectedItem()
	if item == nil {
		return nil
	}
	si, ok := item.(skillItem)
	if !ok {
		return nil
	}

	skillMdPath := filepath.Join(si.skill.Path, "SKILL.md")
	content, err := readSkillMdBody(skillMdPath)
	if err != nil {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("reading SKILL.md: %w", err)}
		}
	}

	title := si.skill.Name

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
	} else if len(m.regSkills) == 0 && len(m.regMCPs) == 0 {
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
	switch m.activeTab {
	case folderTabSkills:
		if len(m.skillsList.Items()) == 0 {
			listView = "\n" + mutedStyle.Render("  No skills installed")
		} else {
			m.skillsList.SetSize(m.width, listH)
			listView = m.skillsList.View()
		}
	case folderTabMCPs:
		if len(m.mcpsList.Items()) == 0 {
			listView = "\n" + mutedStyle.Render("  No MCPs installed")
		} else {
			m.mcpsList.SetSize(m.width, listH)
			listView = m.mcpsList.View()
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
	item := m.skillsList.SelectedItem()
	if item == nil || m.status == nil {
		return nil
	}

	si, ok := item.(skillItem)
	if !ok {
		return nil
	}

	skill := si.skill
	folderPath := app.activeFolder

	// Use the directory name (sanitized) for removal, not the display name.
	skillDirName := filepath.Base(skill.Path)

	deleteCmd := func() tea.Msg {
		remover := core.NewRemover(app.agents)
		_, err := remover.Remove(skillDirName, core.RemoveOptions{TargetDir: folderPath})
		if err != nil {
			return errMsg{err: fmt.Errorf("removing %s: %w", skill.Name, err)}
		}
		// Remove lock entry (TUI always updates lock file).
		_ = core.RemoveLockEntry(folderPath, skillDirName)
		// Reload data to refresh the view after removal.
		return app.loadDataCmd()
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

	item := m.skillsList.SelectedItem()
	if item == nil || m.status == nil {
		return nil
	}

	si, ok := item.(skillItem)
	if !ok {
		return nil
	}

	ui, hasUpdate := m.updateInfo[si.skill.Name]
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
	var lockEntry *core.LockedSkill
	for i := range lf.Skills {
		if lf.Skills[i].Name == ui.Name {
			lockEntry = &lf.Skills[i]
			break
		}
	}
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
	remover := core.NewRemover(app.agents)
	_, removeErr := remover.Remove(ui.Name, core.RemoveOptions{TargetDir: folderPath})
	if removeErr != nil {
		return fmt.Errorf("removing: %w", removeErr)
	}

	// Reinstall at the available commit.
	installer := core.NewInstaller(app.agents)
	result, installErr := installer.InstallFromSource(source, core.InstallOptions{
		TargetDir:   folderPath,
		SkillFilter: ui.Name,
		Commit:      ui.AvailableCommit,
		IsInternal:  true,
	})
	if installErr != nil {
		return fmt.Errorf("installing: %w", installErr)
	}

	// Update lock file with new commit.
	for _, s := range result.InstalledSkills {
		entry := core.LockedSkill{
			Name:   s.Name,
			Source: s.Source,
			Commit: s.Commit,
			Ref:    s.Ref,
		}
		if lockErr := core.AddOrUpdateLockEntry(folderPath, entry); lockErr != nil {
			return fmt.Errorf("updating lock file: %w", lockErr)
		}
	}

	return nil
}

// removeSelectedMCP shows a confirmation dialog for the selected MCP.
// The confirmation message lists the agent config files that will be modified.
func (m folderModel) removeSelectedMCP(app *App) tea.Cmd {
	item := m.mcpsList.SelectedItem()
	if item == nil || m.status == nil {
		return nil
	}

	mcp, ok := item.(mcpItem)
	if !ok {
		return nil
	}

	folderPath := app.activeFolder

	// Build list of agent config files that will be modified.
	var configFiles []string
	for _, agentName := range mcp.locked.Agents {
		for _, agent := range app.agents {
			if agent.Name == agentName && agent.MCPConfigPath != "" {
				configFiles = append(configFiles, core.ResolveMCPConfigPathRel(agent, folderPath))
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
		// Remove from agent config files.
		_, err := core.UninstallMCPConfig(mcp.locked.Name, app.agents, core.MCPUninstallOptions{
			ProjectDir: folderPath,
		})
		if err != nil {
			return errMsg{err: fmt.Errorf("removing MCP %s: %w", mcp.locked.Name, err)}
		}
		// Remove lock entry.
		_ = core.RemoveMCPLockEntry(folderPath, mcp.locked.Name)
		// Reload data to refresh the view.
		return app.loadDataCmd()
	}

	app.confirm = app.confirm.show(confirmMsg, deleteCmd)
	return nil
}

// countAvailable counts registry items (skills + MCPs) NOT already installed in the active folder.
func (m folderModel) countAvailable() int {
	if m.status == nil {
		return len(m.regSkills) + len(m.regMCPs)
	}

	installedSkills := make(map[string]bool)
	for _, s := range m.status.Skills {
		installedSkills[s.Name] = true
	}

	installedMCPs := make(map[string]bool)
	for _, mcp := range m.mcps {
		installedMCPs[mcp.locked.Name] = true
	}

	count := 0
	for _, rs := range m.regSkills {
		if !installedSkills[rs.Skill.Name] {
			count++
		}
	}
	for _, rm := range m.regMCPs {
		if !installedMCPs[rm.MCP.Name] {
			count++
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
