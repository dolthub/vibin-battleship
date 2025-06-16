package main

import (
	"fmt"
	"math/rand"
	"time"
)

type RandomStrategy struct{}

func (r *RandomStrategy) GetNextMove(gameState string) string {
	rows := "ABCDEFGHIJ"
	cols := "1234567890"
	
	row := string(rows[rand.Intn(10)])
	col := string(cols[rand.Intn(10)])
	if col == "0" {
		col = "10"
	}
	return row + col
}

func (r *RandomStrategy) PlaceShips() []ShipPlacement {
	rand.Seed(time.Now().UnixNano())
	
	shipLengths := []int{5, 4, 3, 3, 2} // Carrier, Battleship, Cruiser, Submarine, Destroyer
	rows := "ABCDEFGHIJ"
	placements := []ShipPlacement{}
	
	for _, length := range shipLengths {
		for attempts := 0; attempts < 100; attempts++ {
			row := rows[rand.Intn(10)]
			col := rand.Intn(10) + 1
			horizontal := rand.Intn(2) == 0
			
			position := fmt.Sprintf("%c%d", row, col)
			
			if isValidPlacement(position, horizontal, length) {
				placements = append(placements, ShipPlacement{
					Position:     position,
					IsHorizontal: horizontal,
				})
				break
			}
		}
	}
	
	return placements
}

func isValidPlacement(position string, horizontal bool, length int) bool {
	if len(position) < 2 {
		return false
	}
	
	row := int(position[0] - 'A')
	col := 0
	
	if len(position) == 2 {
		col = int(position[1] - '1')
	} else if len(position) == 3 && position[1:] == "10" {
		col = 9
	} else {
		return false
	}
	
	if row < 0 || row > 9 || col < 0 || col > 9 {
		return false
	}
	
	if horizontal {
		return col+length-1 <= 9
	} else {
		return row+length-1 <= 9
	}
}