package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/barysiuk/duckrow/internal/core"
)

// installPhase tracks which step of the install flow we're in.
type installPhase int

const (
	installPhasePicking         installPhase = iota // Browsing available skills/MCPs
	installPhaseSelectAgents                        // Selecting agents for skill symlinks
	installPhaseInstalling                          // Skill install in progress
	installPhaseSelectMCPAgents                     // Selecting agents for MCP config
	installPhaseMCPPreview                          // MCP preview/confirmation (env var status)
	installPhaseEnvEntry                            // Entering env var values one at a time
	installPhaseMCPInstalling                       // MCP install in progress
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

// installModel is the install picker overlay that shows registry skills
// and MCPs not yet installed in the active folder.
type installModel struct {
	width  int
	height int

	// Bubbles list for available skills/MCPs.
	list list.Model

	// Spinner for install progress.
	spinner spinner.Model

	// State.
	phase installPhase

	// Skill agent selection state.
	universalAgents []core.AgentDef        // Universal agents (always selected, not toggleable)
	agentBoxes      []agentCheckbox        // Non-universal agents to choose from
	agentCursor     int                    // Currently highlighted agent
	selectedSkill   core.RegistrySkillInfo // Skill selected in picking phase

	// MCP agent selection state.
	selectedMCP     core.RegistryMCPInfo // MCP selected in picking phase
	mcpAgentBoxes   []agentCheckbox      // MCP-capable agents (all toggleable)
	mcpAgentCursor  int                  // Currently highlighted MCP agent
	mcpTargetAgents []core.AgentDef      // Resolved target agents after selection

	// MCP env var status (computed at preview).
	mcpEnvStatus []envVarStatus // Env var resolution results

	// Env var entry flow state.
	envInput        textinput.Model // Text input for the current env var value
	envMissingVars  []string        // Missing var names to enter (subset of mcpEnvStatus)
	envCurrentIndex int             // Index into envMissingVars
	envSaveProject  bool            // true = project .env.duckrow, false = global

	// Data (set on activate).
	activeFolder  string
	available     []core.RegistrySkillInfo // Filtered: only NOT installed skills
	availableMCPs []core.RegistryMCPInfo   // Filtered: only NOT installed MCPs
	allAgents     []core.AgentDef          // All agent definitions
	installedMCPs []mcpItem                // Currently installed MCPs (for filtering)
}

// envVarStatus tracks the resolution status of a single env var.
type envVarStatus struct {
	name   string
	isSet  bool
	source string // "project" or "global" if set
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
	return m.phase == installPhaseInstalling || m.phase == installPhaseMCPInstalling
}

func (m installModel) isSelectingAgents() bool {
	return m.phase == installPhaseSelectAgents || m.phase == installPhaseSelectMCPAgents
}

func (m installModel) isMCPPhase() bool {
	return m.phase == installPhaseSelectMCPAgents ||
		m.phase == installPhaseMCPPreview ||
		m.phase == installPhaseEnvEntry ||
		m.phase == installPhaseMCPInstalling
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

// activate is called when the install picker opens. It filters registry items
// to show only those NOT already installed in the active folder.
func (m installModel) activate(activeFolder string, regSkills []core.RegistrySkillInfo, folderStatus *core.FolderStatus, agents []core.AgentDef) installModel {
	m.activeFolder = activeFolder
	m.phase = installPhasePicking
	m.allAgents = agents

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
	// Capture the full registry MCP list before overwriting availableMCPs,
	// since registryMCPs() returns m.availableMCPs.
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

	// Build list items with separators — combined skills + MCPs.
	var items []list.Item
	if len(m.availableMCPs) > 0 {
		items = registryItemsToList(m.available, m.availableMCPs)
	} else {
		items = registrySkillsToItems(m.available)
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

	// Prepare skill agent lists for the selection screen.
	m.universalAgents = core.GetUniversalAgents(agents)
	nonUniversal := core.GetNonUniversalAgents(agents)
	m.agentBoxes = make([]agentCheckbox, len(nonUniversal))
	for i, a := range nonUniversal {
		m.agentBoxes[i] = agentCheckbox{agent: a, checked: false}
	}
	m.agentCursor = 0

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
// This is a convenience to avoid storing another copy; availableMCPs is set
// to the full list in setMCPData and filtered down in activate.
func (m installModel) registryMCPs() []core.RegistryMCPInfo {
	return m.availableMCPs
}

func (m installModel) update(msg tea.Msg, app *App) (installModel, tea.Cmd) {
	// During install, only handle spinner ticks.
	if m.phase == installPhaseInstalling || m.phase == installPhaseMCPInstalling {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Agent selection phase (skills).
	if m.phase == installPhaseSelectAgents {
		return m.updateAgentSelect(msg, app)
	}

	// MCP agent selection phase.
	if m.phase == installPhaseSelectMCPAgents {
		return m.updateMCPAgentSelect(msg, app)
	}

	// MCP preview/confirmation phase.
	if m.phase == installPhaseMCPPreview {
		return m.updateMCPPreview(msg, app)
	}

	// Env var entry phase.
	if m.phase == installPhaseEnvEntry {
		return m.updateEnvEntry(msg, app)
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
			return m.handleItemSelected(app)
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

func (m installModel) updateMCPAgentSelect(msg tea.Msg, app *App) (installModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.mcpAgentCursor > 0 {
				m.mcpAgentCursor--
			}
		case key.Matches(msg, keys.Down):
			if m.mcpAgentCursor < len(m.mcpAgentBoxes)-1 {
				m.mcpAgentCursor++
			}
		case key.Matches(msg, keys.Toggle):
			if len(m.mcpAgentBoxes) > 0 {
				m.mcpAgentBoxes[m.mcpAgentCursor].checked = !m.mcpAgentBoxes[m.mcpAgentCursor].checked
			}
		case key.Matches(msg, keys.ToggleAll):
			m.toggleAllMCPAgents()
		case key.Matches(msg, keys.Enter):
			return m.proceedToMCPPreview(app)
		case key.Matches(msg, keys.Back):
			m.phase = installPhasePicking
			return m, nil
		}
	}
	return m, nil
}

func (m installModel) updateMCPPreview(msg tea.Msg, app *App) (installModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Enter):
			return m.startMCPInstall(app)
		case key.Matches(msg, keys.Configure):
			return m.startEnvEntry()
		case key.Matches(msg, keys.Back):
			// Go back to MCP agent selection.
			m.phase = installPhaseSelectMCPAgents
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

// toggleAllMCPAgents toggles all MCP agents.
func (m *installModel) toggleAllMCPAgents() {
	anyChecked := false
	for _, ab := range m.mcpAgentBoxes {
		if ab.checked {
			anyChecked = true
			break
		}
	}
	for i := range m.mcpAgentBoxes {
		m.mcpAgentBoxes[i].checked = !anyChecked
	}
}

// handleItemSelected is called when the user presses Enter on an item.
// Routes to skill or MCP flow based on the item type.
func (m installModel) handleItemSelected(app *App) (installModel, tea.Cmd) {
	item := m.list.SelectedItem()
	if item == nil {
		return m, nil
	}

	switch it := item.(type) {
	case registrySkillItem:
		return m.handleSkillSelected(it, app)
	case registryMCPItem:
		return m.handleMCPSelected(it, app)
	}

	return m, nil
}

// handleSkillSelected is called when the user presses Enter on a skill.
// If there are non-universal agents to choose from, show the agent selection.
// Otherwise, go straight to install (universal-only).
func (m installModel) handleSkillSelected(rsi registrySkillItem, app *App) (installModel, tea.Cmd) {
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

// handleMCPSelected is called when the user presses Enter on an MCP.
// Shows MCP-specific agent selection (only MCP-capable agents).
func (m installModel) handleMCPSelected(rmi registryMCPItem, app *App) (installModel, tea.Cmd) {
	m.selectedMCP = rmi.info

	// Build MCP agent checkboxes — only MCP-capable agents.
	mcpCapable := core.GetMCPCapableAgents(m.allAgents)
	detected := core.DetectAgentsInFolder(m.allAgents, m.activeFolder)
	detectedNames := make(map[string]bool)
	for _, d := range detected {
		detectedNames[d.Name] = true
	}

	m.mcpAgentBoxes = make([]agentCheckbox, len(mcpCapable))
	for i, a := range mcpCapable {
		// Detected agents are checked by default.
		m.mcpAgentBoxes[i] = agentCheckbox{
			agent:   a,
			checked: detectedNames[a.Name],
		}
	}
	m.mcpAgentCursor = 0
	m.phase = installPhaseSelectMCPAgents

	return m, nil
}

// proceedToMCPPreview transitions from MCP agent selection to the preview screen.
// Resolves env var status for the selected MCP.
func (m installModel) proceedToMCPPreview(app *App) (installModel, tea.Cmd) {
	// Build target agents from checked boxes.
	m.mcpTargetAgents = nil
	for _, ab := range m.mcpAgentBoxes {
		if ab.checked {
			m.mcpTargetAgents = append(m.mcpTargetAgents, ab.agent)
		}
	}

	if len(m.mcpTargetAgents) == 0 {
		// At least one agent must be selected.
		return m, nil
	}

	// Resolve env var status.
	m.mcpEnvStatus = nil
	mcp := m.selectedMCP.MCP
	if mcp.URL == "" && len(mcp.Env) > 0 {
		// Extract required env var names from $VAR references.
		requiredVars := core.ExtractRequiredEnv(mcp.Env)
		if len(requiredVars) > 0 {
			resolver := core.NewEnvResolver(m.activeFolder, "")
			results := resolver.ResolveEnvWithSource(requiredVars)

			for _, r := range results {
				status := envVarStatus{name: r.Name}
				if r.Source != "" {
					status.isSet = true
					status.source = string(r.Source)
				}
				m.mcpEnvStatus = append(m.mcpEnvStatus, status)
			}
		}
	}

	m.phase = installPhaseMCPPreview
	return m, nil
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
	case installPhaseSelectMCPAgents:
		return m.viewMCPAgentSelect()
	case installPhaseMCPPreview:
		return m.viewMCPPreview()
	case installPhaseEnvEntry:
		return m.viewEnvEntry()
	case installPhaseMCPInstalling:
		sectionHeader := renderSectionHeader("INSTALL MCP", m.width) + "\n"
		return sectionHeader + "  " + m.spinner.View() + " Installing... please wait"
	default:
		return m.viewPicking()
	}
}

func (m installModel) viewPicking() string {
	// --- Render-then-measure ---

	// 1. Render fixed chrome.
	sectionHeader := renderSectionHeader("INSTALL", m.width) + "\n"

	if len(m.available) == 0 && len(m.availableMCPs) == 0 {
		return sectionHeader + mutedStyle.Render("  All registry items are already installed.")
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

func (m installModel) viewMCPAgentSelect() string {
	var b strings.Builder

	header := renderSectionHeader("SELECT AGENTS", m.width)
	b.WriteString(header)
	b.WriteString("\n")

	// Show MCP name and description.
	b.WriteString("  Install MCP: ")
	b.WriteString(selectedItemStyle.Render(m.selectedMCP.MCP.Name))
	b.WriteString("\n")
	if m.selectedMCP.MCP.Description != "" {
		b.WriteString("  " + mutedStyle.Render(m.selectedMCP.MCP.Description))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	desc := mutedStyle.Render("   Select target agents:")
	b.WriteString(desc)
	b.WriteString("\n\n")

	// MCP-capable agents — all toggleable.
	for i, ab := range m.mcpAgentBoxes {
		check := "[ ]"
		if ab.checked {
			check = "[x]"
		}

		prefix := "  "
		if i == m.mcpAgentCursor {
			prefix = "> "
		}

		line := prefix + check + " " + ab.agent.DisplayName
		configHint := " (" + ab.agent.MCPConfigPath + ")"
		if i == m.mcpAgentCursor {
			b.WriteString(selectedItemStyle.Render(line))
			b.WriteString(mutedStyle.Render(configHint))
		} else {
			b.WriteString(normalItemStyle.Render(line))
			b.WriteString(mutedStyle.Render(configHint))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m installModel) viewMCPPreview() string {
	var b strings.Builder

	header := renderSectionHeader("INSTALL MCP", m.width)
	b.WriteString(header)
	b.WriteString("\n")

	mcp := m.selectedMCP.MCP

	// MCP name.
	b.WriteString("  Install MCP: ")
	b.WriteString(selectedItemStyle.Render(mcp.Name))
	b.WriteString("\n")
	if mcp.Description != "" {
		b.WriteString("  " + mutedStyle.Render(mcp.Description))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Command or URL.
	if mcp.URL != "" {
		b.WriteString("  URL:      " + normalItemStyle.Render(mcp.URL))
		b.WriteString("\n")
		if mcp.Type != "" {
			b.WriteString("  Type:     " + mutedStyle.Render(mcp.Type))
			b.WriteString("\n")
		}
	} else {
		cmdStr := mcp.Command
		if len(mcp.Args) > 0 {
			cmdStr += " " + strings.Join(mcp.Args, " ")
		}
		b.WriteString("  Command:  " + normalItemStyle.Render(cmdStr))
		b.WriteString("\n")
	}

	// Env var status (stdio MCPs only).
	if len(m.mcpEnvStatus) > 0 {
		for i, ev := range m.mcpEnvStatus {
			prefix := "  Env:      "
			if i > 0 {
				prefix = "            "
			}
			if ev.isSet {
				b.WriteString(prefix + normalItemStyle.Render(ev.name) + "  " + installedStyle.Render("✓ set") + " " + mutedStyle.Render("("+ev.source+")"))
			} else {
				b.WriteString(prefix + normalItemStyle.Render(ev.name) + "  " + warningStyle.Render("! not set"))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Target config files.
	b.WriteString("  Will write to:\n")
	for _, agent := range m.mcpTargetAgents {
		configPath := agent.MCPConfigPath
		b.WriteString("    " + normalItemStyle.Render(configPath) + "  " + mutedStyle.Render("("+agent.DisplayName+")"))
		b.WriteString("\n")
	}

	// Warning if some vars not set.
	hasMissing := false
	for _, ev := range m.mcpEnvStatus {
		if !ev.isSet {
			hasMissing = true
			break
		}
	}
	if hasMissing {
		b.WriteString("\n")
		b.WriteString("  " + warningStyle.Render("! Some vars not set — MCP may not work until added"))
		b.WriteString("\n")
	}

	return b.String()
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

// startEnvEntry transitions from MCP preview to the env var entry flow.
// Collects missing (not-set) vars and starts with the first one.
func (m installModel) startEnvEntry() (installModel, tea.Cmd) {
	// Collect missing var names.
	m.envMissingVars = nil
	for _, ev := range m.mcpEnvStatus {
		if !ev.isSet {
			m.envMissingVars = append(m.envMissingVars, ev.name)
		}
	}

	if len(m.envMissingVars) == 0 {
		// No missing vars — stay on preview.
		return m, nil
	}

	m.envCurrentIndex = 0
	m.envSaveProject = true // Default to project.
	m.envInput = textinput.New()
	m.envInput.Placeholder = "Enter value..."
	m.envInput.CharLimit = 512
	m.envInput.Width = 40
	m.envInput.Focus()

	// Apply masking for sensitive var names.
	if isSensitiveVarName(m.envMissingVars[0]) {
		m.envInput.EchoMode = textinput.EchoPassword
		m.envInput.EchoCharacter = '•'
	} else {
		m.envInput.EchoMode = textinput.EchoNormal
	}

	m.phase = installPhaseEnvEntry
	return m, textinput.Blink
}

// envSaveDoneMsg is sent after saving an env var value.
type envSaveDoneMsg struct {
	err error
}

func (m installModel) updateEnvEntry(msg tea.Msg, app *App) (installModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Enter):
			// Save the current value and advance.
			value := m.envInput.Value()
			if value == "" {
				// Empty value — treat as skip.
				return m.advanceEnvEntry(app)
			}
			return m.saveAndAdvanceEnvEntry(app)

		case key.Matches(msg, keys.Tab):
			// Toggle save location.
			m.envSaveProject = !m.envSaveProject
			return m, nil

		case key.Matches(msg, keys.Back):
			// Skip this var and advance.
			return m.advanceEnvEntry(app)
		}

	case envSaveDoneMsg:
		if msg.err != nil {
			// On error, just advance — the var will remain "not set".
			return m.advanceEnvEntry(app)
		}
		return m.advanceEnvEntry(app)
	}

	// Forward to text input.
	var cmd tea.Cmd
	m.envInput, cmd = m.envInput.Update(msg)
	return m, cmd
}

// saveAndAdvanceEnvEntry saves the current env var value and triggers advance.
func (m installModel) saveAndAdvanceEnvEntry(app *App) (installModel, tea.Cmd) {
	varName := m.envMissingVars[m.envCurrentIndex]
	value := m.envInput.Value()

	var saveDir string
	if m.envSaveProject {
		saveDir = m.activeFolder
	} else {
		// Global dir.
		saveDir = core.GlobalConfigDir()
	}

	saveCmd := func() tea.Msg {
		// Write the env var.
		if err := core.WriteEnvVar(saveDir, varName, value); err != nil {
			return envSaveDoneMsg{err: err}
		}

		// If saving to project, ensure .gitignore has .env.duckrow.
		if saveDir == m.activeFolder {
			_ = core.EnsureGitignore(m.activeFolder)
		}

		return envSaveDoneMsg{}
	}

	return m, saveCmd
}

// advanceEnvEntry moves to the next missing env var, or returns to preview.
func (m installModel) advanceEnvEntry(app *App) (installModel, tea.Cmd) {
	m.envCurrentIndex++

	if m.envCurrentIndex >= len(m.envMissingVars) {
		// All done — refresh env status and go back to preview.
		m.refreshEnvStatus()
		m.phase = installPhaseMCPPreview
		return m, nil
	}

	// Reset input for next var.
	m.envInput.Reset()
	m.envSaveProject = true

	// Apply masking for sensitive var names.
	if isSensitiveVarName(m.envMissingVars[m.envCurrentIndex]) {
		m.envInput.EchoMode = textinput.EchoPassword
		m.envInput.EchoCharacter = '•'
	} else {
		m.envInput.EchoMode = textinput.EchoNormal
	}

	return m, textinput.Blink
}

// refreshEnvStatus re-resolves env var status after the entry flow.
func (m *installModel) refreshEnvStatus() {
	mcp := m.selectedMCP.MCP
	if mcp.URL != "" || len(mcp.Env) == 0 {
		return
	}

	requiredVars := core.ExtractRequiredEnv(mcp.Env)
	if len(requiredVars) == 0 {
		return
	}

	resolver := core.NewEnvResolver(m.activeFolder, "")
	results := resolver.ResolveEnvWithSource(requiredVars)

	m.mcpEnvStatus = make([]envVarStatus, len(results))
	for i, r := range results {
		m.mcpEnvStatus[i] = envVarStatus{name: r.Name}
		if r.Source != "" {
			m.mcpEnvStatus[i].isSet = true
			m.mcpEnvStatus[i].source = string(r.Source)
		}
	}
}

func (m installModel) viewEnvEntry() string {
	var b strings.Builder

	header := renderSectionHeader("CONFIGURE ENV VARS", m.width)
	b.WriteString(header)
	b.WriteString("\n")

	mcpName := m.selectedMCP.MCP.Name
	total := len(m.envMissingVars)
	current := m.envCurrentIndex + 1

	b.WriteString(fmt.Sprintf("  Configure env vars for %s  (%d of %d)",
		selectedItemStyle.Render(mcpName), current, total))
	b.WriteString("\n\n")

	// Current var name.
	varName := m.envMissingVars[m.envCurrentIndex]
	b.WriteString("  " + normalItemStyle.Render(varName))
	b.WriteString("\n\n")

	// Input field.
	b.WriteString("  Value: " + m.envInput.View())
	b.WriteString("\n\n")

	// Save location toggle.
	b.WriteString("  Save to:\n")
	if m.envSaveProject {
		b.WriteString("  " + selectedItemStyle.Render("(*)") + " Project  " + mutedStyle.Render(".env.duckrow"))
		b.WriteString("\n")
		b.WriteString("  " + mutedStyle.Render("( ) Global   ~/.duckrow/.env.duckrow"))
		b.WriteString("\n")
	} else {
		b.WriteString("  " + mutedStyle.Render("( ) Project  .env.duckrow"))
		b.WriteString("\n")
		b.WriteString("  " + selectedItemStyle.Render("(*)") + " Global   " + mutedStyle.Render("~/.duckrow/.env.duckrow"))
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

		// Pass registry commit through if set.
		var registryCommit string
		if skill.Skill.Commit != "" {
			registryCommit = skill.Skill.Commit
		}

		installer := core.NewInstaller(app.agents)
		result, err := installer.InstallFromSource(source, core.InstallOptions{
			TargetDir:    folder,
			IsInternal:   true, // Registry skills always disable telemetry
			TargetAgents: targetAgents,
			Commit:       registryCommit,
		})

		if err != nil {
			return installDoneMsg{
				skillName: skill.Skill.Name,
				folder:    folder,
				err:       err,
			}
		}

		// Write lock file entries for installed skills (TUI always locks).
		for _, s := range result.InstalledSkills {
			if s.Commit != "" {
				entry := core.LockedSkill{
					Name:   s.Name,
					Source: s.Source,
					Commit: s.Commit,
					Ref:    s.Ref,
				}
				_ = core.AddOrUpdateLockEntry(folder, entry)
			}
		}

		return installDoneMsg{
			skillName: skill.Skill.Name,
			folder:    folder,
			err:       nil,
		}
	}

	// Start spinner + launch install.
	return m, tea.Batch(m.spinner.Tick, installCmd)
}

func (m installModel) startMCPInstall(app *App) (installModel, tea.Cmd) {
	mcp := m.selectedMCP.MCP
	folder := m.activeFolder
	targetAgents := m.mcpTargetAgents

	m.phase = installPhaseMCPInstalling

	installCmd := func() tea.Msg {
		// Install MCP config to agent config files.
		_, err := core.InstallMCPConfig(mcp, core.MCPInstallOptions{
			ProjectDir:   folder,
			TargetAgents: targetAgents,
		})
		if err != nil {
			return mcpInstallDoneMsg{
				mcpName: mcp.Name,
				folder:  folder,
				err:     err,
			}
		}

		// Write lock file entry.
		agentNames := make([]string, len(targetAgents))
		for i, a := range targetAgents {
			agentNames[i] = a.Name
		}
		lockEntry := core.LockedMCP{
			Name:        mcp.Name,
			Registry:    m.selectedMCP.RegistryRepo,
			ConfigHash:  core.ComputeConfigHash(mcp),
			Agents:      agentNames,
			RequiredEnv: core.ExtractRequiredEnv(mcp.Env),
		}
		_ = core.AddOrUpdateMCPLockEntry(folder, lockEntry)

		return mcpInstallDoneMsg{
			mcpName: mcp.Name,
			folder:  folder,
		}
	}

	return m, tea.Batch(m.spinner.Tick, installCmd)
}
