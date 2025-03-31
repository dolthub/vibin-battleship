package commands

import (
	"fmt"
	"time"

	"battleship/pkg/database"
	"battleship/pkg/terminal"
)

// Command represents a command that can be executed
type Command interface {
	Execute(gameID string) error
}

// StartCommand handles starting a new game
type StartCommand struct {
	db *database.Database
}

// JoinRedCommand handles joining an existing game as the red team
type JoinRedCommand struct {
	db *database.Database
}

// JoinBlueCommand handles joining an existing game as the blue team
type JoinBlueCommand struct {
	db *database.Database
}

// WatchCommand handles watching an existing game
type WatchCommand struct {
	db   *database.Database
	team string // "red" or "blue"
}

// NewStartCommand creates a new StartCommand
func NewStartCommand(db *database.Database) *StartCommand {
	return &StartCommand{db: db}
}

// NewJoinRedCommand creates a new JoinRedCommand
func NewJoinRedCommand(db *database.Database) *JoinRedCommand {
	return &JoinRedCommand{db: db}
}

// NewJoinBlueCommand creates a new JoinBlueCommand
func NewJoinBlueCommand(db *database.Database) *JoinBlueCommand {
	return &JoinBlueCommand{db: db}
}

// NewWatchCommand creates a new WatchCommand
func NewWatchCommand(db *database.Database, team string) *WatchCommand {
	return &WatchCommand{db: db, team: team}
}

// Execute implements the Command interface for StartCommand
func (c *StartCommand) Execute(gameID string) error {
	if gameID == "" {
		return fmt.Errorf("start command requires a game ID")
	}
	// TODO: Implement game start logic
	// Initialize the database
	if err := c.db.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}

	fmt.Printf("Game with ID %s has been started.\n", gameID)
	return nil
}

// Execute implements the Command interface for JoinRedCommand
func (c *JoinRedCommand) Execute(gameID string) error {
	if gameID == "" {
		return fmt.Errorf("join-red command requires a game ID")
	}
	// TODO: Implement game join logic for red team
	fmt.Printf("Joining game with ID: %s as Red team\n", gameID)

	// Insert random number for red team
	if err := c.db.InsertCoin("red"); err != nil {
		return fmt.Errorf("failed to insert coin: %v", err)
	}

	// Use watch command to show the game state for red team
	watchCmd := NewWatchCommand(c.db, "red")
	return watchCmd.Execute(gameID)
}

// Execute implements the Command interface for JoinBlueCommand
func (c *JoinBlueCommand) Execute(gameID string) error {
	if gameID == "" {
		return fmt.Errorf("join-blue command requires a game ID")
	}
	// TODO: Implement game join logic for blue team
	fmt.Printf("Joining game with ID: %s as Blue team\n", gameID)

	// Insert random number for blue team
	if err := c.db.InsertCoin("blue"); err != nil {
		return fmt.Errorf("failed to insert coin: %v", err)
	}

	// Use watch command to show the game state for blue team
	watchCmd := NewWatchCommand(c.db, "blue")
	return watchCmd.Execute(gameID)
}

