package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
)

// registryWizardModel wraps a wizardModel for the registry-add flow.
// Two steps: Enter URL → Confirm / clone result.
type registryWizardModel struct {
	wizard wizardModel

	// Resolved data passed between steps.
	url string // URL entered in step 1.

	// App reference (set on activate, used for clone commands).
	app *App
}

func newRegistryWizardModel() registryWizardModel {
	return registryWizardModel{}
}

// activate initializes the registry wizard with fresh steps.
func (m registryWizardModel) activate(app *App, width, height int) registryWizardModel {
	m.app = app
	m.url = ""

	urlStep := newRegURLStepModel()
	cloneStep := newRegCloneStepModel()

	m.wizard = newWizardModel("Add Registry", []wizardStep{
		{name: "Enter URL", content: urlStep},
		{name: "Confirm", content: cloneStep},
	})
	m.wizard = m.wizard.setSize(width, height)

	return m
}

// setSize updates the layout dimensions.
func (m registryWizardModel) setSize(width, height int) registryWizardModel {
	m.wizard = m.wizard.setSize(width, height)
	return m
}

// update handles messages for the registry wizard.
func (m registryWizardModel) update(msg tea.Msg, app *App) (registryWizardModel, tea.Cmd) {
	m.app = app

	switch msg := msg.(type) {
	case wizardDoneMsg:
		// The wizard completed — app.go handles this to return to settings.
		return m, nil

	case wizardBackMsg:
		// The wizard was cancelled — app.go handles this to return to settings.
		return m, nil

	case wizardNextMsg:
		// Step is advancing. Capture data from the completed step before
		// the wizard increments activeIdx.
		if m.wizard.activeIdx == 0 {
			// Step 1 completed — capture the URL.
			if step, ok := m.wizard.steps[0].content.(regURLStepModel); ok {
				m.url = step.input.Value()
			}
		}

		// Let the wizard handle the index advancement.
		var cmd tea.Cmd
		m.wizard, cmd = m.wizard.update(msg)

		// If we just moved to step 2, start the clone.
		if m.wizard.activeIdx == 1 {
			cloneStep := m.wizard.steps[1].content.(regCloneStepModel)
			cloneStep = cloneStep.startClone(m.url)
			m.wizard.steps[1].content = cloneStep

			cloneCmd := m.makeCloneCmd(m.url)
			return m, tea.Batch(cmd, cloneStep.spinner.Tick, cloneCmd)
		}

		return m, cmd

	case registryAddDoneMsg:
		// Clone completed — update step 2.
		if m.wizard.activeIdx == 1 {
			cloneStep := m.wizard.steps[1].content.(regCloneStepModel)
			cloneStep = cloneStep.handleResult(msg)
			m.wizard.steps[1].content = cloneStep
		}
		return m, nil
	}

	// Handle esc specially: if we're on step 2 and cloning, ignore esc.
	if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, keys.Back) {
		if m.wizard.activeIdx == 1 {
			cloneStep := m.wizard.steps[1].content.(regCloneStepModel)
			if cloneStep.cloning {
				// Don't go back while cloning.
				return m, nil
			}
			if cloneStep.err != nil {
				// On error, esc goes back to step 1 (re-enter URL).
				m.wizard.activeIdx = 0
				// Reset step 1 with the previous URL.
				urlStep := m.wizard.steps[0].content.(regURLStepModel)
				urlStep.input.SetValue(m.url)
				m.wizard.steps[0].content = urlStep
				return m, urlStep.input.Cursor.BlinkCmd()
			}
		}
	}

	// Forward to the wizard.
	var cmd tea.Cmd
	m.wizard, cmd = m.wizard.update(msg)
	return m, cmd
}

// view renders the wizard.
func (m registryWizardModel) view() string {
	return m.wizard.view()
}

