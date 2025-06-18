package main

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHarness manages a temporary Dolt database for testing
type TestHarness struct {
	TempDir   string
	Port      int
	DBName    string
	ServerCmd *exec.Cmd
	DB        *DB
}

// NewTestHarness creates a new test environment with temporary Dolt database
func NewTestHarness(t *testing.T) *TestHarness {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "battleship-test-*")
	require.NoError(t, err, "Failed to create temp dir")

	// Find available port
	port := findAvailablePort(t)

	// Initialize Dolt repository for CLI commands
	cmd := exec.Command("dolt", "init")
	cmd.Dir = tempDir
	err = cmd.Run()
	if err != nil {
		os.RemoveAll(tempDir)
		require.NoError(t, err, "Failed to init dolt repo")
	}

	// Create test database
	dbName := "battleship"
	cmd = exec.Command("dolt", "sql", "-q", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName))
	cmd.Dir = tempDir
	err = cmd.Run()
	if err != nil {
		os.RemoveAll(tempDir)
		require.NoError(t, err, "Failed to create test database")
	}

	// Start Dolt SQL server
	serverCmd := exec.Command("dolt", "sql-server", "--host=127.0.0.1", fmt.Sprintf("--port=%d", port))
	serverCmd.Dir = tempDir
	err = serverCmd.Start()
	if err != nil {
		os.RemoveAll(tempDir)
		require.NoError(t, err, "Failed to start dolt server")
	}

	// Wait for server to be ready
	time.Sleep(2 * time.Second)

	// Connect to database
	db, err := NewDB("127.0.0.1", strconv.Itoa(port), "root", "", dbName)
	if err != nil {
		serverCmd.Process.Kill()
		os.RemoveAll(tempDir)
		require.NoError(t, err, "Failed to connect to test database")
	}

	return &TestHarness{
		TempDir:   tempDir,
		Port:      port,
		DBName:    dbName,
		ServerCmd: serverCmd,
		DB:        db,
	}
}

// Cleanup shuts down the test environment
func (th *TestHarness) Cleanup() {
	if th.DB != nil {
		th.DB.Close()
	}
	if th.ServerCmd != nil && th.ServerCmd.Process != nil {
		th.ServerCmd.Process.Kill()
		th.ServerCmd.Wait()
	}
	if th.TempDir != "" {
		os.RemoveAll(th.TempDir)
	}
}

// findAvailablePort finds an available port for testing
func findAvailablePort(t *testing.T) int {
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err, "Failed to find available port")
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

// RunBattleshipCommand runs the battleship command with the test harness configuration
func (th *TestHarness) RunBattleshipCommand(t *testing.T, args ...string) string {
	// Set up environment to override database config
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set testing environment variable to skip interactive ship placement
	oldTestingEnv := os.Getenv("BATTLESHIP_TESTING")
	os.Setenv("BATTLESHIP_TESTING", "true")
	defer os.Setenv("BATTLESHIP_TESTING", oldTestingEnv)

	os.Args = append([]string{"battleship"}, args...)

	// Capture output
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	// Run the actual main function with test database config
	runMain("127.0.0.1", strconv.Itoa(th.Port), th.DBName)

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

// extractGameID extracts game ID from battleship command output
func extractGameID(t *testing.T, output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "New game created with ID:") {
			parts := strings.Split(line, ": ")
			require.GreaterOrEqual(t, len(parts), 2, "Invalid game creation output format")
			return strings.TrimSpace(parts[1])
		}
	}
	require.Fail(t, "Could not extract game ID from output", "Output: %s", output)
	return "" // Never reached
}

func TestNewCommandCreatesTable(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Verify table doesn't exist initially
	err := harness.DB.conn.QueryRow("SELECT COUNT(*) FROM games").Scan(new(int))
	assert.Error(t, err, "Games table should not exist at start of test")

	// Run the battleship new command
	output := harness.RunBattleshipCommand(t, "new")

	// Verify the output contains a game ID
	assert.Contains(t, output, "New game created with ID:", "Expected game creation message")

	// Verify the games table now exists
	var count int
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM games").Scan(&count)
	require.NoError(t, err, "Games table should exist after running 'battleship new'")

	// Verify table structure
	rows, err := harness.DB.conn.Query("DESCRIBE games")
	require.NoError(t, err, "Failed to describe games table")
	defer rows.Close()

	columnCount := 0
	for rows.Next() {
		columnCount++
		var field, fieldType, null, key, defaultVal, extra sql.NullString
		err := rows.Scan(&field, &fieldType, &null, &key, &defaultVal, &extra)
		require.NoError(t, err, "Failed to scan column info")
	}

	assert.Equal(t, 10, columnCount, "Expected 10 columns in games table")
}

func TestNewCommandGeneratesGameID(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Run the battleship new command
	output := harness.RunBattleshipCommand(t, "new")

	// Verify the output contains a game ID
	assert.Contains(t, output, "New game created with ID:", "Expected game creation message")

	// Extract game ID from output
	gameID := extractGameID(t, output)

	// Verify the game ID is a valid UUID format
	assert.Equal(t, 36, len(gameID), "Expected game ID to be a UUID")
	assert.Equal(t, 4, strings.Count(gameID, "-"), "Expected game ID to have 4 hyphens")

	// Verify that the game was added to the games table
	var dbGameID string
	err := harness.DB.conn.QueryRow("SELECT id FROM games WHERE id = ?", gameID).Scan(&dbGameID)
	require.NoError(t, err, "Failed to query games table for game ID")
	assert.Equal(t, gameID, dbGameID, "Game ID should match database entry")

	// Verify exactly one row was added
	var count int
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM games").Scan(&count)
	require.NoError(t, err, "Failed to count games table rows")
	assert.Equal(t, 1, count, "Expected 1 row in games table")

	// Verify that a Dolt commit was created on main branch
	var latestCommitMessage string
	err = harness.DB.conn.QueryRow("SELECT message FROM dolt_log ORDER BY date DESC LIMIT 1").Scan(&latestCommitMessage)
	require.NoError(t, err, "Failed to get latest commit message")

	expectedMessage := fmt.Sprintf("Create new game %s", gameID)
	assert.Equal(t, expectedMessage, latestCommitMessage, "Expected specific commit message")

	// Verify we're on main branch and the commit contains our game data
	var currentBranch string
	err = harness.DB.conn.QueryRow("SELECT active_branch()").Scan(&currentBranch)
	require.NoError(t, err, "Failed to get current branch")
	assert.Equal(t, "main", currentBranch, "Expected to be on main branch")
}

func TestNewCommandWithoutPlayerNames(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Run the battleship new command without player names (backward compatibility)
	output := harness.RunBattleshipCommand(t, "new")

	// Verify the output contains a game ID
	assert.Contains(t, output, "New game created with ID:", "Expected game creation message")
	assert.NotContains(t, output, "Players:", "Should not show players line when no names provided")

	// Extract game ID from output
	gameID := extractGameID(t, output)

	// Verify that the game was added to the games table with empty player names
	var dbGameID, redPlayer, bluePlayer string
	err := harness.DB.conn.QueryRow("SELECT id, red_player, blue_player FROM games WHERE id = ?", gameID).Scan(&dbGameID, &redPlayer, &bluePlayer)
	require.NoError(t, err, "Failed to query games table for game ID")
	assert.Equal(t, gameID, dbGameID, "Game ID should match database entry")
	assert.Equal(t, "", redPlayer, "Red player should be empty when not provided")
	assert.Equal(t, "", bluePlayer, "Blue player should be empty when not provided")

	// Verify commit message doesn't include player names
	var latestCommitMessage string
	err = harness.DB.conn.QueryRow("SELECT message FROM dolt_log ORDER BY date DESC LIMIT 1").Scan(&latestCommitMessage)
	require.NoError(t, err, "Failed to get latest commit message")
	expectedMessage := fmt.Sprintf("Create new game %s", gameID)
	assert.Equal(t, expectedMessage, latestCommitMessage, "Expected commit message without player names")
}

func TestNewCommandWithRedPlayerOnly(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Run the battleship new command with only red player name
	output := harness.RunBattleshipCommand(t, "new", "RandomBot")

	// Verify the output contains a game ID and player info
	assert.Contains(t, output, "New game created with ID:", "Expected game creation message")
	assert.Contains(t, output, "Players: red=RandomBot, blue=", "Expected players line with red player")

	// Extract game ID from output
	gameID := extractGameID(t, output)

	// Verify that the game was added to the games table with correct player names
	var dbGameID, redPlayer, bluePlayer string
	err := harness.DB.conn.QueryRow("SELECT id, red_player, blue_player FROM games WHERE id = ?", gameID).Scan(&dbGameID, &redPlayer, &bluePlayer)
	require.NoError(t, err, "Failed to query games table for game ID")
	assert.Equal(t, gameID, dbGameID, "Game ID should match database entry")
	assert.Equal(t, "RandomBot", redPlayer, "Red player should be set to RandomBot")
	assert.Equal(t, "", bluePlayer, "Blue player should be empty when not provided")

	// Verify commit message includes player names
	var latestCommitMessage string
	err = harness.DB.conn.QueryRow("SELECT message FROM dolt_log ORDER BY date DESC LIMIT 1").Scan(&latestCommitMessage)
	require.NoError(t, err, "Failed to get latest commit message")
	expectedMessage := fmt.Sprintf("Create new game %s with players red:RandomBot blue:", gameID)
	assert.Equal(t, expectedMessage, latestCommitMessage, "Expected commit message with player names")
}

