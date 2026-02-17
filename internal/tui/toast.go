package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// toastType defines the visual style and behavior of a toast notification.
type toastType int

const (
	toastSuccess toastType = iota
	toastError
	toastWarning
	toastLoading // Shows a spinner; persists until dismissed programmatically.
)

// toastAutoDismiss is how long success/error toasts stay visible.
const toastAutoDismiss = 3 * time.Second

// toastModel manages a single toast notification displayed in the help bar area.
// At most one toast is visible at a time; showing a new toast replaces the previous one.
//
// This is a reusable component — it only needs show/dismiss/update/view calls
// and does not depend on any App-specific types.
type toastModel struct {
	// Active toast state.
	active  bool
	message string
	kind    toastType
	id      int // Monotonic ID to ignore stale dismiss messages.

	// Spinner for loading toasts.
	spinner spinner.Model

	// Counter for generating unique IDs.
	nextID int
}

// toastDismissMsg is sent by the auto-dismiss timer.
type toastDismissMsg struct {
	id int
}

func newToastModel() toastModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(spinnerStyle),
	)
	return toastModel{
		spinner: s,
	}
}

// show displays a new toast, replacing any existing one.
// Returns the model and any commands needed (spinner tick, auto-dismiss timer).
func (m toastModel) show(message string, kind toastType) (toastModel, tea.Cmd) {
	m.active = true
	m.message = message
	m.kind = kind
	m.id = m.nextID
	m.nextID++

	var cmds []tea.Cmd

	switch kind {
	case toastLoading:
		// Start the spinner; no auto-dismiss.
		cmds = append(cmds, m.spinner.Tick)
	case toastSuccess, toastError, toastWarning:
		// Schedule auto-dismiss.
		id := m.id
		cmds = append(cmds, tea.Tick(toastAutoDismiss, func(_ time.Time) tea.Msg {
			return toastDismissMsg{id: id}
		}))
	}

	return m, tea.Batch(cmds...)
}

// dismiss hides the toast immediately.
func (m toastModel) dismiss() toastModel {
	m.active = false
	m.message = ""
	return m
}

// update handles spinner ticks and auto-dismiss messages.
func (m toastModel) update(msg tea.Msg) (toastModel, tea.Cmd) {
	switch msg := msg.(type) {
	case toastDismissMsg:
		// Only dismiss if the ID matches (prevents stale timers from dismissing newer toasts).
		if msg.id == m.id {
			m = m.dismiss()
		}
		return m, nil

	case spinner.TickMsg:
		if m.active && m.kind == toastLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// view renders the toast notification left-aligned with 1 char indent.
// Returns empty string if no toast is active.
func (m toastModel) view() string {
	if !m.active {
		return ""
	}

	var style lipgloss.Style

	switch m.kind {
	case toastSuccess:
		style = installedStyle
	case toastError:
		style = errorStyle
	case toastWarning:
		style = warningStyle
	case toastLoading:
		return " " + m.spinner.View() + mutedStyle.Render(m.message)
	}

	return " " + style.Render(m.message)
}
