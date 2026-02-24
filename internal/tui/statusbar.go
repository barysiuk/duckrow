package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// statusMsgKind defines the visual style of a transient status message.
type statusMsgKind int

const (
	statusSuccess statusMsgKind = iota
	statusError
	statusWarning
)

// statusAutoDismiss is how long transient messages stay visible.
const statusAutoDismiss = 3 * time.Second

// statusBarModel manages the three-zone status bar at the bottom of the TUI.
//
// Layout: [left: transient message] [center: help keybindings] [right: task counter]
//
// Left zone shows success/error/warning messages that auto-dismiss after 3s.
// Center zone shows context-aware help keybindings (always visible).
// Right zone shows a spinner + progress counter for background tasks.
type statusBarModel struct {
	width int

	// Left zone — transient message.
	msg     string
	msgKind statusMsgKind
	msgID   int // Monotonic; used to ignore stale dismiss timers.
	nextID  int

	// Right zone — background task counter.
	tasks   int // Total tasks started in the current batch.
	done    int // Tasks completed so far.
	spinner spinner.Model
}

// statusDismissMsg is sent by the auto-dismiss timer.
type statusDismissMsg struct {
	id int
}

// taskStartedMsg increments the in-flight counter.
type taskStartedMsg struct{}

// taskDoneMsg increments the completed counter.
type taskDoneMsg struct{}

func newStatusBarModel() statusBarModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(spinnerStyle),
	)
	return statusBarModel{
		spinner: s,
	}
}

// showMsg displays a transient message in the left zone.
// Returns the updated model and a command that auto-dismisses after statusAutoDismiss.
func (m statusBarModel) showMsg(text string, kind statusMsgKind) (statusBarModel, tea.Cmd) {
	m.msg = text
	m.msgKind = kind
	m.msgID = m.nextID
	m.nextID++

	id := m.msgID
	cmd := tea.Tick(statusAutoDismiss, func(_ time.Time) tea.Msg {
		return statusDismissMsg{id: id}
	})
	return m, cmd
}

// dismissMsg clears the transient message.
func (m statusBarModel) dismissMsg() statusBarModel {
	m.msg = ""
	return m
}

// tasksRunning returns true if there are background tasks in progress.
func (m statusBarModel) tasksRunning() bool {
	return m.tasks > 0 && m.done < m.tasks
}

// update handles status bar messages.
func (m statusBarModel) update(msg tea.Msg) (statusBarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case statusDismissMsg:
		if msg.id == m.msgID {
			m = m.dismissMsg()
		}
		return m, nil

	case taskStartedMsg:
		m.tasks++
		if m.tasks == 1 {
			// First task in batch — start the spinner.
			return m, m.spinner.Tick
		}
		return m, nil

	case taskDoneMsg:
		m.done++
		if m.done >= m.tasks {
			// All tasks complete — reset counters.
			m.tasks = 0
			m.done = 0
		}
		return m, nil

	case spinner.TickMsg:
		if m.tasksRunning() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

// view renders the status bar.
// When a transient message is active, it replaces the help content.
// Otherwise the help keybindings are shown, with an optional task spinner on the right.
func (m statusBarModel) view(helpContent string) string {
	left := m.renderLeft()
	right := m.renderRight()

	// If a message is active, hide help — show message + right zone only.
	if left != "" {
		if right == "" {
			return left
		}
		leftW := lipgloss.Width(left)
		rightW := lipgloss.Width(right)
		gap := m.width - leftW - rightW
		if gap < 2 {
			gap = 2
		}
		return left + fmt.Sprintf("%*s%s", gap, "", right)
	}

	// No message — show help + right zone.
	if right == "" {
		return helpContent
	}
	helpW := lipgloss.Width(helpContent)
	rightW := lipgloss.Width(right)
	gap := m.width - helpW - rightW
	if gap < 2 {
		gap = 2
	}
	return helpContent + fmt.Sprintf("%*s%s", gap, "", right)
}

// renderLeft renders the left zone (transient message).
func (m statusBarModel) renderLeft() string {
	if m.msg == "" {
		return ""
	}

	switch m.msgKind {
	case statusSuccess:
		return statusSuccessStyle.Render("✓ " + m.msg)
	case statusError:
		return statusErrorStyle.Render("✗ " + m.msg)
	case statusWarning:
		return statusWarningStyle.Render("⚠ " + m.msg)
	}

	return ""
}

// renderRight renders the right zone (background task counter).
func (m statusBarModel) renderRight() string {
	if !m.tasksRunning() {
		return ""
	}
	return statusTaskStyle.Render(m.spinner.View() + "fetching")
}
