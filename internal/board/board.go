// Package board holds pure board logic: grouping items into columns and
// ordering them. No TUI or API code — fully unit-testable.
package board

import "gpk/internal/gh"

// Card is one item rendered on the board.
type Card struct {
	ID       string
	Title    string
	Type     string // "Issue", "PullRequest", "DraftIssue"
	Number   int    // 0 for draft issues
	URL      string
	Assignee string
}

// Column is one kanban column: a status option plus its cards.
type Column struct {
	Option   gh.SelectOption
	OptionID string // "" for the "No Status" column
	Cards    []Card
}

// Build groups items into columns following the status field's option order.
// An extra "No Status" column (first, like the web UI) collects items whose
// status option is unset or unknown.
func Build(status gh.FieldDef, items []Item) []Column {
	columns := make([]Column, 0, len(status.Options)+1)
	columns = append(columns, Column{Option: gh.SelectOption{Name: "No Status"}})
	for _, o := range status.Options {
		columns = append(columns, Column{Option: o, OptionID: o.ID})
	}

	index := make(map[string]int, len(columns))
	for i, c := range columns {
		if c.OptionID != "" {
			index[c.OptionID] = i
		}
	}

	for _, it := range items {
		card := Card{
			ID:       it.ID,
			Title:    it.Title,
			Type:     it.Type,
			Number:   it.Number,
			URL:      it.URL,
			Assignee: it.Assignee,
		}
		col := 0 // no option id -> "No Status"
		if it.OptionID != "" {
			if i, ok := index[it.OptionID]; ok {
				col = i
			} else {
				col = 0 // unknown option id -> "No Status"
			}
		}
		columns[col].Cards = append(columns[col].Cards, card)
	}
	return columns
}
