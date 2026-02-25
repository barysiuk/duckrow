package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
)

// openMCPWizardMsg is emitted by installModel when an MCP is selected.
type openMCPWizardMsg struct {
	mcp          core.RegistryMCPInfo
	allAgents    []core.AgentDef
	activeFolder string
}

// mcpWizardModel wraps a wizardModel for the MCP install flow.
// Steps: Select Agents → Preview → Env Entry (optional) → Installing.
type mcpWizardModel struct {
	wizard wizardModel

	// MCP being installed.
	mcp          core.RegistryMCPInfo
	activeFolder string

	// Agent selection state.
	mcpAgentBoxes  []agentCheckbox
	mcpAgentCursor int
	targetAgents   []core.AgentDef // Resolved after agent selection.

	// Env var status.
	mcpEnvStatus []envVarStatus

	// Env var entry state.
	envInput        textinput.Model
	envMissingVars  []string
	envCurrentIndex int
	envSaveProject  bool

	// Install state.
	installing bool

	// App reference.
	app *App
}

func newMCPWizardModel() mcpWizardModel {
	return mcpWizardModel{}
}

// activate initializes the MCP wizard with the selected MCP data.
func (m mcpWizardModel) activate(msg openMCPWizardMsg, app *App, width, height int) mcpWizardModel {
	m.app = app
	m.mcp = msg.mcp
	m.activeFolder = msg.activeFolder
	m.installing = false
	m.targetAgents = nil
	m.mcpEnvStatus = nil
	m.envMissingVars = nil

	// Build MCP agent checkboxes — only MCP-capable agents.
	// Pre-select agents actively used in this folder (matching the sidebar).
	mcpCapable := core.GetMCPCapableAgents(msg.allAgents)
	activeAgentNames := core.DetectActiveAgents(msg.allAgents, msg.activeFolder)
	activeSet := make(map[string]bool, len(activeAgentNames))
	for _, name := range activeAgentNames {
		activeSet[name] = true
	}

	m.mcpAgentBoxes = make([]agentCheckbox, len(mcpCapable))
	for i, a := range mcpCapable {
		m.mcpAgentBoxes[i] = agentCheckbox{
			agent:   a,
			checked: activeSet[a.DisplayName],
		}
	}
	m.mcpAgentCursor = 0

	// Build wizard steps.
	agentStep := newMCPAgentStepModel()
	previewStep := newMCPPreviewStepModel()
	installStep := newMCPInstallingStepModel()

	m.wizard = newWizardModel("Install MCP", []wizardStep{
		{name: "Select Agents", content: agentStep},
		{name: "Preview", content: previewStep},
		{name: "Installing", content: installStep},
	})
	m.wizard = m.wizard.setSize(width, height)

	return m
}

// setSize updates the layout dimensions.
func (m mcpWizardModel) setSize(width, height int) mcpWizardModel {
	m.wizard = m.wizard.setSize(width, height)
	return m
}

// isInstalling returns true if the MCP install is in progress.
func (m mcpWizardModel) isInstalling() bool {
	return m.installing
}

// mcpWizardPhase describes the current wizard phase for help keymaps.
type mcpWizardPhase int

const (
	mcpPhaseSelectAgents mcpWizardPhase = iota
	mcpPhasePreview
	mcpPhaseEnvEntry
	mcpPhaseInstalling
)

// currentPhase returns the current wizard phase.
func (m mcpWizardModel) currentPhase() mcpWizardPhase {
	if m.installing {
		return mcpPhaseInstalling
	}
	switch m.wizard.activeIdx {
	case 0:
		return mcpPhaseSelectAgents
	case 1:
		// Preview step — check if we're in env entry mode.
		if _, ok := m.wizard.steps[1].content.(mcpEnvEntryStepModel); ok {
			return mcpPhaseEnvEntry
		}
		return mcpPhasePreview
	default:
		return mcpPhaseInstalling
	}
}

// currentHelpKeyMap returns the help keymap for the current wizard phase.
func (m mcpWizardModel) currentHelpKeyMap() mcpWizardHelpKeyMap {
	return mcpWizardHelpKeyMap{phase: m.currentPhase(), hasEnvVars: len(m.mcpEnvStatus) > 0}
}

