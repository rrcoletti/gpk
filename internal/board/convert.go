package board

import "gpk/internal/gh"

// FromGhItems converts API items to board items.
func FromGhItems(items []gh.Item) []Item {
	out := make([]Item, len(items))
	for i, it := range items {
		out[i] = Item{
			ID:        it.ID,
			Title:     it.Title,
			Type:      it.Type,
			Number:    it.Number,
			URL:       it.URL,
			Assignee:  it.Assignee,
			OptionID:  it.OptionID,
			Body:      it.Body,
			Repo:      it.Repo,
			ContentID: it.ContentID,
		}
	}
	return out
}