// Execute implements the Command interface for WatchCommand
func (c *WatchCommand) Execute(gameID string) error {
	if gameID == "" {
		return fmt.Errorf("watch command requires a game ID")
	}

	// Track the ID of the DB
	var previousRootID string

	for {
		// Call the `SELECT dolt_hashof_db();` function to get the root ID of the DB
		var rootID string
		err := c.db.QueryRow("SELECT dolt_hashof_db()").Scan(&rootID)
		if err != nil {
			return fmt.Errorf("failed to get root ID: %v", err)
		}

		if previousRootID != rootID {
			fmt.Printf("Root ID of the database has changed: %s\n", rootID)
			previousRootID = rootID
		} else {
			// Sleep for a short duration before querying the database again
			time.Sleep(500 * time.Millisecond)
			continue
		}
		// Clear the terminal using a function from the terminal package
		terminal.ClearScreen()

		// Query the database for the current state of the coin table
		coinRows, err := c.db.Query("SELECT team, flip FROM coin ORDER BY team")
		if err != nil {
			return fmt.Errorf("failed to query coin table: %v", err)
		}
		defer coinRows.Close()

		myTurn := false

		// Determine if it's the specified team's turn based on the coin toss
		if c.team != "" {
			var redFlip, blueFlip float64

			// Extract the flip values for both teams from the previously queried data
			rowCount := 0

			for coinRows.Next() {
				rowCount++
				var team string
				var flip float64
				if err := coinRows.Scan(&team, &flip); err != nil {
					return fmt.Errorf("failed to scan coin row: %v", err)
				}
				if team == "red" {
					redFlip = flip
				} else if team == "blue" {
					blueFlip = flip
				}
				fmt.Printf("Team: %s, Flip: %f\n", team, flip)
			}
			if rowCount != 2 {
				fmt.Println("The game hasn't started yet.")
				time.Sleep(500 * time.Millisecond)
				continue
			}

			// Determine if it's the specified team's turn
			if (c.team == "red" && redFlip >= blueFlip) || (c.team == "blue" && blueFlip > redFlip) {
				myTurn = true
			}
		}

		// Query the database for the current state of the game
		rows, err := c.db.Query("SELECT x, y, board, state FROM board_states ORDER BY board, x, y")
		if err != nil {
			return fmt.Errorf("failed to query board state: %v", err)
		}
		defer rows.Close()

		// Initialize maps to hold the positions for both players
		redShips := make(map[terminal.Coordinate]string)
		blueShips := make(map[terminal.Coordinate]string)
		redShots := make(map[terminal.Coordinate]string)
		blueShots := make(map[terminal.Coordinate]string)

		// Read the rows and populate the maps
		for rows.Next() {
			var x, y int
			var board, state string
			if err := rows.Scan(&x, &y, &board, &state); err != nil {
				return fmt.Errorf("failed to scan row: %v", err)
			}

			coord := terminal.Coordinate{X: x, Y: y}
			switch board {
			case "red_ships":
				redShips[coord] = state
			case "blue_ships":
				blueShips[coord] = state
			case "red_shots":
				redShots[coord] = state
			case "blue_shots":
				blueShots[coord] = state
			}
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating rows: %v", err)
		}

		// Print the current state of the game for the current team
		term := terminal.New()
		fmt.Printf("Current time: %s\n", time.Now().Format(time.RFC1123))
		fmt.Printf("Database root ID: %s\n", previousRootID)

		switch c.team {
		case "red":
			term.PrintBoards(redShips, blueShots, redShots, "")
		case "blue":
			term.PrintBoards(blueShips, redShots, blueShots, "")
		default:
			// If no team specified, show both views
			term.PrintBoards(redShips, blueShots, redShots, "red")
			term.PrintBoards(blueShips, redShots, blueShots, "blue")
		}

		if myTurn {
			var input string
			var x, y int
			for {
				fmt.Print("Enter coordinates (e.g. D3): ")
				_, err := fmt.Scan(&input)
				if err != nil {
					fmt.Println("Invalid input. Please enter a letter (A-J) followed by a number (0-9).")
					continue
				}
				if len(input) != 2 {
					fmt.Println("Input must be exactly 2 characters (e.g. D3).")
					continue
				}

				// Convert letter to x coordinate (A=0, B=1, etc.)
				x = int(input[0] - 'A')
				if x < 0 || x > 9 {
					fmt.Println("First character must be a letter A-J.")
					continue
				}

				// Convert number to y coordinate
				y = int(input[1] - '0')
				if y < 0 || y > 9 {
					fmt.Println("Second character must be a number 0-9.")
					continue
				}
				break
			}

			// Process the shot
			opponent := "blue"
			if c.team == "blue" {
				opponent = "red"
			}
			err = c.db.ProcessShot(fmt.Sprintf("%s_shots", c.team), fmt.Sprintf("%s_ships", opponent), x, y)
			if err != nil {
				return fmt.Errorf("failed to process shot: %v", err)
			}

			// Update the coin flip values for both teams in a single query
			query := `
				UPDATE coin 
				SET flip = CASE 
					WHEN team = ? THEN 0.1 
					WHEN team = ? THEN 0.9 
				END
				WHERE team IN (?, ?)
			`
			_, err = c.db.Exec(query, c.team, opponent, c.team, opponent)
			if err != nil {
				return fmt.Errorf("failed to update coin values: %v", err)
			}

			// Determine if the shot was a hit or a miss
			var state string
			query = `
				SELECT state 
				FROM board_states 
				WHERE board = ? AND x = ? AND y = ?
			`
			err = c.db.QueryRow(query, fmt.Sprintf("%s_shots", c.team), x, y).Scan(&state)
			if err != nil {
				return fmt.Errorf("failed to determine shot result: %v", err)
			}

			// Create a detailed commit message
			commitMessage := fmt.Sprintf("Team %s shot at (%c%d) and it was a %s", c.team, 'A'+x, y, map[string]string{"H": "hit", "M": "miss"}[state])

			// Commit the changes to the database with the detailed message
			_, err = c.db.Exec("CALL DOLT_COMMIT('-a', '-m', ?)", commitMessage)
			if err != nil {
				return fmt.Errorf("failed to commit changes: %v", err)
			}

			fmt.Println("Shot processed successfully!")
		} else {
			fmt.Println("Waiting for the other team to make a move...")
		}

	}

	return nil
}

// RunCommand executes the appropriate command based on arguments
func RunCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: battleship <command> <gameID>\ncommands: start, join-red, join-blue, watch")
	}

	command := args[1]
	gameID := args[2]

	// Initialize database connection
	db, err := database.New(gameID)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	defer db.Close()

	var cmd Command
	switch command {
	case "start":
		cmd = NewStartCommand(db)
	case "join-red":
		cmd = NewJoinRedCommand(db)
	case "join-blue":
		cmd = NewJoinBlueCommand(db)
	case "watch":
		// For watch command, we'll show both views
		cmd = NewWatchCommand(db, "")
	default:
		return fmt.Errorf("unknown command: %s\navailable commands: start, join-red, join-blue, watch", command)
	}

	return cmd.Execute(gameID)
}
