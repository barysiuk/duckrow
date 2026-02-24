package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/barysiuk/duckrow/internal/core"
)

// bookmarksModel is the full-screen bookmarks view that lets users
// browse, switch, add, and remove bookmarked folders.
type bookmarksModel struct {
	width  int
	height int

	// Bubbles list for bookmarked folders.
	list list.Model

	// Data (set on activate).
	activeFolder string
	folders      []core.FolderStatus
}

func newBookmarksModel() bookmarksModel {
	l := list.New(nil, folderDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowPagination(false)

	return bookmarksModel{
		list: l,
	}
}

func (m bookmarksModel) setSize(width, height int) bookmarksModel {
	m.width = width
	m.height = height
	m.list.SetSize(width, max(1, height))
	return m
}

// activate is called when the bookmarks view opens. It receives the currently
// active folder path, the full list of bookmarked folder statuses, and agent
// definitions for detecting active agents per folder.
func (m bookmarksModel) activate(activeFolder string, folders []core.FolderStatus, agents []core.AgentDef) bookmarksModel {
	m.activeFolder = activeFolder
	m.folders = folders

	items := foldersToItems(folders, activeFolder, agents)
	m.list.SetItems(items)
	m.list.ResetFilter()

	// Start cursor on the currently active folder.
	for i, fs := range folders {
		if fs.Folder.Path == activeFolder {
			m.list.Select(i)
			break
		}
	}

	return m
}

func (m bookmarksModel) update(msg tea.Msg, app *App) (bookmarksModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys while filtering.
		if m.list.SettingFilter() {
			break
		}

		switch {
		case key.Matches(msg, keys.Enter):
			item := m.list.SelectedItem()
			if item != nil {
				if fi, ok := item.(folderItem); ok {
					app.setActiveFolder(fi.status.Folder.Path)
					app.activeView = viewFolder
				}
			}
			return m, nil

		case key.Matches(msg, keys.Bookmark):
			return m, m.addCurrentDir(app)

		case key.Matches(msg, keys.Delete):
			return m, m.removeSelected(app)
		}
	}

	// Forward to list for navigation + filtering.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m bookmarksModel) view() string {
	if len(m.folders) == 0 {
		hint := "\n" + mutedStyle.Render("  No bookmarks yet.")
		if !m.isActiveBookmarked() {
			hint += "\n" + mutedStyle.Render("  Press [b] to bookmark "+shortenPath(m.activeFolder))
		}
		return hint
	}

	var header string
	listH := m.height
	if !m.isActiveBookmarked() {
		header = mutedStyle.Render("  "+shortenPath(m.activeFolder)) +
			mutedStyle.Render(" is not bookmarked. Press [b] to add it.") + "\n"
		listH = max(1, m.height-lipgloss.Height(header))
	}

	m.list.SetSize(m.width, max(1, listH))
	return header + m.list.View()
}

// isActiveBookmarked returns true if the active folder is in the bookmarks list.
func (m bookmarksModel) isActiveBookmarked() bool {
	for _, fs := range m.folders {
		if fs.Folder.Path == m.activeFolder {
			return true
		}
	}
	return false
}

// bookmarkAddedMsg is sent after successfully adding a folder to bookmarks.
type bookmarkAddedMsg struct {
	path string
}

// bookmarkRemovedMsg is sent after successfully removing a folder from bookmarks.
type bookmarkRemovedMsg struct {
	path string
}

func (m bookmarksModel) addCurrentDir(app *App) tea.Cmd {
	return func() tea.Msg {
		if err := app.folders.Add(app.cwd); err != nil {
			return errMsg{err: fmt.Errorf("adding folder: %w", err)}
		}
		return bookmarkAddedMsg{path: app.cwd}
	}
}

func (m bookmarksModel) removeSelected(app *App) tea.Cmd {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}

	fi, ok := item.(folderItem)
	if !ok {
		return nil
	}

	path := fi.status.Folder.Path
	return func() tea.Msg {
		if err := app.folders.Remove(path); err != nil {
			return errMsg{err: fmt.Errorf("removing folder: %w", err)}
		}
		return bookmarkRemovedMsg{path: path}
	}
}