// makeCloneCmd creates the command that adds the registry.
func (m registryWizardModel) makeCloneCmd(url string) tea.Cmd {
	app := m.app
	return func() tea.Msg {
		regMgr := core.NewRegistryManager(app.config.RegistriesDir())
		manifest, err := regMgr.Add(url)
		if err != nil {
			return registryAddDoneMsg{url: url, err: fmt.Errorf("adding registry: %w", err)}
		}

		// Save registry to config (skip if same repo already exists).
		cfg, err := app.config.Load()
		if err != nil {
			return registryAddDoneMsg{url: url, err: err}
		}
		for _, r := range cfg.Registries {
			if r.Repo == url {
				// Same repo already registered — report success.
				return registryAddDoneMsg{url: url, name: manifest.Name, warnings: manifest.Warnings}
			}
		}
		cfg.Registries = append(cfg.Registries, core.Registry{
			Name: manifest.Name,
			Repo: url,
		})
		if err := app.config.Save(cfg); err != nil {
			return registryAddDoneMsg{url: url, err: err}
		}
		return registryAddDoneMsg{url: url, name: manifest.Name, warnings: manifest.Warnings}
	}
}

// ---------------------------------------------------------------------------
// Step 1: Enter URL
// ---------------------------------------------------------------------------

// regURLStepModel is the first step: a text input for the registry git URL.
type regURLStepModel struct {
	input textinput.Model
}

func newRegURLStepModel() regURLStepModel {
	ti := textinput.New()
	ti.Placeholder = "Git repository URL..."
	ti.CharLimit = 256
	ti.Width = 60
	ti.Focus()
	return regURLStepModel{input: ti}
}

func (m regURLStepModel) Init() tea.Cmd {
	return m.input.Cursor.BlinkCmd()
}

func (m regURLStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, keys.Enter) {
			value := m.input.Value()
			if value != "" {
				// Emit wizardNextMsg to advance to step 2.
				return m, func() tea.Msg { return wizardNextMsg{} }
			}
			return m, nil
		}
		// Don't let esc propagate from here — wizard handles it.
		if key.Matches(msg, keys.Back) {
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m regURLStepModel) View() string {
	return "Registry URL:\n\n" + m.input.View()
}

// ---------------------------------------------------------------------------
// Step 2: Confirm / clone result
// ---------------------------------------------------------------------------

// regCloneStepModel is the second step: shows a spinner while cloning,
// then success or error result.
type regCloneStepModel struct {
	spinner spinner.Model
	cloning bool
	url     string

	// Result (nil while cloning).
	name     string
	warnings []string
	err      error
	done     bool
}

func newRegCloneStepModel() regCloneStepModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(spinnerStyle),
	)
	return regCloneStepModel{spinner: s}
}

// startClone resets the step and marks it as cloning.
func (m regCloneStepModel) startClone(url string) regCloneStepModel {
	m.url = url
	m.cloning = true
	m.done = false
	m.err = nil
	m.name = ""
	m.warnings = nil
	return m
}

// handleResult processes the clone result.
func (m regCloneStepModel) handleResult(msg registryAddDoneMsg) regCloneStepModel {
	m.cloning = false
	m.done = true
	m.err = msg.err
	m.name = msg.name
	m.warnings = msg.warnings
	return m
}

func (m regCloneStepModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m regCloneStepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.cloning {
		// Only handle spinner ticks while cloning.
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.done && m.err == nil {
			// Success — any key dismisses (emit wizardDoneMsg).
			if key.Matches(msg, keys.Enter) || key.Matches(msg, keys.Back) {
				return m, func() tea.Msg { return wizardDoneMsg{} }
			}
		}
		// Error state: esc handled by the parent registryWizardModel
		// to go back to step 1.
	}

	return m, nil
}

func (m regCloneStepModel) View() string {
	if m.cloning {
		return m.spinner.View() + " Cloning " + mutedStyle.Render(m.url) + "..."
	}

	if m.err != nil {
		return errorStyle.Render("Error: ") + m.err.Error() + "\n\n" +
			mutedStyle.Render("Press esc to go back and try again.")
	}

	// Success.
	result := installedStyle.Render("✓") + " Registry " + selectedItemStyle.Render(m.name) + " added successfully."
	if len(m.warnings) > 0 {
		result += "\n\n" + warningStyle.Render(fmt.Sprintf("%d warning(s):", len(m.warnings)))
		for _, w := range m.warnings {
			result += "\n  " + mutedStyle.Render("• "+w)
		}
	}
	result += "\n\n" + mutedStyle.Render("Press enter to continue.")
	return result
}
