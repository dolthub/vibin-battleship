package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Database handles Dolt database operations
type Database struct {
	path string
	db   *sql.DB
}

// New creates a new Database instance
func New(gameId string, path string) (*Database, error) {
	// Ensure the database directory exists
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	// Configure the database connection
	cfg := mysql.NewConfig()
	cfg.User = "root"
	cfg.Passwd = ""
	cfg.Addr = "localhost:9889"
	cfg.DBName = fmt.Sprintf("battleship/game_%s", gameId)
	cfg.ParseTime = true
	cfg.Loc = time.Local

	// Create the connector
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connector: %v", err)
	}

	// Open the database connection
	db := sql.OpenDB(connector)

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &Database{
		path: path,
		db:   db,
	}, nil
}

// Close performs any necessary cleanup
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// GetTables returns a list of all tables in the database
func (d *Database) GetTables() ([]string, error) {
	rows, err := d.db.Query("SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %v", err)
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %v", err)
	}

	return tables, nil
}

// Initialize creates the necessary tables in the database
func (d *Database) Initialize() error {
	if err := d.CreateBoardStatesTable(); err != nil {
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	return nil
}

// CreateBoardStatesTable creates the board states table with the required schema
func (d *Database) CreateBoardStatesTable() error {
	query := `
		CREATE TABLE board_states (
			x INT NOT NULL,
			y INT NOT NULL,
			board ENUM('red_ships', 'blue_ships', 'red_shots', 'blue_shots') NOT NULL,
			state ENUM('H', 'M', 'S') NOT NULL,
			PRIMARY KEY (x, y, board)
		);
	`

	_, err := d.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create board_states table: %v", err)
	}

	// Commit the current state to Dolt
	commitMessage := "Create board_states table"
	_, err = d.db.Exec("CALL DOLT_COMMIT('-A', '-m', ?)", commitMessage)
	if err != nil {
		log.Fatalf("Failed to commit to Dolt: %v", err)
	}

	return nil
}

// Direction represents the orientation of a ship
type Direction bool

const (
	Vertical   Direction = true
	Horizontal Direction = false
)

// InsertShip inserts a ship into the database at the given position with the given length and direction
func (d *Database) InsertShip(board string, x, y int, length int, direction Direction) error {
	// Validate board type
	if board != "red_ships" && board != "blue_ships" {
		return fmt.Errorf("invalid board type: %s", board)
	}

	// Validate coordinates
	if x < 0 || x > 9 || y < 0 || y > 9 {
		return fmt.Errorf("coordinates out of bounds: (%d, %d)", x, y)
	}

	// Validate length
	if length < 2 || length > 5 {
		return fmt.Errorf("invalid ship length: %d", length)
	}

	// Check if ship fits on board based on direction
	switch direction {
	case Vertical:
		if y+length > 9 {
			return fmt.Errorf("ship too long to fit at position: (%d, %d) pointing north", x, y)
		}
	case Horizontal:
		if x+length > 9 {
			return fmt.Errorf("ship too long to fit at position: (%d, %d) pointing south", x, y)
		}
	}

	// Insert each segment of the ship
	for i := 0; i < length; i++ {
		var newX, newY int
		switch direction {
		case Vertical:
			newX, newY = x, y+i
		case Horizontal:
			newX, newY = x+i, y
		}

		query := `
			INSERT INTO board_states (x, y, board, state)
			VALUES (?, ?, ?, 'S')
		`
		_, err := d.db.Exec(query, newX, newY, board)
		if err != nil {
			return fmt.Errorf("failed to insert ship segment: %v", err)
		}
	}

	return nil
}

// Query executes a query that returns rows
func (d *Database) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.Query(query, args...)
}

// Exec executes a query without returning rows
func (d *Database) Exec(query string, args ...interface{}) (sql.Result, error) {
	return d.db.Exec(query, args...)
}
