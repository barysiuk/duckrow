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
// Implements list.DefaultItem (Title + Description + FilterValue).
type skillItem struct {
	skill     core.InstalledSkill
	hasUpdate bool
}

func (i skillItem) Title() string {
	if i.hasUpdate {
		return i.skill.Name + "  " + warningStyle.Render("↓")
	}
	return i.skill.Name
}

func (i skillItem) Description() string {
	if i.skill.Description != "" {
		return i.skill.Description
	}
	return "No description"
}

func (i skillItem) FilterValue() string { return i.skill.Name }

// skillsToItems converts a slice of InstalledSkill to list items,
// optionally marking items that have updates available.
func skillsToItems(skills []core.InstalledSkill, updateInfo map[string]core.UpdateInfo) []list.Item {
	items := make([]list.Item, len(skills))
	for i, s := range skills {
		_, hasUpdate := updateInfo[s.Name]
		items[i] = skillItem{skill: s, hasUpdate: hasUpdate}
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
		_, _ = fmt.Fprint(w, renderSectionHeader(it.registryName, m.Width()))

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

		if it.info.Skill.Description != "" {
			parts = append(parts, mutedStyle.Render(it.info.Skill.Description))
		}

		_, _ = fmt.Fprint(w, indicator+strings.Join(parts, "  "))

	case registryMCPItem:
		isSelected := index == m.Index()

		indicator := "    "
		if isSelected {
			indicator = "  > "
		}

		name := it.info.MCP.Name
		var parts []string
		if isSelected {
			parts = append(parts, selectedItemStyle.Render(name))
		} else {
			parts = append(parts, normalItemStyle.Render(name))
		}

		if it.info.MCP.Description != "" {
			parts = append(parts, mutedStyle.Render(it.info.MCP.Description))
		}

		// Show type indicator for MCPs.
		if it.info.MCP.URL != "" {
			parts = append(parts, mutedStyle.Render("(remote)"))
		}

		_, _ = fmt.Fprint(w, indicator+strings.Join(parts, "  "))
	}
}

// registrySkillsToItems converts registry skills to list items, inserting
// separator items between different registries.
// Groups by RegistryRepo (unique) but displays RegistryName.
func registrySkillsToItems(available []core.RegistrySkillInfo) []list.Item {
	// Group by registry repo URL, preserving order.
	type group struct {
		name   string
		skills []core.RegistrySkillInfo
	}
	groupMap := make(map[string]*group)
	var order []string
	for _, s := range available {
		g, ok := groupMap[s.RegistryRepo]
		if !ok {
			g = &group{name: s.RegistryName}
			groupMap[s.RegistryRepo] = g
			order = append(order, s.RegistryRepo)
		}
		g.skills = append(g.skills, s)
	}

	var items []list.Item
	for _, repoURL := range order {
		g := groupMap[repoURL]
		items = append(items, registrySeparatorItem{registryName: g.name})
		for _, skill := range g.skills {
			items = append(items, registrySkillItem{info: skill})
		}
	}
	return items
}

// registryMCPsToItems converts registry MCPs to list items, inserting
// separator items between different registries.
// Groups by RegistryRepo (unique) but displays RegistryName.
func registryMCPsToItems(available []core.RegistryMCPInfo) []list.Item {
	// Group by registry repo URL, preserving order.
	type group struct {
		name string
		mcps []core.RegistryMCPInfo
	}
	groupMap := make(map[string]*group)
	var order []string
	for _, m := range available {
		g, ok := groupMap[m.RegistryRepo]
		if !ok {
			g = &group{name: m.RegistryName}
			groupMap[m.RegistryRepo] = g
			order = append(order, m.RegistryRepo)
		}
		g.mcps = append(g.mcps, m)
	}

	var items []list.Item
	for _, repoURL := range order {
		g := groupMap[repoURL]
		items = append(items, registrySeparatorItem{registryName: g.name})
		for _, mcp := range g.mcps {
			items = append(items, registryMCPItem{info: mcp})
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// MCP items (folder view — installed MCPs from lock file)
// ---------------------------------------------------------------------------

// mcpItem represents an installed MCP for display in the folder view.
// Implements list.DefaultItem (Title + Description + FilterValue).
type mcpItem struct {
	locked core.LockedMCP
	desc   string // Description from registry, if available
}

func (i mcpItem) Title() string       { return i.locked.Name }
func (i mcpItem) FilterValue() string { return i.locked.Name }

func (i mcpItem) Description() string {
	parts := []string{}
	if i.desc != "" {
		parts = append(parts, i.desc)
	}
	if len(i.locked.Agents) > 0 {
		parts = append(parts, strings.Join(i.locked.Agents, ", "))
	}
	if len(parts) == 0 {
		return "MCP server"
	}
	return strings.Join(parts, " · ")
}

// mcpsToItems converts a slice of mcpItem to list items.
func mcpsToItems(mcps []mcpItem) []list.Item {
	items := make([]list.Item, len(mcps))
	for i, m := range mcps {
		items[i] = m
	}
	return items
}

// registryMCPItem wraps a RegistryMCPInfo for the install picker list.
type registryMCPItem struct {
	info core.RegistryMCPInfo
}

func (i registryMCPItem) FilterValue() string { return i.info.MCP.Name }

// ---------------------------------------------------------------------------
// Folder items (folder picker)
// ---------------------------------------------------------------------------

// folderItem wraps a FolderStatus for the bookmarks list.
type folderItem struct {
	status    core.FolderStatus
	isActive  bool
	agents    []string // display names from DetectActiveAgents
	installed int      // skills + MCPs managed by duckrow (from lock file)
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
	badge := badgeStyle.Render(fmt.Sprintf(" %d installed", fi.installed))

	agents := ""
	if len(fi.agents) > 0 {
		agents = "  " + mutedStyle.Render(strings.Join(fi.agents, ", "))
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
// It calls DetectActiveAgents per folder so the bookmark list shows
// agents based on config artifacts, not duckrow-managed skill directories.
// The installed count comes from the lock file (skills + MCPs managed by duckrow).
func foldersToItems(folders []core.FolderStatus, activeFolder string, agents []core.AgentDef) []list.Item {
	items := make([]list.Item, len(folders))
	for i, fs := range folders {
		var installed int
		if lf, err := core.ReadLockFile(fs.Folder.Path); err == nil && lf != nil {
			installed = len(lf.Skills) + len(lf.MCPs)
		}
		items[i] = folderItem{
			status:    fs,
			isActive:  fs.Folder.Path == activeFolder,
			agents:    core.DetectActiveAgents(agents, fs.Folder.Path),
			installed: installed,
		}
	}
	return items
}
