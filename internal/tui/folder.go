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

// folderModel is the main folder view showing installed skills
// for the active folder.
type folderModel struct {
	width  int
	height int

	// Bubbles list for installed skills.
	list list.Model

	// Data (pushed from App).
	status     *core.FolderStatus
	isTracked  bool
	regSkills  []core.RegistrySkillInfo
	availCount int // Number of registry skills NOT installed

	// Update info: skill name -> update info.
	updateInfo  map[string]core.UpdateInfo
	updateCount int // Number of skills with updates available
}

func newFolderModel() folderModel {
	d := newSkillDelegate()
	l := list.New(nil, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowPagination(false)

	return folderModel{
		list: l,
	}
}

func (m folderModel) setSize(width, height int) folderModel {
	m.width = width
	m.height = height
	// List sizing happens dynamically in view() via render-then-measure.
	// We store dimensions here so view() can compute the list height.
	m.list.SetSize(width, max(1, height))
	return m
}

func (m folderModel) setData(status *core.FolderStatus, isTracked bool, regSkills []core.RegistrySkillInfo, updateInfo map[string]core.UpdateInfo) folderModel {
	m.status = status
	m.isTracked = isTracked
	m.regSkills = regSkills
	m.availCount = m.countAvailable()
	m.updateInfo = updateInfo

	// Count skills with updates.
	m.updateCount = 0
	for _, ui := range updateInfo {
		if ui.HasUpdate {
			m.updateCount++
		}
	}

	// Convert skills to list items.
	if status != nil {
		items := skillsToItems(status.Skills, updateInfo)
		m.list.SetItems(items)
	} else {
		m.list.SetItems(nil)
	}

	return m
}

func (m folderModel) update(msg tea.Msg, app *App) (folderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys while filtering.
		if m.list.SettingFilter() {
			break
		}

		switch {
		case key.Matches(msg, keys.Delete):
			return m, m.removeSelectedSkill(app)

		case key.Matches(msg, keys.Update):
			return m, m.updateSelectedSkill(app)

		case key.Matches(msg, keys.UpdateAll):
			return m, m.updateAllSkills(app)

		case key.Matches(msg, keys.Refresh):
			return m, m.refreshWithRegistries(app)

		case key.Matches(msg, keys.Enter):
			// Open SKILL.md preview for the selected skill.
			return m, m.openPreview(app)
		}
	}

	// Forward all other messages to the list (handles j/k, filtering, etc.)
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// openPreview reads the selected skill's SKILL.md and triggers the preview overlay.
func (m folderModel) openPreview(app *App) tea.Cmd {
	item := m.list.SelectedItem()
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

	// --- Render-then-measure: render all fixed chrome first, measure, size the list. ---

	// 1. Render fixed chrome parts.
	var banner string
	if !m.isTracked {
		banner = warningStyle.Render("  This folder is not tracked.") +
			"  " + mutedStyle.Render("Press [a] to add it.") + "\n\n"
	}

	skillCount := len(m.status.Skills)
	var sectionHeader string
	if skillCount == 0 {
		sectionHeader = renderSectionHeader("SKILLS", m.width) + "\n"
	} else if m.updateCount > 0 {
		label := fmt.Sprintf("SKILLS (%d installed, ", skillCount)
		updateLabel := fmt.Sprintf("%d updates available", m.updateCount)
		closing := ")"
		sectionHeader = renderSectionHeaderWithUpdate(label, updateLabel, closing, m.width) + "\n"
	} else {
		sectionHeader = renderSectionHeader(fmt.Sprintf("SKILLS (%d installed)", skillCount), m.width) + "\n"
	}

	// Build footer: optional update prefix + registry status.
	var parts []string
	if m.updateCount > 0 {
		parts = append(parts,
			warningStyle.Render(fmt.Sprintf("%d updates available", m.updateCount))+
				"  "+headerHintStyle.Render("[u] Update"))
	}

	if m.availCount > 0 {
		parts = append(parts,
			mutedStyle.Render(fmt.Sprintf("%d skills available from registries", m.availCount))+
				"  "+headerHintStyle.Render("[i] Install"))
	} else if len(m.regSkills) == 0 {
		parts = append(parts,
			mutedStyle.Render("No registries configured.")+
				"  "+headerHintStyle.Render("[s] Settings to add"))
	} else {
		parts = append(parts,
			mutedStyle.Render("All registry skills installed"))
	}

	footer := "  " + strings.Join(parts, "  |  ")
	// Blank line padding between list and footer message.
	footerBlock := "\n\n" + footer

	// 2. Measure chrome height.
	chromeH := lipgloss.Height(banner) + lipgloss.Height(sectionHeader) + lipgloss.Height(footerBlock)

	// 3. Size the list to fit its content, capped by available space.
	//    DefaultDelegate: Height()=2 (title+desc), Spacing()=1.
	//    The list calculates PerPage = availHeight / (Height+Spacing) using
	//    integer division. It also internally subtracts chrome from the height
	//    we give it (e.g. title/filter bar = 1 even when empty). We add that
	//    back so the items-per-page calculation comes out right.
	if skillCount > 0 {
		maxH := m.height - chromeH
		if maxH < 1 {
			maxH = 1
		}
		itemSlot := 3 // Height(2) + Spacing(1)
		// +1 compensates for the list's internal title/filter bar line
		// (lipgloss.Height("") == 1, so it always steals 1 line).
		listH := skillCount*itemSlot + 1
		if listH > maxH {
			listH = maxH
		}
		m.list.SetSize(m.width, listH)
	}

	// 4. Assemble.
	var b strings.Builder
	b.WriteString(banner)
	b.WriteString(sectionHeader)

	if skillCount == 0 {
		b.WriteString("\n" + mutedStyle.Render("  No skills installed"))
	} else {
		b.WriteString(m.list.View())
	}

	b.WriteString(footerBlock)

	return b.String()
}

func (m folderModel) removeSelectedSkill(app *App) tea.Cmd {
	item := m.list.SelectedItem()
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

	item := m.list.SelectedItem()
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

// countAvailable counts registry skills NOT already installed in the active folder.
func (m folderModel) countAvailable() int {
	if m.status == nil {
		return len(m.regSkills)
	}

	installed := make(map[string]bool)
	for _, s := range m.status.Skills {
		installed[s.Name] = true
	}

	count := 0
	for _, rs := range m.regSkills {
		if !installed[rs.Skill.Name] {
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
