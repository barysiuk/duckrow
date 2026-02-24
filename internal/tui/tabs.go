package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tabActiveMsg is emitted after the active tab changes.
type tabActiveMsg int

// tabDef holds a tab's display data.
type tabDef struct {
	label string // plain text label, e.g. "Skills (3)" — used for width calculation
	extra string // optional styled suffix appended inside the label, e.g. " ↓2" in warning color
}

// tabsModel is a reusable horizontal tab bar.
//
// Visual style:
//
//	Skills (3 ↓2)  │  MCPs (2)
//	─────────────
type tabsModel struct {
	tabs      []tabDef
	activeTab int
}

func newTabsModel(tabs []tabDef) tabsModel {
	return tabsModel{tabs: tabs}
}

func (m tabsModel) setTabs(tabs []tabDef) tabsModel {
	m.tabs = tabs
	if m.activeTab >= len(tabs) {
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
// The extra suffix (if any) is inserted before the closing paren in warning color.
// Tabs are separated by a styled │.
func (m tabsModel) view() string {
	if len(m.tabs) == 0 {
		return ""
	}

	sep := tabSeparatorStyle.Render("│")

	var parts []string
	var rawWidths []int
	for i, tab := range m.tabs {
		var rendered string
		if tab.extra != "" {
			// Split label at the last ')' to insert the styled extra text inside parens.
			// e.g. label="Skills (3)", extra=" ↓2" → "Skills (3" + styled(" ↓2") + ")"
			base := tab.label
			suffix := ""
			if idx := strings.LastIndex(base, ")"); idx >= 0 {
				suffix = base[idx:] // ")"
				base = base[:idx]   // "Skills (3"
			}
			if i == m.activeTab {
				rendered = tabActiveStyle.Render(base) + warningStyle.Bold(true).Render(tab.extra) + tabActiveStyle.Render(suffix)
			} else {
				rendered = tabInactiveStyle.Render(base) + warningStyle.Render(tab.extra) + tabInactiveStyle.Render(suffix)
			}
		} else {
			if i == m.activeTab {
				rendered = tabActiveStyle.Render(tab.label)
			} else {
				rendered = tabInactiveStyle.Render(tab.label)
			}
		}
		parts = append(parts, rendered)
		// Raw width = label + extra (plain text widths).
		rawWidths = append(rawWidths, lipgloss.Width(tab.label)+lipgloss.Width(tab.extra))
	}

	tabLine := "  " + strings.Join(parts, sep)

	// Draw an underline below the active tab.
	activeW := rawWidths[m.activeTab]

	// Calculate the offset: sum widths of all tabs + separators before the active one.
	offset := 2 // leading indent "  "
	for i := 0; i < m.activeTab; i++ {
		offset += rawWidths[i]
		offset += lipgloss.Width(sep)
	}

	underline := strings.Repeat(" ", offset) +
		tabUnderlineStyle.Render(strings.Repeat("─", activeW))

	return tabLine + "\n" + underline
}
