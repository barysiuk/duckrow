package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/barysiuk/duckrow/internal/core"
)

// installModel is the install picker overlay that shows registry skills
// not yet installed in the active folder.
type installModel struct {
	width  int
	height int

	// Bubbles list for available skills.
	list list.Model

	// Spinner for install progress.
	spinner spinner.Model

	// State.
	installing bool

	// Data (set on activate).
	activeFolder string
	available    []core.RegistrySkillInfo // Filtered: only NOT installed
}

func newInstallModel() installModel {
	l := list.New(nil, registrySkillDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowPagination(false)

	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(spinnerStyle),
	)

	return installModel{
		list:    l,
		spinner: s,
	}
}

func (m installModel) setSize(width, height int) installModel {
	m.width = width
	m.height = height
	// List sizing happens dynamically in view() via render-then-measure.
	m.list.SetSize(width, max(1, height))
	return m
}

func (m *installModel) setInstalling(v bool) {
	m.installing = v
}

func (m installModel) isInstalling() bool {
	return m.installing
}

// selectedSkillInfo returns the currently selected registry skill info,
// used when a clone error occurs and we need to pass context to the error overlay.
func (m installModel) selectedSkillInfo() core.RegistrySkillInfo {
	item := m.list.SelectedItem()
	if item == nil {
		return core.RegistrySkillInfo{}
	}
	rsi, ok := item.(registrySkillItem)
	if !ok {
		return core.RegistrySkillInfo{}
	}
	return rsi.info
}

// activate is called when the install picker opens. It filters registry skills
// to show only those NOT already installed in the active folder.
func (m installModel) activate(activeFolder string, regSkills []core.RegistrySkillInfo, folderStatus *core.FolderStatus) installModel {
	m.activeFolder = activeFolder
	m.installing = false

	// Build set of installed skill names.
	installed := make(map[string]bool)
	if folderStatus != nil {
		for _, s := range folderStatus.Skills {
			installed[s.Name] = true
		}
	}

	// Filter to available (not installed) skills.
	m.available = nil
	for _, rs := range regSkills {
		if !installed[rs.Skill.Name] {
			m.available = append(m.available, rs)
		}
	}

	// Build list items with separators.
	items := registrySkillsToItems(m.available)
	m.list.SetItems(items)
	m.list.ResetFilter()

	// Select first selectable item (skip separator).
	if len(items) > 0 {
		if _, ok := items[0].(registrySeparatorItem); ok && len(items) > 1 {
			m.list.Select(1)
		} else {
			m.list.Select(0)
		}
	}

	return m
}

func (m installModel) update(msg tea.Msg, app *App) (installModel, tea.Cmd) {
	// During install, only handle spinner ticks.
	if m.installing {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys while filtering.
		if m.list.SettingFilter() {
			break
		}

		switch {
		case key.Matches(msg, keys.Enter):
			return m.startInstall(app)
		}
	}

	// Forward to list for navigation + filtering.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	// Skip separator items — if cursor landed on one, move past it.
	m.skipSeparators()

	return m, cmd
}

// skipSeparators moves the cursor off separator items.
func (m *installModel) skipSeparators() {
	items := m.list.Items()
	idx := m.list.Index()
	if idx >= 0 && idx < len(items) {
		if _, ok := items[idx].(registrySeparatorItem); ok {
			// Try moving down first.
			if idx+1 < len(items) {
				m.list.Select(idx + 1)
			} else if idx-1 >= 0 {
				m.list.Select(idx - 1)
			}
		}
	}
}

func (m installModel) view() string {
	// --- Render-then-measure ---

	// 1. Render fixed chrome.
	sectionHeader := renderSectionHeader("INSTALL SKILL", m.width) + "\n"

	if m.installing {
		return sectionHeader + "  " + m.spinner.View() + " Installing... please wait"
	}

	if len(m.available) == 0 {
		return sectionHeader + mutedStyle.Render("  All registry skills are already installed.")
	}

	// 2. Measure chrome, size list to fill remaining space.
	chromeH := lipgloss.Height(sectionHeader)
	listH := m.height - chromeH
	if listH < 1 {
		listH = 1
	}
	m.list.SetSize(m.width, listH)

	// 3. Assemble.
	return sectionHeader + m.list.View()
}

func (m installModel) startInstall(app *App) (installModel, tea.Cmd) {
	item := m.list.SelectedItem()
	if item == nil {
		return m, nil
	}

	rsi, ok := item.(registrySkillItem)
	if !ok {
		return m, nil // Selected a separator somehow
	}

	skill := rsi.info
	folder := m.activeFolder
	m.installing = true

	installCmd := func() tea.Msg {
		source, err := core.ParseSource(skill.Skill.Source)
		if err != nil {
			return installDoneMsg{
				skillName: skill.Skill.Name,
				folder:    folder,
				err:       fmt.Errorf("parsing source %q: %w", skill.Skill.Source, err),
			}
		}

		// Apply clone URL override if one exists for this repo.
		cfg, cfgErr := app.config.Load()
		if cfgErr == nil {
			source.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)
		}

		installer := core.NewInstaller(app.agents)
		_, err = installer.InstallFromSource(source, core.InstallOptions{
			TargetDir:  folder,
			IsInternal: true, // Registry skills always disable telemetry
		})

		return installDoneMsg{
			skillName: skill.Skill.Name,
			folder:    folder,
			err:       err,
		}
	}

	// Start spinner + launch install.
	return m, tea.Batch(m.spinner.Tick, installCmd)
}