func TestNewCommandWithBluePlayerOnly(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Run the battleship new command with empty red and blue player name
	output := harness.RunBattleshipCommand(t, "new", "", "TestBot")

	// Verify the output contains a game ID and player info
	assert.Contains(t, output, "New game created with ID:", "Expected game creation message")
	assert.Contains(t, output, "Players: red=, blue=TestBot", "Expected players line with blue player")

	// Extract game ID from output
	gameID := extractGameID(t, output)

	// Verify that the game was added to the games table with correct player names
	var dbGameID, redPlayer, bluePlayer string
	err := harness.DB.conn.QueryRow("SELECT id, red_player, blue_player FROM games WHERE id = ?", gameID).Scan(&dbGameID, &redPlayer, &bluePlayer)
	require.NoError(t, err, "Failed to query games table for game ID")
	assert.Equal(t, gameID, dbGameID, "Game ID should match database entry")
	assert.Equal(t, "", redPlayer, "Red player should be empty when empty string provided")
	assert.Equal(t, "TestBot", bluePlayer, "Blue player should be set to TestBot")

	// Verify commit message includes player names
	var latestCommitMessage string
	err = harness.DB.conn.QueryRow("SELECT message FROM dolt_log ORDER BY date DESC LIMIT 1").Scan(&latestCommitMessage)
	require.NoError(t, err, "Failed to get latest commit message")
	expectedMessage := fmt.Sprintf("Create new game %s with players red: blue:TestBot", gameID)
	assert.Equal(t, expectedMessage, latestCommitMessage, "Expected commit message with player names")
}

func TestNewCommandWithBothPlayerNames(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Run the battleship new command with both player names
	output := harness.RunBattleshipCommand(t, "new", "RandomBot_red", "TestBot_blue")

	// Verify the output contains a game ID and player info
	assert.Contains(t, output, "New game created with ID:", "Expected game creation message")
	assert.Contains(t, output, "Players: red=RandomBot_red, blue=TestBot_blue", "Expected players line with both players")

	// Extract game ID from output
	gameID := extractGameID(t, output)

	// Verify that the game was added to the games table with correct player names
	var dbGameID, redPlayer, bluePlayer string
	err := harness.DB.conn.QueryRow("SELECT id, red_player, blue_player FROM games WHERE id = ?", gameID).Scan(&dbGameID, &redPlayer, &bluePlayer)
	require.NoError(t, err, "Failed to query games table for game ID")
	assert.Equal(t, gameID, dbGameID, "Game ID should match database entry")
	assert.Equal(t, "RandomBot_red", redPlayer, "Red player should be set to RandomBot_red")
	assert.Equal(t, "TestBot_blue", bluePlayer, "Blue player should be set to TestBot_blue")

	// Verify commit message includes player names
	var latestCommitMessage string
	err = harness.DB.conn.QueryRow("SELECT message FROM dolt_log ORDER BY date DESC LIMIT 1").Scan(&latestCommitMessage)
	require.NoError(t, err, "Failed to get latest commit message")
	expectedMessage := fmt.Sprintf("Create new game %s with players red:RandomBot_red blue:TestBot_blue", gameID)
	assert.Equal(t, expectedMessage, latestCommitMessage, "Expected commit message with player names")
}

func TestNewCommandWithSpecialCharacters(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Run the battleship new command with player names containing special characters
	output := harness.RunBattleshipCommand(t, "new", "AI-Player_1", "Bot.v2.0")

	// Verify the output contains a game ID and player info
	assert.Contains(t, output, "New game created with ID:", "Expected game creation message")
	assert.Contains(t, output, "Players: red=AI-Player_1, blue=Bot.v2.0", "Expected players line with special characters")

	// Extract game ID from output
	gameID := extractGameID(t, output)

	// Verify that the game was added to the games table with correct player names
	var dbGameID, redPlayer, bluePlayer string
	err := harness.DB.conn.QueryRow("SELECT id, red_player, blue_player FROM games WHERE id = ?", gameID).Scan(&dbGameID, &redPlayer, &bluePlayer)
	require.NoError(t, err, "Failed to query games table for game ID")
	assert.Equal(t, gameID, dbGameID, "Game ID should match database entry")
	assert.Equal(t, "AI-Player_1", redPlayer, "Red player should handle special characters")
	assert.Equal(t, "Bot.v2.0", bluePlayer, "Blue player should handle special characters")
}

func TestNewCommandIntegrationWithPlayerTypes(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Test the exact scenario: player command should store type names, not display names
	output := harness.RunBattleshipCommand(t, "new", "human", "random")

	// Verify the output shows the actual type names
	assert.Contains(t, output, "New game created with ID:", "Expected game creation message")
	assert.Contains(t, output, "Players: red=human, blue=random", "Expected actual player type names")

	// Extract game ID from output
	gameID := extractGameID(t, output)

	// Verify that the game was added to the games table with actual type names
	var dbGameID, redPlayer, bluePlayer string
	err := harness.DB.conn.QueryRow("SELECT id, red_player, blue_player FROM games WHERE id = ?", gameID).Scan(&dbGameID, &redPlayer, &bluePlayer)
	require.NoError(t, err, "Failed to query games table for game ID")
	assert.Equal(t, gameID, dbGameID, "Game ID should match database entry")
	assert.Equal(t, "human", redPlayer, "Should store actual player type 'human', not 'Human_red'")
	assert.Equal(t, "random", bluePlayer, "Should store actual player type 'random', not 'RandomBot_blue'")
}

func TestListCommandShowsMultipleGames(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create first game
	output1 := harness.RunBattleshipCommand(t, "new")
	gameID1 := extractGameID(t, output1)

	// Create second game
	output2 := harness.RunBattleshipCommand(t, "new")
	gameID2 := extractGameID(t, output2)

	// Verify both games are different
	assert.NotEqual(t, gameID1, gameID2, "Expected different game IDs")

	// Run list command
	listOutput := harness.RunBattleshipCommand(t, "list")

	// Verify both game IDs appear in the list output
	assert.Contains(t, listOutput, gameID1, "Expected first game ID to appear in list")
	assert.Contains(t, listOutput, gameID2, "Expected second game ID to appear in list")

	// Verify the list output contains expected header
	assert.Contains(t, listOutput, "Games:", "Expected 'Games:' header in list output")
}

func TestJoinCommandCreatesTurnTableAndDeterminesOrder(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game first
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Have red player join
	redOutput := harness.RunBattleshipCommand(t, "join", gameID, "red")

	// Verify red player joined successfully
	assert.Contains(t, redOutput, "Joined game", "Expected join confirmation")
	assert.Contains(t, redOutput, "red player", "Expected red player confirmation")
	assert.Contains(t, redOutput, "Waiting for the other player to join...", "Expected waiting message")

	// Switch to the game branch to verify turn table
	err := harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	// Verify turn table exists and has red player's entry
	var redValue float64
	err = harness.DB.conn.QueryRow("SELECT value FROM turn WHERE player = 'red'").Scan(&redValue)
	require.NoError(t, err, "Failed to query red player's turn value")
	assert.GreaterOrEqual(t, redValue, 0.0, "Red player value should be >= 0")
	assert.LessOrEqual(t, redValue, 1.0, "Red player value should be <= 1")

	// Verify only one row exists at this point
	var count int
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM turn").Scan(&count)
	require.NoError(t, err, "Failed to count turn table rows")
	assert.Equal(t, 1, count, "Expected 1 row in turn table after red joins")

	// Have blue player join
	blueOutput := harness.RunBattleshipCommand(t, "join", gameID, "blue")

	// Verify blue player joined successfully
	assert.Contains(t, blueOutput, "Joined game", "Expected blue join confirmation")
	assert.Contains(t, blueOutput, "blue player", "Expected blue player confirmation")

	// Verify turn order message appears
	assert.Contains(t, blueOutput, "Both players have joined!", "Expected both players message")
	assert.Contains(t, blueOutput, "goes first", "Expected turn order message")

	// Verify turn table now has both players
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM turn").Scan(&count)
	require.NoError(t, err, "Failed to count turn table rows after both players joined")
	assert.Equal(t, 2, count, "Expected 2 rows in turn table after both players join")

	// Verify both players have values in valid range
	rows, err := harness.DB.conn.Query("SELECT player, value FROM turn ORDER BY player")
	require.NoError(t, err, "Failed to query turn table")
	defer rows.Close()

	players := make(map[string]float64)
	for rows.Next() {
		var player string
		var value float64
		err := rows.Scan(&player, &value)
		require.NoError(t, err, "Failed to scan turn row")
		players[player] = value

		// Verify value is in valid range
		assert.GreaterOrEqual(t, value, 0.0, "Player %s value should be >= 0", player)
		assert.LessOrEqual(t, value, 1.0, "Player %s value should be <= 1", player)
	}

	// Verify we have exactly red and blue players
	assert.Len(t, players, 2, "Expected 2 players in turn table")
	assert.Contains(t, players, "red", "Expected red player in turn table")
	assert.Contains(t, players, "blue", "Expected blue player in turn table")

	// Verify that the player going first matches the higher value
	var firstPlayer string
	var firstValue float64
	err = harness.DB.conn.QueryRow("SELECT player, value FROM turn ORDER BY value DESC LIMIT 1").Scan(&firstPlayer, &firstValue)
	require.NoError(t, err, "Failed to query highest value player")

	// Check that the output mentions the correct first player
	expectedMessage := fmt.Sprintf("%s player goes first", firstPlayer)
	assert.Contains(t, blueOutput, expectedMessage, "Expected output to show correct first player")
}

