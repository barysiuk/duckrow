package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
)

// installFilter determines which artifact types are shown in the install picker.
type installFilter int

const (
	installFilterSkills installFilter = iota
	installFilterMCPs
)

// agentCheckbox represents one agent in the selection list.
type agentCheckbox struct {
	agent   core.AgentDef
	checked bool
}

// mcpInstallDoneMsg is sent when an MCP install completes.
type mcpInstallDoneMsg struct {
	mcpName string
	folder  string
	err     error
}

// envVarStatus tracks the resolution status of a single env var.
type envVarStatus struct {
	name   string
	isSet  bool
	source string // "project" or "global" if set
}

// envSaveDoneMsg is sent after saving an env var value.
type envSaveDoneMsg struct {
	err error
}

// isSensitiveVarName returns true if the var name suggests a sensitive value
// that should be masked during input.
func isSensitiveVarName(name string) bool {
	upper := strings.ToUpper(name)
	return strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "KEY") ||
		strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "PASSWORD")
}

// installModel is the install picker that shows registry skills and MCPs not
// yet installed in the active folder. When the user selects an item, it emits
// openSkillWizardMsg or openMCPWizardMsg; the actual install flow is handled
// by the dedicated wizard models (skill_wizard.go, mcp_wizard.go).
type installModel struct {
	width  int
	height int

	// Bubbles list for available skills/MCPs.
	list list.Model

	// Filter: skills-only or MCPs-only.
	filter installFilter

	// Data (set on activate).
	activeFolder  string
	available     []core.RegistrySkillInfo // Filtered: only NOT installed skills
	availableMCPs []core.RegistryMCPInfo   // Filtered: only NOT installed MCPs
	allAgents     []core.AgentDef          // All agent definitions
	installedMCPs []mcpItem                // Currently installed MCPs (for filtering)
}

func newInstallModel() installModel {
	l := list.New(nil, registrySkillDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowPagination(false)

	return installModel{
		list: l,
	}
}

func (m installModel) setSize(width, height int) installModel {
	m.width = width
	m.height = height
	m.list.SetSize(width, max(1, height))
	return m
}

// activate is called when the install picker opens. It filters registry items
// to show only those NOT already installed in the active folder, scoped to
// the given filter (skills-only or MCPs-only).
func (m installModel) activate(filter installFilter, activeFolder string, regSkills []core.RegistrySkillInfo, folderStatus *core.FolderStatus, agents []core.AgentDef) installModel {
	m.activeFolder = activeFolder
	m.allAgents = agents
	m.filter = filter

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

	// Filter to available (not installed) MCPs.
	allRegMCPs := m.registryMCPs()
	installedMCPNames := make(map[string]bool)
	for _, mcp := range m.installedMCPs {
		installedMCPNames[mcp.locked.Name] = true
	}
	m.availableMCPs = nil
	for _, rm := range allRegMCPs {
		if !installedMCPNames[rm.MCP.Name] {
			m.availableMCPs = append(m.availableMCPs, rm)
		}
	}

	// Build list items scoped to the active filter.
	var items []list.Item
	switch m.filter {
	case installFilterSkills:
		items = registrySkillsToItems(m.available)
	case installFilterMCPs:
		items = registryMCPsToItems(m.availableMCPs)
	}
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

// setMCPData sets the MCP data needed for filtering in the install picker.
// Called from app.go before activate.
func (m installModel) setMCPData(regMCPs []core.RegistryMCPInfo, installedMCPs []mcpItem) installModel {
	m.availableMCPs = regMCPs
	m.installedMCPs = installedMCPs
	return m
}

// registryMCPs returns the full registry MCP list from availableMCPs.
func (m installModel) registryMCPs() []core.RegistryMCPInfo {
	return m.availableMCPs
}

func (m installModel) update(msg tea.Msg) (installModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys while filtering.
		if m.list.SettingFilter() {
			break
		}

		switch {
		case key.Matches(msg, keys.Enter):
			return m.handleItemSelected()
		}
	}

	// Forward to list for navigation + filtering.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	// Skip separator items.
	m.skipSeparators()

	return m, cmd
}

// handleItemSelected emits the appropriate wizard message for the selected item.
func (m installModel) handleItemSelected() (installModel, tea.Cmd) {
	item := m.list.SelectedItem()
	if item == nil {
		return m, nil
	}

	switch it := item.(type) {
	case registrySkillItem:
		universalAgents := core.GetUniversalAgents(m.allAgents)
		nonUniversal := core.GetNonUniversalAgents(m.allAgents)
		return m, func() tea.Msg {
			return openSkillWizardMsg{
				skill:           it.info,
				universalAgents: universalAgents,
				nonUniversal:    nonUniversal,
				activeFolder:    m.activeFolder,
			}
		}
	case registryMCPItem:
		return m, func() tea.Msg {
			return openMCPWizardMsg{
				mcp:          it.info,
				allAgents:    m.allAgents,
				activeFolder: m.activeFolder,
			}
		}
	}

	return m, nil
}

// skipSeparators moves the cursor off separator items.
func (m *installModel) skipSeparators() {
	items := m.list.Items()
	idx := m.list.Index()
	if idx >= 0 && idx < len(items) {
		if _, ok := items[idx].(registrySeparatorItem); ok {
			if idx+1 < len(items) {
				m.list.Select(idx + 1)
			} else if idx-1 >= 0 {
				m.list.Select(idx - 1)
			}
		}
	}
}

func (m installModel) view() string {
	isEmpty := false
	switch m.filter {
	case installFilterSkills:
		isEmpty = len(m.available) == 0
	case installFilterMCPs:
		isEmpty = len(m.availableMCPs) == 0
	}

	if isEmpty {
		switch m.filter {
		case installFilterSkills:
			return mutedStyle.Render("  All registry skills are already installed.")
		case installFilterMCPs:
			return mutedStyle.Render("  All registry MCPs are already installed.")
		}
	}

	// Size list to fill available space.
	m.list.SetSize(m.width, max(1, m.height))

	return m.list.View()
}
