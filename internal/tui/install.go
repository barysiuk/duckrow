package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/barysiuk/duckrow/internal/core"
)

// installPhase tracks which step of the install flow we're in.
type installPhase int

const (
	installPhasePicking      installPhase = iota // Browsing available skills
	installPhaseSelectAgents                     // Selecting agents for symlinks
	installPhaseInstalling                       // Install in progress
)

// agentCheckbox represents one non-universal agent in the selection list.
type agentCheckbox struct {
	agent   core.AgentDef
	checked bool
}

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
	phase installPhase

	// Agent selection state.
	universalAgents []core.AgentDef        // Universal agents (always selected, not toggleable)
	agentBoxes      []agentCheckbox        // Non-universal agents to choose from
	agentCursor     int                    // Currently highlighted agent
	selectedSkill   core.RegistrySkillInfo // Skill selected in picking phase

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
		phase:   installPhasePicking,
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
	if v {
		m.phase = installPhaseInstalling
	} else {
		m.phase = installPhasePicking
	}
}

func (m installModel) isInstalling() bool {
	return m.phase == installPhaseInstalling
}

func (m installModel) isSelectingAgents() bool {
	return m.phase == installPhaseSelectAgents
}

// selectedSkillInfo returns the currently selected registry skill info,
// used when a clone error occurs and we need to pass context to the error overlay.
func (m installModel) selectedSkillInfo() core.RegistrySkillInfo {
	if m.phase == installPhaseSelectAgents || m.phase == installPhaseInstalling {
		return m.selectedSkill
	}
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

// selectedTargetAgents returns the agents the user checked during agent selection.
// Used to pass context to the clone error overlay for retries.
func (m installModel) selectedTargetAgents() []core.AgentDef {
	var agents []core.AgentDef
	for _, ab := range m.agentBoxes {
		if ab.checked {
			agents = append(agents, ab.agent)
		}
	}
	return agents
}

// activate is called when the install picker opens. It filters registry skills
// to show only those NOT already installed in the active folder.
func (m installModel) activate(activeFolder string, regSkills []core.RegistrySkillInfo, folderStatus *core.FolderStatus, agents []core.AgentDef) installModel {
	m.activeFolder = activeFolder
	m.phase = installPhasePicking

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

	// Prepare agent lists for the selection screen.
	m.universalAgents = core.GetUniversalAgents(agents)
	nonUniversal := core.GetNonUniversalAgents(agents)
	m.agentBoxes = make([]agentCheckbox, len(nonUniversal))
	for i, a := range nonUniversal {
		m.agentBoxes[i] = agentCheckbox{agent: a, checked: false}
	}
	m.agentCursor = 0

	return m
}

func (m installModel) update(msg tea.Msg, app *App) (installModel, tea.Cmd) {
	// During install, only handle spinner ticks.
	if m.phase == installPhaseInstalling {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Agent selection phase.
	if m.phase == installPhaseSelectAgents {
		return m.updateAgentSelect(msg, app)
	}

	// Picking phase.
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys while filtering.
		if m.list.SettingFilter() {
			break
		}

		switch {
		case key.Matches(msg, keys.Enter):
			return m.handleSkillSelected(app)
		}
	}

	// Forward to list for navigation + filtering.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	// Skip separator items -- if cursor landed on one, move past it.
	m.skipSeparators()

	return m, cmd
}

func (m installModel) updateAgentSelect(msg tea.Msg, app *App) (installModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.agentCursor > 0 {
				m.agentCursor--
			}
		case key.Matches(msg, keys.Down):
			if m.agentCursor < len(m.agentBoxes)-1 {
				m.agentCursor++
			}
		case key.Matches(msg, keys.Toggle):
			if len(m.agentBoxes) > 0 {
				m.agentBoxes[m.agentCursor].checked = !m.agentBoxes[m.agentCursor].checked
			}
		case key.Matches(msg, keys.ToggleAll):
			m.toggleAllAgents()
		case key.Matches(msg, keys.Enter):
			return m.startInstall(app)
		case key.Matches(msg, keys.Back):
			m.phase = installPhasePicking
			return m, nil
		}
	}
	return m, nil
}

