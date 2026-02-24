package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tabActiveMsg is emitted after the active tab changes.
type tabActiveMsg int

// tabsModel is a reusable horizontal tab bar.
//
// Visual style:
//
//	Skills (3)  │  MCPs (2)
//	──────────
type tabsModel struct {
	tabs      []string // labels including counts, e.g. "Skills (3)"
	activeTab int
}

func newTabsModel(labels []string) tabsModel {
	return tabsModel{tabs: labels}
}

func (m tabsModel) setLabels(labels []string) tabsModel {
	m.tabs = labels
	if m.activeTab >= len(labels) {
		m.activeTab = 0
	}
	return m
}

// update handles Tab / Shift+Tab to cycle through tabs.
// Returns the updated model, an optional command, and whether the key was consumed.
// blocked should be true when the parent wants to prevent tab switching (e.g. during filter mode).
func (m tabsModel) update(msg tea.Msg, blocked bool) (tabsModel, tea.Cmd, bool) {
	if blocked {
		return m, nil, false
	}

	kmsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil, false
	}

	n := len(m.tabs)
	if n == 0 {
		return m, nil, false
	}

	switch {
	case key.Matches(kmsg, keys.Tab):
		m.activeTab = (m.activeTab + 1) % n
		return m, func() tea.Msg { return tabActiveMsg(m.activeTab) }, true

	case key.Matches(kmsg, keys.ShiftTab):
		m.activeTab = (m.activeTab - 1 + n) % n
		return m, func() tea.Msg { return tabActiveMsg(m.activeTab) }, true
	}

	return m, nil, false
}

// view renders the tab bar as a single line.
//
// Active tab uses tabActiveStyle (bold, secondary color).
// Inactive tabs use tabInactiveStyle (muted).
// Tabs are separated by a styled │.
func (m tabsModel) view() string {
	if len(m.tabs) == 0 {
		return ""
	}

	sep := tabSeparatorStyle.Render("│")

	var parts []string
	for i, label := range m.tabs {
		if i == m.activeTab {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabInactiveStyle.Render(label))
		}
	}

	tabLine := "  " + strings.Join(parts, sep)

	// Draw an underline below the active tab.
	// The underline spans only the active tab label width.
	activeW := lipgloss.Width(m.tabs[m.activeTab])

	// Calculate the offset: sum widths of all tabs + separators before the active one.
	offset := 2 // leading indent "  "
	for i := 0; i < m.activeTab; i++ {
		offset += lipgloss.Width(m.tabs[i])
		offset += lipgloss.Width(sep)
	}

	underline := strings.Repeat(" ", offset) +
		tabUnderlineStyle.Render(strings.Repeat("─", activeW))

	return tabLine + "\n" + underline
}