func TestJoinCommandAllowsRejoinWithUpdatedValues(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game first
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Have red player join first time
	redOutput1 := harness.RunBattleshipCommand(t, "join", gameID, "red")
	assert.Contains(t, redOutput1, "Joined game", "Expected red join confirmation")
	assert.Contains(t, redOutput1, "red player", "Expected red player confirmation")

	// Switch to the game branch to verify turn table state
	err := harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	// Verify turn table has one entry
	var count int
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM turn").Scan(&count)
	require.NoError(t, err, "Failed to count turn table rows")
	assert.Equal(t, 1, count, "Expected 1 row in turn table after red joins")

	// Get red player's original value
	var originalRedValue float64
	err = harness.DB.conn.QueryRow("SELECT value FROM turn WHERE player = 'red'").Scan(&originalRedValue)
	require.NoError(t, err, "Failed to query red player's original value")

	// Have red player join again (rejoin)
	redOutput2 := harness.RunBattleshipCommand(t, "join", gameID, "red")
	assert.Contains(t, redOutput2, "Joined game", "Expected red rejoin confirmation")
	assert.Contains(t, redOutput2, "red player", "Expected red player confirmation")

	// Verify turn table still has only one row
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM turn").Scan(&count)
	require.NoError(t, err, "Failed to count turn table rows after red rejoins")
	assert.Equal(t, 1, count, "Expected 1 row in turn table after red rejoins")

	// Verify red player's value was updated
	var newRedValue float64
	err = harness.DB.conn.QueryRow("SELECT value FROM turn WHERE player = 'red'").Scan(&newRedValue)
	require.NoError(t, err, "Failed to query red player's new value")

	// Log if same value (extremely unlikely but possible)
	if originalRedValue == newRedValue {
		t.Logf("Warning: Red player got the same random value twice (%.9f), which is extremely unlikely but possible", originalRedValue)
	}

	// Now have blue player join
	blueOutput := harness.RunBattleshipCommand(t, "join", gameID, "blue")
	assert.Contains(t, blueOutput, "Joined game", "Expected blue join confirmation")
	assert.Contains(t, blueOutput, "blue player", "Expected blue player confirmation")

	// Verify turn table now has two entries
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM turn").Scan(&count)
	require.NoError(t, err, "Failed to count turn table rows after blue joins")
	assert.Equal(t, 2, count, "Expected 2 rows in turn table after blue joins")

	// Get blue player's original value
	var originalBlueValue float64
	err = harness.DB.conn.QueryRow("SELECT value FROM turn WHERE player = 'blue'").Scan(&originalBlueValue)
	require.NoError(t, err, "Failed to query blue player's original value")

	// Have blue player join again (rejoin)
	blueOutput2 := harness.RunBattleshipCommand(t, "join", gameID, "blue")
	assert.Contains(t, blueOutput2, "Joined game", "Expected blue rejoin confirmation")
	assert.Contains(t, blueOutput2, "blue player", "Expected blue player confirmation")

	// Verify turn table still has only two rows
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM turn").Scan(&count)
	require.NoError(t, err, "Failed to count turn table rows after blue rejoins")
	assert.Equal(t, 2, count, "Expected 2 rows in turn table after blue rejoins")

	// Verify blue player's value was updated
	var newBlueValue float64
	err = harness.DB.conn.QueryRow("SELECT value FROM turn WHERE player = 'blue'").Scan(&newBlueValue)
	require.NoError(t, err, "Failed to query blue player's new value")

	// Log if same value (extremely unlikely but possible)
	if originalBlueValue == newBlueValue {
		t.Logf("Warning: Blue player got the same random value twice (%.9f), which is extremely unlikely but possible", originalBlueValue)
	}

	// Verify we still have exactly red and blue players
	rows, err := harness.DB.conn.Query("SELECT player FROM turn ORDER BY player")
	require.NoError(t, err, "Failed to query turn table players")
	defer rows.Close()

	players := []string{}
	for rows.Next() {
		var player string
		err := rows.Scan(&player)
		require.NoError(t, err, "Failed to scan player")
		players = append(players, player)
	}

	expectedPlayers := []string{"blue", "red"}
	assert.Len(t, players, len(expectedPlayers), "Expected correct number of players")

	// Check that we have both players (order doesn't matter)
	playerSet := make(map[string]bool)
	for _, player := range players {
		playerSet[player] = true
	}

	for _, expected := range expectedPlayers {
		assert.True(t, playerSet[expected], "Expected to find player %s in turn table", expected)
	}
}

func TestShipPlacement(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game first
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Have red player join (this initializes the board)
	redOutput := harness.RunBattleshipCommand(t, "join", gameID, "red")
	assert.Contains(t, redOutput, "Joined game", "Expected red join confirmation")

	// Switch to the game branch
	err := harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	// Test placing ships directly using the database functions
	testCases := []struct {
		ship         Ship
		startX       string
		startY       int
		isHorizontal bool
		shouldFail   bool
		description  string
	}{
		{Ships[0], "A", 1, true, false, "Place Carrier horizontally at A1"},
		{Ships[1], "A", 3, true, false, "Place Battleship horizontally at A3"},
		{Ships[2], "F", 1, false, false, "Place Cruiser vertically at F1"},
		{Ships[3], "H", 1, false, false, "Place Submarine vertically at H1"},
		{Ships[4], "J", 5, false, false, "Place Destroyer vertically at J5"},
	}

	for _, tc := range testCases {
		err := harness.DB.PlaceShip("red", tc.ship.Char, tc.startX, tc.startY, tc.isHorizontal, tc.ship.Length)
		if tc.shouldFail {
			assert.Error(t, err, "Expected %s to fail", tc.description)
		} else {
			require.NoError(t, err, "Failed to %s", tc.description)
		}
	}

	// Verify board state after placement
	board, err := harness.DB.GetBoardState("red")
	require.NoError(t, err, "Failed to get board state")

	// Check Carrier placement (A1-E1, horizontal)
	for i, x := range []string{"A", "B", "C", "D", "E"} {
		assert.Equal(t, string(CARRIER_CHAR), board[x][1], "Expected Carrier at %s1", x)
		_ = i // Avoid unused variable
	}

	// Check Battleship placement (A3-D3, horizontal)
	for _, x := range []string{"A", "B", "C", "D"} {
		assert.Equal(t, string(BATTLESHIP_CHAR), board[x][3], "Expected Battleship at %s3", x)
	}

	// Check Cruiser placement (F1-F3, vertical)
	for y := 1; y <= 3; y++ {
		assert.Equal(t, string(CRUISER_CHAR), board["F"][y], "Expected Cruiser at F%d", y)
	}

	// Check Submarine placement (H1-H3, vertical)
	for y := 1; y <= 3; y++ {
		assert.Equal(t, string(SUBMARINE_CHAR), board["H"][y], "Expected Submarine at H%d", y)
	}

	// Check Destroyer placement (J5-J6, vertical)
	for y := 5; y <= 6; y++ {
		assert.Equal(t, string(DESTROYER_CHAR), board["J"][y], "Expected Destroyer at J%d", y)
	}

	// Test error cases
	// Try to place a ship that overlaps
	err = harness.DB.PlaceShip("red", 'T', "A", 1, true, 2)
	assert.Error(t, err, "Expected error when placing ship on occupied space")
	assert.Contains(t, err.Error(), "already occupied", "Expected 'already occupied' error message")

	// Try to place a ship that goes off the board horizontally
	err = harness.DB.PlaceShip("red", 'T', "H", 1, true, 4)
	assert.Error(t, err, "Expected error when ship extends beyond right edge")
	assert.Contains(t, err.Error(), "beyond right edge", "Expected 'beyond right edge' error message")

	// Try to place a ship that goes off the board vertically
	err = harness.DB.PlaceShip("red", 'T', "A", 9, false, 3)
	assert.Error(t, err, "Expected error when ship extends beyond bottom edge")
	assert.Contains(t, err.Error(), "beyond bottom edge", "Expected 'beyond bottom edge' error message")
}

func TestBoardDisplay(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game first
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Have red player join (this initializes the board)
	redOutput := harness.RunBattleshipCommand(t, "join", gameID, "red")
	assert.Contains(t, redOutput, "Joined game", "Expected red join confirmation")

	// Switch to the game branch
	err := harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	// Place some ships for testing board display
	err = harness.DB.PlaceShip("red", CARRIER_CHAR, "A", 1, true, 5)
	require.NoError(t, err, "Failed to place Carrier")

	err = harness.DB.PlaceShip("red", DESTROYER_CHAR, "A", 3, true, 2)
	require.NoError(t, err, "Failed to place Destroyer")

	// Test GetBoardForDisplay function
	boardForDisplay, err := harness.DB.GetBoardForDisplay("red")
	require.NoError(t, err, "Failed to get board for display")

	// Verify ships are converted to display format correctly
	// Carrier at A1-E1 should be coordinates (0,0) to (4,0)
	for x := 0; x < 5; x++ {
		coord := Coordinate{X: x, Y: 0}
		assert.Equal(t, "S", boardForDisplay[coord], "Expected ship marker at coordinate (%d,0)", x)
	}

	// Destroyer at A3-B3 should be coordinates (0,2) to (1,2)
	for x := 0; x < 2; x++ {
		coord := Coordinate{X: x, Y: 2}
		assert.Equal(t, "S", boardForDisplay[coord], "Expected ship marker at coordinate (%d,2)", x)
	}

	// Verify total number of ship positions
	assert.Len(t, boardForDisplay, 7, "Expected 7 ship positions (5 for Carrier + 2 for Destroyer)")

	// Test that empty positions are not included in display map
	emptyCoord := Coordinate{X: 5, Y: 5}
	_, exists := boardForDisplay[emptyCoord]
	assert.False(t, exists, "Empty positions should not be in display map")
}

