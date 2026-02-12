package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
)

// ---------------------------------------------------------------------------
// Skill items (folder view — installed skills)
// ---------------------------------------------------------------------------

// skillItem wraps an InstalledSkill for the bubbles list.
type skillItem struct {
	skill core.InstalledSkill
}

func (i skillItem) FilterValue() string { return i.skill.Name }

// skillDelegate renders installed skills as: ✓ name v1.0  description
type skillDelegate struct{}

func (d skillDelegate) Height() int                             { return 1 }
func (d skillDelegate) Spacing() int                            { return 0 }
func (d skillDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d skillDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	si, ok := item.(skillItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	indicator := "    "
	if isSelected {
		indicator = "  > "
	}

	check := installedStyle.Render("✓")

	var nameStr string
	if isSelected {
		nameStr = selectedItemStyle.Render(si.skill.Name)
	} else {
		nameStr = skillNameStyle.Render(si.skill.Name)
	}

	if si.skill.Version != "" {
		nameStr += " " + skillVersionStyle.Render("v"+si.skill.Version)
	}

	if si.skill.Description != "" {
		nameStr += "  " + mutedStyle.Render(si.skill.Description)
	}

	_, _ = fmt.Fprint(w, indicator+check+" "+nameStr)
}

// skillsToItems converts a slice of InstalledSkill to list items.
func skillsToItems(skills []core.InstalledSkill) []list.Item {
	items := make([]list.Item, len(skills))
	for i, s := range skills {
		items[i] = skillItem{skill: s}
	}
	return items
}

// ---------------------------------------------------------------------------
// Registry skill items (install picker)
// ---------------------------------------------------------------------------

// registrySkillItem wraps a RegistrySkillInfo for the install picker list.
type registrySkillItem struct {
	info core.RegistrySkillInfo
}

func (i registrySkillItem) FilterValue() string { return i.info.Skill.Name }

// registrySeparatorItem is a non-selectable group header for registry names.
type registrySeparatorItem struct {
	registryName string
}

// FilterValue returns empty so separators are excluded from filter results.
func (i registrySeparatorItem) FilterValue() string { return "" }

// registrySkillDelegate renders registry skills and separator headers.
type registrySkillDelegate struct{}

func (d registrySkillDelegate) Height() int                             { return 1 }
func (d registrySkillDelegate) Spacing() int                            { return 0 }
func (d registrySkillDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d registrySkillDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	switch it := item.(type) {
	case registrySeparatorItem:
		_, _ = fmt.Fprint(w, sectionHeaderStyle.Render("  "+it.registryName))

	case registrySkillItem:
		isSelected := index == m.Index()

		indicator := "    "
		if isSelected {
			indicator = "  > "
		}

		name := it.info.Skill.Name
		var parts []string
		if isSelected {
			parts = append(parts, selectedItemStyle.Render(name))
		} else {
			parts = append(parts, normalItemStyle.Render(name))
		}

		if it.info.Skill.Version != "" {
			parts = append(parts, skillVersionStyle.Render("v"+it.info.Skill.Version))
		}

		if it.info.Skill.Description != "" {
			parts = append(parts, mutedStyle.Render(it.info.Skill.Description))
		}

		_, _ = fmt.Fprint(w, indicator+strings.Join(parts, "  "))
	}
}

// registrySkillsToItems converts registry skills to list items, inserting
// separator items between different registries.
func registrySkillsToItems(available []core.RegistrySkillInfo) []list.Item {
	// Group by registry name, preserving order.
	type group struct {
		name   string
		skills []core.RegistrySkillInfo
	}
	groupMap := make(map[string]*group)
	var order []string
	for _, s := range available {
		g, ok := groupMap[s.RegistryName]
		if !ok {
			g = &group{name: s.RegistryName}
			groupMap[s.RegistryName] = g
			order = append(order, s.RegistryName)
		}
		g.skills = append(g.skills, s)
	}

	var items []list.Item
	for _, regName := range order {
		g := groupMap[regName]
		items = append(items, registrySeparatorItem{registryName: g.name})
		for _, skill := range g.skills {
			items = append(items, registrySkillItem{info: skill})
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// Folder items (folder picker)
// ---------------------------------------------------------------------------

// folderItem wraps a FolderStatus for the folder picker list.
type folderItem struct {
	status   core.FolderStatus
	isActive bool
}

func (i folderItem) FilterValue() string { return i.status.Folder.Path }

// folderDelegate renders folder items as: path  3 skills  Agent1, Agent2  (active)
type folderDelegate struct{}

func (d folderDelegate) Height() int                             { return 1 }
func (d folderDelegate) Spacing() int                            { return 0 }
func (d folderDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d folderDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(folderItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	indicator := "    "
	if isSelected {
		indicator = "  > "
	}

	path := shortenPath(fi.status.Folder.Path)
	skillCount := len(fi.status.Skills)
	badge := badgeStyle.Render(fmt.Sprintf(" %d skills", skillCount))

	agents := ""
	if len(fi.status.Agents) > 0 {
		agents = "  " + mutedStyle.Render(strings.Join(fi.status.Agents, ", "))
	}

	active := ""
	if fi.isActive {
		active = "  " + installedStyle.Render("(active)")
	}

	if isSelected {
		_, _ = fmt.Fprint(w, indicator+selectedItemStyle.Render(path)+badge+agents+active)
	} else {
		_, _ = fmt.Fprint(w, indicator+normalItemStyle.Render(path)+badge+agents+active)
	}
}

// foldersToItems converts folder statuses to list items.
func foldersToItems(folders []core.FolderStatus, activeFolder string) []list.Item {
	items := make([]list.Item, len(folders))
	for i, fs := range folders {
		items[i] = folderItem{
			status:   fs,
			isActive: fs.Folder.Path == activeFolder,
		}
	}
	return items
}
