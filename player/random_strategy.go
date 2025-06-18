package main

import (
	"fmt"
	"math/rand"
	"time"
)

type RandomStrategy struct {
	playerColor         string
	unattackedPositions []string
}

func (r *RandomStrategy) GetNextMove(gameState string, lastMoveHit bool, sunkShipType string) string {
	// Initialize the unattacked positions list if it doesn't exist
	if r.unattackedPositions == nil {
		r.unattackedPositions = make([]string, 0, 100)
		rows := "ABCDEFGHIJ"

		// Build all 100 possible board positions
		for _, row := range rows {
			for col := 1; col <= 10; col++ {
				if col == 10 {
					r.unattackedPositions = append(r.unattackedPositions, fmt.Sprintf("%c10", row))
				} else {
					r.unattackedPositions = append(r.unattackedPositions, fmt.Sprintf("%c%d", row, col))
				}
			}
		}
	}

	// Select a random position from remaining unattacked positions
	index := rand.Intn(len(r.unattackedPositions))
	coordinate := r.unattackedPositions[index]

	// Remove the selected position from unattacked list
	r.unattackedPositions[index] = r.unattackedPositions[len(r.unattackedPositions)-1]
	r.unattackedPositions = r.unattackedPositions[:len(r.unattackedPositions)-1]

	return coordinate
}

func (r *RandomStrategy) PlaceShips() []ShipPlacement {
	rand.Seed(time.Now().UnixNano())

	shipLengths := []int{5, 4, 3, 3, 2} // Carrier, Battleship, Cruiser, Submarine, Destroyer
	rows := "ABCDEFGHIJ"

	// Try multiple times to generate a complete valid layout
	for layoutAttempts := 0; layoutAttempts < 1000; layoutAttempts++ {
		placements := []ShipPlacement{}
		occupiedPositions := make(map[string]bool) // Track occupied board positions
		allShipsPlaced := true

		for _, length := range shipLengths {
			shipPlaced := false
			for attempts := 0; attempts < 100; attempts++ {
				row := rows[rand.Intn(10)]
				col := rand.Intn(10) + 1
				horizontal := rand.Intn(2) == 0

				position := fmt.Sprintf("%c%d", row, col)

				if isValidPlacement(position, horizontal, length, occupiedPositions) {
					// Mark all positions this ship will occupy
					shipPositions := getShipPositions(position, horizontal, length)
					for _, pos := range shipPositions {
						occupiedPositions[pos] = true
					}

					placements = append(placements, ShipPlacement{
						Position:     position,
						IsHorizontal: horizontal,
					})
					shipPlaced = true
					break
				}
			}

			if !shipPlaced {
				allShipsPlaced = false
				break // Failed to place this ship, try a new layout
			}
		}

		if allShipsPlaced {
			return placements // Successfully placed all ships
		}
	}

	// If we couldn't generate a valid layout after many attempts,
	// return an empty slice - the AI player should fail
	return []ShipPlacement{}
}

func isValidPlacement(position string, horizontal bool, length int, occupiedPositions map[string]bool) bool {
	if len(position) < 2 {
		return false
	}

	// Fix coordinate system: letter = column, number = row
	col := int(position[0] - 'A') // A-J = columns 0-9
	row := 0

	if len(position) == 2 {
		row = int(position[1] - '1') // 1-9 = rows 0-8
	} else if len(position) == 3 && position[1:] == "10" {
		row = 9 // 10 = row 9
	} else {
		return false
	}

	if row < 0 || row > 9 || col < 0 || col > 9 {
		return false
	}

	// Check board boundaries
	if horizontal {
		// Horizontal: ship extends across columns (A->B->C...)
		if col+length-1 > 9 {
			return false
		}
	} else {
		// Vertical: ship extends down rows (1->2->3...)
		if row+length-1 > 9 {
			return false
		}
	}

	// Check for overlaps with existing ships
	shipPositions := getShipPositions(position, horizontal, length)
	for _, pos := range shipPositions {
		if occupiedPositions[pos] {
			return false // Position already occupied
		}
	}

	return true
}

func getShipPositions(position string, horizontal bool, length int) []string {
	positions := []string{}

	// Fix coordinate system: letter = column, number = row
	col := int(position[0] - 'A') // A-J = columns 0-9
	row := 0

	if len(position) == 2 {
		row = int(position[1] - '1') // 1-9 = rows 0-8
	} else if len(position) == 3 && position[1:] == "10" {
		row = 9 // 10 = row 9
	}

	if horizontal {
		// Place horizontally: extend across columns (A->B->C...)
		for i := 0; i < length; i++ {
			newCol := string(rune('A' + col + i))
			newRow := row + 1 // Convert back to 1-10 format
			if newRow == 10 {
				positions = append(positions, fmt.Sprintf("%s10", newCol))
			} else {
				positions = append(positions, fmt.Sprintf("%s%d", newCol, newRow))
			}
		}
	} else {
		// Place vertically: extend down rows (1->2->3...)
		for i := 0; i < length; i++ {
			newCol := string(rune('A' + col))
			newRow := (row + i) + 1 // Convert back to 1-10 format
			if newRow == 10 {
				positions = append(positions, fmt.Sprintf("%s10", newCol))
			} else {
				positions = append(positions, fmt.Sprintf("%s%d", newCol, newRow))
			}
		}
	}

	return positions
}

func (r *RandomStrategy) SetPlayerColor(color string) {
	r.playerColor = color
}
