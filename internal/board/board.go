// Package board holds pure board logic: grouping items into columns and
// ordering them. No TUI or API code — fully unit-testable.
package board

import (
	"sort"

	"gpk/internal/gh"
)

// Card is one item rendered on the board.
type Card struct {
	ID        string
	Title     string
	Type      string // "Issue", "PullRequest", "DraftIssue"
	Number    int    // 0 for draft issues
	URL       string
	Assignee  string
	Body      string
	Repo      string
	ContentID string
	Fields    []gh.ItemField // other single-select values, for the detail view
}

// Column is one kanban column: a status option plus its cards.
type Column struct {
	Option   gh.SelectOption
	OptionID string // "" for the "No Status" column
	Cards    []Card
}

// Build groups items into columns following the status field's option order.
// Items with no status (or an option deleted from the field) land in a
// "No Status" column — which, like the web UI, only exists when it has
// items. Cards of empty No-Status columns don't appear.
func Build(status gh.FieldDef, items []Item) []Column {
	columns := make([]Column, 0, len(status.Options)+1)
	for _, o := range status.Options {
		columns = append(columns, Column{Option: o, OptionID: o.ID})
	}

	index := make(map[string]int, len(columns))
	for i, c := range columns {
		index[c.OptionID] = i
	}

	var noStatus Column
	noStatus.Option = gh.SelectOption{Name: "No Status"}
	for _, it := range items {
		card := Card{
			ID:        it.ID,
			Title:     it.Title,
			Type:      it.Type,
			Number:    it.Number,
			URL:       it.URL,
			Assignee:  it.Assignee,
			Body:      it.Body,
			Repo:      it.Repo,
			ContentID: it.ContentID,
			Fields:    it.Fields,
		}
		i, ok := index[it.OptionID]
		if !ok {
			noStatus.Cards = append(noStatus.Cards, card) // unset or unknown
			continue
		}
		columns[i].Cards = append(columns[i].Cards, card)
	}

	// the web UI only shows "No Status" when it has items
	if len(noStatus.Cards) > 0 {
		columns = append([]Column{noStatus}, columns...)
	}
	return columns
}

// DefaultRepo returns the single repository all board items come from, or
// "" when items have no repository or the board mixes repositories.
func DefaultRepo(cols []Column) string {
	repo := ""
	for _, c := range cols {
		for _, card := range c.Cards {
			if card.Repo == "" {
				continue
			}
			if repo == "" {
				repo = card.Repo
			} else if repo != card.Repo {
				return ""
			}
		}
	}
	return repo
}

// ItemRepos returns the distinct, sorted repositories of all cards on the
// board ("" repos excluded). Used to offer a repo menu when a board mixes
// repositories and has no configured default.
func ItemRepos(cols []Column) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range cols {
		for _, card := range c.Cards {
			if card.Repo != "" && !seen[card.Repo] {
				seen[card.Repo] = true
				out = append(out, card.Repo)
			}
		}
	}
	sort.Strings(out)
	return out
}
