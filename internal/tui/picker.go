package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/barysiuk/duckrow/internal/core"
)

// pickerModel is the folder picker overlay that lets users switch
// the active folder context.
type pickerModel struct {
	width  int
	height int

	// Bubbles list for tracked folders.
	list list.Model

	// Data (set on activate).
	activeFolder string
	folders      []core.FolderStatus
}

func newPickerModel() pickerModel {
	l := list.New(nil, folderDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	l.SetShowPagination(false)

	return pickerModel{
		list: l,
	}
}

func (m pickerModel) setSize(width, height int) pickerModel {
	m.width = width
	m.height = height
	// List sizing happens dynamically in view() via render-then-measure.
	m.list.SetSize(width, max(1, height))
	return m
}

// activate is called when the picker opens. It receives the currently
// active folder path and the full list of tracked folder statuses.
func (m pickerModel) activate(activeFolder string, folders []core.FolderStatus) pickerModel {
	m.activeFolder = activeFolder
	m.folders = folders

	items := foldersToItems(folders, activeFolder)
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

func (m pickerModel) update(msg tea.Msg, app *App) (pickerModel, tea.Cmd) {
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

		case key.Matches(msg, keys.AddFolder):
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

func (m pickerModel) view() string {
	// --- Render-then-measure ---

	// 1. Render fixed chrome.
	sectionHeader := sectionHeaderStyle.Render("  SELECT FOLDER") + "\n"

	if len(m.folders) == 0 {
		return sectionHeader +
			mutedStyle.Render("  No folders tracked yet.") + "\n" +
			mutedStyle.Render("  Press [a] to add the current directory.")
	}

	// 2. Measure chrome, size list to fill remaining space.
	chromeH := lipgloss.Height(sectionHeader)
	listH := m.height - chromeH
	if listH < 1 {
		listH = 1
	}
	m.list.SetSize(m.width, listH)

	// 3. Assemble.
	return sectionHeader + m.list.View()
}

func (m pickerModel) addCurrentDir(app *App) tea.Cmd {
	return func() tea.Msg {
		if err := app.folders.Add(app.cwd); err != nil {
			return errMsg{err: fmt.Errorf("adding folder: %w", err)}
		}
		return app.loadDataCmd()
	}
}

func (m pickerModel) removeSelected(app *App) tea.Cmd {
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
		return app.loadDataCmd()
	}
}
