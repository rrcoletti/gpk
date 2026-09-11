package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gpk/internal/board"
	"gpk/internal/gh"
)

// TestAppBoardBackToPickerAndCachedReEntry covers the screen-transition
// flow: board -> esc -> picker -> pick the same project -> the cached board
// is shown instantly (not loading) in the same program, then refreshed.
func TestAppBoardBackToPickerAndCachedReEntry(t *testing.T) {
	status := gh.FieldDef{Name: "Status", Options: []gh.SelectOption{
		{ID: "o1", Name: "Todo"},
	}}
	items := []board.Item{{ID: "i1", Title: "a card", Type: "Issue", OptionID: "o1"}}

	am := NewAppModel(nil)
	am.picker.projects = []gh.Project{{ID: "p1", Number: 7, Title: "proj"}}

	// first open: picker -> pick -> board starts loading
	up, _ := am.Update(ProjectPickedMsg{Project: gh.Project{ID: "p1", Number: 7, Title: "proj"}})
	am = up.(AppModel)
	if am.screen != ScreenBoard {
		t.Fatal("picking should switch to the board screen")
	}
	if !am.board.loading {
		t.Fatal("first open should be loading")
	}

	// data arrives
	up, _ = am.Update(boardDataMsg{
		status:     status,
		titleField: gh.FieldDef{ID: "tf"},
		items:      items,
	})
	am = up.(AppModel)
	if am.board.loading {
		t.Fatal("boardDataMsg should end loading")
	}
	if got := am.board.header(); got != "gpk dev · Project proj · 1 item(s)" { // project title only, no number
		t.Fatalf("header = %q", got)
	}

	// esc on the board -> back to picker, same program, board kept in cache
	// (the esc handler returns backMsg as a cmd; run it like the real loop)
	up, cmd := am.Update(key("esc"))
	am = up.(AppModel)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			up, _ = am.Update(msg)
			am = up.(AppModel)
		}
	}
	if am.screen != ScreenPicker {
		t.Fatal("esc should return to the picker screen")
	}
	if am.board == nil {
		t.Fatal("board should stay cached after going back")
	}

	// pick the same project again: instant, cached, refreshed
	up, cmd2 := am.Update(ProjectPickedMsg{Project: gh.Project{ID: "p1", Number: 7, Title: "proj"}})
	am = up.(AppModel)
	if am.screen != ScreenBoard {
		t.Fatal("cached pick should switch to the board screen")
	}
	if am.board.loading {
		t.Fatal("cached board must not show loading state")
	}
	if cmd2 == nil {
		t.Fatal("cached re-entry should kick a refresh cmd")
	}
	if am.board != am.boards["p1"] {
		t.Fatal("cached re-entry should reuse the same board model")
	}
}

// TestAppFreshPickShowsLoading verifies a different project gets its own
// loading board instead of reusing the cache.
func TestAppFreshPickShowsLoading(t *testing.T) {
	am := NewAppModel(nil)
	am.picker.projects = []gh.Project{{ID: "p1"}, {ID: "p2"}}
	am.boards["p1"] = &BoardModel{loading: false}
	am.screen = ScreenPicker

	up, _ := am.Update(ProjectPickedMsg{Project: gh.Project{ID: "p2", Title: "other"}})
	am = up.(AppModel)
	if am.screen != ScreenBoard || !am.board.loading {
		t.Fatal("a different project must start a fresh loading board")
	}
}

// TestAppBoardWiring locks the field wiring: boards built by the app shell
// must carry the project and status field ids, or writes (add, delete,
// move, title) fail with "project or status field unknown".
func TestAppBoardWiring(t *testing.T) {
	am := NewAppModel(nil)
	am.picker.projects = []gh.Project{{ID: "p1", Number: 1, Title: "proj"}}
	up, _ := am.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	am = up.(AppModel)
	up, _ = am.Update(ProjectPickedMsg{Project: gh.Project{ID: "p1", Number: 1, Title: "proj"}})
	am = up.(AppModel)
	up, _ = am.Update(boardDataMsg{
		status:     gh.FieldDef{ID: "PVTSSF_status", Name: "Status", Options: []gh.SelectOption{{ID: "o1", Name: "Todo"}}},
		titleField: gh.FieldDef{ID: "PVTF_title"},
		items:      nil,
	})
	am = up.(AppModel)
	if am.board.projectID != "p1" {
		t.Fatalf("board projectID = %q, want p1", am.board.projectID)
	}
	if am.board.fieldID != "PVTSSF_status" {
		t.Fatalf("board fieldID = %q, want PVTSSF_status (writes would fail)", am.board.fieldID)
	}
	if am.board.titleFieldID != "PVTF_title" {
		t.Fatalf("board titleFieldID = %q, want PVTF_title", am.board.titleFieldID)
	}
}

// TestAppBoardInheritsTerminalSize is a regression test: boards created
// after the initial WindowSizeMsg must inherit the terminal size, or they
// render as a single tiny column.
func TestAppBoardInheritsTerminalSize(t *testing.T) {
	am := NewAppModel(nil)
	am.picker.projects = []gh.Project{{ID: "p1", Number: 1, Title: "proj"}}

	// terminal size arrives at app start, while the picker is showing
	up, _ := am.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	am = up.(AppModel)

	// then the user picks a project
	up, _ = am.Update(ProjectPickedMsg{Project: gh.Project{ID: "p1", Number: 1, Title: "proj"}})
	am = up.(AppModel)
	if am.board.width != 200 || am.board.height != 50 {
		t.Fatalf("fresh board size = %dx%d, want 200x50", am.board.width, am.board.height)
	}

	up, _ = am.Update(boardDataMsg{
		status:     gh.FieldDef{Name: "Status", Options: []gh.SelectOption{{ID: "o1", Name: "Todo"}}},
		titleField: gh.FieldDef{ID: "tf"},
		items:      []board.Item{{ID: "i1", Title: "a", Type: "Issue", OptionID: "o1"}},
	})
	am = up.(AppModel)
	if am.board.width != 200 || am.board.height != 50 {
		t.Fatalf("loaded board size = %dx%d, want 200x50", am.board.width, am.board.height)
	}
	// 2 columns at 200 cells: both visible, each 100 wide (no artificial cap)
	if got := am.board.columnWidth(); got < minColWidth*2 {
		t.Fatalf("columnWidth = %d at 200 cells with 2 columns, space is wasted", got)
	}
	if h := am.board.bodyHeight(); h < 30 {
		t.Fatalf("bodyHeight = %d at 50 rows, board would be tiny", h)
	}
}
