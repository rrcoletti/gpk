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

import "fmt"

// MoveCard moves the card at (fromCol, cardIdx) to the end of toCol and
// returns the card's new index in the destination column.
func MoveCard(cols []Column, fromCol, cardIdx, toCol int) (int, error) {
	if fromCol < 0 || fromCol >= len(cols) || toCol < 0 || toCol >= len(cols) {
		return 0, fmt.Errorf("column index out of range (from %d to %d, have %d columns)", fromCol, toCol, len(cols))
	}
	if toCol == fromCol {
		return cardIdx, nil
	}
	cards := cols[fromCol].Cards
	if cardIdx < 0 || cardIdx >= len(cards) {
		return 0, fmt.Errorf("card index %d out of range in column %d", cardIdx, fromCol)
	}
	card := cards[cardIdx]
	cols[fromCol].Cards = append(cards[:cardIdx], cards[cardIdx+1:]...)
	cols[toCol].Cards = append(cols[toCol].Cards, card)
	return len(cols[toCol].Cards) - 1, nil
}
