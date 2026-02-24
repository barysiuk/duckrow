package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// Color palette.
var (
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple
	colorSecondary = lipgloss.Color("#A78BFA") // Light purple
	colorSuccess   = lipgloss.Color("#10B981") // Green (installed)
	colorDanger    = lipgloss.Color("#EF4444") // Red (errors)
	colorMuted     = lipgloss.Color("#6B7280") // Gray
	colorBorder    = lipgloss.Color("#374151") // Dark gray
	colorWarning   = lipgloss.Color("#F59E0B") // Amber
)

// Shared styles used across TUI views.
var (
	// Panel border color and title style.
	panelBorderStyle = lipgloss.NewStyle().
				Foreground(colorBorder)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#D1D5DB"))

	// Section header within a panel (e.g. "INSTALLED", "AVAILABLE").
	// NOTE: No MarginBottom — use explicit \n in view functions for predictable height.
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorMuted)

	// Selected item in a list.
	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	// Normal (unselected) item in a list.
	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB"))

	// Muted text (descriptions, secondary info).
	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Badge for skill counts.
	badgeStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// Installed / success indicator.
	installedStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	// Error text.
	errorStyle = lipgloss.NewStyle().
			Foreground(colorDanger)

	// Warning / banner text.
	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	// Help text at the bottom.
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Viewport overlay (SKILL.md preview).
	viewportTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#D1D5DB")).
				Background(colorBorder).
				Padding(0, 1)

	// Preview scroll percentage badge.
	previewPctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB")).
			Background(colorBorder)

	// Spinner style.
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	// Section header rule (the ─── line after the label).
	sectionRuleStyle = lipgloss.NewStyle().
				Foreground(colorBorder)

	// Confirmation dialog.
	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)

	dialogButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFF7DB")).
				Background(colorMuted).
				Padding(0, 2)

	dialogActiveButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFF7DB")).
				Background(colorDanger).
				Padding(0, 2).
				Bold(true)

	// Status bar styles.
	statusSuccessStyle = lipgloss.NewStyle().Foreground(colorSuccess)
	statusErrorStyle   = lipgloss.NewStyle().Foreground(colorDanger)
	statusWarningStyle = lipgloss.NewStyle().Foreground(colorWarning)
	statusTaskStyle    = lipgloss.NewStyle().Foreground(colorMuted)

	// Sidebar styles.
	sidebarLabelStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Bold(true)

	sidebarPathStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F3F4F6"))

	sidebarHintStyle = lipgloss.NewStyle().
				Foreground(colorSecondary)

	sidebarAgentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D1D5DB"))
)

// renderSectionHeader renders a section label with short rules on both sides:
// "  ── SKILLS ──────"
func renderSectionHeader(label string, _ int) string {
	rule := sectionRuleStyle.Render("──")
	text := sectionHeaderStyle.Render(" " + label + " ")
	return "  " + rule + text + rule
}

// renderSectionHeaderWithUpdate renders a section header with an amber-colored
// update portion, e.g. "  ── SKILLS (3 installed, 2 updates available) ──"
func renderSectionHeaderWithUpdate(prefix, updatePart, suffix string, _ int) string {
	rule := sectionRuleStyle.Render("──")
	prefixText := sectionHeaderStyle.Render(" " + prefix)
	updateText := warningStyle.Render(updatePart)
	suffixText := sectionHeaderStyle.Render(suffix + " ")
	return "  " + rule + prefixText + updateText + suffixText + rule
}

// newSkillDelegate creates a DefaultDelegate styled to match the DuckRow theme.
// Uses the fancy list pattern: vertical bar for selection, title + description
// on two lines, filter match highlighting.
func newSkillDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.Styles.NormalTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F3F4F6")).
		Padding(0, 0, 0, 2)

	d.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 0, 0, 2)

	d.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorPrimary).
		Foreground(colorSecondary).
		Bold(true).
		Padding(0, 0, 0, 1)

	d.Styles.SelectedDesc = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorPrimary).
		Foreground(colorMuted).
		Padding(0, 0, 0, 1)

	d.Styles.DimmedTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorMuted).
		Padding(0, 0, 0, 2)

	d.Styles.DimmedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4D4D4D")).
		Padding(0, 0, 0, 2)

	d.SetSpacing(1)

	return d
}