// update handles messages for the MCP wizard.
func (m mcpWizardModel) update(msg tea.Msg, app *App) (mcpWizardModel, tea.Cmd) {
	m.app = app

	switch msg := msg.(type) {
	case wizardDoneMsg, wizardBackMsg:
		// Handled by app.go.
		return m, nil

	case wizardNextMsg:
		return m.handleNext()

	case mcpInstallDoneMsg:
		m.installing = false
		// app.go handles the result.
		return m, nil

	case envSaveDoneMsg:
		if msg.err != nil {
			return m.advanceEnvEntry()
		}
		return m.advanceEnvEntry()
	}

	// Route to current phase handler.
	switch m.currentPhase() {
	case mcpPhaseSelectAgents:
		return m.handleAgentSelectKey(msg)
	case mcpPhasePreview:
		return m.handlePreviewKey(msg)
	case mcpPhaseEnvEntry:
		return m.handleEnvEntryKey(msg)
	case mcpPhaseInstalling:
		// Only handle spinner ticks.
		if _, ok := msg.(spinner.TickMsg); ok {
			step := m.wizard.activeStep()
			if step != nil {
				var cmd tea.Cmd
				step.content, cmd = step.content.Update(msg)
				return m, cmd
			}
		}
		return m, nil
	}

	// Forward to wizard for esc handling.
	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.update(msg)
	return m, cmd
}

// handleNext handles wizardNextMsg for step transitions.
func (m mcpWizardModel) handleNext() (mcpWizardModel, tea.Cmd) {
	currentIdx := m.wizard.activeIdx

	if currentIdx == 0 {
		// Agent selection → Preview. Resolve target agents and env status.
		m.targetAgents = nil
		for _, ab := range m.mcpAgentBoxes {
			if ab.checked {
				m.targetAgents = append(m.targetAgents, ab.agent)
			}
		}
		if len(m.targetAgents) == 0 {
			return m, nil // At least one agent required.
		}

		// Resolve env var status.
		m.resolveEnvStatus()

		// Update preview step with data.
		m.wizard.steps[1].content = mcpPreviewStepModel{
			mcp:          m.mcp.MCP,
			targetAgents: m.targetAgents,
			envStatus:    m.mcpEnvStatus,
			activeFolder: m.activeFolder,
		}

		// Advance wizard index.
		var cmd tea.Cmd
		m.wizard, cmd = m.wizard.update(wizardNextMsg{})
		return m, cmd
	}

	if currentIdx == 1 {
		// Preview → Installing. Start the install.
		var cmd tea.Cmd
		m.wizard, cmd = m.wizard.update(wizardNextMsg{})
		installCmd := m.startMCPInstall()
		return m, tea.Batch(cmd, installCmd)
	}

	return m, nil
}

// handleAgentSelectKey handles keys during agent selection.
func (m mcpWizardModel) handleAgentSelectKey(msg tea.Msg) (mcpWizardModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Up):
			if m.mcpAgentCursor > 0 {
				m.mcpAgentCursor--
			}
			return m, nil
		case key.Matches(keyMsg, keys.Down):
			if m.mcpAgentCursor < len(m.mcpAgentBoxes)-1 {
				m.mcpAgentCursor++
			}
			return m, nil
		case key.Matches(keyMsg, keys.Toggle):
			if len(m.mcpAgentBoxes) > 0 {
				m.mcpAgentBoxes[m.mcpAgentCursor].checked = !m.mcpAgentBoxes[m.mcpAgentCursor].checked
			}
			return m, nil
		case key.Matches(keyMsg, keys.ToggleAll):
			m.toggleAllAgents()
			return m, nil
		case key.Matches(keyMsg, keys.Enter):
			return m, func() tea.Msg { return wizardNextMsg{} }
		}
	}

	// Forward to wizard for esc.
	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.update(msg)
	return m, cmd
}

// handlePreviewKey handles keys during the preview step.
func (m mcpWizardModel) handlePreviewKey(msg tea.Msg) (mcpWizardModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Enter):
			return m, func() tea.Msg { return wizardNextMsg{} }
		case key.Matches(keyMsg, keys.Configure):
			return m.startEnvEntry()
		}
	}

	// Forward to wizard for esc (back to agent selection).
	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.update(msg)
	return m, cmd
}

// handleEnvEntryKey handles keys during the env var entry sub-flow.
func (m mcpWizardModel) handleEnvEntryKey(msg tea.Msg) (mcpWizardModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Enter):
			value := m.envInput.Value()
			if value == "" {
				return m.advanceEnvEntry()
			}
			return m.saveAndAdvanceEnvEntry()
		case key.Matches(keyMsg, keys.TabSaveLocation):
			m.envSaveProject = !m.envSaveProject
			return m, nil
		case key.Matches(keyMsg, keys.Back):
			// Skip this var and advance.
			return m.advanceEnvEntry()
		}
	}

	// Forward to text input.
	var cmd tea.Cmd
	m.envInput, cmd = m.envInput.Update(msg)
	return m, cmd
}

