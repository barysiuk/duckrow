package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sidebarWidth is the visible column count for the sidebar panel.
const sidebarWidth = 38

// minContentWidth is the minimum content panel width (in columns) required
// to show the sidebar. If the content panel would be narrower, the sidebar
// is hidden and the content panel takes the full terminal width.
const minContentWidth = 60

// sidebarModel renders the fixed left panel shown only in the folder view.
//
// Layout (top to bottom):
//
//	Current Folder:
//	~/path/to/project
//
//	[b] Bookmark this       ← omitted when isBookmarked
//
//	Tools:                  ← omitted when no agents detected
//	· OpenCode
//	· Cursor
type sidebarModel struct {
	height int

	// Data pushed from App.
	activeFolder string
	isBookmarked bool
	agents       []string // detected agent names for the active folder
}

func newSidebarModel() sidebarModel {
	return sidebarModel{}
}

func (m sidebarModel) setSize(height int) sidebarModel {
	m.height = height
	return m
}

func (m sidebarModel) setData(activeFolder string, isBookmarked bool, agents []string) sidebarModel {
	m.activeFolder = activeFolder
	m.isBookmarked = isBookmarked
	m.agents = agents
	return m
}

func (m sidebarModel) view() string {
	// Inner width: sidebar width minus border (2) minus padding on each side.
	innerW := sidebarWidth - panelBorderH - sidebarPadH*2

	var lines []string

	// Current folder — truncate from the left so the folder name stays visible.
	lines = append(lines, sidebarLabelStyle.Render("Folder:"))
	path := shortenPath(m.activeFolder)
	for lipgloss.Width(path) > innerW {
		// Drop one character from the front until it fits, then prepend ellipsis.
		path = path[1:]
	}
	if path != shortenPath(m.activeFolder) {
		path = "…" + path[1:] // replace the first remaining char with …
	}
	lines = append(lines, sidebarPathStyle.Render(path))

	// Bookmark hint (only if not bookmarked).
	if !m.isBookmarked {
		lines = append(lines, "")
		lines = append(lines, sidebarHintStyle.Render("[b] bookmark this folder"))
	}

	// Tools section (only if agents detected).
	if len(m.agents) > 0 {
		lines = append(lines, "")
		lines = append(lines, sidebarLabelStyle.Render("Agents:"))
		for _, name := range m.agents {
			lines = append(lines, sidebarAgentStyle.Render("· "+name))
		}
	}

	content := strings.Join(lines, "\n")

	return renderPanel("Info", content, sidebarWidth, m.height, sidebarPadH, sidebarPadV)
}
