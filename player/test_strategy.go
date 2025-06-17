package main

import "fmt"

type TestStrategy struct {
	attackIndex int // Tracks which coordinate to attack next
}

func (t *TestStrategy) GetNextMove(gameState string) string {
	// Attack pattern: A1, B1, C1, ..., J1, A2, B2, ..., J10
	rows := "ABCDEFGHIJ"
	
	row := t.attackIndex % 10      // 0-9 for A-J
	col := (t.attackIndex / 10) + 1 // 1-10 for columns
	
	// Increment for next attack
	t.attackIndex++
	
	// Reset if we've gone through all 100 positions
	if t.attackIndex >= 100 {
		t.attackIndex = 0
	}
	
	return fmt.Sprintf("%c%d", rows[row], col)
}

func (t *TestStrategy) PlaceShips() []ShipPlacement {
	// Deterministic ship placement: all ships vertical starting from A1, B1, C1, etc.
	shipLengths := []int{5, 4, 3, 3, 2} // Carrier, Battleship, Cruiser, Submarine, Destroyer
	rows := "ABCDEFGHIJ"
	
	placements := []ShipPlacement{}
	
	for i := range shipLengths {
		position := fmt.Sprintf("%c1", rows[i]) // A1, B1, C1, D1, E1
		placements = append(placements, ShipPlacement{
			Position:     position,
			IsHorizontal: false, // All ships placed vertically (North direction)
		})
	}
	
	return placements
}