// view renders the wizard.
func (m mcpWizardModel) view() string {
	step := m.wizard.activeStep()
	if step == nil {
		return ""
	}

	// Inject current state into the active step.
	switch step.content.(type) {
	case mcpAgentStepModel:
		step.content = mcpAgentStepModel{
			mcp:            m.mcp.MCP,
			mcpAgentBoxes:  m.mcpAgentBoxes,
			mcpAgentCursor: m.mcpAgentCursor,
			activeFolder:   m.activeFolder,
		}
	case mcpPreviewStepModel:
		step.content = mcpPreviewStepModel{
			mcp:          m.mcp.MCP,
			targetAgents: m.targetAgents,
			envStatus:    m.mcpEnvStatus,
			activeFolder: m.activeFolder,
		}
	case mcpEnvEntryStepModel:
		step.content = mcpEnvEntryStepModel{
			mcpName:         m.mcp.MCP.Name,
			envInput:        m.envInput,
			envMissingVars:  m.envMissingVars,
			envCurrentIndex: m.envCurrentIndex,
			envSaveProject:  m.envSaveProject,
		}
	}

	return m.wizard.view()
}

// startEnvEntry transitions from preview to the env var entry sub-flow.
func (m mcpWizardModel) startEnvEntry() (mcpWizardModel, tea.Cmd) {
	m.envMissingVars = nil
	for _, ev := range m.mcpEnvStatus {
		if !ev.isSet {
			m.envMissingVars = append(m.envMissingVars, ev.name)
		}
	}

	if len(m.envMissingVars) == 0 {
		return m, nil
	}

	m.envCurrentIndex = 0
	m.envSaveProject = true
	m.envInput = textinput.New()
	m.envInput.Placeholder = "Enter value..."
	m.envInput.CharLimit = 512
	m.envInput.Width = 40
	m.envInput.Focus()

	if isSensitiveVarName(m.envMissingVars[0]) {
		m.envInput.EchoMode = textinput.EchoPassword
		m.envInput.EchoCharacter = '•'
	} else {
		m.envInput.EchoMode = textinput.EchoNormal
	}

	// Replace the preview step with the env entry step temporarily.
	m.wizard.steps[1].content = mcpEnvEntryStepModel{
		mcpName:         m.mcp.MCP.Name,
		envInput:        m.envInput,
		envMissingVars:  m.envMissingVars,
		envCurrentIndex: m.envCurrentIndex,
		envSaveProject:  m.envSaveProject,
	}
	m.wizard.steps[1].name = "Configure"

	return m, textinput.Blink
}

// saveAndAdvanceEnvEntry saves the current env var value and triggers advance.
func (m mcpWizardModel) saveAndAdvanceEnvEntry() (mcpWizardModel, tea.Cmd) {
	varName := m.envMissingVars[m.envCurrentIndex]
	value := m.envInput.Value()

	var saveDir string
	if m.envSaveProject {
		saveDir = m.activeFolder
	} else {
		saveDir = core.GlobalConfigDir()
	}

	saveCmd := func() tea.Msg {
		if err := core.WriteEnvVar(saveDir, varName, value); err != nil {
			return envSaveDoneMsg{err: err}
		}
		if saveDir == m.activeFolder {
			_ = core.EnsureGitignore(m.activeFolder)
		}
		return envSaveDoneMsg{}
	}

	return m, saveCmd
}

// advanceEnvEntry moves to the next missing env var, or returns to preview.
func (m mcpWizardModel) advanceEnvEntry() (mcpWizardModel, tea.Cmd) {
	m.envCurrentIndex++

	if m.envCurrentIndex >= len(m.envMissingVars) {
		// All done — refresh env status and go back to preview.
		m.resolveEnvStatus()

		// Restore preview step.
		m.wizard.steps[1].name = "Preview"
		m.wizard.steps[1].content = mcpPreviewStepModel{
			mcp:          m.mcp.MCP,
			targetAgents: m.targetAgents,
			envStatus:    m.mcpEnvStatus,
			activeFolder: m.activeFolder,
		}

		return m, nil
	}

	// Reset input for next var.
	m.envInput.Reset()
	m.envSaveProject = true

	if isSensitiveVarName(m.envMissingVars[m.envCurrentIndex]) {
		m.envInput.EchoMode = textinput.EchoPassword
		m.envInput.EchoCharacter = '•'
	} else {
		m.envInput.EchoMode = textinput.EchoNormal
	}

	return m, textinput.Blink
}