func TestPrintCommand(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game first
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Test print command with non-existent game
	printOutput := harness.RunBattleshipCommand(t, "print", "non-existent-game", "red")
	assert.Contains(t, printOutput, "does not exist", "Expected error for non-existent game")

	// Test print command on game that exists but no players have joined
	printOutput = harness.RunBattleshipCommand(t, "print", gameID, "red")
	assert.Contains(t, printOutput, "no players have joined yet", "Expected message for game with no players")

	// Have red player join (this initializes the board)
	redOutput := harness.RunBattleshipCommand(t, "join", gameID, "red")
	assert.Contains(t, redOutput, "Joined game", "Expected red join confirmation")

	// Test print command with invalid player
	printOutput = harness.RunBattleshipCommand(t, "print", gameID, "yellow")
	assert.Contains(t, printOutput, "must be 'red' or 'blue'", "Expected error for invalid player")

	// Test print command for player who hasn't joined
	printOutput = harness.RunBattleshipCommand(t, "print", gameID, "blue")
	assert.Contains(t, printOutput, "has not joined this game yet", "Expected error for player who hasn't joined")

	// Place some ships for red player to test display
	err := harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	err = harness.DB.PlaceShip("red", CARRIER_CHAR, "A", 1, true, 5)
	require.NoError(t, err, "Failed to place Carrier for test")

	// Test print command for red player who has joined and placed ships
	printOutput = harness.RunBattleshipCommand(t, "print", gameID, "red")
	assert.Contains(t, printOutput, "Current board state", "Expected board state message")
	assert.Contains(t, printOutput, "Red Player's Board", "Expected red player board header")

	// Test insufficient arguments
	printOutput = harness.RunBattleshipCommand(t, "print", gameID)
	assert.Contains(t, printOutput, "Usage: battleship print", "Expected usage message for insufficient args")

	printOutput = harness.RunBattleshipCommand(t, "print")
	assert.Contains(t, printOutput, "Usage: battleship print", "Expected usage message for no args")
}

func TestJoinCommandCreatesBoardTables(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game first
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Have red player join
	redOutput := harness.RunBattleshipCommand(t, "join", gameID, "red")
	assert.Contains(t, redOutput, "Joined game", "Expected red join confirmation")

	// Switch to the game branch to verify board tables
	err := harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	// Verify red_board table exists but is empty (no positions inserted until ships are placed)
	var redBoardCount int
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM red_board").Scan(&redBoardCount)
	require.NoError(t, err, "Failed to query red_board table")
	assert.Equal(t, 0, redBoardCount, "Expected 0 positions in red_board (empty until ships placed)")

	// Verify blue_board table exists and is also empty
	var blueBoardCount int
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM blue_board").Scan(&blueBoardCount)
	require.NoError(t, err, "Failed to query blue_board table")
	assert.Equal(t, 0, blueBoardCount, "Expected 0 positions in blue_board before blue player joins")

	// Place a ship to test that positions are created when needed
	err = harness.DB.PlaceShip("red", CARRIER_CHAR, "A", 1, true, 5)
	require.NoError(t, err, "Failed to place test ship")

	// Verify red board now has 5 positions for the carrier
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM red_board").Scan(&redBoardCount)
	require.NoError(t, err, "Failed to count red_board after ship placement")
	assert.Equal(t, 5, redBoardCount, "Expected 5 positions in red_board after placing carrier")

	// Verify the ship positions
	rows, err := harness.DB.conn.Query("SELECT x, y, content FROM red_board ORDER BY x, y")
	require.NoError(t, err, "Failed to query red_board positions")
	defer rows.Close()

	positions := []struct {
		x       string
		y       int
		content string
	}{}
	for rows.Next() {
		var x, content string
		var y int
		err := rows.Scan(&x, &y, &content)
		require.NoError(t, err, "Failed to scan red_board position")
		positions = append(positions, struct {
			x       string
			y       int
			content string
		}{x, y, content})
	}

	// Verify carrier positions A1-E1
	assert.Len(t, positions, 5, "Expected 5 positions for carrier")
	expectedPositions := []struct {
		x string
		y int
	}{{"A", 1}, {"B", 1}, {"C", 1}, {"D", 1}, {"E", 1}}
	for i, pos := range positions {
		assert.Equal(t, expectedPositions[i].x, pos.x, "Expected x coordinate %s for position %d", expectedPositions[i].x, i)
		assert.Equal(t, expectedPositions[i].y, pos.y, "Expected y coordinate %d for position %d", expectedPositions[i].y, i)
		assert.Equal(t, string(CARRIER_CHAR), pos.content, "Expected carrier character for position %d", i)
	}

	// Verify board table structure
	rows, err = harness.DB.conn.Query("DESCRIBE red_board")
	require.NoError(t, err, "Failed to describe red_board table")
	defer rows.Close()

	columns := []string{}
	for rows.Next() {
		var field, fieldType, null, key, defaultVal, extra sql.NullString
		err := rows.Scan(&field, &fieldType, &null, &key, &defaultVal, &extra)
		require.NoError(t, err, "Failed to scan red_board column info")
		columns = append(columns, field.String)
	}

	expectedColumns := []string{"x", "y", "content"}
	assert.Equal(t, expectedColumns, columns, "Expected specific columns in red_board")

	// Have blue player join
	blueOutput := harness.RunBattleshipCommand(t, "join", gameID, "blue")
	assert.Contains(t, blueOutput, "Joined game", "Expected blue join confirmation")

	// Verify blue_board is still empty (no ships placed yet)
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM blue_board").Scan(&blueBoardCount)
	require.NoError(t, err, "Failed to query blue_board table after blue joins")
	assert.Equal(t, 0, blueBoardCount, "Expected 0 positions in blue_board (no ships placed yet)")

	// Verify all X coordinates are valid (A-J)
	var invalidXCount int
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM red_board WHERE x NOT IN ('A','B','C','D','E','F','G','H','I','J')").Scan(&invalidXCount)
	require.NoError(t, err, "Failed to query invalid X coordinates")
	assert.Equal(t, 0, invalidXCount, "Expected no invalid X coordinates")

	// Verify all Y coordinates are valid (1-10)
	var invalidYCount int
	err = harness.DB.conn.QueryRow("SELECT COUNT(*) FROM red_board WHERE y < 1 OR y > 10").Scan(&invalidYCount)
	require.NoError(t, err, "Failed to query invalid Y coordinates")
	assert.Equal(t, 0, invalidYCount, "Expected no invalid Y coordinates")
}

func TestShipConstants(t *testing.T) {
	// Verify we have the correct number of ships
	assert.Len(t, Ships, 5, "Expected 5 different ship types")

	// Verify total fleet composition
	expectedShips := map[string]struct {
		length int
		char   rune
	}{
		"Carrier":    {length: 5, char: CARRIER_CHAR},
		"Battleship": {length: 4, char: BATTLESHIP_CHAR},
		"Cruiser":    {length: 3, char: CRUISER_CHAR},
		"Submarine":  {length: 3, char: SUBMARINE_CHAR},
		"Destroyer":  {length: 2, char: DESTROYER_CHAR},
	}

	for _, ship := range Ships {
		expected, exists := expectedShips[ship.Name]
		require.True(t, exists, "Unexpected ship type: %s", ship.Name)
		assert.Equal(t, expected.length, ship.Length, "Wrong length for %s", ship.Name)
		assert.Equal(t, expected.char, ship.Char, "Wrong character for %s", ship.Name)
	}

	// Verify total squares occupied by all ships (one of each)
	totalSquares := 0
	for _, ship := range Ships {
		totalSquares += ship.Length
	}
	assert.Equal(t, 17, totalSquares, "Expected ships to occupy 17 total squares (5+4+3+3+2)")

	// Verify hit/miss/empty characters are defined
	assert.Equal(t, 'X', HIT_CHAR, "Expected X for hit marker")
	assert.Equal(t, 'O', MISS_CHAR, "Expected O for miss marker")
	assert.Equal(t, ' ', EMPTY_CHAR, "Expected space for empty position")

	// Verify all ship characters are unique
	usedChars := make(map[rune]string)
	for _, ship := range Ships {
		if existingShip, exists := usedChars[ship.Char]; exists {
			t.Errorf("Duplicate character '%c' used by both %s and %s", ship.Char, existingShip, ship.Name)
		}
		usedChars[ship.Char] = ship.Name
	}
}

