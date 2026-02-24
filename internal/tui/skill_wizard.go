package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
)

// openSkillWizardMsg is emitted by installModel when a skill is selected.
// It carries all the data needed to start the skill install wizard.
type openSkillWizardMsg struct {
	skill           core.RegistrySkillInfo
	universalAgents []core.AgentDef
	nonUniversal    []core.AgentDef
	activeFolder    string
}

// skillWizardModel wraps a wizardModel for the skill install flow.
// Steps: Select Agents → Installing.
// If there are no non-universal agents, the wizard skips straight to installing.
type skillWizardModel struct {
	wizard wizardModel

	// Skill being installed.
	skill        core.RegistrySkillInfo
	activeFolder string

	// Agent data.
	universalAgents []core.AgentDef
	agentBoxes      []agentCheckbox
	agentCursor     int

	// Install state.
	installing bool

	// App reference.
	app *App
}

func newSkillWizardModel() skillWizardModel {
	return skillWizardModel{}
}

// activate initializes the skill wizard with the selected skill data.
func (m skillWizardModel) activate(msg openSkillWizardMsg, app *App, width, height int) skillWizardModel {
	m.app = app
	m.skill = msg.skill
	m.activeFolder = msg.activeFolder
	m.universalAgents = msg.universalAgents
	m.installing = false

	// Build agent checkboxes — pre-select non-universal agents that are
	// actively used in this folder (matching the sidebar's detection).
	activeAgentNames := core.DetectActiveAgents(app.agents, msg.activeFolder)
	activeSet := make(map[string]bool, len(activeAgentNames))
	for _, name := range activeAgentNames {
		activeSet[name] = true
	}
	m.agentBoxes = make([]agentCheckbox, len(msg.nonUniversal))
	for i, a := range msg.nonUniversal {
		m.agentBoxes[i] = agentCheckbox{agent: a, checked: activeSet[a.DisplayName]}
	}
	m.agentCursor = 0

	if len(msg.nonUniversal) == 0 {
		// No non-universal agents — single-step wizard (just installing).
		installStep := newSkillInstallingStepModel()
		m.wizard = newWizardModel("Install Skill", []wizardStep{
			{name: "Installing", content: installStep},
		})
	} else {
		// Two-step wizard: Select Agents → Installing.
		agentStep := newSkillAgentStepModel()
		installStep := newSkillInstallingStepModel()
		m.wizard = newWizardModel("Install Skill", []wizardStep{
			{name: "Select Agents", content: agentStep},
			{name: "Installing", content: installStep},
		})
	}
	m.wizard = m.wizard.setSize(width, height)

	return m
}

// setSize updates the layout dimensions.
func (m skillWizardModel) setSize(width, height int) skillWizardModel {
	m.wizard = m.wizard.setSize(width, height)
	return m
}

// selectedSkillInfo returns the skill being installed.
// Used by app.go for clone error context.
func (m skillWizardModel) selectedSkillInfo() core.RegistrySkillInfo {
	return m.skill
}

// selectedTargetAgents returns the checked non-universal agents.
// Used by app.go for clone error context.
func (m skillWizardModel) selectedTargetAgents() []core.AgentDef {
	var agents []core.AgentDef
	for _, ab := range m.agentBoxes {
		if ab.checked {
			agents = append(agents, ab.agent)
		}
	}
	return agents
}

// isInstalling returns true if the skill install is in progress.
func (m skillWizardModel) isInstalling() bool {
	return m.installing
}

// isSelectingAgents returns true if the agent selection step is active.
func (m skillWizardModel) isSelectingAgents() bool {
	if len(m.wizard.steps) == 1 {
		return false // No agent selection step (universal-only)
	}
	return m.wizard.activeIdx == 0
}

// currentHelpKeyMap returns the help keymap for the current wizard step.
func (m skillWizardModel) currentHelpKeyMap() skillWizardHelpKeyMap {
	return skillWizardHelpKeyMap{
		selectingAgents: m.isSelectingAgents(),
		installing:      m.isInstalling(),
	}
}