// resolveEnvStatus resolves env var status for the selected MCP.
func (m *mcpWizardModel) resolveEnvStatus() {
	m.mcpEnvStatus = nil
	mcp := m.mcp.MCP
	if mcp.URL == "" && len(mcp.Env) > 0 {
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
}

// startMCPInstall kicks off the MCP installation.
func (m *mcpWizardModel) startMCPInstall() tea.Cmd {
	m.installing = true

	mcp := m.mcp.MCP
	folder := m.activeFolder
	targetAgents := m.targetAgents
	registryRepo := m.mcp.RegistryRepo

	installCmd := func() tea.Msg {
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

		agentNames := make([]string, len(targetAgents))
		for i, a := range targetAgents {
			agentNames[i] = a.Name
		}
		lockEntry := core.LockedMCP{
			Name:        mcp.Name,
			Registry:    registryRepo,
			ConfigHash:  core.ComputeConfigHash(mcp.ToMCPMeta()),
			Agents:      agentNames,
			RequiredEnv: core.ExtractRequiredEnv(mcp.Env),
		}
		_ = core.AddOrUpdateMCPLockEntry(folder, lockEntry)

		return mcpInstallDoneMsg{
			mcpName: mcp.Name,
			folder:  folder,
		}
	}

	// Get spinner tick from the installing step.
	step := m.wizard.activeStep()
	if step != nil {
		if is, ok := step.content.(mcpInstallingStepModel); ok {
			return tea.Batch(is.spinner.Tick, installCmd)
		}
	}

	return installCmd
}

// toggleAllAgents toggles all MCP agents.
func (m *mcpWizardModel) toggleAllAgents() {
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

// ---------------------------------------------------------------------------
// Step 1: Select Agents
// ---------------------------------------------------------------------------

type mcpAgentStepModel struct {
	mcp            core.MCPEntry
	mcpAgentBoxes  []agentCheckbox
	mcpAgentCursor int
	activeFolder   string
}

func newMCPAgentStepModel() mcpAgentStepModel {
	return mcpAgentStepModel{}
}

func (m mcpAgentStepModel) Init() tea.Cmd { return nil }

func (m mcpAgentStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m mcpAgentStepModel) View() string {
	var b strings.Builder

	b.WriteString("Install MCP: ")
	b.WriteString(selectedItemStyle.Render(m.mcp.Name))
	b.WriteString("\n")
	if m.mcp.Description != "" {
		b.WriteString(mutedStyle.Render(m.mcp.Description))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	desc := mutedStyle.Render("Select target agents:")
	b.WriteString(desc)
	b.WriteString("\n\n")

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
		resolvedPath := core.ResolveMCPConfigPathRel(ab.agent, m.activeFolder)
		configHint := " (" + resolvedPath + ")"
		if i == m.mcpAgentCursor {
			b.WriteString(selectedItemStyle.Render(line))
			b.WriteString(mutedStyle.Render(configHint))
		} else {
			b.WriteString(normalItemStyle.Render(line))
			b.WriteString(mutedStyle.Render(configHint))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Press enter to continue"))

	return b.String()
}

// ---------------------------------------------------------------------------
// Step 2: Preview
// ---------------------------------------------------------------------------

type mcpPreviewStepModel struct {
	mcp          core.MCPEntry
	targetAgents []core.AgentDef
	envStatus    []envVarStatus
	activeFolder string
}

func newMCPPreviewStepModel() mcpPreviewStepModel {
	return mcpPreviewStepModel{}
}

func (m mcpPreviewStepModel) Init() tea.Cmd { return nil }

func (m mcpPreviewStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m mcpPreviewStepModel) View() string {
	var b strings.Builder

	mcp := m.mcp

	b.WriteString("Install MCP: ")
	b.WriteString(selectedItemStyle.Render(mcp.Name))
	b.WriteString("\n")
	if mcp.Description != "" {
		b.WriteString(mutedStyle.Render(mcp.Description))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Command or URL.
	if mcp.URL != "" {
		b.WriteString("URL:      " + normalItemStyle.Render(mcp.URL))
		b.WriteString("\n")
		if mcp.Type != "" {
			b.WriteString("Type:     " + mutedStyle.Render(mcp.Type))
			b.WriteString("\n")
		}
	} else {
		cmdStr := mcp.Command
		if len(mcp.Args) > 0 {
			cmdStr += " " + strings.Join(mcp.Args, " ")
		}
		b.WriteString("Command:  " + normalItemStyle.Render(cmdStr))
		b.WriteString("\n")
	}

	// Env var status.
	if len(m.envStatus) > 0 {
		for i, ev := range m.envStatus {
			prefix := "Env:      "
			if i > 0 {
				prefix = "          "
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
	b.WriteString("Will write to:\n")
	for _, agent := range m.targetAgents {
		configPath := core.ResolveMCPConfigPathRel(agent, m.activeFolder)
		b.WriteString("  " + normalItemStyle.Render(configPath) + "  " + mutedStyle.Render("("+agent.DisplayName+")"))
		b.WriteString("\n")
	}

	// Warning if some vars not set.
	hasMissing := false
	for _, ev := range m.envStatus {
		if !ev.isSet {
			hasMissing = true
			break
		}
	}
	if hasMissing {
		b.WriteString("\n")
		b.WriteString(warningStyle.Render("! Some vars not set — MCP may not work until added"))
		b.WriteString("\n")
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Env Entry (replaces preview step temporarily)
// ---------------------------------------------------------------------------

type mcpEnvEntryStepModel struct {
	mcpName         string
	envInput        textinput.Model
	envMissingVars  []string
	envCurrentIndex int
	envSaveProject  bool
}

func (m mcpEnvEntryStepModel) Init() tea.Cmd { return textinput.Blink }

func (m mcpEnvEntryStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m mcpEnvEntryStepModel) View() string {
	var b strings.Builder

	total := len(m.envMissingVars)
	current := m.envCurrentIndex + 1

	b.WriteString(fmt.Sprintf("Configure env vars for %s  (%d of %d)",
		selectedItemStyle.Render(m.mcpName), current, total))
	b.WriteString("\n\n")

	varName := m.envMissingVars[m.envCurrentIndex]
	b.WriteString(normalItemStyle.Render(varName))
	b.WriteString("\n\n")

	b.WriteString("Value: " + m.envInput.View())
	b.WriteString("\n\n")

	b.WriteString("Save to:\n")
	if m.envSaveProject {
		b.WriteString(selectedItemStyle.Render("(*)") + " Project  " + mutedStyle.Render(".env.duckrow"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("( ) Global   ~/.duckrow/.env.duckrow"))
		b.WriteString("\n")
	} else {
		b.WriteString(mutedStyle.Render("( ) Project  .env.duckrow"))
		b.WriteString("\n")
		b.WriteString(selectedItemStyle.Render("(*)") + " Global   " + mutedStyle.Render("~/.duckrow/.env.duckrow"))
		b.WriteString("\n")
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Step 3: Installing
// ---------------------------------------------------------------------------

type mcpInstallingStepModel struct {
	spinner spinner.Model
}

func newMCPInstallingStepModel() mcpInstallingStepModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(spinnerStyle),
	)
	return mcpInstallingStepModel{spinner: s}
}

func (m mcpInstallingStepModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m mcpInstallingStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m mcpInstallingStepModel) View() string {
	return m.spinner.View() + " Installing... please wait"
}

// ---------------------------------------------------------------------------
// Help keymap
// ---------------------------------------------------------------------------

type mcpWizardHelpKeyMap struct {
	phase      mcpWizardPhase
	hasEnvVars bool
}

func (k mcpWizardHelpKeyMap) ShortHelp() []key.Binding {
	switch k.phase {
	case mcpPhaseSelectAgents:
		return []key.Binding{
			keys.Up, keys.Down, keys.Toggle, keys.ToggleAll,
			keys.Next, keys.Back,
		}
	case mcpPhasePreview:
		bindings := []key.Binding{keys.Confirm}
		if k.hasEnvVars {
			bindings = append(bindings, keys.Configure)
		}
		bindings = append(bindings, keys.Back)
		return bindings
	case mcpPhaseEnvEntry:
		return []key.Binding{
			keys.Enter, keys.TabSaveLocation, keys.Back,
		}
	case mcpPhaseInstalling:
		return []key.Binding{} // No keys during install.
	}
	return []key.Binding{keys.Enter, keys.Back}
}

func (k mcpWizardHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
