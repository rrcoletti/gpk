// gpk - read-only TUI for GitHub Projects v2 kanban boards
// Copyright (C) 2026  Rafael Coletti
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
package board

import (
	"strings"
	"testing"

	"gpk/internal/gh"
)

func cols() []Column {
	s := gh.FieldDef{Options: []gh.SelectOption{
		{ID: "o1", Name: "Todo"},
		{ID: "o2", Name: "Done"},
	}}
	// one item without status so the No Status column exists at index 0
	return Build(s, []Item{
		{ID: "i0", Title: "x"},
		{ID: "i1", Title: "a", OptionID: "o1"},
		{ID: "i2", Title: "b", OptionID: "o1"},
		{ID: "i3", Title: "c", OptionID: "o2"},
	})
}

func TestMoveCardBetweenColumns(t *testing.T) {
	cs := cols()
	newIdx, err := MoveCard(cs, 1, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if newIdx != 1 {
		t.Errorf("new index = %d, want 1", newIdx)
	}
	if len(cs[1].Cards) != 1 || cs[1].Cards[0].ID != "i2" {
		t.Errorf("source column wrong: %+v", cs[1].Cards)
	}
	if len(cs[2].Cards) != 2 || cs[2].Cards[1].ID != "i1" {
		t.Errorf("dest column wrong: %+v", cs[2].Cards)
	}
}

func TestMoveCardToNoStatus(t *testing.T) {
	cs := cols() // [No Status: i0 | Todo: i1,i2 | Done: i3]
	if _, err := MoveCard(cs, 2, 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := len(cs[0].Cards); got != 2 {
		t.Fatalf("No Status should hold 2 cards, got %d", got)
	}
	if last := cs[0].Cards[len(cs[0].Cards)-1]; last.ID != "i3" {
		t.Errorf("moved card should be at the end: %+v", cs[0].Cards)
	}
	if len(cs[2].Cards) != 0 {
		t.Errorf("source should be empty: %+v", cs[2].Cards)
	}
}

func TestMoveCardErrors(t *testing.T) {
	cs := cols()
	if _, err := MoveCard(cs, 0, 5, 1); err == nil || !strings.Contains(err.Error(), "card index") {
		t.Errorf("want card index error, got %v", err)
	}
	if _, err := MoveCard(cs, 0, 0, 9); err == nil {
		t.Error("want column range error, got nil")
	}
	// same column is a no-op, not an error
	if _, err := MoveCard(cs, 1, 0, 1); err != nil {
		t.Errorf("same-column move should be no-op, got %v", err)
	}
}
