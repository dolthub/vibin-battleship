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

// JoinCommand handles joining an existing game
type JoinCommand struct {
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

// NewJoinCommand creates a new JoinCommand
func NewJoinCommand(db *database.Database) *JoinCommand {
	return &JoinCommand{db: db}
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
	fmt.Printf("Starting new game with ID: %s\n", gameID)
	return nil
}

// Execute implements the Command interface for JoinCommand
func (c *JoinCommand) Execute(gameID string) error {
	if gameID == "" {
		return fmt.Errorf("join command requires a game ID")
	}
	// TODO: Implement game join logic
	fmt.Printf("Joining game with ID: %s\n", gameID)
	return nil
}

// Execute implements the Command interface for WatchCommand
func (c *WatchCommand) Execute(gameID string) error {
	if gameID == "" {
		return fmt.Errorf("watch command requires a game ID")
	}

	for {
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
		term.PrintBoards(redShips, blueShots, redShots)
		term.PrintBoards(blueShips, redShots, blueShots)

		// Sleep for a short duration before querying the database again
		time.Sleep(2 * time.Second)
	}

	// TODO: Implement game watch logic
	fmt.Printf("Watching game with ID: %s\n", gameID)
	return nil
}

// RunCommand executes the appropriate command based on arguments
func RunCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: battleship <command> <gameID>\ncommands: start, join, watch")
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
	case "join":
		cmd = NewJoinCommand(db)
	case "watch":
		cmd = NewWatchCommand(db)
	default:
		return fmt.Errorf("unknown command: %s\navailable commands: start, join, watch", command)
	}

	return cmd.Execute(gameID)
}
