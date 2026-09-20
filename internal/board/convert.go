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
			Fields:    it.Fields,
		}
	}
	return out
}