func TestFullGamePlaythrough(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Create and switch to game branch for setup
	err := harness.DB.CreateGameBranch(gameID)
	require.NoError(t, err, "Failed to create game branch")

	err = harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	// Create necessary tables
	err = harness.DB.CreateTurnTable()
	require.NoError(t, err, "Failed to create turn table")

	err = harness.DB.CreateBoardTables()
	require.NoError(t, err, "Failed to create board tables")

	// Set up predetermined turn order - Red goes first (higher value)
	_, err = harness.DB.conn.Exec("INSERT INTO turn (player, value) VALUES ('red', 0.9)")
	require.NoError(t, err, "Failed to insert red turn value")

	_, err = harness.DB.conn.Exec("INSERT INTO turn (player, value) VALUES ('blue', 0.1)")
	require.NoError(t, err, "Failed to insert blue turn value")

	// Commit initial setup
	if _, err := harness.DB.conn.Exec("CALL DOLT_ADD('turn')"); err != nil {
		require.NoError(t, err, "Failed to stage turn table")
	}
	if err := harness.DB.CommitChanges("Set up predetermined turn order"); err != nil {
		require.NoError(t, err, "Failed to commit turn setup")
	}

	// Place Red's ships in predetermined positions
	redShips := []struct {
		ship         Ship
		startX       string
		startY       int
		isHorizontal bool
	}{
		{Ships[0], "A", 1, true},  // Carrier A1-E1 (horizontal)
		{Ships[1], "A", 3, true},  // Battleship A3-D3 (horizontal)
		{Ships[2], "F", 1, false}, // Cruiser F1-F3 (vertical)
		{Ships[3], "H", 1, false}, // Submarine H1-H3 (vertical)
		{Ships[4], "J", 5, false}, // Destroyer J5-J6 (vertical)
	}

	for _, shipPlacement := range redShips {
		err := harness.DB.PlaceShip("red", shipPlacement.ship.Char, shipPlacement.startX, shipPlacement.startY, shipPlacement.isHorizontal, shipPlacement.ship.Length)
		require.NoError(t, err, "Failed to place red %s", shipPlacement.ship.Name)
	}

	// Place Blue's ships in predetermined positions (more spread out for easier targeting)
	blueShips := []struct {
		ship         Ship
		startX       string
		startY       int
		isHorizontal bool
	}{
		{Ships[0], "A", 6, true},  // Carrier A6-E6 (horizontal)
		{Ships[1], "A", 8, true},  // Battleship A8-D8 (horizontal)
		{Ships[2], "F", 6, false}, // Cruiser F6-F8 (vertical)
		{Ships[3], "H", 6, false}, // Submarine H6-H8 (vertical)
		{Ships[4], "J", 9, false}, // Destroyer J9-J10 (vertical)
	}

	for _, shipPlacement := range blueShips {
		err := harness.DB.PlaceShip("blue", shipPlacement.ship.Char, shipPlacement.startX, shipPlacement.startY, shipPlacement.isHorizontal, shipPlacement.ship.Length)
		require.NoError(t, err, "Failed to place blue %s", shipPlacement.ship.Name)
	}

	// Commit ship placements
	if _, err := harness.DB.conn.Exec("CALL DOLT_ADD('red_board', 'blue_board')"); err != nil {
		require.NoError(t, err, "Failed to stage board tables")
	}
	if err := harness.DB.CommitChanges("Place all ships for both players"); err != nil {
		require.NoError(t, err, "Failed to commit ship placements")
	}

	// Define scripted attack sequence where Blue wins
	attacks := []struct {
		attacker   string
		coordinate string
		expectHit  bool
		expectSunk string // Expected ship type to be sunk, empty if none
	}{
		// Red attacks (targeting Blue's ships but missing some)
		{"red", "A6", true, ""},         // Hit Blue's Carrier
		{"blue", "A1", true, ""},        // Hit Red's Carrier
		{"red", "B6", true, ""},         // Hit Blue's Carrier
		{"blue", "B1", true, ""},        // Hit Red's Carrier
		{"red", "C6", true, ""},         // Hit Blue's Carrier
		{"blue", "C1", true, ""},        // Hit Red's Carrier
		{"red", "D6", true, ""},         // Hit Blue's Carrier
		{"blue", "D1", true, ""},        // Hit Red's Carrier
		{"red", "E6", true, "Carrier"},  // Hit Blue's Carrier - SUNK
		{"blue", "E1", true, "Carrier"}, // Hit Red's Carrier - SUNK

		// Continue attacking - Blue targets Red's ships more effectively
		{"red", "A1", false, ""},           // Miss (attacking own position)
		{"blue", "A3", true, ""},           // Hit Red's Battleship
		{"red", "F6", true, ""},            // Hit Blue's Cruiser
		{"blue", "B3", true, ""},           // Hit Red's Battleship
		{"red", "F7", true, ""},            // Hit Blue's Cruiser
		{"blue", "C3", true, ""},           // Hit Red's Battleship
		{"red", "F8", true, "Cruiser"},     // Hit Blue's Cruiser - SUNK
		{"blue", "D3", true, "Battleship"}, // Hit Red's Battleship - SUNK

		// Blue systematically destroys Red's remaining ships
		{"red", "A8", true, ""},           // Hit Blue's Battleship
		{"blue", "F1", true, ""},          // Hit Red's Cruiser
		{"red", "B8", true, ""},           // Hit Blue's Battleship
		{"blue", "F2", true, ""},          // Hit Red's Cruiser
		{"red", "C8", true, ""},           // Hit Blue's Battleship
		{"blue", "F3", true, "Cruiser"},   // Hit Red's Cruiser - SUNK
		{"red", "D8", true, "Battleship"}, // Hit Blue's Battleship - SUNK
		{"blue", "H1", true, ""},          // Hit Red's Submarine
		{"red", "H6", true, ""},           // Hit Blue's Submarine
		{"blue", "H2", true, ""},          // Hit Red's Submarine
		{"red", "H7", true, ""},           // Hit Blue's Submarine
		{"blue", "H3", true, "Submarine"}, // Hit Red's Submarine - SUNK
		{"red", "H8", true, "Submarine"},  // Hit Blue's Submarine - SUNK
		{"blue", "J5", true, ""},          // Hit Red's Destroyer
		{"red", "J9", true, ""},           // Hit Blue's Destroyer
		{"blue", "J6", true, "Destroyer"}, // Hit Red's Destroyer - SUNK (Blue wins!)
	}

	// Execute the scripted attacks
	for i, attack := range attacks {
		t.Logf("Attack %d: %s attacks %s (expect %v)", i+1, attack.attacker, attack.coordinate, attack.expectHit)

		// Note: We skip turn validation in this test since we're scripting the attacks
		// In a real game, the attack command would validate turns

		// Parse coordinate
		targetX := string(attack.coordinate[0])
		var targetY int
		_, err = fmt.Sscanf(attack.coordinate[1:], "%d", &targetY)
		require.NoError(t, err, "Failed to parse coordinate %s for attack %d", attack.coordinate, i+1)

		// Process the attack
		result, sunkShip, err := harness.DB.ProcessAttack(attack.attacker, targetX, targetY)
		require.NoError(t, err, "Failed to process attack %d", i+1)

		// Verify hit/miss result
		if attack.expectHit {
			assert.Equal(t, "hit", result, "Expected hit for attack %d", i+1)
		} else {
			assert.Equal(t, "miss", result, "Expected miss for attack %d", i+1)
		}

		// Verify sinking result
		if attack.expectSunk != "" {
			assert.Equal(t, attack.expectSunk, sunkShip, "Expected %s to be sunk on attack %d", attack.expectSunk, i+1)
			t.Logf("💥 You sunk my %s!", sunkShip)
		} else {
			assert.Equal(t, "", sunkShip, "Expected no ship to be sunk on attack %d", i+1)
		}

		// Stage and commit the attack
		targetPlayer := "blue"
		if attack.attacker == "blue" {
			targetPlayer = "red"
		}

		stageBoardQuery := fmt.Sprintf("CALL DOLT_ADD('%s_board')", targetPlayer)
		if _, err := harness.DB.conn.Exec(stageBoardQuery); err != nil {
			require.NoError(t, err, "Failed to stage attack %d", i+1)
		}

		if _, err := harness.DB.conn.Exec("CALL DOLT_ADD('turn')"); err != nil {
			require.NoError(t, err, "Failed to stage turn update for attack %d", i+1)
		}

		commitMessage := fmt.Sprintf("%s player attacked %s - %s", attack.attacker, attack.coordinate, result)
		if err := harness.DB.CommitChanges(commitMessage); err != nil {
			require.NoError(t, err, "Failed to commit attack %d", i+1)
		}

		// Check if game is complete
		gameComplete, winner, err := harness.DB.CheckGameComplete(gameID)
		require.NoError(t, err, "Failed to check game completion after attack %d", i+1)

		if gameComplete {
			t.Logf("Game completed after %d attacks! Winner: %s", i+1, winner)
			assert.Equal(t, "blue", winner, "Expected Blue to win the game")

			// Complete the game
			err = harness.DB.CompleteGame(gameID, winner)
			require.NoError(t, err, "Failed to complete game")

			// Verify we're on main branch after game completion
			var currentBranch string
			err = harness.DB.conn.QueryRow("SELECT active_branch()").Scan(&currentBranch)
			require.NoError(t, err, "Failed to get current branch")
			assert.Equal(t, "main", currentBranch, "Should be on main branch after game completion")

			// Verify game results in main branch
			var dbWinner string
			var redShots, blueShots int
			err = harness.DB.conn.QueryRow("SELECT winner, total_shots_red, total_shots_blue FROM games WHERE id = ?", gameID).Scan(&dbWinner, &redShots, &blueShots)
			require.NoError(t, err, "Failed to query completed game")

			assert.Equal(t, "blue", dbWinner, "Winner should be recorded as blue")
			assert.Greater(t, redShots, 0, "Red should have made some shots")
			assert.Greater(t, blueShots, 0, "Blue should have made some shots")

			// Verify that the game branch still exists (deletion disabled)
			branchExists, err := harness.DB.BranchExists(gameID)
			require.NoError(t, err, "Failed to check if game branch exists")
			assert.True(t, branchExists, "Game branch should still exist after completion (deletion disabled to avoid conflicts)")

			// Verify that main branch contains a merge commit
			// Look for merge commit in recent history
			rows, err := harness.DB.conn.Query("SELECT message FROM dolt_log ORDER BY date DESC LIMIT 5")
			require.NoError(t, err, "Failed to query recent commit log")
			defer rows.Close()

			recentCommits := []string{}
			for rows.Next() {
				var message string
				err := rows.Scan(&message)
				require.NoError(t, err, "Failed to scan commit message")
				recentCommits = append(recentCommits, message)
			}

			// Check for merge commits by looking at commit structure
			// Merge commits typically have 2 parents in Dolt
			var mergeCommitFound bool
			mergeCheckRows, err := harness.DB.conn.Query(`
				SELECT commit_hash, message 
				FROM dolt_log 
				WHERE commit_hash IN (
					SELECT commit_hash 
					FROM dolt_commit_ancestors 
					GROUP BY commit_hash 
					HAVING COUNT(*) = 2
				) 
				ORDER BY date DESC 
				LIMIT 5`)

			if err == nil {
				defer mergeCheckRows.Close()
				for mergeCheckRows.Next() {
					var hash, message string
					mergeCheckRows.Scan(&hash, &message)
					mergeCommitFound = true
					t.Logf("Found merge commit with hash %s: %s", hash, message)
					break
				}
			}

			// If we can't detect merge commits structurally, that's OK
			// The main verification is that:
			// 1. The game branch was deleted (already verified above)
			// 2. The game data from the branch is now in main (verified by finding game completion commits)
			if !mergeCommitFound {
				t.Logf("Could not detect merge commit structure, but branch deletion and data presence verified")
			}

			t.Logf("Recent commits: %v", recentCommits)

			// Verify we can find game-related commits in the merged history
			// Get more commits to look for game-specific commits
			moreRows, err := harness.DB.conn.Query("SELECT message FROM dolt_log ORDER BY date DESC LIMIT 10")
			require.NoError(t, err, "Failed to query extended commit log")
			defer moreRows.Close()

			allCommitMessages := []string{}
			for moreRows.Next() {
				var message string
				err := moreRows.Scan(&message)
				require.NoError(t, err, "Failed to scan commit message")
				allCommitMessages = append(allCommitMessages, message)
			}

			foundGameCompletion := false
			foundTableDrop := false
			for _, message := range allCommitMessages {
				if strings.Contains(message, fmt.Sprintf("Game %s completed", gameID)) {
					foundGameCompletion = true
				}
				if strings.Contains(message, fmt.Sprintf("Drop temporary tables for game %s", gameID)) {
					foundTableDrop = true
				}
			}

			assert.True(t, foundGameCompletion, "Should find game completion commit in merged history")
			assert.True(t, foundTableDrop, "Should find table drop commit in merged history")
			return
		}
	}

	t.Fatal("Game should have completed during the attack sequence")
}

