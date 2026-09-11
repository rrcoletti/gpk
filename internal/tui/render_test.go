package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gpk/internal/board"
	"gpk/internal/gh"
)

// TestPeekShowsFourChars verifies the peek sliver at a small terminal:
// border + 4 content chars of the next column.
func TestPeekShowsFourChars(t *testing.T) {
	status := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{
		{ID: "o1", Name: "A"}, {ID: "o2", Name: "Done"},
	}}
	items := []board.Item{{ID: "i1", Title: "a", OptionID: "o1"}}
	m := NewBoardModel("t", nil, "", "", "", status, nil)
	m.SetColumns(board.Build(status, items))
	// 2 columns at 40 cells: (40-4)/2 = 18 < 22 -> 1 visible + peek of Done
	up, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 40})
	m = up.(BoardModel)
	lines := strings.Split(m.View(), "\n")
	borderLine, nameLine := peekLines(lines)
	if borderLine == "" {
		t.Fatalf("no peek border row in view")
	}
	if !strings.HasSuffix(borderLine, "╭────") {
		t.Fatalf("peek border row must end with a 5-cell sliver: %q", borderLine)
	}
	if !strings.Contains(nameLine, "│Done") {
		t.Errorf("peek should show border + 4 chars of the next column: %q", nameLine)
	}
}

// TestPeekAtUserBoardWidths reproduces the ca-go board: 6 columns on a wide
// terminal. 5 columns visible plus a peek sliver showing border + 4 chars.
func TestPeekAtUserBoardWidths(t *testing.T) {
	status := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{
		{ID: "o1", Name: "No Status"}, {ID: "o2", Name: "Backlog"}, {ID: "o3", Name: "Ready"},
		{ID: "o4", Name: "In progress"}, {ID: "o5", Name: "In review"}, {ID: "o6", Name: "Done"},
	}}
	items := []board.Item{{ID: "i1", Title: "x", OptionID: "o6"}}
	m := NewBoardModel("t", nil, "", "", "", status, nil)
	m.SetColumns(board.Build(status, items))
	for _, w := range []int{134, 135, 140} {
		up, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 50})
		mm := up.(BoardModel)
		if n := mm.visibleColumnCount(); n != 5 {
			t.Fatalf("w=%d: visible=%d, want 5", w, n)
		}
		lines := strings.Split(mm.View(), "\n")
		borderLine, nameLine := peekLines(lines)
		if borderLine == "" {
			t.Fatalf("w=%d: no peek sliver rendered in view", w)
		}
		if !strings.Contains(nameLine, "│In r") {
			t.Errorf("w=%d: peek should show border + 4 chars of 'In review', name row: %q", w, nameLine)
		}
	}
}

// peekLines finds the top border row and header row containing a peek
// sliver (a line ending with the sliver's border cells).
func peekLines(lines []string) (border, name string) {
	for i, l := range lines {
		trimmed := strings.TrimRight(l, " ")
		idx := strings.LastIndex(trimmed, "╭")
		if idx < 0 || strings.Contains(trimmed[idx:], "╮") {
			continue
		}
		if len(trimmed)-idx < peekWidth {
			continue
		}
		if i+1 < len(lines) {
			return trimmed, lines[i+1]
		}
	}
	return "", ""
}
