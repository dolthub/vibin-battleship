package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Ship types and their properties for standard Battleship game
const (
	// Ship type characters used on the board
	CARRIER_CHAR    = 'C'
	BATTLESHIP_CHAR = 'B'
	CRUISER_CHAR    = 'R'
	SUBMARINE_CHAR  = 'S'
	DESTROYER_CHAR  = 'D'

	// Hit and miss markers
	HIT_CHAR   = 'X'
	MISS_CHAR  = 'O'
	EMPTY_CHAR = ' '
)

// Ship represents a ship type with its properties
type Ship struct {
	Name   string
	Length int
	Char   rune
}

// Standard fleet composition for Battleship game
var Ships = []Ship{
	{Name: "Carrier", Length: 5, Char: CARRIER_CHAR},
	{Name: "Battleship", Length: 4, Char: BATTLESHIP_CHAR},
	{Name: "Cruiser", Length: 3, Char: CRUISER_CHAR},
	{Name: "Submarine", Length: 3, Char: SUBMARINE_CHAR},
	{Name: "Destroyer", Length: 2, Char: DESTROYER_CHAR},
}

// GetShipNameByChar returns the ship name for a given character
func GetShipNameByChar(char rune) string {
	for _, ship := range Ships {
		if ship.Char == char {
			return ship.Name
		}
	}
	return "Unknown"
}

func main() {
	runMain("127.0.0.1", "3306", "battleship")
}

func runMain(host, port, database string) {
	// Initialize database connection
	db, err := NewDB(host, port, "root", "", database)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		return
	}
	defer db.Close()

	// Ensure games table exists
	if err := db.CreateGamesTable(); err != nil {
		fmt.Printf("Failed to create games table: %v\n", err)
		return
	}

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	switch command {
	case "new":
		handleNewGame(db)
	case "join":
		handleJoinGame(db)
	case "play":
		handlePlay(db)
	case "list":
		handleListGames(db)
	case "print":
		handlePrintBoard(db)
	case "attack":
		handleAttack(db)
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Battleship - Command Line Game")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  battleship new                           - Create a new game")
	fmt.Println("  battleship join <game_id> <red|blue>     - Join a game as red or blue player")
	fmt.Println("  battleship play <game_id> <red|blue>     - Play an interactive game")
	fmt.Println("  battleship print <game_id> <red|blue>    - Print current board state")
	fmt.Println("  battleship attack <game_id> <red|blue> <coordinate> - Attack a coordinate (e.g., A5)")
	fmt.Println("  battleship list                          - List all games")
	fmt.Println("  battleship help                          - Show this help")
}

func handleNewGame(db *DB) {
	gameID := uuid.New().String()

	// Insert initial game row into games table
	query := `INSERT INTO games (id, player1_name, player2_name, winner, total_shots_player1, total_shots_player2, game_duration_minutes) 
			  VALUES (?, '', '', '', 0, 0, 0)`
	_, err := db.conn.Exec(query, gameID)
	if err != nil {
		fmt.Printf("Failed to create game: %v\n", err)
		return
	}

	// Stage and commit the game creation to Dolt
	if _, err := db.conn.Exec("CALL DOLT_ADD('games')"); err != nil {
		fmt.Printf("Failed to stage changes: %v\n", err)
		return
	}

	commitMessage := fmt.Sprintf("Create new game %s", gameID)
	if err := db.CommitChanges(commitMessage); err != nil {
		fmt.Printf("Failed to commit game creation: %v\n", err)
		return
	}

	fmt.Printf("New game created with ID: %s\n", gameID)
	fmt.Println("Share this ID with another player to join the game.")
}

