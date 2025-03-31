package main

import (
	"fmt"
	"log"
	"path/filepath"

	"battleship/pkg/database"
	"battleship/pkg/terminal"
)

func main() {
	// Initialize terminal colors
	term := terminal.New()

	gameId := "1"

	// Initialize database connection
	dbPath := filepath.Join("..", "data")
	db, err := database.New(gameId, dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize database tables
	if err := db.Initialize(); err != nil {
		log.Fatalf("Failed to initialize database tables: %v", err)
	}

	// Insert a carrier (5 units) for the red player at position (0,0) pointing east
	if err := db.InsertShip("red_ships", 0, 0, 5, database.Horizontal); err != nil {
		log.Printf("Warning: Failed to insert ship: %v", err)
	} else {
		fmt.Println("Successfully inserted carrier ship for red player")
	}

	// Print welcome message in color
	term.PrintWelcome()

	// Check for existing tables
	tables, err := db.GetTables()
	if err != nil {
		log.Printf("Warning: Failed to get tables: %v", err)
	} else {
		if len(tables) == 0 {
			fmt.Println("No tables found in the database.")
		} else {
			fmt.Println("Existing tables in the database:")
			for _, table := range tables {
				fmt.Printf("- %s\n", table)
			}
		}
	}

	// Retrieve the state of the board from the database
	rows, err := db.Query("SELECT x, y, board, state FROM board_states")
	if err != nil {
		log.Fatalf("Failed to retrieve board state: %v", err)
	}
	defer rows.Close()

	// Maps to hold the state of the board
	myShips := make(map[terminal.Coordinate]string)
	opponentShots := make(map[terminal.Coordinate]string)
	myShots := make(map[terminal.Coordinate]string)

	// Populate the maps with the retrieved data
	for rows.Next() {
		var x, y int
		var board, state string
		if err := rows.Scan(&x, &y, &board, &state); err != nil {
			log.Fatalf("Failed to scan board state: %v", err)
		}
		coord := terminal.Coordinate{X: x, Y: y}
		switch board {
		case "red_ships":
			myShips[coord] = state
		case "blue_ships":
			// Assuming blue_ships are opponent's ships
			// This can be adjusted based on the actual game logic
		case "red_shots":
			myShots[coord] = state
		case "blue_shots":
			opponentShots[coord] = state
		}
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating over board state rows: %v", err)
	}

	// Display both boards
	fmt.Println("\nGame Boards:")
	term.PrintBoards(myShips, opponentShots, myShots)

}
