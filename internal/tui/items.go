package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/barysiuk/duckrow/internal/core"
	"github.com/barysiuk/duckrow/internal/core/asset"
	"github.com/barysiuk/duckrow/internal/core/system"
)

// ---------------------------------------------------------------------------
// Installed asset items (folder view — unified for all asset kinds)
// ---------------------------------------------------------------------------

// assetItem represents an installed asset for display in the folder view.
// It handles both disk-scanned assets (skills) and lock-file-only assets (MCPs).
// Implements list.DefaultItem (Title + Description + FilterValue).
type assetItem struct {
	kind      asset.Kind
	name      string
	desc      string
	path      string                // On-disk path (for skills with disk presence)
	hasUpdate bool                  // Whether an update is available
	installed *asset.InstalledAsset // Set for disk-scanned assets (skills)
	locked    *asset.LockedAsset    // Set for lock-file-only assets (MCPs)
}

func (i assetItem) Title() string {
	if i.hasUpdate {
		return i.name + "  " + warningStyle.Render("↓")
	}
	return i.name
}

func (i assetItem) Description() string {
	// For lock-file items, show system names.
	if i.locked != nil {
		parts := []string{}
		if i.desc != "" {
			parts = append(parts, i.desc)
		}
		if systems := displaySystemNames(lockedSystems(*i.locked)); len(systems) > 0 {
			parts = append(parts, strings.Join(systems, ", "))
		}
		if len(parts) == 0 {
			handler, _ := asset.Get(i.kind)
			if handler != nil {
				return handler.DisplayName()
			}
			return string(i.kind)
		}
		return strings.Join(parts, " · ")
	}

	// For disk-scanned items.
	if i.desc != "" {
		return i.desc
	}
	return "No description"
}

func (i assetItem) FilterValue() string { return i.name }

// installedAssetsToItems converts a slice of InstalledAsset to list items,
// optionally marking items that have updates available.
func installedAssetsToItems(kind asset.Kind, assets []asset.InstalledAsset, updateInfo map[string]core.UpdateInfo) []list.Item {
	items := make([]list.Item, len(assets))
	for i, a := range assets {
		_, hasUpdate := updateInfo[a.Name]
		items[i] = assetItem{
			kind:      kind,
			name:      a.Name,
			desc:      a.Description,
			path:      a.Path,
			hasUpdate: hasUpdate,
			installed: &assets[i],
		}
	}
	return items
}

