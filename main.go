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

	// Initialize database connection
	dbPath := filepath.Join("..", "data")
	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize database tables
	if err := db.Initialize(); err != nil {
		log.Fatalf("Failed to initialize database tables: %v", err)
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

	// Example data for demonstration
	myShips := map[terminal.Coordinate]string{
		{X: 0, Y: 0}: "S", // Ship at A0
		{X: 1, Y: 0}: "S", // Ship at B0
		{X: 2, Y: 0}: "S", // Ship at C0
	}

	opponentShots := map[terminal.Coordinate]string{
		{X: 7, Y: 3}: "H", // Hit at H3
		{X: 5, Y: 5}: "X", // Miss at F5
	}

	myShots := map[terminal.Coordinate]string{
		{X: 3, Y: 4}: "H", // Hit at D4
		{X: 8, Y: 2}: "X", // Miss at I2
	}

	// Display both boards
	fmt.Println("\nGame Boards:")
	term.PrintBoards(myShips, opponentShots, myShots)
}
