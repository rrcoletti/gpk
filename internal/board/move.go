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
