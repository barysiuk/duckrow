package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// confirmModel is a reusable confirmation dialog that renders as a centered
// bordered modal overlay on the content area. When active it intercepts all
// key input.
//
// Navigation: left/right/tab/shift+tab move focus between Yes and No buttons.
// Enter activates the focused button. y/n/esc are shortcut accelerators.
//
// Usage:
//
//	app.confirm = app.confirm.show("Remove registry foo?", deleteCmd)
//
// On confirm the stored onConfirm command is executed and a confirmResultMsg
// is sent. On cancel the dialog is dismissed silently.
type confirmModel struct {
	active    bool
	message   string
	onConfirm tea.Cmd // Command to execute on confirmation.
	focusYes  bool    // true = Yes focused, false = No focused.

	// Layout dimensions — set by the app so the dialog can center itself.
	width  int
	height int
}

// confirmResultMsg is sent after the user responds to a confirmation dialog.
type confirmResultMsg struct {
	confirmed bool
}

func newConfirmModel() confirmModel {
	return confirmModel{}
}

// show activates the confirmation dialog with the given prompt and action.
// Focus defaults to the No button (safe default for destructive actions).
func (m confirmModel) show(message string, onConfirm tea.Cmd) confirmModel {
	m.active = true
	m.message = message
	m.onConfirm = onConfirm
	m.focusYes = false // Default to No — safe choice for destructive actions.
	return m
}

// setSize updates the available area for centering the dialog.
func (m confirmModel) setSize(width, height int) confirmModel {
	m.width = width
	m.height = height
	return m
}

// dismiss hides the confirmation dialog without executing anything.
func (m confirmModel) dismiss() confirmModel {
	m.active = false
	m.message = ""
	m.onConfirm = nil
	m.focusYes = false
	return m
}

// confirm executes the stored action and dismisses the dialog.
func (m confirmModel) confirm() (confirmModel, tea.Cmd) {
	cmd := m.onConfirm
	m = m.dismiss()
	return m, tea.Batch(cmd, func() tea.Msg {
		return confirmResultMsg{confirmed: true}
	})
}

// cancel dismisses the dialog without executing anything.
func (m confirmModel) cancel() (confirmModel, tea.Cmd) {
	m = m.dismiss()
	return m, func() tea.Msg {
		return confirmResultMsg{confirmed: false}
	}
}

// update handles key input while the confirm dialog is active.
// Returns the updated model, any commands to run, and whether the message was consumed.
func (m confirmModel) update(msg tea.Msg) (confirmModel, tea.Cmd, bool) {
	if !m.active {
		return m, nil, false
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil, false
	}

	switch {
	// Shortcut accelerators.
	case key.Matches(keyMsg, confirmYesKey):
		m, cmd := m.confirm()
		return m, cmd, true

	case key.Matches(keyMsg, confirmNoKey):
		m, cmd := m.cancel()
		return m, cmd, true

	case key.Matches(keyMsg, keys.Back):
		m, cmd := m.cancel()
		return m, cmd, true

	// Enter activates the focused button.
	case key.Matches(keyMsg, keys.Enter):
		if m.focusYes {
			m, cmd := m.confirm()
			return m, cmd, true
		}
		m, cmd := m.cancel()
		return m, cmd, true

	// Navigation between buttons.
	case key.Matches(keyMsg, confirmLeft), key.Matches(keyMsg, confirmRight),
		key.Matches(keyMsg, confirmTab), key.Matches(keyMsg, confirmShiftTab):
		m.focusYes = !m.focusYes
		return m, nil, true
	}

	// Consume all other keys while the dialog is active — don't let them
	// propagate to sub-models or global handlers.
	return m, nil, true
}

// view renders a centered bordered dialog box with the confirmation message
// and Yes / No buttons, placed in the middle of the available area.
func (m confirmModel) view() string {
	if !m.active {
		return ""
	}

	// Build the dialog content: message + buttons.
	question := lipgloss.NewStyle().
		Width(40).
		Align(lipgloss.Center).
		Render(m.message)

	var yesBtn, noBtn string
	if m.focusYes {
		yesBtn = dialogActiveButtonStyle.Render("Yes")
		noBtn = dialogButtonStyle.Render("No")
	} else {
		yesBtn = dialogButtonStyle.Render("Yes")
		noBtn = dialogActiveButtonStyle.Render("No")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Top, yesBtn, "  ", noBtn)
	ui := lipgloss.JoinVertical(lipgloss.Center, question, "", buttons)
	dialog := dialogBoxStyle.Render(ui)

	// Center the dialog in the available space.
	w := m.width
	h := m.height
	if w <= 0 || h <= 0 {
		return dialog
	}

	return lipgloss.Place(w, h,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}

// Key bindings for the confirm dialog (not part of the global keyMap).
var (
	confirmYesKey = key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y", "confirm"),
	)
	confirmNoKey = key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n", "cancel"),
	)
	confirmLeft = key.NewBinding(
		key.WithKeys("left", "h"),
	)
	confirmRight = key.NewBinding(
		key.WithKeys("right", "l"),
	)
	confirmTab = key.NewBinding(
		key.WithKeys("tab"),
	)
	confirmShiftTab = key.NewBinding(
		key.WithKeys("shift+tab"),
	)
)
