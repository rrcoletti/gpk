package tui

// Shared visual language for every screen: one source of truth for colors,
// borders, header/footer bars and selection styling.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Version is shown in the picker header; main sets it at startup.
var Version = "dev"

// User is the GitHub login shown in the picker header; main sets it after
// verifying the token.
var User = ""

// Accent and neutral palette (256-color).
const (
	colorAccent = "75" // selection / focus (blue)
	colorDim    = "241"
	colorBorder = "238"
	colorError  = "196"
	colorInfo   = "39" // numbers, links
	colorRow    = "250"
	colorSelBg  = "236"
)

var (
	// themeTitle is the header/title bar style, shared by picker and board.
	themeTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))

	// themeDim is secondary info: footers, counts, hints.
	themeDim = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))

	// themeErr is error toasts/messages.
	themeErr = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorError))

	// themeList is the rounded bordered pane both screens use.
	themeList = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder))

	// themeSelList is the focused pane's border.
	themeSelList = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorAccent))

	// themeSelRow highlights the selected row: blue, no background — the
	// "> " prefix is the visual marker (board cards, repo menu).
	themeSelRow = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(colorAccent))

	// pickerSelStyle marks the menu's selected row: blue, no background —
	// a "> " prefix on the row is the visual marker instead.
	pickerSelStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(colorAccent))

	// themeNum tints numbers (issue/project numbers, URLs).
	themeNum = lipgloss.NewStyle().Foreground(lipgloss.Color(colorInfo))

	// themeCard is normal card text.
	themeCard = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRow))
)

// helpBox renders the bordered command-list box used by the ? overlays.
func helpBox(rows [][2]string) string {
	var lines []string
	lines = append(lines, themeTitle.Render("Commands"), "")
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%-14s%s", r[0], r[1]))
	}
	lines = append(lines, "", modalFoot("? or Esc close"))
	w := 0
	for _, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			w = lw
		}
	}
	return themeSelList.Width(w).Render(strings.Join(lines, "\n"))
}

// overlayCenter stamps box centered over bg. Stamping is ANSI-aware:
// styled bg lines are cut with x/ansi so escape sequences survive (the
// peek sliver needed the same treatment).
func overlayCenter(bg, box string) string {
	x := (lipgloss.Width(bg) - lipgloss.Width(box)) / 2
	y := (lipgloss.Height(bg) - lipgloss.Height(box)) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	bgLines := strings.Split(bg, "\n")
	boxLines := strings.Split(box, "\n")
	for i, b := range boxLines {
		by := y + i
		if by < 0 || by >= len(bgLines) {
			continue
		}
		left := ansi.Truncate(bgLines[by], x, "")
		bgLines[by] = left + b + ansi.TruncateLeft(bgLines[by], x+lipgloss.Width(b), "")
	}
	return strings.Join(bgLines, "\n")
}

// frame renders a full-window screen: header line, a bordered pane filling
// the terminal, and a footer line. Every modal screen (picker, add, edit,
// delete, detail) goes through this so they all look like one app.
func frame(width, height int, header, pane, footer string) string {
	if width == 0 {
		width = 76
	}
	if width < 20 {
		width = 20
	}
	h := height - 3 // header + borders + footer
	if height == 0 {
		h = 10
	}
	if h < 3 {
		h = 3
	}
	list := themeList.Width(width - 2).Height(h).Render(pane)
	return header + "\n" + list + "\n" + footer
}

// modalHeader is the standard title line for modal screens.
func modalTitle(s string) string { return themeTitle.Render(s) }

// modalFoot is the standard footer line for modal screens.
func modalFoot(s string) string { return themeDim.Render(s) }
