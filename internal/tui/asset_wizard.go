package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/barysiuk/duckrow/internal/core/system"
)

// assetWizardModel wraps a wizardModel for asset install flows.
// Skills: Select Agents → Installing (agent step optional)
// MCPs: Select Agents → Preview → Env Entry (optional) → Installing
type assetWizardModel struct {
	wizard wizardModel

	asset        core.RegistryAssetInfo
	activeFolder string

	allSystems       []system.System
	universalSystems []system.System
	systemBoxes      []agentCheckbox
	systemCursor     int
	targetSystems    []system.System

	// MCP env var status.
	envStatus []envVarStatus

	// Env var entry state.
	envInput        textinput.Model
	envMissingVars  []string
	envCurrentIndex int
	envSaveProject  bool

	installing bool

	app *App
}

// ---------------------------------------------------------------------------
// Skill: Select Agents step
// ---------------------------------------------------------------------------

type skillAgentStepModel struct {
	universalAgents []system.System
	agentBoxes      []agentCheckbox
	agentCursor     int
	skillName       string
}

func newSkillAgentStepModel() skillAgentStepModel {
	return skillAgentStepModel{}
}

func (m skillAgentStepModel) Init() tea.Cmd { return nil }

func (m skillAgentStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m skillAgentStepModel) View() string {
	var b strings.Builder

	desc := mutedStyle.Render("Select which agents should have access to this skill.")
	b.WriteString(desc)
	b.WriteString("\n\n")

	universalLabel := mutedStyle.Render(".agents/skills/ (always installed)")
	b.WriteString(universalLabel)
	b.WriteString("\n")
	for _, a := range m.universalAgents {
		line := "[x] " + a.DisplayName()
		b.WriteString(mutedStyle.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")

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

		line := prefix + check + " " + ab.system.DisplayName()
		skillsDir := ""
		if accessor, ok := ab.system.(interface{ SkillsDir() string }); ok {
			skillsDir = accessor.SkillsDir()
		}
		dirHint := " (" + skillsDir + ")"
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
// MCP: Select Agents step
// ---------------------------------------------------------------------------

type mcpAgentStepModel struct {
	mcp            asset.RegistryEntry
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

		line := prefix + check + " " + ab.system.DisplayName()
		resolvedPath := resolveMCPConfigPathRel(ab.system, m.activeFolder)
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
// MCP: Preview step
// ---------------------------------------------------------------------------

type mcpPreviewStepModel struct {
	mcp          asset.RegistryEntry
	targetAgents []system.System
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

	b.WriteString("Install MCP: ")
	b.WriteString(selectedItemStyle.Render(m.mcp.Name))
	b.WriteString("\n")
	if m.mcp.Description != "" {
		b.WriteString(mutedStyle.Render(m.mcp.Description))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	meta, ok := m.mcp.Meta.(asset.MCPMeta)
	if ok && meta.URL != "" {
		b.WriteString("URL:      " + normalItemStyle.Render(meta.URL))
		b.WriteString("\n")
		if meta.Transport != "" {
			b.WriteString("Type:     " + mutedStyle.Render(meta.Transport))
			b.WriteString("\n")
		}
	} else {
		cmdStr := meta.Command
		if len(meta.Args) > 0 {
			cmdStr += " " + strings.Join(meta.Args, " ")
		}
		b.WriteString("Command:  " + normalItemStyle.Render(cmdStr))
		b.WriteString("\n")
	}

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

	b.WriteString("Will write to:\n")
	for _, sys := range m.targetAgents {
		configPath := resolveMCPConfigPathRel(sys, m.activeFolder)
		b.WriteString("  " + normalItemStyle.Render(configPath) + "  " + mutedStyle.Render("("+sys.DisplayName()+")"))
		b.WriteString("\n")
	}

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
// MCP: Env Entry step
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

func resolveMCPConfigPathRel(sys system.System, projectDir string) string {
	if resolver, ok := sys.(interface{ ResolveMCPConfigPathRel(string) string }); ok {
		return resolver.ResolveMCPConfigPathRel(projectDir)
	}
	return ""
}

func newAssetWizardModel() assetWizardModel {
	return assetWizardModel{}
}

// activate initializes the asset wizard with the selected asset data.
func (m assetWizardModel) activate(msg openAssetWizardMsg, app *App, width, height int) assetWizardModel {
	m.app = app
	m.asset = msg.asset
	m.activeFolder = msg.activeFolder
	m.allSystems = msg.allSystems
	m.installing = false
	m.targetSystems = nil
	m.envStatus = nil
	m.envMissingVars = nil

	activeSystemNames := system.DisplayNames(system.ActiveInFolder(msg.activeFolder))
	activeSet := make(map[string]bool, len(activeSystemNames))
	for _, name := range activeSystemNames {
		activeSet[name] = true
	}

	switch m.asset.Kind {
	case asset.KindSkill:
		m.universalSystems = system.Universal()
		nonUniversal := system.NonUniversal()
		m.systemBoxes = make([]agentCheckbox, len(nonUniversal))
		for i, s := range nonUniversal {
			m.systemBoxes[i] = agentCheckbox{system: s, checked: activeSet[s.DisplayName()]}
		}
		m.systemCursor = 0

		installStep := newAssetInstallingStepModel()
		if len(nonUniversal) == 0 {
			m.wizard = newWizardModel("Install Skill", []wizardStep{{name: "Installing", content: installStep}})
		} else {
			selectStep := newSkillAgentStepModel()
			m.wizard = newWizardModel("Install Skill", []wizardStep{
				{name: "Select Agents", content: selectStep},
				{name: "Installing", content: installStep},
			})
		}
	case asset.KindMCP:
		mcpCapable := system.Supporting(asset.KindMCP)
		m.systemBoxes = make([]agentCheckbox, len(mcpCapable))
		for i, s := range mcpCapable {
			m.systemBoxes[i] = agentCheckbox{system: s, checked: activeSet[s.DisplayName()]}
		}
		m.systemCursor = 0

		selectStep := newMCPAgentStepModel()
		previewStep := newMCPPreviewStepModel()
		installStep := newAssetInstallingStepModel()
		m.wizard = newWizardModel("Install MCP", []wizardStep{
			{name: "Select Agents", content: selectStep},
			{name: "Preview", content: previewStep},
			{name: "Installing", content: installStep},
		})
	default:
		supported := system.Supporting(m.asset.Kind)
		m.systemBoxes = make([]agentCheckbox, len(supported))
		for i, s := range supported {
			m.systemBoxes[i] = agentCheckbox{system: s, checked: activeSet[s.DisplayName()]}
		}
		m.systemCursor = 0

		title := "Install " + string(m.asset.Kind)
		if handler, ok := asset.Get(m.asset.Kind); ok {
			title = "Install " + handler.DisplayName()
		}
		selectStep := newGenericAgentStepModel()
		installStep := newAssetInstallingStepModel()
		m.wizard = newWizardModel(title, []wizardStep{
			{name: "Select Agents", content: selectStep},
			{name: "Installing", content: installStep},
		})
	}

	m.wizard = m.wizard.setSize(width, height)
	return m
}

func (m assetWizardModel) setSize(width, height int) assetWizardModel {
	m.wizard = m.wizard.setSize(width, height)
	return m
}

func (m assetWizardModel) isInstalling() bool {
	return m.installing
}

func (m assetWizardModel) selectedRegistryAssetInfo() core.RegistryAssetInfo {
	return m.asset
}

func (m assetWizardModel) selectedTargetSystems() []system.System {
	var systems []system.System
	for _, box := range m.systemBoxes {
		if box.checked {
			systems = append(systems, box.system)
		}
	}
	return systems
}

func (m assetWizardModel) canAutoInstall() bool {
	return m.asset.Kind == asset.KindSkill && len(m.systemBoxes) == 0
}

type assetWizardPhase int

const (
	assetPhaseSelectAgents assetWizardPhase = iota
	assetPhasePreview
	assetPhaseEnvEntry
	assetPhaseInstalling
)

func (m assetWizardModel) currentPhase() assetWizardPhase {
	if m.installing {
		return assetPhaseInstalling
	}
	if m.asset.Kind == asset.KindMCP {
		switch m.wizard.activeIdx {
		case 0:
			return assetPhaseSelectAgents
		case 1:
			if _, ok := m.wizard.steps[1].content.(mcpEnvEntryStepModel); ok {
				return assetPhaseEnvEntry
			}
			return assetPhasePreview
		default:
			return assetPhaseInstalling
		}
	}
	if len(m.wizard.steps) > 1 && m.wizard.activeIdx == 0 {
		return assetPhaseSelectAgents
	}
	return assetPhaseInstalling
}

func (m assetWizardModel) currentHelpKeyMap() assetWizardHelpKeyMap {
	return assetWizardHelpKeyMap{phase: m.currentPhase(), hasEnvVars: len(m.envStatus) > 0}
}

func (m assetWizardModel) update(msg tea.Msg, app *App) (assetWizardModel, tea.Cmd) {
	m.app = app

	switch msg.(type) {
	case wizardDoneMsg, wizardBackMsg:
		return m, nil

	case wizardNextMsg:
		return m.handleNext()

	case assetInstalledMsg:
		m.installing = false
		return m, nil

	case envSaveDoneMsg:
		return m.advanceEnvEntry()
	}

	if m.asset.Kind == asset.KindSkill {
		return m.handleSkillKeys(msg)
	}
	if m.asset.Kind == asset.KindMCP {
		switch m.currentPhase() {
		case assetPhaseSelectAgents:
			return m.handleAgentSelectKey(msg)
		case assetPhasePreview:
			return m.handlePreviewKey(msg)
		case assetPhaseEnvEntry:
			return m.handleEnvEntryKey(msg)
		case assetPhaseInstalling:
			return m.handleInstalling(msg)
		}
	}

	return m.handleGenericKeys(msg)
}

func (m assetWizardModel) handleNext() (assetWizardModel, tea.Cmd) {
	currentIdx := m.wizard.activeIdx

	if m.asset.Kind == asset.KindSkill {
		var cmd tea.Cmd
		m.wizard, cmd = m.wizard.update(wizardNextMsg{})
		installCmd := m.startInstall()
		if installCmd != nil {
			return m, tea.Batch(cmd, installCmd)
		}
		return m, cmd
	}

	if m.asset.Kind == asset.KindMCP {
		if currentIdx == 0 {
			m.targetSystems = m.selectedTargetSystems()
			if len(m.targetSystems) == 0 {
				return m, nil
			}

			m.resolveEnvStatus()
			m.wizard.steps[1].content = mcpPreviewStepModel{
				mcp:          m.asset.Entry,
				targetAgents: m.targetSystems,
				envStatus:    m.envStatus,
				activeFolder: m.activeFolder,
			}

			var cmd tea.Cmd
			m.wizard, cmd = m.wizard.update(wizardNextMsg{})
			return m, cmd
		}
		if currentIdx == 1 {
			var cmd tea.Cmd
			m.wizard, cmd = m.wizard.update(wizardNextMsg{})
			installCmd := m.startInstall()
			return m, tea.Batch(cmd, installCmd)
		}
		return m, nil
	}

	if currentIdx == 0 {
		m.targetSystems = m.selectedTargetSystems()
		var cmd tea.Cmd
		m.wizard, cmd = m.wizard.update(wizardNextMsg{})
		installCmd := m.startInstall()
		return m, tea.Batch(cmd, installCmd)
	}

	return m, nil
}

func (m assetWizardModel) handleSkillKeys(msg tea.Msg) (assetWizardModel, tea.Cmd) {
	if m.currentPhase() == assetPhaseSelectAgents {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(keyMsg, keys.Up):
				if m.systemCursor > 0 {
					m.systemCursor--
				}
				return m, nil
			case key.Matches(keyMsg, keys.Down):
				if m.systemCursor < len(m.systemBoxes)-1 {
					m.systemCursor++
				}
				return m, nil
			case key.Matches(keyMsg, keys.Toggle):
				if len(m.systemBoxes) > 0 {
					m.systemBoxes[m.systemCursor].checked = !m.systemBoxes[m.systemCursor].checked
				}
				return m, nil
			case key.Matches(keyMsg, keys.ToggleAll):
				m.toggleAllAgents()
				return m, nil
			case key.Matches(keyMsg, keys.Enter):
				return m, func() tea.Msg { return wizardNextMsg{} }
			}
		}
	}

	if m.installing {
		return m.handleInstalling(msg)
	}

	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.update(msg)
	return m, cmd
}

func (m assetWizardModel) handleGenericKeys(msg tea.Msg) (assetWizardModel, tea.Cmd) {
	if m.currentPhase() == assetPhaseSelectAgents {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(keyMsg, keys.Up):
				if m.systemCursor > 0 {
					m.systemCursor--
				}
				return m, nil
			case key.Matches(keyMsg, keys.Down):
				if m.systemCursor < len(m.systemBoxes)-1 {
					m.systemCursor++
				}
				return m, nil
			case key.Matches(keyMsg, keys.Toggle):
				if len(m.systemBoxes) > 0 {
					m.systemBoxes[m.systemCursor].checked = !m.systemBoxes[m.systemCursor].checked
				}
				return m, nil
			case key.Matches(keyMsg, keys.ToggleAll):
				m.toggleAllAgents()
				return m, nil
			case key.Matches(keyMsg, keys.Enter):
				return m, func() tea.Msg { return wizardNextMsg{} }
			}
		}
	}

	if m.installing {
		return m.handleInstalling(msg)
	}

	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.update(msg)
	return m, cmd
}

func (m assetWizardModel) handleInstalling(msg tea.Msg) (assetWizardModel, tea.Cmd) {
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

func (m assetWizardModel) handleAgentSelectKey(msg tea.Msg) (assetWizardModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Up):
			if m.systemCursor > 0 {
				m.systemCursor--
			}
			return m, nil
		case key.Matches(keyMsg, keys.Down):
			if m.systemCursor < len(m.systemBoxes)-1 {
				m.systemCursor++
			}
			return m, nil
		case key.Matches(keyMsg, keys.Toggle):
			if len(m.systemBoxes) > 0 {
				m.systemBoxes[m.systemCursor].checked = !m.systemBoxes[m.systemCursor].checked
			}
			return m, nil
		case key.Matches(keyMsg, keys.ToggleAll):
			m.toggleAllAgents()
			return m, nil
		case key.Matches(keyMsg, keys.Enter):
			return m, func() tea.Msg { return wizardNextMsg{} }
		}
	}

	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.update(msg)
	return m, cmd
}