// lockedAssetsToItems converts locked assets (from lock file) to list items.
// The descLookup provides descriptions from registry data keyed by asset name.
func lockedAssetsToItems(kind asset.Kind, locked []asset.LockedAsset, descLookup map[string]string) []list.Item {
	items := make([]list.Item, len(locked))
	for i, la := range locked {
		items[i] = assetItem{
			kind:   kind,
			name:   la.Name,
			desc:   descLookup[la.Name],
			locked: &locked[i],
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// Registry asset items (install picker)
// ---------------------------------------------------------------------------

// registryAssetItem wraps a core.RegistryAssetInfo for the install picker list.
type registryAssetItem struct {
	info core.RegistryAssetInfo
}

func (i registryAssetItem) FilterValue() string { return i.info.Entry.Name }

// registrySeparatorItem is a non-selectable group header for registry names.
type registrySeparatorItem struct {
	registryName string
}

// FilterValue returns empty so separators are excluded from filter results.
func (i registrySeparatorItem) FilterValue() string { return "" }

// registryAssetDelegate renders registry assets and separator headers.
type registryAssetDelegate struct{}

func (d registryAssetDelegate) Height() int                             { return 1 }
func (d registryAssetDelegate) Spacing() int                            { return 0 }
func (d registryAssetDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d registryAssetDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	switch it := item.(type) {
	case registrySeparatorItem:
		_, _ = fmt.Fprint(w, renderSectionHeader(it.registryName, m.Width()))

	case registryAssetItem:
		isSelected := index == m.Index()

		indicator := "    "
		if isSelected {
			indicator = "  > "
		}

		name := it.info.Entry.Name
		var parts []string
		if isSelected {
			parts = append(parts, selectedItemStyle.Render(name))
		} else {
			parts = append(parts, normalItemStyle.Render(name))
		}

		if it.info.Entry.Description != "" {
			parts = append(parts, mutedStyle.Render(it.info.Entry.Description))
		}

		// Show type indicator for MCPs with remote URLs.
		if it.info.Kind == asset.KindMCP {
			if meta, ok := it.info.Entry.Meta.(asset.MCPMeta); ok && meta.URL != "" {
				parts = append(parts, mutedStyle.Render("(remote)"))
			}
		}

		_, _ = fmt.Fprint(w, indicator+strings.Join(parts, "  "))
	}
}

// registryAssetsToItems converts registry assets to list items, inserting
// separator items between different registries.
// Groups by RegistryRepo (unique) but displays RegistryName.
func registryAssetsToItems(available []core.RegistryAssetInfo) []list.Item {
	// Group by registry repo URL, preserving order.
	type group struct {
		name   string
		assets []core.RegistryAssetInfo
	}
	groupMap := make(map[string]*group)
	var order []string
	for _, entry := range available {
		g, ok := groupMap[entry.RegistryRepo]
		if !ok {
			g = &group{name: entry.RegistryName}
			groupMap[entry.RegistryRepo] = g
			order = append(order, entry.RegistryRepo)
		}
		g.assets = append(g.assets, entry)
	}

	var items []list.Item
	for _, repoURL := range order {
		g := groupMap[repoURL]
		items = append(items, registrySeparatorItem{registryName: g.name})
		for _, entry := range g.assets {
			items = append(items, registryAssetItem{info: entry})
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// Folder items (folder picker)
// ---------------------------------------------------------------------------

// folderItem wraps a FolderStatus for the bookmarks list.
type folderItem struct {
	status    core.FolderStatus
	isActive  bool     // this folder is currently being viewed
	isCurrent bool     // synthetic entry for the cwd (not bookmarked)
	systems   []string // display names from system detection
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

	systems := ""
	if len(fi.systems) > 0 {
		systems = "  " + mutedStyle.Render(strings.Join(fi.systems, ", "))
	}

	active := ""
	if fi.isCurrent {
		active = "  " + mutedStyle.Render("(current, not bookmarked)")
	} else if fi.isActive {
		active = "  " + installedStyle.Render("(active)")
	}

	if isSelected {
		_, _ = fmt.Fprint(w, indicator+selectedItemStyle.Render(path)+badge+systems+active)
	} else {
		_, _ = fmt.Fprint(w, indicator+normalItemStyle.Render(path)+badge+systems+active)
	}
}

// foldersToItems converts folder statuses to list items.
// It detects active systems per folder so the bookmark list shows
// systems based on config artifacts, not duckrow-managed skill directories.
// The installed count comes from the lock file (all asset kinds managed by duckrow).
func foldersToItems(folders []core.FolderStatus, activeFolder string) []list.Item {
	items := make([]list.Item, len(folders))
	for i, fs := range folders {
		var installed int
		if lf, err := core.ReadLockFile(fs.Folder.Path); err == nil && lf != nil {
			installed = len(lf.Assets)
		}
		items[i] = folderItem{
			status:    fs,
			isActive:  fs.Folder.Path == activeFolder,
			systems:   system.DisplayNames(system.DetectInFolder(fs.Folder.Path)),
			installed: installed,
		}
	}
	return items
}

// lockedFromAssetItems extracts the LockedAsset values from a slice of assetItem.
// Used when converting pre-built assetItem lists back to LockedAsset slices.
func lockedFromAssetItems(items []assetItem) []asset.LockedAsset {
	locked := make([]asset.LockedAsset, 0, len(items))
	for _, it := range items {
		if it.locked != nil {
			locked = append(locked, *it.locked)
		}
	}
	return locked
}

// descLookupFromAssetItems builds a name->description map from assetItem slices.
func descLookupFromAssetItems(items []assetItem) map[string]string {
	m := make(map[string]string, len(items))
	for _, it := range items {
		if it.desc != "" {
			m[it.name] = it.desc
		}
	}
	return m
}

func lockedSystems(locked asset.LockedAsset) []string {
	if locked.Data == nil {
		return nil
	}
	if systems, ok := locked.Data["systems"]; ok {
		switch v := systems.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func displaySystemNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	result := make([]string, 0, len(names))
	for _, name := range names {
		if sys, ok := system.ByName(name); ok {
			result = append(result, sys.DisplayName())
		} else {
			result = append(result, name)
		}
	}
	return result
}