// toggleAllAgents toggles all agents: if any are checked, uncheck all; otherwise check all.
func (m *installModel) toggleAllAgents() {
	anyChecked := false
	for _, ab := range m.agentBoxes {
		if ab.checked {
			anyChecked = true
			break
		}
	}
	for i := range m.agentBoxes {
		m.agentBoxes[i].checked = !anyChecked
	}
}

// handleSkillSelected is called when the user presses Enter on a skill.
// If there are non-universal agents to choose from, show the agent selection.
// Otherwise, go straight to install (universal-only).
func (m installModel) handleSkillSelected(app *App) (installModel, tea.Cmd) {
	item := m.list.SelectedItem()
	if item == nil {
		return m, nil
	}
	rsi, ok := item.(registrySkillItem)
	if !ok {
		return m, nil
	}

	m.selectedSkill = rsi.info

	if len(m.agentBoxes) > 0 {
		// Show agent selection phase.
		m.phase = installPhaseSelectAgents
		m.agentCursor = 0
		// Reset checkboxes.
		for i := range m.agentBoxes {
			m.agentBoxes[i].checked = false
		}
		return m, nil
	}

	// No non-universal agents detected -- install with universal-only.
	return m.startInstall(app)
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
	switch m.phase {
	case installPhaseSelectAgents:
		return m.viewAgentSelect()
	case installPhaseInstalling:
		sectionHeader := renderSectionHeader("INSTALL SKILL", m.width) + "\n"
		return sectionHeader + "  " + m.spinner.View() + " Installing... please wait"
	default:
		return m.viewPicking()
	}
}

func (m installModel) viewPicking() string {
	// --- Render-then-measure ---

	// 1. Render fixed chrome.
	sectionHeader := renderSectionHeader("INSTALL SKILL", m.width) + "\n"

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

func (m installModel) viewAgentSelect() string {
	var b strings.Builder

	header := renderSectionHeader("SELECT AGENTS", m.width)
	b.WriteString(header)
	b.WriteString("\n")

	desc := mutedStyle.Render("   Select which agents should have access to this skill.")
	b.WriteString(desc)
	b.WriteString("\n\n")

	// Universal agents — always selected, not toggleable.
	universalLabel := mutedStyle.Render("   .agents/skills/ (always installed)")
	b.WriteString(universalLabel)
	b.WriteString("\n")
	for _, a := range m.universalAgents {
		line := "  [x] " + a.DisplayName
		b.WriteString(mutedStyle.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Non-universal agents — toggleable checkboxes.
	symlinkLabel := mutedStyle.Render("   Agent-specific (optional)")
	b.WriteString(symlinkLabel)
	b.WriteString("\n")
	for i, ab := range m.agentBoxes {
		check := "[ ]"
		if ab.checked {
			check = "[x]"
		}

		prefix := "  "
		if i == m.agentCursor {
			prefix = "> "
		}

		line := prefix + check + " " + ab.agent.DisplayName
		dirHint := " (" + ab.agent.SkillsDir + ")"
		if i == m.agentCursor {
			b.WriteString(selectedItemStyle.Render(line))
			b.WriteString(mutedStyle.Render(dirHint))
		} else {
			b.WriteString(normalItemStyle.Render(line))
			b.WriteString(mutedStyle.Render(dirHint))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m installModel) startInstall(app *App) (installModel, tea.Cmd) {
	skill := m.selectedSkill
	// If selectedSkill wasn't set (direct install without agent select), get from list.
	if skill.Skill.Name == "" {
		item := m.list.SelectedItem()
		if item == nil {
			return m, nil
		}
		rsi, ok := item.(registrySkillItem)
		if !ok {
			return m, nil
		}
		skill = rsi.info
		m.selectedSkill = skill
	}

	// Build target agents from checked boxes.
	var targetAgents []core.AgentDef
	for _, ab := range m.agentBoxes {
		if ab.checked {
			targetAgents = append(targetAgents, ab.agent)
		}
	}

	folder := m.activeFolder
	m.phase = installPhaseInstalling

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
			TargetDir:    folder,
			IsInternal:   true, // Registry skills always disable telemetry
			TargetAgents: targetAgents,
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