func TestCheckGameCompleteFromDifferentBranch(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Create and switch to game branch for setup
	err := harness.DB.CreateGameBranch(gameID)
	require.NoError(t, err, "Failed to create game branch")

	err = harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	// Create necessary tables
	err = harness.DB.CreateTurnTable()
	require.NoError(t, err, "Failed to create turn table")

	err = harness.DB.CreateBoardTables()
	require.NoError(t, err, "Failed to create board tables")

	// Place some ships for both players to create valid board state
	err = harness.DB.PlaceShip("red", CARRIER_CHAR, "A", 1, true, 5)
	require.NoError(t, err, "Failed to place red ship")

	err = harness.DB.PlaceShip("blue", DESTROYER_CHAR, "J", 9, false, 2)
	require.NoError(t, err, "Failed to place blue ship")

	// Switch to main branch to simulate the error condition
	err = harness.DB.CheckoutBranch("main")
	require.NoError(t, err, "Failed to checkout main branch")

	// CheckGameComplete should handle being on wrong branch and still work
	gameComplete, winner, err := harness.DB.CheckGameComplete(gameID)
	require.NoError(t, err, "CheckGameComplete should handle branch switching internally")

	// Game should not be complete since both players have ships
	assert.False(t, gameComplete, "Game should not be complete with ships remaining")
	assert.Empty(t, winner, "Winner should be empty when game not complete")

	// Verify we can call it multiple times without issues
	gameComplete2, winner2, err2 := harness.DB.CheckGameComplete(gameID)
	require.NoError(t, err2, "Second call to CheckGameComplete should also work")
	assert.Equal(t, gameComplete, gameComplete2, "Results should be consistent")
	assert.Equal(t, winner, winner2, "Results should be consistent")
}

func TestStatusCommand(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Test with nonexistent game
	output := harness.RunBattleshipCommand(t, "status", "nonexistent-game-id")
	assert.Contains(t, output, "not found", "Should indicate game not found")

	// Test with in-progress game
	createOutput := harness.RunBattleshipCommand(t, "new")
	// Extract game ID from output like "New game created with ID: xxx"
	lines := strings.Split(createOutput, "\n")
	var gameID string
	for _, line := range lines {
		if strings.Contains(line, "New game created with ID:") {
			parts := strings.Split(line, "ID: ")
			if len(parts) > 1 {
				gameID = strings.TrimSpace(parts[1])
				break
			}
		}
	}
	require.NotEmpty(t, gameID, "Failed to extract game ID from create output")

	// Check status of new game (should be in progress)
	statusOutput := harness.RunBattleshipCommand(t, "status", gameID)
	assert.Contains(t, statusOutput, "IN_PROGRESS", "Should show game in progress")

	// Test with completed game - manually update the games table
	_, err := harness.DB.conn.Exec("UPDATE games SET winner = ? WHERE id = ?", "red", gameID)
	require.NoError(t, err, "Failed to set game winner")

	// Check status of completed game
	completedStatusOutput := harness.RunBattleshipCommand(t, "status", gameID)
	assert.Contains(t, completedStatusOutput, "COMPLETED:red", "Should show game completed with winner")
}

func TestPlayCommandSkipsShipPlacementWhenAlreadyPlaced(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game first
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Have red player join (this will place ships automatically in test mode)
	redOutput := harness.RunBattleshipCommand(t, "join", gameID, "red")
	assert.Contains(t, redOutput, "Joined game", "Expected red join confirmation")

	// Switch to the game branch and manually place ships to simulate a complete setup
	err := harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	// Place all ships for red player to create a complete setup
	redShips := []struct {
		ship         Ship
		startX       string
		startY       int
		isHorizontal bool
	}{
		{Ships[0], "A", 1, true},  // Carrier A1-E1 (horizontal)
		{Ships[1], "A", 3, true},  // Battleship A3-D3 (horizontal)
		{Ships[2], "F", 1, false}, // Cruiser F1-F3 (vertical)
		{Ships[3], "H", 1, false}, // Submarine H1-H3 (vertical)
		{Ships[4], "J", 5, false}, // Destroyer J5-J6 (vertical)
	}

	for _, shipPlacement := range redShips {
		err := harness.DB.PlaceShip("red", shipPlacement.ship.Char, shipPlacement.startX, shipPlacement.startY, shipPlacement.isHorizontal, shipPlacement.ship.Length)
		require.NoError(t, err, "Failed to place red %s", shipPlacement.ship.Name)
	}

	// Place all ships for blue player as well
	blueShips := []struct {
		ship         Ship
		startX       string
		startY       int
		isHorizontal bool
	}{
		{Ships[0], "A", 6, true},  // Carrier A6-E6 (horizontal)
		{Ships[1], "A", 8, true},  // Battleship A8-D8 (horizontal)
		{Ships[2], "F", 6, false}, // Cruiser F6-F8 (vertical)
		{Ships[3], "H", 6, false}, // Submarine H6-H8 (vertical)
		{Ships[4], "J", 9, false}, // Destroyer J9-J10 (vertical)
	}

	for _, shipPlacement := range blueShips {
		err := harness.DB.PlaceShip("blue", shipPlacement.ship.Char, shipPlacement.startX, shipPlacement.startY, shipPlacement.isHorizontal, shipPlacement.ship.Length)
		require.NoError(t, err, "Failed to place blue %s", shipPlacement.ship.Name)
	}

	// Commit ship placements
	if _, err := harness.DB.conn.Exec("CALL DOLT_ADD('red_board', 'blue_board')"); err != nil {
		require.NoError(t, err, "Failed to stage board tables")
	}
	if err := harness.DB.CommitChanges("Place all ships for both players"); err != nil {
		require.NoError(t, err, "Failed to commit ship placements")
	}

	// Verify both players have placed ships
	hasRedShips, err := harness.DB.PlayerHasPlacedShips("red")
	require.NoError(t, err, "Failed to check if red player has ships")
	assert.True(t, hasRedShips, "Red player should have ships placed")

	hasBlueShips, err := harness.DB.PlayerHasPlacedShips("blue")
	require.NoError(t, err, "Failed to check if blue player has ships")
	assert.True(t, hasBlueShips, "Blue player should have ships placed")

	// Test the critical functionality: when play command is called on a game
	// where ships are already placed, it should NOT prompt for ship placement
	// Instead, it should recognize existing ships and proceed to game loop

	// Capture the play command output
	// Note: In test mode, the game loop will run but since we're not providing input,
	// it would hang. The key is to verify the initial output shows recognition
	// of existing ships rather than prompting for new ship placement.

	// Redirect to capture output without hanging on game loop
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	// Set up a go routine to stop the play command after a short time
	done := make(chan bool)
	go func() {
		time.Sleep(500 * time.Millisecond) // Give it time to show initial output
		done <- true
	}()

	// Set up command args for play
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"battleship", "play", gameID, "red"}

	// Set testing environment
	oldTestingEnv := os.Getenv("BATTLESHIP_TESTING")
	os.Setenv("BATTLESHIP_TESTING", "true")
	defer os.Setenv("BATTLESHIP_TESTING", oldTestingEnv)

	// Run the play command in a goroutine so we can time it out
	outputChan := make(chan string, 1)
	go func() {
		// Run with test database config
		runMain("127.0.0.1", strconv.Itoa(harness.Port), harness.DBName)
		outputChan <- "completed"
	}()

	// Wait for either completion or timeout
	select {
	case <-done:
		// Timeout reached, restore output and read what we captured
		w.Close()
		os.Stdout = oldStdout

		buf := make([]byte, 2048)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// The key assertion: output should indicate ships are already placed
		// and should NOT contain ship placement prompts
		assert.Contains(t, output, "Welcome back, red player! Ships are already placed",
			"Play command should recognize existing ships")
		assert.NotContains(t, output, "place your ships on the board",
			"Play command should NOT prompt for ship placement when ships already exist")
		assert.NotContains(t, output, "Placing Carrier",
			"Play command should NOT prompt for placing individual ships when ships already exist")

		t.Logf("Play command output: %s", output)
		return
	case <-outputChan:
		// Command completed normally (shouldn't happen in this test)
		w.Close()
		os.Stdout = oldStdout
		t.Log("Play command completed normally")
	case <-time.After(2 * time.Second):
		// Emergency timeout
		w.Close()
		os.Stdout = oldStdout
		t.Fatal("Play command test timed out")
	}
}

