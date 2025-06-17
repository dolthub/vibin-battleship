package main

import "fmt"

type TestStrategy struct {
	attackIndex int    // Tracks which coordinate to attack next
	playerColor string // Player color (red or blue)
}

func (t *TestStrategy) GetNextMove(gameState string) string {
	rows := "ABCDEFGHIJ"
	
	var coordinate string
	
	if t.playerColor == "blue" {
		// Blue attack pattern: A1, A2, A3, ..., A10, B1, B2, ..., J10 (column-first)
		row := t.attackIndex / 10      // 0-9 for A-J
		col := (t.attackIndex % 10) + 1 // 1-10 for columns
		coordinate = fmt.Sprintf("%c%d", rows[row], col)
	} else {
		// Red attack pattern: A1, B1, C1, ..., J1, A2, B2, ..., J10 (row-first)
		row := t.attackIndex % 10      // 0-9 for A-J
		col := (t.attackIndex / 10) + 1 // 1-10 for columns
		coordinate = fmt.Sprintf("%c%d", rows[row], col)
	}
	
	// Increment for next attack
	t.attackIndex++
	
	// Reset if we've gone through all 100 positions
	if t.attackIndex >= 100 {
		t.attackIndex = 0
	}
	
	return coordinate
}

func (t *TestStrategy) PlaceShips() []ShipPlacement {
	shipLengths := []int{5, 4, 3, 3, 2} // Carrier, Battleship, Cruiser, Submarine, Destroyer
	placements := []ShipPlacement{}
	
	if t.playerColor == "blue" {
		// Blue ships in lower half of board (rows 6-10) without overlaps
		// Place ships at different positions to avoid collisions
		shipPositions := []struct{
			pos string
			horizontal bool
		}{
			{"F6", true},  // Carrier (5): F6→J6
			{"A8", true},  // Battleship (4): A8→D8  
			{"F8", true},  // Cruiser (3): F8→H8
			{"A10", true}, // Submarine (3): A10→C10
			{"J9", false}, // Destroyer (2): J9→J10 (vertical)
		}
		
		for i := range shipLengths {
			placements = append(placements, ShipPlacement{
				Position:     shipPositions[i].pos,
				IsHorizontal: shipPositions[i].horizontal,
			})
		}
	} else {
		// Red ships in upper area (A-E, starting at row 1)
		rows := "ABCDEFGHIJ"
		
		for i := range shipLengths {
			position := fmt.Sprintf("%c1", rows[i]) // A1, B1, C1, D1, E1
			placements = append(placements, ShipPlacement{
				Position:     position,
				IsHorizontal: false, // Vertical placement (North direction)
			})
		}
	}
	
	return placements
}

func (t *TestStrategy) SetPlayerColor(color string) {
	t.playerColor = color
}