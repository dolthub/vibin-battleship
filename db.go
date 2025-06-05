package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	conn *sql.DB
}

func NewDB(host, port, user, password, database string) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, password, host, port, database)
	
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) CreateGamesTable() error {
	query := `CREATE TABLE IF NOT EXISTS games (
		id VARCHAR(36) PRIMARY KEY,
		player1_name VARCHAR(100) NOT NULL,
		player2_name VARCHAR(100) NOT NULL,
		winner VARCHAR(100) NOT NULL,
		total_shots_player1 INT NOT NULL,
		total_shots_player2 INT NOT NULL,
		game_duration_minutes INT,
		completed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create games table: %w", err)
	}

	return nil
}

func (db *DB) CreateGameBranch(gameID string) error {
	query := fmt.Sprintf("CALL DOLT_BRANCH('%s')", gameID)
	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create game branch %s: %w", gameID, err)
	}
	return nil
}

func (db *DB) CheckoutBranch(branch string) error {
	query := fmt.Sprintf("CALL DOLT_CHECKOUT('%s')", branch)
	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
	}
	return nil
}

func (db *DB) CommitChanges(message string) error {
	query := fmt.Sprintf("CALL DOLT_COMMIT('-m', '%s')", message)
	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}

func (db *DB) MergeBranch(branch, targetBranch string) error {
	if err := db.CheckoutBranch(targetBranch); err != nil {
		return err
	}
	
	query := fmt.Sprintf("CALL DOLT_MERGE('%s')", branch)
	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to merge branch %s into %s: %w", branch, targetBranch, err)
	}
	return nil
}

func (db *DB) BranchExists(branch string) (bool, error) {
	query := "SELECT COUNT(*) FROM DOLT_BRANCHES WHERE name = ?"
	var count int
	err := db.conn.QueryRow(query, branch).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check if branch exists: %w", err)
	}
	return count > 0, nil
}

func (db *DB) CreateTurnTable() error {
	// Check if table already exists
	var tableExists int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = 'turn' AND TABLE_SCHEMA = DATABASE()").Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("failed to check if turn table exists: %w", err)
	}

	if tableExists > 0 {
		return nil // Table already exists, no creation needed
	}

	query := `CREATE TABLE turn (
		player ENUM('red', 'blue') NOT NULL,
		value DECIMAL(15,9) NOT NULL,
		PRIMARY KEY (player)
	)`
	_, err = db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create turn table: %w", err)
	}

	// Stage and commit the turn table
	if _, err := db.conn.Exec("CALL DOLT_ADD('turn')"); err != nil {
		return fmt.Errorf("failed to stage turn table: %w", err)
	}

	if _, err := db.conn.Exec("CALL DOLT_COMMIT('-m', 'Create turn table')"); err != nil {
		return fmt.Errorf("failed to commit turn table creation: %w", err)
	}

	return nil
}

func (db *DB) CreateBoardTables() error {
	// Check if tables already exist
	var redTableExists, blueTableExists int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = 'red_board' AND TABLE_SCHEMA = DATABASE()").Scan(&redTableExists)
	if err != nil {
		return fmt.Errorf("failed to check if red_board table exists: %w", err)
	}
	
	err = db.conn.QueryRow("SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = 'blue_board' AND TABLE_SCHEMA = DATABASE()").Scan(&blueTableExists)
	if err != nil {
		return fmt.Errorf("failed to check if blue_board table exists: %w", err)
	}

	if redTableExists > 0 && blueTableExists > 0 {
		return nil // Both tables already exist
	}

	tablesCreated := false

	// Create red_board table if it doesn't exist
	if redTableExists == 0 {
		redBoardQuery := `CREATE TABLE red_board (
			x ENUM('A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J') NOT NULL,
			y INT NOT NULL CHECK (y >= 1 AND y <= 10),
			content CHAR(1) NOT NULL DEFAULT ' ',
			PRIMARY KEY (x, y)
		)`
		_, err := db.conn.Exec(redBoardQuery)
		if err != nil {
			return fmt.Errorf("failed to create red_board table: %w", err)
		}
		tablesCreated = true
	}

	// Create blue_board table if it doesn't exist
	if blueTableExists == 0 {
		blueBoardQuery := `CREATE TABLE blue_board (
			x ENUM('A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J') NOT NULL,
			y INT NOT NULL CHECK (y >= 1 AND y <= 10),
			content CHAR(1) NOT NULL DEFAULT ' ',
			PRIMARY KEY (x, y)
		)`
		_, err := db.conn.Exec(blueBoardQuery)
		if err != nil {
			return fmt.Errorf("failed to create blue_board table: %w", err)
		}
		tablesCreated = true
	}

	// Stage and commit board tables if any were created
	if tablesCreated {
		if _, err := db.conn.Exec("CALL DOLT_ADD('red_board', 'blue_board')"); err != nil {
			return fmt.Errorf("failed to stage board tables: %w", err)
		}

		if _, err := db.conn.Exec("CALL DOLT_COMMIT('-m', 'Create board tables')"); err != nil {
			return fmt.Errorf("failed to commit board tables creation: %w", err)
		}
	}

	return nil
}


func (db *DB) PlaceShip(player string, shipChar rune, startX string, startY int, isHorizontal bool, length int) error {
	tableName := fmt.Sprintf("%s_board", player)
	
	// Validate ship placement positions
	positions := []struct{ x string; y int }{}
	
	if isHorizontal {
		// Place horizontally
		for i := 0; i < length; i++ {
			xIndex := int(startX[0]-'A') + i
			if xIndex > 9 { // J is index 9
				return fmt.Errorf("ship extends beyond right edge of board")
			}
			newX := string(rune('A' + xIndex))
			positions = append(positions, struct{ x string; y int }{newX, startY})
		}
	} else {
		// Place vertically
		for i := 0; i < length; i++ {
			newY := startY + i
			if newY > 10 {
				return fmt.Errorf("ship extends beyond bottom edge of board")
			}
			positions = append(positions, struct{ x string; y int }{startX, newY})
		}
	}
	
	// Check for conflicts with existing ships
	for _, pos := range positions {
		var currentContent string
		query := fmt.Sprintf("SELECT content FROM %s WHERE x = ? AND y = ?", tableName)
		err := db.conn.QueryRow(query, pos.x, pos.y).Scan(&currentContent)
		if err != nil {
			// If no row exists, position is empty (which is fine)
			if err.Error() == "sql: no rows in result set" {
				continue
			}
			return fmt.Errorf("failed to check position %s%d: %w", pos.x, pos.y, err)
		}
		// If we found a row, position is occupied
		return fmt.Errorf("position %s%d is already occupied", pos.x, pos.y)
	}
	
	// Place the ship
	for _, pos := range positions {
		query := fmt.Sprintf("INSERT INTO %s (x, y, content) VALUES (?, ?, ?)", tableName)
		_, err := db.conn.Exec(query, pos.x, pos.y, string(shipChar))
		if err != nil {
			return fmt.Errorf("failed to place ship at position %s%d: %w", pos.x, pos.y, err)
		}
	}
	
	return nil
}

func (db *DB) GetBoardState(player string) (map[string]map[int]string, error) {
	tableName := fmt.Sprintf("%s_board", player)
	
	rows, err := db.conn.Query(fmt.Sprintf("SELECT x, y, content FROM %s", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query %s board: %w", player, err)
	}
	defer rows.Close()
	
	board := make(map[string]map[int]string)
	
	for rows.Next() {
		var x, content string
		var y int
		
		err := rows.Scan(&x, &y, &content)
		if err != nil {
			return nil, fmt.Errorf("failed to scan board position: %w", err)
		}
		
		if board[x] == nil {
			board[x] = make(map[int]string)
		}
		board[x][y] = content
	}
	
	return board, nil
}

func (db *DB) GetBoardForDisplay(player string) (map[Coordinate]string, error) {
	tableName := fmt.Sprintf("%s_board", player)
	
	// Only get actual ship positions, exclude hit/miss markers
	query := fmt.Sprintf("SELECT x, y, content FROM %s WHERE content NOT IN (' ', ?, ?)", tableName)
	rows, err := db.conn.Query(query, string(HIT_CHAR), string(MISS_CHAR))
	if err != nil {
		return nil, fmt.Errorf("failed to query %s board: %w", player, err)
	}
	defer rows.Close()
	
	board := make(map[Coordinate]string)
	
	for rows.Next() {
		var x, content string
		var y int
		
		err := rows.Scan(&x, &y, &content)
		if err != nil {
			return nil, fmt.Errorf("failed to scan board position: %w", err)
		}
		
		// Convert our A-J, 1-10 format to 0-9, 0-9 format for terminal display
		xIndex := int(x[0] - 'A')
		yIndex := y - 1
		
		coord := Coordinate{X: xIndex, Y: yIndex}
		board[coord] = "S" // Mark as ship for display
	}
	
	return board, nil
}

func (db *DB) GetCurrentTurn(gameID string) (string, error) {
	// Get player with highest value - it's their turn
	var currentPlayer string
	err := db.conn.QueryRow("SELECT player FROM turn ORDER BY value DESC LIMIT 1").Scan(&currentPlayer)
	if err != nil {
		return "", fmt.Errorf("failed to query current turn: %w", err)
	}

	return currentPlayer, nil
}


func (db *DB) ProcessAttack(attacker, targetX string, targetY int) (string, string, error) {
	// Determine the target player (opponent of attacker)
	var targetPlayer string
	if attacker == "red" {
		targetPlayer = "blue"
	} else {
		targetPlayer = "red"
	}

	// Check if the target position has a ship
	tableName := fmt.Sprintf("%s_board", targetPlayer)
	var shipChar string
	query := fmt.Sprintf("SELECT content FROM %s WHERE x = ? AND y = ?", tableName)
	err := db.conn.QueryRow(query, targetX, targetY).Scan(&shipChar)
	
	var result string
	var sunkShip string
	var resultChar rune
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			// No ship at this position - it's a miss
			result = "miss"
			resultChar = MISS_CHAR
			// Insert miss marker into target player's board
			insertQuery := fmt.Sprintf("INSERT INTO %s (x, y, content) VALUES (?, ?, ?)", tableName)
			_, err = db.conn.Exec(insertQuery, targetX, targetY, string(resultChar))
			if err != nil {
				return "", "", fmt.Errorf("failed to record miss: %w", err)
			}
		} else {
			return "", "", fmt.Errorf("failed to check target position: %w", err)
		}
	} else {
		// Ship found at this position - it's a hit
		result = "hit"
		resultChar = HIT_CHAR
		hitShipChar := rune(shipChar[0])
		
		// Update the existing ship position with hit marker
		updateQuery := fmt.Sprintf("UPDATE %s SET content = ? WHERE x = ? AND y = ?", tableName)
		_, err = db.conn.Exec(updateQuery, string(resultChar), targetX, targetY)
		if err != nil {
			return "", "", fmt.Errorf("failed to record hit: %w", err)
		}
		
		// Check if this was the last piece of this ship type
		remainingQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE content = ?", tableName)
		var remaining int
		err = db.conn.QueryRow(remainingQuery, string(hitShipChar)).Scan(&remaining)
		if err != nil {
			return "", "", fmt.Errorf("failed to check remaining ship pieces: %w", err)
		}
		
		// If no more pieces of this ship type remain, it's sunk
		if remaining == 0 {
			sunkShip = GetShipNameByChar(hitShipChar)
		}
	}

	// Increment the opponent's roll value to make it their turn
	updateTurnQuery := "UPDATE turn SET value = value + 1 WHERE player = ?"
	_, err = db.conn.Exec(updateTurnQuery, targetPlayer)
	if err != nil {
		return "", "", fmt.Errorf("failed to update turn order: %w", err)
	}

	return result, sunkShip, nil
}

func (db *DB) GetShotsMadeBy(player string) (map[Coordinate]string, error) {
	// Determine the opponent's board to check for shots made by this player
	var opponentPlayer string
	if player == "red" {
		opponentPlayer = "blue"
	} else {
		opponentPlayer = "red"
	}

	tableName := fmt.Sprintf("%s_board", opponentPlayer)
	
	// Get all HIT_CHAR and MISS_CHAR entries from opponent's board (these are shots made by this player)
	query := fmt.Sprintf("SELECT x, y, content FROM %s WHERE content IN (?, ?)", tableName)
	rows, err := db.conn.Query(query, string(HIT_CHAR), string(MISS_CHAR))
	if err != nil {
		return nil, fmt.Errorf("failed to query shots made by %s: %w", player, err)
	}
	defer rows.Close()
	
	shots := make(map[Coordinate]string)
	
	for rows.Next() {
		var x, content string
		var y int
		
		err := rows.Scan(&x, &y, &content)
		if err != nil {
			return nil, fmt.Errorf("failed to scan shot position: %w", err)
		}
		
		// Convert our A-J, 1-10 format to 0-9, 0-9 format for terminal display
		xIndex := int(x[0] - 'A')
		yIndex := y - 1
		
		coord := Coordinate{X: xIndex, Y: yIndex}
		if content == string(HIT_CHAR) {
			shots[coord] = "H" // Hit
		} else {
			shots[coord] = "M" // Miss
		}
	}
	
	return shots, nil
}

func (db *DB) GetShotsAgainst(player string) (map[Coordinate]string, error) {
	// Get all HIT_CHAR and MISS_CHAR entries from this player's board (these are shots made by opponent)
	tableName := fmt.Sprintf("%s_board", player)
	
	query := fmt.Sprintf("SELECT x, y, content FROM %s WHERE content IN (?, ?)", tableName)
	rows, err := db.conn.Query(query, string(HIT_CHAR), string(MISS_CHAR))
	if err != nil {
		return nil, fmt.Errorf("failed to query shots against %s: %w", player, err)
	}
	defer rows.Close()
	
	shots := make(map[Coordinate]string)
	
	for rows.Next() {
		var x, content string
		var y int
		
		err := rows.Scan(&x, &y, &content)
		if err != nil {
			return nil, fmt.Errorf("failed to scan shot position: %w", err)
		}
		
		// Convert our A-J, 1-10 format to 0-9, 0-9 format for terminal display
		xIndex := int(x[0] - 'A')
		yIndex := y - 1
		
		coord := Coordinate{X: xIndex, Y: yIndex}
		if content == string(HIT_CHAR) {
			shots[coord] = "H" // Hit
		} else {
			shots[coord] = "M" // Miss
		}
	}
	
	return shots, nil
}

func (db *DB) CheckGameComplete(gameID string) (bool, string, error) {
	// Count remaining ships for each player (not hit)
	var redShipsRemaining, blueShipsRemaining int
	
	// Count red's remaining ships (ship characters that aren't HIT_CHAR)
	redQuery := "SELECT COUNT(*) FROM red_board WHERE content NOT IN (' ', ?, ?)"
	err := db.conn.QueryRow(redQuery, string(HIT_CHAR), string(MISS_CHAR)).Scan(&redShipsRemaining)
	if err != nil {
		return false, "", fmt.Errorf("failed to count red ships: %w", err)
	}
	
	// Count blue's remaining ships (ship characters that aren't HIT_CHAR)
	blueQuery := "SELECT COUNT(*) FROM blue_board WHERE content NOT IN (' ', ?, ?)"
	err = db.conn.QueryRow(blueQuery, string(HIT_CHAR), string(MISS_CHAR)).Scan(&blueShipsRemaining)
	if err != nil {
		return false, "", fmt.Errorf("failed to count blue ships: %w", err)
	}
	
	// Game is complete if either player has no ships remaining
	if redShipsRemaining == 0 {
		return true, "blue", nil // Blue wins
	} else if blueShipsRemaining == 0 {
		return true, "red", nil // Red wins
	}
	
	return false, "", nil // Game continues
}

func (db *DB) CompleteGame(gameID, winner string) error {
	// Count total shots made by each player
	var redShots, blueShots int
	
	// Red's shots are hits/misses on blue's board
	err := db.conn.QueryRow("SELECT COUNT(*) FROM blue_board WHERE content IN (?, ?)", string(HIT_CHAR), string(MISS_CHAR)).Scan(&redShots)
	if err != nil {
		return fmt.Errorf("failed to count red shots: %w", err)
	}
	
	// Blue's shots are hits/misses on red's board
	err = db.conn.QueryRow("SELECT COUNT(*) FROM red_board WHERE content IN (?, ?)", string(HIT_CHAR), string(MISS_CHAR)).Scan(&blueShots)
	if err != nil {
		return fmt.Errorf("failed to count blue shots: %w", err)
	}
	
	// Switch to main branch to update games table
	if err := db.CheckoutBranch("main"); err != nil {
		return fmt.Errorf("failed to checkout main branch: %w", err)
	}
	
	// Update the games table with results
	updateQuery := `UPDATE games SET 
		winner = ?, 
		total_shots_player1 = ?, 
		total_shots_player2 = ?, 
		game_duration_minutes = 0
		WHERE id = ?`
	_, err = db.conn.Exec(updateQuery, winner, redShots, blueShots, gameID)
	if err != nil {
		return fmt.Errorf("failed to update games table: %w", err)
	}
	
	// Stage and commit the game completion
	if _, err := db.conn.Exec("CALL DOLT_ADD('games')"); err != nil {
		return fmt.Errorf("failed to stage game completion: %w", err)
	}
	
	commitMessage := fmt.Sprintf("Game %s completed - %s wins (%d vs %d shots)", gameID, winner, redShots, blueShots)
	if err := db.CommitChanges(commitMessage); err != nil {
		return fmt.Errorf("failed to commit game completion: %w", err)
	}
	
	// Merge the game branch into main
	if err := db.MergeBranch(gameID, "main"); err != nil {
		return fmt.Errorf("failed to merge game branch: %w", err)
	}
	
	return nil
}

func (db *DB) GetTableHash(tableName string) (string, error) {
	var hash string
	query := fmt.Sprintf("SELECT dolt_hashof_table('%s')", tableName)
	err := db.conn.QueryRow(query).Scan(&hash)
	if err != nil {
		return "", fmt.Errorf("failed to get hash of table %s: %w", tableName, err)
	}
	return hash, nil
}

func (db *DB) BothPlayersJoined() (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM turn").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to count players: %w", err)
	}
	return count == 2, nil
}

func (db *DB) BothPlayersPlacedShips() (bool, error) {
	// Check if both players have ships on their boards
	var redShips, blueShips int
	
	err := db.conn.QueryRow("SELECT COUNT(*) FROM red_board WHERE content NOT IN (?, ?)", string(HIT_CHAR), string(MISS_CHAR)).Scan(&redShips)
	if err != nil {
		return false, fmt.Errorf("failed to count red ships: %w", err)
	}
	
	err = db.conn.QueryRow("SELECT COUNT(*) FROM blue_board WHERE content NOT IN (?, ?)", string(HIT_CHAR), string(MISS_CHAR)).Scan(&blueShips)
	if err != nil {
		return false, fmt.Errorf("failed to count blue ships: %w", err)
	}
	
	// Both players should have 17 total ship squares (5+4+3+3+2)
	return redShips == 17 && blueShips == 17, nil
}