func (m assetWizardModel) handlePreviewKey(msg tea.Msg) (assetWizardModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Enter):
			return m, func() tea.Msg { return wizardNextMsg{} }
		case key.Matches(keyMsg, keys.Configure):
			return m.startEnvEntry()
		}
	}

	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.update(msg)
	return m, cmd
}

func (m assetWizardModel) handleEnvEntryKey(msg tea.Msg) (assetWizardModel, tea.Cmd) {
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
			return m.advanceEnvEntry()
		}
	}

	var cmd tea.Cmd
	m.envInput, cmd = m.envInput.Update(msg)
	return m, cmd
}

func (m assetWizardModel) view() string {
	step := m.wizard.activeStep()
	if step == nil {
		return ""
	}

	switch step.content.(type) {
	case skillAgentStepModel:
		step.content = skillAgentStepModel{
			universalAgents: m.universalSystems,
			agentBoxes:      m.systemBoxes,
			agentCursor:     m.systemCursor,
			skillName:       m.asset.Entry.Name,
		}
	case mcpAgentStepModel:
		step.content = mcpAgentStepModel{
			mcp:            m.asset.Entry,
			mcpAgentBoxes:  m.systemBoxes,
			mcpAgentCursor: m.systemCursor,
			activeFolder:   m.activeFolder,
		}
	case mcpPreviewStepModel:
		step.content = mcpPreviewStepModel{
			mcp:          m.asset.Entry,
			targetAgents: m.targetSystems,
			envStatus:    m.envStatus,
			activeFolder: m.activeFolder,
		}
	case mcpEnvEntryStepModel:
		step.content = mcpEnvEntryStepModel{
			mcpName:         m.asset.Entry.Name,
			envInput:        m.envInput,
			envMissingVars:  m.envMissingVars,
			envCurrentIndex: m.envCurrentIndex,
			envSaveProject:  m.envSaveProject,
		}
	case genericAgentStepModel:
		step.content = genericAgentStepModel{
			assetName:   m.asset.Entry.Name,
			assetKind:   m.asset.Kind,
			agentBoxes:  m.systemBoxes,
			agentCursor: m.systemCursor,
		}
	}

	return m.wizard.view()
}

