package board

import (
	"strings"
	"testing"

	"gpk/internal/gh"
)

func status() gh.FieldDef {
	return gh.FieldDef{
		Name: "Status",
		Options: []gh.SelectOption{
			{ID: "o1", Name: "Todo"},
			{ID: "o2", Name: "In Progress"},
			{ID: "o3", Name: "Done"},
		},
	}
}

func TestBuildGroupsByOptionOrder(t *testing.T) {
	// no-status item present: No Status column first, then options in order
	items := []Item{
		{ID: "i1", Title: "b", OptionID: "o3"},
		{ID: "i2", Title: "a", OptionID: "o1"},
		{ID: "i3", Title: "c", OptionID: "o1"},
		{ID: "i4", Title: "no status"},
	}
	cols := Build(status(), items)

	if len(cols) != 4 { // No Status + 3 options
		t.Fatalf("got %d columns, want 4", len(cols))
	}
	if cols[0].Option.Name != "No Status" {
		t.Errorf("first column = %q, want No Status", cols[0].Option.Name)
	}
	for i, want := range []string{"Todo", "In Progress", "Done"} {
		if cols[i+1].Option.Name != want {
			t.Errorf("column %d = %q, want %q", i+1, cols[i+1].Option.Name, want)
		}
	}
	if len(cols[1].Cards) != 2 || cols[1].Cards[0].ID != "i2" || cols[1].Cards[1].ID != "i3" {
		t.Errorf("Todo cards wrong: %+v", cols[1].Cards)
	}
	if len(cols[3].Cards) != 1 || cols[3].Cards[0].ID != "i1" {
		t.Errorf("Done cards wrong: %+v", cols[3].Cards)
	}
}

func TestBuildHidesEmptyNoStatus(t *testing.T) {
	// all items have a status: no No Status column (matches the web UI)
	items := []Item{
		{ID: "i2", Title: "a", OptionID: "o1"},
		{ID: "i1", Title: "b", OptionID: "o3"},
	}
	cols := Build(status(), items)
	if len(cols) != 3 {
		t.Fatalf("got %d columns, want 3 (No Status hidden)", len(cols))
	}
	if cols[0].Option.Name != "Todo" {
		t.Fatalf("first column = %q, want Todo", cols[0].Option.Name)
	}
	// unknown option id also counts as no-status
	items = append(items, Item{ID: "i9", Title: "stale", OptionID: "zzz"})
	cols = Build(status(), items)
	if cols[0].Option.Name != "No Status" || len(cols[0].Cards) != 1 {
		t.Fatalf("unknown option id should create a No Status column: %+v", cols)
	}
}

func TestBuildNoStatusAndUnknownOptions(t *testing.T) {
	items := []Item{
		{ID: "i1", Title: "no option id"},
		{ID: "i2", Title: "stale option", OptionID: "zzz"},
	}
	cols := Build(status(), items)
	if len(cols[0].Cards) != 2 {
		t.Fatalf("No Status should hold 2 cards, got %d", len(cols[0].Cards))
	}
	for _, c := range cols[0].Cards {
		if !strings.Contains(c.Title, "i") && c.ID != "i1" && c.ID != "i2" {
			t.Errorf("unexpected card %+v", c)
		}
	}
}

func TestBuildEmpty(t *testing.T) {
	cols := Build(status(), nil)
	if len(cols) != 3 { // option columns only, no empty No Status
		t.Fatalf("got %d columns, want 3", len(cols))
	}
	for _, c := range cols {
		if len(c.Cards) != 0 {
			t.Errorf("column %q should be empty", c.Option.Name)
		}
	}
}