func handleJoinGame(db *DB) {
	if len(os.Args) < 4 {
		fmt.Println("Usage: battleship join <game_id> <red|blue>")
		return
	}

	gameID := os.Args[2]
	color := os.Args[3]

	if color != "red" && color != "blue" {
		fmt.Println("Player color must be 'red' or 'blue'")
		return
	}

	// Check if game branch exists, create if it doesn't
	branchExists, err := db.BranchExists(gameID)
	if err != nil {
		fmt.Printf("Failed to check if game branch exists: %v\n", err)
		return
	}

	if !branchExists {
		if err := db.CreateGameBranch(gameID); err != nil {
			fmt.Printf("Failed to create game branch: %v\n", err)
			return
		}
	}

	// Checkout to the game branch
	if err := db.CheckoutBranch(gameID); err != nil {
		fmt.Printf("Failed to checkout game branch: %v\n", err)
		return
	}

	// Create turn table (handles its own commit)
	if err := db.CreateTurnTable(); err != nil {
		fmt.Printf("Failed to create turn table: %v\n", err)
		return
	}

	// Create board tables (handles its own commit)
	if err := db.CreateBoardTables(); err != nil {
		fmt.Printf("Failed to create board tables: %v\n", err)
		return
	}

	// Generate random value between 0-1
	randomValue := rand.Float64()

	// Insert player's turn value
	query := `INSERT INTO turn (player, value) VALUES (?, ?) 
			  ON DUPLICATE KEY UPDATE value = VALUES(value)`
	_, err = db.conn.Exec(query, color, randomValue)
	if err != nil {
		fmt.Printf("Failed to insert turn value: %v\n", err)
		return
	}

	// Check if this is the second player
	var playerCount int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM turn").Scan(&playerCount)
	if err != nil {
		fmt.Printf("Failed to count players: %v\n", err)
		return
	}

	// Stage and commit player's roll
	if _, err := db.conn.Exec("CALL DOLT_ADD('turn')"); err != nil {
		fmt.Printf("Failed to stage player roll: %v\n", err)
		return
	}

	commitMessage := fmt.Sprintf("Player %s rolled %.9f", color, randomValue)
	if _, err := db.conn.Exec(fmt.Sprintf("CALL DOLT_COMMIT('-m', '%s')", commitMessage)); err != nil {
		fmt.Printf("Failed to commit player roll: %v\n", err)
		return
	}

	fmt.Printf("Joined game %s as %s player with value: %.9f\n", gameID, color, randomValue)

	// Skip ship placement during testing by checking for test environment
	if os.Getenv("BATTLESHIP_TESTING") != "true" {
		// Start ship placement process
		fmt.Println("\nNow place your ships on the board!")
		if err := placeShipsForPlayer(db, color); err != nil {
			fmt.Printf("Failed to place ships: %v\n", err)
			return
		}

		// Commit ship placements
		if _, err := db.conn.Exec(fmt.Sprintf("CALL DOLT_ADD('%s_board')", color)); err != nil {
			fmt.Printf("Failed to stage ship placements: %v\n", err)
			return
		}

		if _, err := db.conn.Exec(fmt.Sprintf("CALL DOLT_COMMIT('-m', '%s player placed ships')", color)); err != nil {
			fmt.Printf("Failed to commit ship placements: %v\n", err)
			return
		}

		fmt.Printf("\n%s player has completed ship placement!\n", color)

		// Display the board with ships placed
		displayPlayerBoard(db, color)
	} else {
		fmt.Printf("\n%s player joined (ship placement skipped during testing)\n", color)
	}

	// If this is the second player, determine turn order
	if playerCount == 2 {
		rows, err := db.conn.Query("SELECT player, value FROM turn ORDER BY value DESC")
		if err != nil {
			fmt.Printf("Failed to query turn order: %v\n", err)
			return
		}
		defer rows.Close()

		var firstPlayer string
		var firstValue float64
		if rows.Next() {
			rows.Scan(&firstPlayer, &firstValue)
			fmt.Printf("\nBoth players have joined! %s player goes first (value: %.9f)\n", firstPlayer, firstValue)
		}
	} else {
		fmt.Println("Waiting for the other player to join...")
	}
}

