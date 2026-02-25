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
//	Bookmarked: Yes         ← green when bookmarked
//	Bookmarked: No          ← red when not bookmarked
//	[b] to bookmark it      ← dimmed hint, only when not bookmarked
//
//	Systems:                ← omitted when no systems detected
//	· OpenCode
//	· Cursor
type sidebarModel struct {
	height int

	// Data pushed from App.
	activeFolder string
	isBookmarked bool
	systems      []string // detected system names for the active folder
}

func newSidebarModel() sidebarModel {
	return sidebarModel{}
}

func (m sidebarModel) setSize(height int) sidebarModel {
	m.height = height
	return m
}

func (m sidebarModel) setData(activeFolder string, isBookmarked bool, systems []string) sidebarModel {
	m.activeFolder = activeFolder
	m.isBookmarked = isBookmarked
	m.systems = systems
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

	// Bookmark status.
	lines = append(lines, "")
	if m.isBookmarked {
		lines = append(lines, sidebarLabelStyle.Render("Bookmarked: ")+sidebarAgentStyle.Render("Yes"))
	} else {
		hint := mutedStyle.Italic(true).Render("([b] to bookmark)")
		lines = append(lines, sidebarLabelStyle.Render("Bookmarked: ")+sidebarAgentStyle.Render("No ")+hint)
	}

	// Systems section (only if systems detected).
	if len(m.systems) > 0 {
		lines = append(lines, "")
		lines = append(lines, sidebarLabelStyle.Render("Systems:"))
		for _, name := range m.systems {
			lines = append(lines, sidebarAgentStyle.Render("· "+name))
		}
	}

	content := strings.Join(lines, "\n")

	return renderPanel("Info", content, sidebarWidth, m.height, sidebarPadH, sidebarPadV)
}
