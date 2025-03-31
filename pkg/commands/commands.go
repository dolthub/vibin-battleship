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
	db *database.Database
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
func NewWatchCommand(db *database.Database) *WatchCommand {
	return &WatchCommand{db: db}
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

	term := terminal.New()

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

		// Query the database for the current state of the red player's board and shots
		rows, err := c.db.Query("SELECT x, y, board, state FROM board_states WHERE board IN ('red_ships', 'red_shots') ORDER BY board, x, y")
		if err != nil {
			return fmt.Errorf("failed to query board state: %v", err)
		}
		defer rows.Close()

		// Initialize maps to hold the positions for the red player's ships and shots
		redShips := make(map[terminal.Coordinate]string)
		redShots := make(map[terminal.Coordinate]string)

		for rows.Next() {
			var x, y int
			var board, state string
			if err := rows.Scan(&x, &y, &board, &state); err != nil {
				return fmt.Errorf("failed to scan row: %v", err)
			}
			coord := terminal.Coordinate{X: x, Y: y}
			if board == "red_ships" {
				redShips[coord] = state
			} else if board == "red_shots" {
				redShots[coord] = state
			}
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating rows: %v", err)
		}

		// Print the red player's board and shots
		term.PrintBoards(redShips, redShots, nil)
	}
}

// Execute implements the Command interface for JoinBlueCommand
func (c *JoinBlueCommand) Execute(gameID string) error {
	if gameID == "" {
		return fmt.Errorf("join-blue command requires a game ID")
	}

	// Initialize the terminal for output
	term := terminal.New()

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

		// Query the database for the current state of the blue player's board and shots
		rows, err := c.db.Query("SELECT x, y, board, state FROM board_states WHERE board IN ('blue_ships', 'blue_shots') ORDER BY board, x, y")
		if err != nil {
			return fmt.Errorf("failed to query board state: %v", err)
		}
		defer rows.Close()

		// Initialize maps to hold the positions for the blue player's ships and shots
		blueShips := make(map[terminal.Coordinate]string)
		blueShots := make(map[terminal.Coordinate]string)

		for rows.Next() {
			var x, y int
			var board, state string
			if err := rows.Scan(&x, &y, &board, &state); err != nil {
				return fmt.Errorf("failed to scan row: %v", err)
			}
			coord := terminal.Coordinate{X: x, Y: y}
			if board == "blue_ships" {
				blueShips[coord] = state
			} else if board == "blue_shots" {
				blueShots[coord] = state
			}
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating rows: %v", err)
		}

		// Print the blue player's board and shots
		term.PrintBoards(blueShips, blueShots, nil)
	}
}

// Execute implements the Command interface for WatchCommand
func (c *WatchCommand) Execute(gameID string) error {
	if gameID == "" {
		return fmt.Errorf("watch command requires a game ID")
	}

	// Track the ID of the DB
	var previousRootID string

	for {
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

		// Print the current state of the game for both players
		term := terminal.New()
		fmt.Printf("Current time: %s\n", time.Now().Format(time.RFC1123))
		term.PrintBoards(redShips, blueShots, redShots)
		term.PrintBoards(blueShips, redShots, blueShots)

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
		cmd = NewWatchCommand(db)
	default:
		return fmt.Errorf("unknown command: %s\navailable commands: start, join-red, join-blue, watch", command)
	}

	return cmd.Execute(gameID)
}