func placeShipsForPlayer(db *DB, player string) error {
	fmt.Printf("\nPlacing ships for %s player:\n", player)
	
	// Show initial empty board
	fmt.Printf("\nStarting with empty board:\n")
	displayPlayerBoard(db, player)

	for _, ship := range Ships {
		fmt.Printf("\nPlacing %s (length %d):\n", ship.Name, ship.Length)

		for {
			// Get starting position
			fmt.Printf("Enter starting position (e.g., A1): ")
			var position string
			fmt.Scanln(&position)

			if len(position) < 2 {
				fmt.Println("Invalid position. Please use format like A1, B5, etc.")
				continue
			}

			startX := string(position[0])
			startY := 0

			// Parse Y coordinate
			if _, err := fmt.Sscanf(position[1:], "%d", &startY); err != nil {
				fmt.Println("Invalid Y coordinate. Please use numbers 1-10.")
				continue
			}

			// Validate coordinates
			if startX < "A" || startX > "J" {
				fmt.Println("Invalid X coordinate. Please use letters A-J.")
				continue
			}

			if startY < 1 || startY > 10 {
				fmt.Println("Invalid Y coordinate. Please use numbers 1-10.")
				continue
			}

			// Get orientation
			fmt.Printf("Place horizontally? (y/n): ")
			var orientation string
			fmt.Scanln(&orientation)

			isHorizontal := orientation == "y" || orientation == "Y"

			// Try to place the ship
			err := db.PlaceShip(player, ship.Char, startX, startY, isHorizontal, ship.Length)
			if err != nil {
				fmt.Printf("Cannot place ship: %v\n", err)
				fmt.Println("Please try again.")
				continue
			}

			fmt.Printf("%s placed successfully!\n", ship.Name)
			
			// Show updated board after each ship placement
			fmt.Printf("\nCurrent board state:\n")
			displayPlayerBoard(db, player)
			break
		}
	}

	return nil
}

func displayPlayerBoard(db *DB, player string) {
	// Get board state for display
	myShips, err := db.GetBoardForDisplay(player)
	if err != nil {
		fmt.Printf("Failed to get board state for display: %v\n", err)
		return
	}

	// Get shots made by this player (to show on right board)
	myShots, err := db.GetShotsMadeBy(player)
	if err != nil {
		fmt.Printf("Failed to get shots made by player: %v\n", err)
		return
	}

	// Get shots made against this player (to show on left board)
	opponentShots, err := db.GetShotsAgainst(player)
	if err != nil {
		fmt.Printf("Failed to get shots made against player: %v\n", err)
		return
	}

	// Create terminal instance and display board
	terminal := New()
	fmt.Printf("\n%s Player's Board:\n", strings.ToUpper(player[:1])+player[1:])
	terminal.PrintBoards(myShips, opponentShots, myShots, player)
}

func handlePrintBoard(db *DB) {
	if len(os.Args) < 4 {
		fmt.Println("Usage: battleship print <game_id> <red|blue>")
		return
	}

	gameID := os.Args[2]
	player := os.Args[3]

	if player != "red" && player != "blue" {
		fmt.Println("Player must be 'red' or 'blue'")
		return
	}

	// First check if game exists in main branch (games table)
	var gameExists int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM games WHERE id = ?", gameID).Scan(&gameExists)
	if err != nil {
		fmt.Printf("Failed to check if game exists: %v\n", err)
		return
	}

	if gameExists == 0 {
		fmt.Printf("Game %s does not exist\n", gameID)
		return
	}

	// Check if game branch exists (created when first player joins)
	branchExists, err := db.BranchExists(gameID)
	if err != nil {
		fmt.Printf("Failed to check if game branch exists: %v\n", err)
		return
	}

	if !branchExists {
		fmt.Printf("Game %s exists but no players have joined yet\n", gameID)
		return
	}

	// Checkout to the game branch
	if err := db.CheckoutBranch(gameID); err != nil {
		fmt.Printf("Failed to checkout game branch: %v\n", err)
		return
	}

	// Check if the player has actually joined by looking at the turn table
	var playerInTurn int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM turn WHERE player = ?", player).Scan(&playerInTurn)
	if err != nil {
		fmt.Printf("Failed to check if %s player has joined: %v\n", player, err)
		return
	}

	if playerInTurn == 0 {
		fmt.Printf("%s player has not joined this game yet\n", strings.ToUpper(player[:1])+player[1:])
		return
	}

	// Display the board
	fmt.Printf("Current board state for game %s:\n", gameID)
	displayPlayerBoard(db, player)
}