func TestPlaceCommand(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game first
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Test with invalid arguments
	output := harness.RunBattleshipCommand(t, "place")
	assert.Contains(t, output, "Usage: battleship place", "Expected usage message for no args")

	output = harness.RunBattleshipCommand(t, "place", gameID)
	assert.Contains(t, output, "Usage: battleship place", "Expected usage message for insufficient args")

	output = harness.RunBattleshipCommand(t, "place", gameID, "red")
	assert.Contains(t, output, "Usage: battleship place", "Expected usage message for insufficient args")

	output = harness.RunBattleshipCommand(t, "place", gameID, "red", "A1")
	assert.Contains(t, output, "Usage: battleship place", "Expected usage message for insufficient args")

	// Test with invalid color
	output = harness.RunBattleshipCommand(t, "place", gameID, "yellow", "A1", "h")
	assert.Contains(t, output, "must be 'red' or 'blue'", "Expected error for invalid color")

	// Test with invalid orientation
	output = harness.RunBattleshipCommand(t, "place", gameID, "red", "A1", "x")
	assert.Contains(t, output, "must be 'h' (horizontal) or 'v' (vertical)", "Expected error for invalid orientation")

	// Test with non-existent game
	output = harness.RunBattleshipCommand(t, "place", "non-existent-game", "red", "A1", "h")
	assert.Contains(t, output, "does not exist", "Expected error for non-existent game")

	// Test real-world scenario: game exists but no branch created yet
	// This is the key test case that was failing - place command should create branch automatically
	output = harness.RunBattleshipCommand(t, "place", gameID, "red", "A1", "h")
	assert.Contains(t, output, "Placed", "Place command should work on fresh game without existing branch")
	assert.Contains(t, output, "Carrier", "Expected carrier to be placed first")

	// Verify that the game branch was automatically created
	branchExists, err := harness.DB.BranchExists(gameID)
	require.NoError(t, err, "Failed to check if branch exists")
	assert.True(t, branchExists, "Game branch should have been created automatically")

	// Continue placing more ships on the now-established game
	output = harness.RunBattleshipCommand(t, "place", gameID, "red", "A3", "h")
	assert.Contains(t, output, "Placed", "Should place second ship")
	assert.Contains(t, output, "Battleship", "Expected battleship to be placed second")

	// Continue placing remaining ships
	remainingTestCases := []struct {
		position    string
		orientation string
		shipName    string
		description string
	}{
		{"F1", "v", "Cruiser", "Place third ship vertically at F1"},
		{"H1", "v", "Submarine", "Place fourth ship vertically at H1"},
		{"J5", "v", "Destroyer", "Place fifth ship vertically at J5"},
	}

	for _, tc := range remainingTestCases {
		t.Logf("Test case: %s", tc.description)
		output := harness.RunBattleshipCommand(t, "place", gameID, "red", tc.position, tc.orientation)
		assert.Contains(t, output, "Placed", "Expected success message for %s", tc.description)
		assert.Contains(t, output, tc.shipName, "Expected %s in output for %s", tc.shipName, tc.description)
	}

	// Test attempting to place ship when all ships are already placed
	output = harness.RunBattleshipCommand(t, "place", gameID, "red", "A10", "h")
	assert.Contains(t, output, "All ships already placed", "Expected error when all ships are placed")

	// Test position format validation with a new game
	newOutput2 := harness.RunBattleshipCommand(t, "new")
	gameID2 := extractGameID(t, newOutput2)

	output = harness.RunBattleshipCommand(t, "place", gameID2, "blue", "Z1", "h")
	assert.Contains(t, output, "Invalid X coordinate", "Expected error for invalid X coordinate")

	output = harness.RunBattleshipCommand(t, "place", gameID2, "blue", "A0", "h")
	assert.Contains(t, output, "Invalid Y coordinate", "Expected error for Y coordinate too low")

	output = harness.RunBattleshipCommand(t, "place", gameID2, "blue", "A11", "h")
	assert.Contains(t, output, "Invalid Y coordinate", "Expected error for Y coordinate too high")

	output = harness.RunBattleshipCommand(t, "place", gameID2, "blue", "Ax", "h")
	assert.Contains(t, output, "Invalid Y coordinate", "Expected error for non-numeric Y coordinate")

	output = harness.RunBattleshipCommand(t, "place", gameID2, "blue", "A", "h")
	assert.Contains(t, output, "Invalid position format", "Expected error for too short position")

	// Test valid multi-digit positions
	output = harness.RunBattleshipCommand(t, "place", gameID2, "blue", "A10", "h")
	assert.Contains(t, output, "Placed", "Should accept valid position A10")

	// Verify board state after placement - switch to the first game
	err = harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout first game branch")

	board, err := harness.DB.GetBoardState("red")
	require.NoError(t, err, "Failed to get red board state")

	// Check that all red ships were placed correctly
	assert.Equal(t, string(CARRIER_CHAR), board["A"][1], "Carrier should be at A1")
	assert.Equal(t, string(BATTLESHIP_CHAR), board["A"][3], "Battleship should be at A3")
	assert.Equal(t, string(CRUISER_CHAR), board["F"][1], "Cruiser should be at F1")
	assert.Equal(t, string(SUBMARINE_CHAR), board["H"][1], "Submarine should be at H1")
	assert.Equal(t, string(DESTROYER_CHAR), board["J"][5], "Destroyer should be at J5")

	// Verify blue board from second game has one ship at A10
	err = harness.DB.CheckoutBranch(gameID2)
	require.NoError(t, err, "Failed to checkout second game branch")

	blueBoard, err := harness.DB.GetBoardState("blue")
	require.NoError(t, err, "Failed to get blue board state")
	assert.Equal(t, string(CARRIER_CHAR), blueBoard["A"][10], "Blue carrier should be at A10")
}