// Panel layout constants.
// A panel has: 1 border char on each side + inner padding on each side.
const (
	// panelPadH is the horizontal padding inside the content panel (left + right).
	panelPadH = 2
	// panelPadV is the vertical padding inside the content panel (top + bottom).
	panelPadV = 1
	// panelBorderH is the horizontal border size (1 left + 1 right).
	panelBorderH = 2
	// panelBorderV is the vertical border size (1 top + 1 bottom).
	panelBorderV = 2

	// sidebarPadH is the horizontal padding inside the sidebar panel.
	sidebarPadH = 1
	// sidebarPadV is the vertical padding inside the sidebar panel.
	sidebarPadV = 1
)

// renderPanel draws a bordered box with a title inlined into the top border.
//
// Layout:
//
//	╭─ Title ────────────╮
//	│                    │
//	│  content here      │
//	│                    │
//	╰────────────────────╯
//
// outerW and outerH are the total dimensions including border characters.
// padH and padV are the inner padding (each side), e.g. padH=2 means 2 left + 2 right.
func renderPanel(title, content string, outerW, outerH, padH, padV int) string {
	// Text content width: outer minus borders minus padding on each side.
	// This is the width available for actual content text.
	textW := max(0, outerW-panelBorderH-padH*2)

	// lipgloss Width() and Height() include padding in their dimensions.
	// So we set them to the full space between the border characters.
	boxW := max(0, outerW-panelBorderH)
	boxH := max(0, outerH-panelBorderV)

	// Pad and clamp the content to fit.
	_ = textW // available for callers; used indirectly via Width clamp
	padded := lipgloss.NewStyle().
		Width(boxW).
		Height(boxH).
		Padding(padV, padH).
		Render(content)

	// Build top border: ╭─ Title ───...───╮
	border := lipgloss.RoundedBorder()
	styledBorder := panelBorderStyle

	titleText := ""
	if title != "" {
		titleText = " " + panelTitleStyle.Render(title) + " "
	}

	titleW := lipgloss.Width(titleText)
	// Available width for rules: outer width minus corners (2) minus title.
	ruleTotal := max(0, outerW-2-titleW)
	ruleLeft := 1 // single ─ before title
	ruleRight := max(0, ruleTotal-ruleLeft)

	topLine := styledBorder.Render(border.TopLeft) +
		styledBorder.Render(strings.Repeat(border.Top, ruleLeft)) +
		titleText +
		styledBorder.Render(strings.Repeat(border.Top, ruleRight)) +
		styledBorder.Render(border.TopRight)

	// Build bottom border: ╰───...───╯
	bottomRule := max(0, outerW-2)
	bottomLine := styledBorder.Render(border.BottomLeft) +
		styledBorder.Render(strings.Repeat(border.Bottom, bottomRule)) +
		styledBorder.Render(border.BottomRight)

	// Build middle rows: │ padded content │
	// Split padded content into lines and wrap each with border chars.
	middleLines := strings.Split(padded, "\n")
	var sb strings.Builder
	sb.WriteString(topLine)
	for _, line := range middleLines {
		sb.WriteByte('\n')
		sb.WriteString(styledBorder.Render(border.Left))
		sb.WriteString(line)
		// Pad line to fill inner width + padding if needed.
		lineW := lipgloss.Width(line)
		targetW := outerW - panelBorderH
		if lineW < targetW {
			sb.WriteString(strings.Repeat(" ", targetW-lineW))
		}
		sb.WriteString(styledBorder.Render(border.Right))
	}
	sb.WriteByte('\n')
	sb.WriteString(bottomLine)

	return sb.String()
}