func handleListGames(db *DB) {
	fmt.Println("Games:")

	// Query all games from the database
	rows, err := db.conn.Query("SELECT id FROM games ORDER BY completed_at DESC")
	if err != nil {
		fmt.Printf("Failed to query games: %v\n", err)
		return
	}
	defer rows.Close()

	// Display each game
	for rows.Next() {
		var id string

		err := rows.Scan(&id)
		if err != nil {
			fmt.Printf("Failed to scan game row: %v\n", err)
			continue
		}

		fmt.Printf("Game ID: %s\n", id)
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("Error iterating over games: %v\n", err)
	}
}

func handleAttack(db *DB) {
	if len(os.Args) < 5 {
		fmt.Println("Usage: battleship attack <game_id> <red|blue> <coordinate>")
		return
	}

	gameID := os.Args[2]
	player := os.Args[3]
	coordinate := os.Args[4]

	if player != "red" && player != "blue" {
		fmt.Println("Player must be 'red' or 'blue'")
		return
	}

	// Parse coordinate (e.g., "A5" -> x="A", y=5)
	if len(coordinate) < 2 {
		fmt.Println("Invalid coordinate format. Use format like A1, B5, etc.")
		return
	}

	targetX := string(coordinate[0])
	var targetY int
	if _, err := fmt.Sscanf(coordinate[1:], "%d", &targetY); err != nil {
		fmt.Println("Invalid Y coordinate. Please use numbers 1-10.")
		return
	}

	// Validate coordinates
	if targetX < "A" || targetX > "J" {
		fmt.Println("Invalid X coordinate. Please use letters A-J.")
		return
	}

	if targetY < 1 || targetY > 10 {
		fmt.Println("Invalid Y coordinate. Please use numbers 1-10.")
		return
	}

	// First check if game exists in main branch
	var gameExists int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM games WHERE id = ?", gameID).Scan(&gameExists)
	if err != nil {
		fmt.Printf("Failed to check if game exists: %v\n", err)
		return
	}

	if gameExists == 0 {
		fmt.Printf("Game %s does not exist\n", gameID)
		return
	}

	// Check if game branch exists
	branchExists, err := db.BranchExists(gameID)
	if err != nil {
		fmt.Printf("Failed to check if game branch exists: %v\n", err)
		return
	}

	if !branchExists {
		fmt.Printf("Game %s exists but no players have joined yet\n", gameID)
		return
	}

	// Checkout to the game branch
	if err := db.CheckoutBranch(gameID); err != nil {
		fmt.Printf("Failed to checkout game branch: %v\n", err)
		return
	}

	// Check if the attacking player has joined
	var playerInTurn int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM turn WHERE player = ?", player).Scan(&playerInTurn)
	if err != nil {
		fmt.Printf("Failed to check if %s player has joined: %v\n", player, err)
		return
	}

	if playerInTurn == 0 {
		fmt.Printf("%s player has not joined this game yet\n", strings.ToUpper(player[:1])+player[1:])
		return
	}

	// Verify it's the player's turn
	currentTurn, err := db.GetCurrentTurn(gameID)
	if err != nil {
		fmt.Printf("Failed to determine current turn: %v\n", err)
		return
	}

	if currentTurn != player {
		fmt.Printf("It's not your turn. Current turn: %s player\n", currentTurn)
		return
	}

	// Determine the target player (opponent of attacker) 
	var targetPlayer string
	if player == "red" {
		targetPlayer = "blue"
	} else {
		targetPlayer = "red"
	}

	// Check if this coordinate has already been attacked by checking for HIT_CHAR or MISS_CHAR
	tableName := fmt.Sprintf("%s_board", targetPlayer)
	var existingContent string
	checkQuery := fmt.Sprintf("SELECT content FROM %s WHERE x = ? AND y = ?", tableName)
	err = db.conn.QueryRow(checkQuery, targetX, targetY).Scan(&existingContent)
	if err == nil {
		// Position exists, check if it's already been shot at
		if existingContent == string(HIT_CHAR) || existingContent == string(MISS_CHAR) {
			fmt.Printf("Coordinate %s has already been attacked\n", coordinate)
			return
		}
	} else if err.Error() != "sql: no rows in result set" {
		fmt.Printf("Failed to check coordinate: %v\n", err)
		return
	}

	// Process the attack
	result, sunkShip, err := db.ProcessAttack(player, targetX, targetY)
	if err != nil {
		fmt.Printf("Failed to process attack: %v\n", err)
		return
	}

	// Stage and commit the attack on the target player's board and turn table
	stageBoardQuery := fmt.Sprintf("CALL DOLT_ADD('%s_board')", targetPlayer)
	if _, err := db.conn.Exec(stageBoardQuery); err != nil {
		fmt.Printf("Failed to stage attack: %v\n", err)
		return
	}
	
	if _, err := db.conn.Exec("CALL DOLT_ADD('turn')"); err != nil {
		fmt.Printf("Failed to stage turn update: %v\n", err)
		return
	}

	commitMessage := fmt.Sprintf("%s player attacked %s - %s", player, coordinate, result)
	if err := db.CommitChanges(commitMessage); err != nil {
		fmt.Printf("Failed to commit attack: %v\n", err)
		return
	}

	// Display result
	if result == "hit" {
		fmt.Printf("🎯 HIT! %s player hit a ship at %s\n", strings.ToUpper(player[:1])+player[1:], coordinate)
		if sunkShip != "" {
			fmt.Printf("💥 You sunk my %s!\n", sunkShip)
		}
	} else {
		fmt.Printf("💦 MISS! %s player missed at %s\n", strings.ToUpper(player[:1])+player[1:], coordinate)
	}

	// Determine next player
	nextTurn, err := db.GetCurrentTurn(gameID)
	if err != nil {
		fmt.Printf("Failed to determine next turn: %v\n", err)
		return
	}

	fmt.Printf("Next turn: %s player\n", nextTurn)
}