func (m *assetWizardModel) startInstall() tea.Cmd {
	m.installing = true

	assetInfo := m.asset
	folder := m.activeFolder
	app := m.app

	installCmd := func() tea.Msg {
		switch assetInfo.Kind {
		case asset.KindSkill:
			sourceStr := assetInfo.Entry.Source
			if sourceStr == "" {
				return assetInstalledMsg{kind: assetInfo.Kind, name: assetInfo.Entry.Name, folder: folder, err: fmt.Errorf("missing source")}
			}
			source, err := core.ParseSource(sourceStr)
			if err != nil {
				return assetInstalledMsg{kind: assetInfo.Kind, name: assetInfo.Entry.Name, folder: folder, err: fmt.Errorf("parsing source %q: %w", sourceStr, err)}
			}

			cfg, cfgErr := app.config.Load()
			if cfgErr == nil {
				source.ApplyCloneURLOverride(cfg.Settings.CloneURLOverrides)
			}

			var registryCommit string
			if assetInfo.Entry.Commit != "" {
				registryCommit = assetInfo.Entry.Commit
			}

			targetSystems := m.selectedTargetSystems()
			results, err := app.orch.InstallFromSource(source, asset.KindSkill, core.OrchestratorInstallOptions{
				TargetDir:       folder,
				TargetSystems:   targetSystems,
				IncludeInternal: true,
				Commit:          registryCommit,
			})
			if err != nil {
				return assetInstalledMsg{kind: assetInfo.Kind, name: assetInfo.Entry.Name, folder: folder, err: err}
			}

			for _, r := range results {
				entry := asset.LockedAsset{
					Kind:   asset.KindSkill,
					Name:   r.Asset.Name,
					Source: r.Asset.Source,
					Commit: r.Commit,
					Ref:    r.Ref,
				}
				_ = core.AddOrUpdateAsset(folder, entry)
			}

			return assetInstalledMsg{kind: assetInfo.Kind, name: assetInfo.Entry.Name, folder: folder}
		case asset.KindMCP:
			meta, ok := assetInfo.Entry.Meta.(asset.MCPMeta)
			if !ok {
				return assetInstalledMsg{kind: assetInfo.Kind, name: assetInfo.Entry.Name, folder: folder, err: fmt.Errorf("invalid MCP metadata")}
			}

			mcpAsset := asset.Asset{
				Kind:        asset.KindMCP,
				Name:        assetInfo.Entry.Name,
				Description: assetInfo.Entry.Description,
				Meta:        meta,
			}

			targetSystems := m.targetSystems
			for _, sys := range targetSystems {
				if err := sys.Install(mcpAsset, folder, system.InstallOptions{}); err != nil {
					return assetInstalledMsg{kind: assetInfo.Kind, name: assetInfo.Entry.Name, folder: folder, err: err}
				}
			}

			lockEntry := asset.LockedAsset{
				Kind: asset.KindMCP,
				Name: mcpAsset.Name,
				Data: map[string]any{
					"registry":   assetInfo.RegistryRepo,
					"configHash": core.ComputeConfigHash(meta),
					"systems":    system.Names(targetSystems),
				},
			}
			if required := core.ExtractRequiredEnv(meta.Env); len(required) > 0 {
				lockEntry.Data["requiredEnv"] = required
			}
			_ = core.AddOrUpdateAsset(folder, lockEntry)

			return assetInstalledMsg{kind: assetInfo.Kind, name: assetInfo.Entry.Name, folder: folder}
		default:
			return assetInstalledMsg{kind: assetInfo.Kind, name: assetInfo.Entry.Name, folder: folder, err: fmt.Errorf("unsupported asset kind %s", assetInfo.Kind)}
		}
	}

	step := m.wizard.activeStep()
	if step != nil {
		if is, ok := step.content.(assetInstallingStepModel); ok {
			return tea.Batch(is.spinner.Tick, installCmd)
		}
	}

	return installCmd
}

