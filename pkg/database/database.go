package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

// Database handles Dolt database operations
type Database struct {
	path string
	db   *sql.DB
}

// New creates a new Database instance
func New(path string) (*Database, error) {
	// Ensure the database directory exists
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	// Connect to the Dolt database
	db, err := sql.Open("mysql", "root:@tcp(localhost:9889)/battleship")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

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

	return nil
}
