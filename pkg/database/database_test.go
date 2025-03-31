package database

import (
	"testing"
)

func setupTestDB(t *testing.T) (*Database, func()) {
	// Create a new database instance
	db, err := New("testId")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Initialize the database
	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestShipInsertionAndRetrieval(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert a carrier (5 units) horizontally at (0,0)
	err := db.InsertShip("red_ships", 0, 0, 5, Horizontal)
	if err != nil {
		t.Fatalf("Failed to insert ship: %v", err)
	}

	// Query the database to verify the ship was inserted correctly
	rows, err := db.Query("SELECT x, y, state FROM board_states WHERE board = 'red_ships' ORDER BY x, y")
	if err != nil {
		t.Fatalf("Failed to query board state: %v", err)
	}
	defer rows.Close()

	// Verify all 5 segments are present
	expectedPositions := []struct{ x, y int }{
		{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0},
	}

	for i, expected := range expectedPositions {
		if !rows.Next() {
			t.Fatalf("Expected more rows, got %d", i)
		}

		var x, y int
		var state string
		if err := rows.Scan(&x, &y, &state); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		if x != expected.x || y != expected.y {
			t.Errorf("Position mismatch at index %d: got (%d,%d), want (%d,%d)",
				i, x, y, expected.x, expected.y)
		}

		if state != "S" {
			t.Errorf("Invalid state at (%d,%d): got %s, want S", x, y, state)
		}
	}

	if rows.Next() {
		t.Error("Got more rows than expected")
	}
}

func TestShipBoundsValidation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name      string
		x, y      int
		length    int
		direction Direction
		wantErr   bool
	}{
		{
			name: "Ship fits horizontally",
			x:    0, y: 0,
			length:    5,
			direction: Horizontal,
			wantErr:   false,
		},
		{
			name: "Ship fits vertically",
			x:    0, y: 0,
			length:    5,
			direction: Vertical,
			wantErr:   false,
		},
		{
			name: "Ship too long horizontally",
			x:    7, y: 0,
			length:    5,
			direction: Horizontal,
			wantErr:   true,
		},
		{
			name: "Ship too long vertically",
			x:    0, y: 7,
			length:    5,
			direction: Vertical,
			wantErr:   true,
		},
		{
			name: "Ship out of bounds x",
			x:    -1, y: 0,
			length:    3,
			direction: Horizontal,
			wantErr:   true,
		},
		{
			name: "Ship out of bounds y",
			x:    0, y: -1,
			length:    3,
			direction: Vertical,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.InsertShip("red_ships", tt.x, tt.y, tt.length, tt.direction)
			if (err != nil) != tt.wantErr {
				t.Errorf("InsertShip() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInvalidShipLength(t *testing.T) {

	db, cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name      string
		length    int
		direction Direction
		wantErr   bool
	}{
		{
			name:      "Valid length (2)",
			length:    2,
			direction: Horizontal,
			wantErr:   false,
		},
		{
			name:      "Valid length (5)",
			length:    5,
			direction: Horizontal,
			wantErr:   false,
		},
		{
			name:      "Invalid length (1)",
			length:    1,
			direction: Horizontal,
			wantErr:   true,
		},
		{
			name:      "Invalid length (6)",
			length:    6,
			direction: Horizontal,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.InsertShip("red_ships", 0, 0, tt.length, tt.direction)
			if (err != nil) != tt.wantErr {
				t.Errorf("InsertShip() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Reset the Dolt database to its initial state after each test case
			_, err = db.Exec("CALL DOLT_RESET('--hard')")
			if err != nil {
				t.Fatalf("Failed to reset Dolt database: %v", err)
			}
		})
	}
}
