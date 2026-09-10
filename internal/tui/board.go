package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gpk/internal/board"
)

const (
	minColWidth  = 22
	maxColWidth  = 32
	cardMaxLines = 4
)

// GitHub status colors -> lipgloss colors (web UI palette).
var optionColors = map[string]string{
	"GRAY": "241", "PURPLE": "135", "BLUE": "39", "GREEN": "36",
	"YELLOW": "178", "ORANGE": "208", "RED": "196", "PINK": "204",
}

var (
	bTitleStyle   = lipgloss.NewStyle().Bold(true)
	bDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	bColHeadStyle = lipgloss.NewStyle().Bold(true)
	bColCount     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	bCardStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	bSelStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	bColStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238"))
	bSelColStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205"))
)

// BoardModel renders the kanban board (read-only in v0).
type BoardModel struct {
	projectTitle string
	columns      []board.Column

	colSelected int         // index into columns
	cardSel     map[int]int // column index -> selected card
	visibleCols int         // columns that fit
	colOffset   int         // first visible column index
	width       int
	height      int
	spinner     spinner.Model
	loading     bool
}

// NewBoardModel creates the board; cards are set later via SetColumns.
func NewBoardModel(title string) BoardModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = bTitleStyle
	return BoardModel{
		projectTitle: title,
		spinner:      sp,
		loading:      true,
		cardSel:      map[int]int{},
	}
}

// SetColumns swaps in (mock or real) data and turns off the loading state.
func (m *BoardModel) SetColumns(cols []board.Column) {
	m.columns = cols
	m.loading = false
	if m.colSelected >= len(m.columns) {
		m.colSelected = 0
	}
}
func (m BoardModel) Init() tea.Cmd {
	if m.loading {
		return m.spinner.Tick
	}
	return nil
}

func (m BoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampOffset()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "left", "h":
			if m.colSelected > 0 {
				m.colSelected--
				m.clampOffset()
			}
		case "right", "l":
			if m.colSelected < len(m.columns)-1 {
				m.colSelected++
				m.clampOffset()
			}
		case "up", "k":
			if n := len(m.cards(m.colSelected)); n > 0 {
				i := m.cardSel[m.colSelected]
				if i > 0 {
					m.cardSel[m.colSelected] = i - 1
				}
			}
		case "down", "j":
			if n := len(m.cards(m.colSelected)); n > 0 {
				i := m.cardSel[m.colSelected]
				if i < n-1 {
					m.cardSel[m.colSelected] = i + 1
				}
			}
		case "home", "g":
			m.cardSel[m.colSelected] = 0
		case "end", "G":
			if n := len(m.cards(m.colSelected)); n > 0 {
				m.cardSel[m.colSelected] = n - 1
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m BoardModel) cards(col int) []board.Card {
	if col < 0 || col >= len(m.columns) {
		return nil
	}
	return m.columns[col].Cards
}

// columnWidth computes equal column width for the current terminal size.
func (m BoardModel) columnWidth() int {
	if m.width == 0 {
		return maxColWidth
	}
	w := (m.width - 4) / m.visibleColumnCount()
	if w < minColWidth {
		w = minColWidth
	}
	if w > maxColWidth {
		w = maxColWidth
	}
	return w
}

// visibleColumnCount is 1 before the first WindowSizeMsg.
func (m BoardModel) visibleColumnCount() int {
	if m.width < minColWidth*2 {
		return 1
	}
	n := m.width / minColWidth
	if n < 1 {
		n = 1
	}
	return n
}

// clampOffset keeps the selected column visible when scrolling horizontally.
func (m *BoardModel) clampOffset() {
	n := m.visibleColumnCount()
	if m.colSelected < m.colOffset {
		m.colOffset = m.colSelected
	}
	if m.colSelected >= m.colOffset+n {
		m.colOffset = m.colSelected - n + 1
	}
	if m.colOffset < 0 {
		m.colOffset = 0
	}
}

func (m BoardModel) View() string {
	if m.loading {
		return "\n" + m.spinner.View() + " loading board...\n"
	}
	if len(m.columns) == 0 {
		return "\nNo columns found for this project.\n"
	}

	colW := m.columnWidth()
	n := m.visibleColumnCount()
	end := m.colOffset + n
	if end > len(m.columns) {
		end = len(m.columns)
	}

	var cols []string
	for i := m.colOffset; i < end; i++ {
		cols = append(cols, m.renderColumn(i, colW))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	head := bTitleStyle.Render(m.projectTitle)
	scroll := ""
	if m.colOffset > 0 || end < len(m.columns) {
		scroll = bDimStyle.Render(fmt.Sprintf("  ← %d/%d →", m.colOffset+1, len(m.columns)))
	}

	foot := bDimStyle.Render("h/l columns · j/k cards · q quit")
	return head + "\n" + scroll + "\n\n" + row + "\n\n" + foot
}

// renderColumn draws one column with header (colored, with count) and cards.
func (m BoardModel) renderColumn(i, colW int) string {
	c := m.columns[i]
	selected := i == m.colSelected

	color, ok := optionColors[c.Option.Color]
	if !ok {
		color = "241"
	}
	head := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).
		Render(truncate(c.Option.Name, colW-4))
	count := bColCount.Render(fmt.Sprintf(" %d", len(c.Cards)))
	header := head + count

	var lines []string
	lines = append(lines, header, "")

	if len(c.Cards) == 0 {
		lines = append(lines, bDimStyle.Render("· empty"))
	}
	sel := m.cardSel[i]
	for j, card := range c.Cards {
		for k, ln := range cardLines(card, colW-4) {
			prefix := "  "
			if k == 0 {
				if j == sel && selected {
					prefix = "> "
				} else {
					prefix = "  "
				}
			}
			style := bCardStyle
			if j == sel && selected {
				style = bSelStyle
			}
			lines = append(lines, style.Render(prefix+ln))
		}
	}

	style := bColStyle
	if selected {
		style = bSelColStyle
	}
	return style.Width(colW).Height(m.bodyHeight()).Render(strings.Join(lines, "\n"))
}

// cardLines wraps a card into at most cardMaxLines view lines.
func cardLines(c board.Card, width int) []string {
	first := c.Title
	if c.Number > 0 {
		first = fmt.Sprintf("%s #%d", c.Title, c.Number)
	}
	wrapped := wrap(first, width)
	if len(wrapped) > cardMaxLines {
		wrapped = wrapped[:cardMaxLines]
		wrapped[len(wrapped)-1] = truncate(wrapped[len(wrapped)-1], width-1) + "…"
	}
	lines := wrapped
	if c.Assignee != "" && len(lines) < cardMaxLines {
		lines = append(lines, bDimStyle.Render("@"+c.Assignee))
	}
	return lines
}

func (m BoardModel) bodyHeight() int {
	if m.height == 0 {
		return 12
	}
	h := m.height - 7
	if h < 6 {
		h = 6
	}
	return h
}

// wrap breaks s into lines of at most width display cells, on spaces when
// possible.
func wrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, w := range words {
			switch {
			case line == "":
				line = w
			case lipgloss.Width(line+" "+w) <= width:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		out = append(out, line)
	}
	return out
}

// truncate cuts s to at most width display cells.
func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	runes := []rune(s)
	for lipgloss.Width(string(runes)) > width-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