// update handles messages for the skill wizard.
func (m skillWizardModel) update(msg tea.Msg, app *App) (skillWizardModel, tea.Cmd) {
	m.app = app

	switch msg := msg.(type) {
	case wizardDoneMsg, wizardBackMsg:
		// Handled by app.go.
		return m, nil

	case wizardNextMsg:
		// Agent selection completed — start the install.
		// Capture data before the wizard advances.
		var cmd tea.Cmd
		m.wizard, cmd = m.wizard.update(msg)

		// Start the install after advancing.
		installCmd := m.startInstall()
		if installCmd != nil {
			return m, tea.Batch(cmd, installCmd)
		}
		return m, cmd

	case installDoneMsg:
		// Install completed — update the installing step.
		m.installing = false
		// app.go handles the result (success/error/clone error).
		return m, nil
	}

	// Handle agent selection keys when on that step.
	if m.isSelectingAgents() {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(keyMsg, keys.Up):
				if m.agentCursor > 0 {
					m.agentCursor--
				}
				return m, nil
			case key.Matches(keyMsg, keys.Down):
				if m.agentCursor < len(m.agentBoxes)-1 {
					m.agentCursor++
				}
				return m, nil
			case key.Matches(keyMsg, keys.Toggle):
				if len(m.agentBoxes) > 0 {
					m.agentBoxes[m.agentCursor].checked = !m.agentBoxes[m.agentCursor].checked
				}
				return m, nil
			case key.Matches(keyMsg, keys.ToggleAll):
				m.toggleAllAgents()
				return m, nil
			case key.Matches(keyMsg, keys.Enter):
				// Emit wizardNextMsg to advance to installing step.
				return m, func() tea.Msg { return wizardNextMsg{} }
			}
		}
	}

	// Installing step: only handle spinner ticks.
	if m.installing {
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

	// Forward to the wizard for esc handling.
	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.update(msg)
	return m, cmd
}

// view renders the wizard.
func (m skillWizardModel) view() string {
	// Inject current state into the active step's view.
	step := m.wizard.activeStep()
	if step == nil {
		return ""
	}

	switch step.content.(type) {
	case skillAgentStepModel:
		// Update the step with current agent selection state.
		step.content = skillAgentStepModel{
			universalAgents: m.universalAgents,
			agentBoxes:      m.agentBoxes,
			agentCursor:     m.agentCursor,
			skillName:       m.skill.Skill.Name,
		}
	}

	return m.wizard.view()
}

// startInstall kicks off the skill installation.
func (m *skillWizardModel) startInstall() tea.Cmd {
	m.installing = true

	skill := m.skill
	folder := m.activeFolder
	app := m.app

	// Build target agents from checked boxes.
	var targetAgents []core.AgentDef
	for _, ab := range m.agentBoxes {
		if ab.checked {
			targetAgents = append(targetAgents, ab.agent)
		}
	}

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

	// Get the installing step's spinner tick.
	step := m.wizard.activeStep()
	if step != nil {
		if is, ok := step.content.(skillInstallingStepModel); ok {
			return tea.Batch(is.spinner.Tick, installCmd)
		}
	}

	return installCmd
}

// toggleAllAgents toggles all agents: if any are checked, uncheck all; otherwise check all.
func (m *skillWizardModel) toggleAllAgents() {
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

// ---------------------------------------------------------------------------
// Step 1: Select Agents
// ---------------------------------------------------------------------------

// skillAgentStepModel renders the agent selection UI for a skill install.
type skillAgentStepModel struct {
	universalAgents []core.AgentDef
	agentBoxes      []agentCheckbox
	agentCursor     int
	skillName       string
}

func newSkillAgentStepModel() skillAgentStepModel {
	return skillAgentStepModel{}
}

func (m skillAgentStepModel) Init() tea.Cmd { return nil }

func (m skillAgentStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// All key handling is done by the parent skillWizardModel.
	return m, nil
}

func (m skillAgentStepModel) View() string {
	var b strings.Builder

	desc := mutedStyle.Render("Select which agents should have access to this skill.")
	b.WriteString(desc)
	b.WriteString("\n\n")

	// Universal agents — always selected, not toggleable.
	universalLabel := mutedStyle.Render(".agents/skills/ (always installed)")
	b.WriteString(universalLabel)
	b.WriteString("\n")
	for _, a := range m.universalAgents {
		line := "[x] " + a.DisplayName
		b.WriteString(mutedStyle.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Non-universal agents — toggleable checkboxes.
	symlinkLabel := mutedStyle.Render("Agent-specific (optional)")
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

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Press enter to continue"))

	return b.String()
}

// ---------------------------------------------------------------------------
// Step 2: Installing
// ---------------------------------------------------------------------------

// skillInstallingStepModel shows a spinner while the skill is being installed.
type skillInstallingStepModel struct {
	spinner spinner.Model
}

func newSkillInstallingStepModel() skillInstallingStepModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(spinnerStyle),
	)
	return skillInstallingStepModel{spinner: s}
}

func (m skillInstallingStepModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m skillInstallingStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m skillInstallingStepModel) View() string {
	return m.spinner.View() + " Installing... please wait"
}

// ---------------------------------------------------------------------------
// Help keymap
// ---------------------------------------------------------------------------

// skillWizardHelpKeyMap provides context-sensitive help for the skill wizard.
type skillWizardHelpKeyMap struct {
	selectingAgents bool
	installing      bool
}

func (k skillWizardHelpKeyMap) ShortHelp() []key.Binding {
	if k.installing {
		return []key.Binding{} // No keys during install.
	}
	if k.selectingAgents {
		return []key.Binding{
			keys.Up, keys.Down, keys.Toggle, keys.ToggleAll,
			keys.Next, keys.Back,
		}
	}
	return []key.Binding{keys.Enter, keys.Back}
}

func (k skillWizardHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