func (m *assetWizardModel) toggleAllAgents() {
	anyChecked := false
	for _, ab := range m.systemBoxes {
		if ab.checked {
			anyChecked = true
			break
		}
	}
	for i := range m.systemBoxes {
		m.systemBoxes[i].checked = !anyChecked
	}
}

// ---------------------------------------------------------------------------
// Generic: Select Agents step
// ---------------------------------------------------------------------------

type genericAgentStepModel struct {
	assetName   string
	assetKind   asset.Kind
	agentBoxes  []agentCheckbox
	agentCursor int
}

func newGenericAgentStepModel() genericAgentStepModel {
	return genericAgentStepModel{}
}

func (m genericAgentStepModel) Init() tea.Cmd { return nil }

func (m genericAgentStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m genericAgentStepModel) View() string {
	var b strings.Builder

	label := strings.ToUpper(string(m.assetKind))
	if handler, ok := asset.Get(m.assetKind); ok {
		label = handler.DisplayName()
	}

	b.WriteString("Install " + label + ": ")
	b.WriteString(selectedItemStyle.Render(m.assetName))
	b.WriteString("\n\n")

	desc := mutedStyle.Render("Select target agents:")
	b.WriteString(desc)
	b.WriteString("\n\n")

	for i, ab := range m.agentBoxes {
		check := "[ ]"
		if ab.checked {
			check = "[x]"
		}

		prefix := "  "
		if i == m.agentCursor {
			prefix = "> "
		}

		line := prefix + check + " " + ab.system.DisplayName()
		if i == m.agentCursor {
			b.WriteString(selectedItemStyle.Render(line))
		} else {
			b.WriteString(normalItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Press enter to continue"))

	return b.String()
}

// ---------------------------------------------------------------------------
// Installing step
// ---------------------------------------------------------------------------

type assetInstallingStepModel struct {
	spinner spinner.Model
}

func newAssetInstallingStepModel() assetInstallingStepModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(spinnerStyle),
	)
	return assetInstallingStepModel{spinner: s}
}

func (m assetInstallingStepModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m assetInstallingStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m assetInstallingStepModel) View() string {
	return m.spinner.View() + " Installing... please wait"
}

// ---------------------------------------------------------------------------
// MCP env resolution helpers
// ---------------------------------------------------------------------------

func (m *assetWizardModel) resolveEnvStatus() {
	m.envStatus = nil
	meta, ok := m.asset.Entry.Meta.(asset.MCPMeta)
	if ok && meta.URL == "" && len(meta.Env) > 0 {
		requiredVars := core.ExtractRequiredEnv(meta.Env)
		if len(requiredVars) > 0 {
			resolver := core.NewEnvResolver(m.activeFolder, "")
			results := resolver.ResolveEnvWithSource(requiredVars)
			for _, r := range results {
				status := envVarStatus{name: r.Name}
				if r.Source != "" {
					status.isSet = true
					status.source = string(r.Source)
				}
				m.envStatus = append(m.envStatus, status)
			}
		}
	}
}

func (m assetWizardModel) startEnvEntry() (assetWizardModel, tea.Cmd) {
	m.envMissingVars = nil
	for _, ev := range m.envStatus {
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

	m.wizard.steps[1].content = mcpEnvEntryStepModel{
		mcpName:         m.asset.Entry.Name,
		envInput:        m.envInput,
		envMissingVars:  m.envMissingVars,
		envCurrentIndex: m.envCurrentIndex,
		envSaveProject:  m.envSaveProject,
	}
	m.wizard.steps[1].name = "Configure"

	return m, textinput.Blink
}

func (m assetWizardModel) saveAndAdvanceEnvEntry() (assetWizardModel, tea.Cmd) {
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

func (m assetWizardModel) advanceEnvEntry() (assetWizardModel, tea.Cmd) {
	m.envCurrentIndex++

	if m.envCurrentIndex >= len(m.envMissingVars) {
		m.resolveEnvStatus()
		m.wizard.steps[1].name = "Preview"
		m.wizard.steps[1].content = mcpPreviewStepModel{
			mcp:          m.asset.Entry,
			targetAgents: m.targetSystems,
			envStatus:    m.envStatus,
			activeFolder: m.activeFolder,
		}
		return m, nil
	}

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

// ---------------------------------------------------------------------------
// Help keymap
// ---------------------------------------------------------------------------

type assetWizardHelpKeyMap struct {
	phase      assetWizardPhase
	hasEnvVars bool
}

func (k assetWizardHelpKeyMap) ShortHelp() []key.Binding {
	switch k.phase {
	case assetPhaseSelectAgents:
		return []key.Binding{keys.Up, keys.Down, keys.Toggle, keys.ToggleAll, keys.Next, keys.Back}
	case assetPhasePreview:
		bindings := []key.Binding{keys.Confirm}
		if k.hasEnvVars {
			bindings = append(bindings, keys.Configure)
		}
		bindings = append(bindings, keys.Back)
		return bindings
	case assetPhaseEnvEntry:
		return []key.Binding{keys.Enter, keys.TabSaveLocation, keys.Back}
	case assetPhaseInstalling:
		return []key.Binding{}
	}
	return []key.Binding{keys.Enter, keys.Back}
}

func (k assetWizardHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