func handlePlay(db *DB) {
	if len(os.Args) < 4 {
		fmt.Println("Usage: battleship play <game_id> <red|blue>")
		return
	}

	gameID := os.Args[2]
	player := os.Args[3]

	if player != "red" && player != "blue" {
		fmt.Println("Player color must be 'red' or 'blue'")
		return
	}

	// Use existing join logic to set up the game and place ships
	handleJoinGame(db)

	// Checkout to the game branch for the play session
	if err := db.CheckoutBranch(gameID); err != nil {
		fmt.Printf("Failed to checkout game branch: %v\n", err)
		return
	}

	// Wait for both players to place ships
	fmt.Println("Waiting for opponent to place ships...")
	for {
		bothPlaced, err := db.BothPlayersPlacedShips()
		if err != nil {
			fmt.Printf("Failed to check if both players placed ships: %v\n", err)
			return
		}
		if bothPlaced {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	fmt.Println("Both players have placed ships! Game starting...")

	// Determine turn order
	rows, err := db.conn.Query("SELECT player, value FROM turn ORDER BY value DESC")
	if err != nil {
		fmt.Printf("Failed to query turn order: %v\n", err)
		return
	}
	defer rows.Close()

	var firstPlayer string
	var firstValue float64
	if rows.Next() {
		rows.Scan(&firstPlayer, &firstValue)
		fmt.Printf("%s player goes first (value: %.9f)\n", strings.ToUpper(firstPlayer[:1])+firstPlayer[1:], firstValue)
	}

	// Main game loop
	fmt.Println("\n=== GAME STARTED ===")
	for {
		// Check if game is complete
		gameComplete, winner, err := db.CheckGameComplete(gameID)
		if err != nil {
			fmt.Printf("Failed to check game completion: %v\n", err)
			return
		}

		if gameComplete {
			fmt.Printf("\n🎉 GAME OVER! %s player wins!\n", strings.ToUpper(winner[:1])+winner[1:])
			
			// Complete the game
			if err := db.CompleteGame(gameID, winner); err != nil {
				fmt.Printf("Failed to complete game: %v\n", err)
				return
			}
			
			fmt.Println("Game results have been saved.")
			return
		}

		// Get current turn
		currentTurn, err := db.GetCurrentTurn(gameID)
		if err != nil {
			fmt.Printf("Failed to determine current turn: %v\n", err)
			return
		}

		if currentTurn == player {
			// It's this player's turn - prompt for attack
			for {
				fmt.Printf("\n🎯 Your turn! Enter attack coordinate (e.g., A5): ")
				var coordinate string
				fmt.Scanln(&coordinate)

				if len(coordinate) < 2 {
					fmt.Println("Invalid coordinate format. Use format like A1, B5, etc.")
					continue
				}

				targetX := string(coordinate[0])
				var targetY int
				if _, err := fmt.Sscanf(coordinate[1:], "%d", &targetY); err != nil {
					fmt.Println("Invalid Y coordinate. Please use numbers 1-10.")
					continue
				}

				// Validate coordinates
				if targetX < "A" || targetX > "J" {
					fmt.Println("Invalid X coordinate. Please use letters A-J.")
					continue
				}

				if targetY < 1 || targetY > 10 {
					fmt.Println("Invalid Y coordinate. Please use numbers 1-10.")
					continue
				}

				// Determine the target player
				var targetPlayer string
				if player == "red" {
					targetPlayer = "blue"
				} else {
					targetPlayer = "red"
				}

				// Check if this coordinate has already been attacked
				tableName := fmt.Sprintf("%s_board", targetPlayer)
				var existingContent string
				checkQuery := fmt.Sprintf("SELECT content FROM %s WHERE x = ? AND y = ?", tableName)
				err = db.conn.QueryRow(checkQuery, targetX, targetY).Scan(&existingContent)
				if err == nil {
					// Position exists, check if it's already been shot at
					if existingContent == string(HIT_CHAR) || existingContent == string(MISS_CHAR) {
						fmt.Printf("Coordinate %s has already been attacked\n", coordinate)
						continue
					}
				} else if err.Error() != "sql: no rows in result set" {
					fmt.Printf("Failed to check coordinate: %v\n", err)
					continue
				}

				// Process the attack
				result, sunkShip, err := db.ProcessAttack(player, targetX, targetY)
				if err != nil {
					fmt.Printf("Failed to process attack: %v\n", err)
					continue
				}

				// Stage and commit the attack
				stageBoardQuery := fmt.Sprintf("CALL DOLT_ADD('%s_board')", targetPlayer)
				if _, err := db.conn.Exec(stageBoardQuery); err != nil {
					fmt.Printf("Failed to stage attack: %v\n", err)
					continue
				}

				if _, err := db.conn.Exec("CALL DOLT_ADD('turn')"); err != nil {
					fmt.Printf("Failed to stage turn update: %v\n", err)
					continue
				}

				commitMessage := fmt.Sprintf("%s player attacked %s - %s", player, coordinate, result)
				if err := db.CommitChanges(commitMessage); err != nil {
					fmt.Printf("Failed to commit attack: %v\n", err)
					continue
				}

				// Display result
				if result == "hit" {
					fmt.Printf("🎯 HIT! You hit a ship at %s\n", coordinate)
					if sunkShip != "" {
						fmt.Printf("💥 You sunk their %s!\n", sunkShip)
					}
				} else {
					fmt.Printf("💦 MISS! You missed at %s\n", coordinate)
				}

				// Show updated board
				fmt.Println("\nYour current view:")
				displayPlayerBoard(db, player)
				break
			}

		} else {
			// Wait for opponent's turn
			fmt.Printf("Waiting for %s player's move...\n", currentTurn)
			
			// Get initial table hashes
			initialTurnHash, err := db.GetTableHash("turn")
			if err != nil {
				fmt.Printf("Failed to get turn table hash: %v\n", err)
				return
			}
			
			initialBoardHash, err := db.GetTableHash(fmt.Sprintf("%s_board", player))
			if err != nil {
				fmt.Printf("Failed to get board table hash: %v\n", err)
				return
			}

			// Poll for changes
			var currentBoardHash string
			for {
				time.Sleep(250 * time.Millisecond)
				
				currentTurnHash, err := db.GetTableHash("turn")
				if err != nil {
					fmt.Printf("Failed to get current turn table hash: %v\n", err)
					return
				}
				
				currentBoardHash, err = db.GetTableHash(fmt.Sprintf("%s_board", player))
				if err != nil {
					fmt.Printf("Failed to get current board table hash: %v\n", err)
					return
				}

				// Check if turn has changed or our board was attacked
				if currentTurnHash != initialTurnHash || currentBoardHash != initialBoardHash {
					break
				}
			}

			// Show updated board if we were attacked
			if initialBoardHash != currentBoardHash {
				fmt.Println("\n💥 You were attacked!")
				displayPlayerBoard(db, player)
			}
		}
	}
}