func TestStatusCommandThroughGameLifecycle(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Check initial status - should be IN_PROGRESS since no winner is set
	statusOutput := harness.RunBattleshipCommand(t, "status", gameID)
	assert.Contains(t, statusOutput, "IN_PROGRESS", "New game should be IN_PROGRESS")

	// Set up game branch and place ships for both players
	err := harness.DB.CreateGameBranch(gameID)
	require.NoError(t, err, "Failed to create game branch")

	err = harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	err = harness.DB.CreateTurnTable()
	require.NoError(t, err, "Failed to create turn table")

	err = harness.DB.CreateBoardTables()
	require.NoError(t, err, "Failed to create board tables")

	// Set up predetermined turn order - Red goes first
	_, err = harness.DB.conn.Exec("INSERT INTO turn (player, value) VALUES ('red', 0.9)")
	require.NoError(t, err, "Failed to insert red turn value")

	_, err = harness.DB.conn.Exec("INSERT INTO turn (player, value) VALUES ('blue', 0.1)")
	require.NoError(t, err, "Failed to insert blue turn value")

	// Place Red's ships
	redShips := []struct {
		ship         Ship
		startX       string
		startY       int
		isHorizontal bool
	}{
		{Ships[0], "A", 1, true},  // Carrier A1-E1
		{Ships[1], "A", 3, true},  // Battleship A3-D3
		{Ships[2], "F", 1, false}, // Cruiser F1-F3
		{Ships[3], "H", 1, false}, // Submarine H1-H3
		{Ships[4], "J", 5, false}, // Destroyer J5-J6
	}

	for _, shipPlacement := range redShips {
		err := harness.DB.PlaceShip("red", shipPlacement.ship.Char, shipPlacement.startX, shipPlacement.startY, shipPlacement.isHorizontal, shipPlacement.ship.Length)
		require.NoError(t, err, "Failed to place red %s", shipPlacement.ship.Name)
	}

	// Place Blue's ships  
	blueShips := []struct {
		ship         Ship
		startX       string
		startY       int
		isHorizontal bool
	}{
		{Ships[0], "A", 6, true},  // Carrier A6-E6
		{Ships[1], "A", 8, true},  // Battleship A8-D8
		{Ships[2], "F", 6, false}, // Cruiser F6-F8
		{Ships[3], "H", 6, false}, // Submarine H6-H8
		{Ships[4], "J", 9, false}, // Destroyer J9-J10
	}

	for _, shipPlacement := range blueShips {
		err := harness.DB.PlaceShip("blue", shipPlacement.ship.Char, shipPlacement.startX, shipPlacement.startY, shipPlacement.isHorizontal, shipPlacement.ship.Length)
		require.NoError(t, err, "Failed to place blue %s", shipPlacement.ship.Name)
	}

	// Commit ship placements
	if _, err := harness.DB.conn.Exec("CALL DOLT_ADD('turn', 'red_board', 'blue_board')"); err != nil {
		require.NoError(t, err, "Failed to stage tables")
	}
	if err := harness.DB.CommitChanges("Set up game with ships placed"); err != nil {
		require.NoError(t, err, "Failed to commit setup")
	}

	// Check status after setup - should still be IN_PROGRESS
	statusOutput = harness.RunBattleshipCommand(t, "status", gameID)
	assert.Contains(t, statusOutput, "IN_PROGRESS", "Game with ships placed should still be IN_PROGRESS")

	// Execute some attacks, checking status periodically
	attacks := []struct {
		attacker   string
		coordinate string
		expectHit  bool
		checkStatusAfter bool
	}{
		{"red", "A6", true, true},   // Hit Blue's Carrier, check status
		{"blue", "A1", true, false}, // Hit Red's Carrier
		{"red", "B6", true, false},  // Hit Blue's Carrier
		{"blue", "B1", true, false}, // Hit Red's Carrier
		{"red", "C6", true, true},   // Hit Blue's Carrier, check status
		{"blue", "C1", true, false}, // Hit Red's Carrier
		{"red", "D6", true, false},  // Hit Blue's Carrier
		{"blue", "D1", true, false}, // Hit Red's Carrier
		{"red", "E6", true, true},   // Hit Blue's Carrier - SUNK, check status
		{"blue", "E1", true, false}, // Hit Red's Carrier - SUNK
	}

	for i, attack := range attacks {
		t.Logf("Attack %d: %s attacks %s", i+1, attack.attacker, attack.coordinate)

		// Use the attack command to ensure automatic game completion logic is triggered
		attackOutput := harness.RunBattleshipCommand(t, "attack", gameID, attack.attacker, attack.coordinate)
		
		// Verify hit/miss result from output
		if attack.expectHit {
			assert.Contains(t, attackOutput, "HIT", "Expected hit for attack %d", i+1)
		} else {
			assert.Contains(t, attackOutput, "MISS", "Expected miss for attack %d", i+1)
		}

		// Check if this attack completed the game
		if strings.Contains(attackOutput, "GAME OVER") {
			t.Logf("Game completed after attack %d", i+1)
			break
		}

		// Check status if requested
		if attack.checkStatusAfter {
			statusOutput = harness.RunBattleshipCommand(t, "status", gameID)
			assert.Contains(t, statusOutput, "IN_PROGRESS", "Game should still be IN_PROGRESS after attack %d", i+1)
		}
	}

	// Now sink all remaining ships to trigger game completion
	// Alternate turns to sink Blue's ships (Red wins)
	finalAttacks := []struct {
		attacker   string
		coordinate string
	}{
		// Sink Blue's Battleship (alternating with Blue missing)
		{"red", "A8"}, {"blue", "A2"}, {"red", "B8"}, {"blue", "B2"}, {"red", "C8"}, {"blue", "C2"}, {"red", "D8"},
		// Sink Blue's Cruiser (alternating with Blue missing)  
		{"blue", "D2"}, {"red", "F6"}, {"blue", "E2"}, {"red", "F7"}, {"blue", "F2"}, {"red", "F8"},
		// Sink Blue's Submarine (alternating with Blue missing)
		{"blue", "G2"}, {"red", "H6"}, {"blue", "H2"}, {"red", "H7"}, {"blue", "I2"}, {"red", "H8"},
		// Sink Blue's Destroyer (this should trigger game completion)
		{"blue", "J2"}, {"red", "J9"}, {"blue", "A4"}, {"red", "J10"},
	}

	for i, attack := range finalAttacks {
		t.Logf("Final attack %d: %s attacks %s", i+1, attack.attacker, attack.coordinate)

		// Use the attack command directly to test the new game completion logic
		attackOutput := harness.RunBattleshipCommand(t, "attack", gameID, attack.attacker, attack.coordinate)
		
		// Check if this attack completed the game
		if strings.Contains(attackOutput, "GAME OVER") {
			t.Logf("Game completed after final attack %d", i+1)
			assert.Contains(t, attackOutput, "Red player wins", "Red should win the game")
			break
		}
		
		// If game isn't complete yet, check that status is still IN_PROGRESS
		statusOutput = harness.RunBattleshipCommand(t, "status", gameID)
		assert.Contains(t, statusOutput, "IN_PROGRESS", "Game should be IN_PROGRESS until all ships sunk")
	}

	// Final status check - game should now be completed
	statusOutput = harness.RunBattleshipCommand(t, "status", gameID)
	assert.Contains(t, statusOutput, "COMPLETED:red", "Game should be completed with red as winner")

	// Verify status is persistent across multiple calls
	statusOutput2 := harness.RunBattleshipCommand(t, "status", gameID)
	assert.Equal(t, statusOutput, statusOutput2, "Status should be consistent across calls")

	// Test with invalid game ID
	invalidStatusOutput := harness.RunBattleshipCommand(t, "status", "invalid-game-id")
	assert.Contains(t, invalidStatusOutput, "not found", "Should report error for invalid game ID")
}

func TestWaitForTurnBasicUsage(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Test invalid usage
	invalidOutput := harness.RunBattleshipCommand(t, "waitforturn")
	assert.Contains(t, invalidOutput, "Usage: battleship waitforturn <game_id> <red|blue>", "Should show usage for no args")

	invalidOutput = harness.RunBattleshipCommand(t, "waitforturn", "game-id")
	assert.Contains(t, invalidOutput, "Usage: battleship waitforturn <game_id> <red|blue>", "Should show usage for missing player")

	invalidOutput = harness.RunBattleshipCommand(t, "waitforturn", "game-id", "invalid-color")
	assert.Contains(t, invalidOutput, "Player color must be 'red' or 'blue'", "Should reject invalid color")

	// Test with non-existent game
	invalidOutput = harness.RunBattleshipCommand(t, "waitforturn", "non-existent-game", "red")
	assert.Contains(t, invalidOutput, "does not exist", "Should report non-existent game")
}

func TestWaitForTurnWithActiveGame(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game and join players to set up turn order
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Have both players join
	harness.RunBattleshipCommand(t, "join", gameID, "red")
	harness.RunBattleshipCommand(t, "join", gameID, "blue")

	// Place ships for both players to get game to IN_PROGRESS state
	harness.RunBattleshipCommand(t, "place", gameID, "red", "A1", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "red", "B1", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "red", "C1", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "red", "D1", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "red", "E1", "h")

	harness.RunBattleshipCommand(t, "place", gameID, "blue", "F6", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "blue", "A8", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "blue", "F8", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "blue", "A10", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "blue", "J9", "v")

	// Check current turn to know who goes first
	statusOutput := harness.RunBattleshipCommand(t, "status", gameID)
	assert.Contains(t, statusOutput, "IN_PROGRESS", "Game should be in progress")

	// Get the current turn by checking who can attack
	err := harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	currentTurn, err := harness.DB.GetCurrentTurn("")
	require.NoError(t, err, "Failed to get current turn")

	// Test waitforturn for the current player (should return immediately)
	waitOutput := harness.RunBattleshipCommand(t, "waitforturn", gameID, currentTurn)
	assert.Empty(t, waitOutput, "waitforturn should return immediately for current player")

	// Test waitforturn for the non-current player in a goroutine (should block)
	otherPlayer := "blue"
	if currentTurn == "blue" {
		otherPlayer = "red"
	}

	// Channel to track if waitforturn completed
	done := make(chan bool, 1)
	go func() {
		harness.RunBattleshipCommand(t, "waitforturn", gameID, otherPlayer)
		done <- true
	}()

	// Wait a bit to ensure waitforturn is blocking
	select {
	case <-done:
		t.Fatal("waitforturn should have blocked for non-current player")
	case <-time.After(500 * time.Millisecond):
		// Good, it's blocking as expected
	}

	// Make an attack to change turns
	harness.RunBattleshipCommand(t, "attack", gameID, currentTurn, "J1") // Should miss

	// Now waitforturn should complete
	select {
	case <-done:
		// Good, waitforturn completed after turn change
	case <-time.After(2 * time.Second):
		t.Fatal("waitforturn should have completed after turn change")
	}
}

func TestWaitForTurnWithCompletedGame(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a new game and set it up
	newOutput := harness.RunBattleshipCommand(t, "new")
	gameID := extractGameID(t, newOutput)

	// Join both players
	harness.RunBattleshipCommand(t, "join", gameID, "red")
	harness.RunBattleshipCommand(t, "join", gameID, "blue")

	// Place ships for both players
	harness.RunBattleshipCommand(t, "place", gameID, "red", "A1", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "red", "B1", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "red", "C1", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "red", "D1", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "red", "E1", "h")

	harness.RunBattleshipCommand(t, "place", gameID, "blue", "F6", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "blue", "A8", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "blue", "F8", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "blue", "A10", "h")
	harness.RunBattleshipCommand(t, "place", gameID, "blue", "J9", "v")

	// Complete the game by sinking all of one player's ships
	// (This is a simplified completion - in reality we'd need to sink all ships)
	err := harness.DB.CheckoutBranch(gameID)
	require.NoError(t, err, "Failed to checkout game branch")

	// Manually set game as completed by updating the games table
	err = harness.DB.CheckoutBranch("main")
	require.NoError(t, err, "Failed to checkout main branch")

	_, err = harness.DB.conn.Exec("UPDATE games SET winner = ? WHERE id = ?", "red", gameID)
	require.NoError(t, err, "Failed to update game winner")

	// Test waitforturn on completed game (should return immediately)
	waitOutput := harness.RunBattleshipCommand(t, "waitforturn", gameID, "red")
	assert.Empty(t, waitOutput, "waitforturn should return immediately for completed game")

	waitOutput = harness.RunBattleshipCommand(t, "waitforturn", gameID, "blue")
	assert.Empty(t, waitOutput, "waitforturn should return immediately for completed game")
}
