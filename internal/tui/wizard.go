package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// wizardNextMsg is emitted by a step model when it is ready to advance.
type wizardNextMsg struct{}

// wizardDoneMsg is emitted by wizardModel when the last step completes.
type wizardDoneMsg struct{}

// wizardBackMsg is emitted by wizardModel when esc is pressed on step 0.
type wizardBackMsg struct{}

// wizardStep defines one step in a wizard flow.
type wizardStep struct {
	name    string    // Displayed in the step indicator.
	content tea.Model // The step's own Bubble Tea model.
}

// wizardModel provides a shared multi-step wizard wrapper with a step
// indicator breadcrumb. It renders as plain content inside the app's
// renderPanel — the panel border and title are handled by app.go.
//
// Each concrete flow (registry add, skill install, MCP install) owns a
// wizardModel and sets its steps. Step content models emit wizardNextMsg
// to advance; the wizard emits wizardDoneMsg when the last step completes.
type wizardModel struct {
	width, height int

	title     string       // Used by app.go for the panel title.
	steps     []wizardStep // Ordered list of steps.
	activeIdx int
}

// newWizardModel creates a wizard with the given title and steps.
func newWizardModel(title string, steps []wizardStep) wizardModel {
	return wizardModel{
		title: title,
		steps: steps,
	}
}

// setSize updates the content area dimensions (inside the panel).
func (m wizardModel) setSize(width, height int) wizardModel {
	m.width = width
	m.height = height
	return m
}

// activeStep returns the currently active step, or nil if out of bounds.
func (m wizardModel) activeStep() *wizardStep {
	if m.activeIdx >= 0 && m.activeIdx < len(m.steps) {
		return &m.steps[m.activeIdx]
	}
	return nil
}

// update handles wizard navigation. It intercepts wizardNextMsg to advance
// steps, and esc to go back. All other messages are forwarded to the active
// step's content model.
func (m wizardModel) update(msg tea.Msg) (wizardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case wizardNextMsg:
		if m.activeIdx >= len(m.steps)-1 {
			// Last step completed — emit done.
			return m, func() tea.Msg { return wizardDoneMsg{} }
		}
		m.activeIdx++
		// Initialize the new step if it has an Init method worth calling.
		step := m.steps[m.activeIdx].content
		return m, step.Init()

	case tea.KeyMsg:
		// Don't intercept keys while a text input may be focused —
		// let the step handle esc itself if needed. Only handle esc
		// at the wizard level for non-input steps.
		if key.Matches(msg, keys.Back) {
			if m.activeIdx > 0 {
				m.activeIdx--
				return m, nil
			}
			// Step 0: emit back to close the wizard.
			return m, func() tea.Msg { return wizardBackMsg{} }
		}
	}

	// Forward to the active step's content model.
	if step := m.activeStep(); step != nil {
		var cmd tea.Cmd
		step.content, cmd = step.content.Update(msg)
		return m, cmd
	}

	return m, nil
}

// view renders the step indicator + active step content as plain content.
// The surrounding panel border is handled by app.go's renderPanel.
// Content is indented to match the clone error view's visual style.
func (m wizardModel) view() string {
	if len(m.steps) == 0 {
		return ""
	}

	indicator := m.renderStepIndicator()
	stepView := m.steps[m.activeIdx].content.View()

	var content string
	if indicator != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, indicator, "", stepView)
	} else {
		content = stepView
	}

	// Add left padding so wizard content sits inset from the panel border,
	// matching the indentation used by the clone error view.
	return wizardContentStyle.Render(content)
}

// renderStepIndicator draws the breadcrumb strip:
//
//	Enter URL → Confirm
//	─────────
func (m wizardModel) renderStepIndicator() string {
	if len(m.steps) <= 1 {
		// Single-step wizard: no indicator needed.
		return ""
	}

	var parts []string
	var activeLabel string

	for i, step := range m.steps {
		var label string
		if i == m.activeIdx {
			label = wizardStepActiveStyle.Render(step.name)
			activeLabel = step.name
		} else {
			label = wizardStepInactiveStyle.Render(step.name)
		}
		parts = append(parts, label)
	}

	sep := wizardStepSeparatorStyle.Render(" → ")
	breadcrumb := strings.Join(parts, sep)

	// Underline below the active step label.
	underline := wizardStepActiveStyle.Render(strings.Repeat("─", len(activeLabel)))

	// Calculate offset: the visible width of all labels + separators before
	// the active step. This positions the underline below the active label.
	offset := 0
	sepWidth := lipgloss.Width(sep)
	for i := 0; i < m.activeIdx; i++ {
		offset += len(m.steps[i].name) + sepWidth
	}

	padding := strings.Repeat(" ", offset)

	return breadcrumb + "\n" + padding + underline
}
