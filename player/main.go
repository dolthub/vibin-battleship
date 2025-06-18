package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

var globalPrettyPlayer string // Global variable to track which player to show clean output for

type Player struct {
	Name     string
	Color    string
	GameID   string
	Strategy PlayerStrategy
}

func main() {
	// Parse command line arguments
	redPlayerType := flag.String("red-player", "random", "Type of red player: random, human, or test")
	bluePlayerType := flag.String("blue-player", "random", "Type of blue player: random, human, or test")
	prettyPrint := flag.String("pretty", "", "Print board for specified player (red or blue) and exit")
	flag.Parse()

	// Handle pretty print option - this will run the game but only show output from specified player
	var prettyPlayer string
	if *prettyPrint != "" {
		if *prettyPrint != "red" && *prettyPrint != "blue" {
			fmt.Printf("Invalid player color for --pretty: %s. Must be 'red' or 'blue'\n", *prettyPrint)
			os.Exit(1)
		}
		prettyPlayer = *prettyPrint
		globalPrettyPlayer = prettyPlayer
	}

	// Validate player types
	if !isValidPlayerType(*redPlayerType) {
		fmt.Printf("Invalid red player type: %s. Must be 'random', 'human', or 'test'\n", *redPlayerType)
		os.Exit(1)
	}
	if !isValidPlayerType(*bluePlayerType) {
		fmt.Printf("Invalid blue player type: %s. Must be 'random', 'human', or 'test'\n", *bluePlayerType)
		os.Exit(1)
	}

	if prettyPlayer == "" {
		fmt.Println("Starting Battleship Game Orchestrator...")
		fmt.Printf("Red player: %s, Blue player: %s\n", *redPlayerType, *bluePlayerType)
	}

	// Create a new game with player type names (not display names)
	gameID, err := createNewGameWithPlayers(*redPlayerType, *bluePlayerType)
	if err != nil {
		log.Fatalf("Failed to create new game: %v", err)
	}

	if prettyPlayer == "" {
		fmt.Printf("Created game: %s\n", gameID)
	}

	// Create players based on specified types
	player1 := &Player{
		Name:     getPlayerName("red", *redPlayerType),
		Color:    "red",
		GameID:   gameID,
		Strategy: createStrategy(*redPlayerType),
	}
	player1.Strategy.SetPlayerColor("red")

	player2 := &Player{
		Name:     getPlayerName("blue", *bluePlayerType),
		Color:    "blue",
		GameID:   gameID,
		Strategy: createStrategy(*bluePlayerType),
	}
	player2.Strategy.SetPlayerColor("blue")

	// Start only AI players in goroutines
	var wg sync.WaitGroup
	aiPlayers := []*Player{}

	// Check which players are AI and need to be started
	if _, isHuman := player1.Strategy.(*HumanStrategy); !isHuman {
		aiPlayers = append(aiPlayers, player1)
	} else if prettyPlayer == "" {
		fmt.Printf("\n🎮 Human player %s (%s) instructions:\n", player1.Name, player1.Color)
		fmt.Printf("   Open a new terminal and run:\n")
		fmt.Printf("   battleship play %s %s\n", gameID, player1.Color)
	}

	if _, isHuman := player2.Strategy.(*HumanStrategy); !isHuman {
		aiPlayers = append(aiPlayers, player2)
	} else if prettyPlayer == "" {
		fmt.Printf("🎮 Human player %s (%s) instructions:\n", player2.Name, player2.Color)
		fmt.Printf("   Open a new terminal and run:\n")
		fmt.Printf("   battleship play %s %s\n", gameID, player2.Color)
	}

	// Wait for human players to connect before starting AI players
	humanPlayers := []*Player{}
	if _, isHuman := player1.Strategy.(*HumanStrategy); isHuman {
		humanPlayers = append(humanPlayers, player1)
	}
	if _, isHuman := player2.Strategy.(*HumanStrategy); isHuman {
		humanPlayers = append(humanPlayers, player2)
	}

	// Handle different game types
	if len(aiPlayers) == 2 {
		// AI vs AI - simplified direct gameplay
		if prettyPlayer == "" {
			fmt.Printf("🤖 Starting AI vs AI game...\n")
		}
		playAIvsAIGame(player1, player2)
	} else if len(aiPlayers) == 1 {
		// AI vs Human - wait for human to connect first, then start AI
		if prettyPlayer == "" {
			fmt.Printf("🤖 Waiting for human player to connect before starting AI...\n")
		}

		// Wait for human players to join FIRST
		for _, humanPlayer := range humanPlayers {
			if prettyPlayer == "" {
				fmt.Printf("⏳ Waiting for %s player to connect...\n", humanPlayer.Color)
			}
			waitForPlayerToJoin(gameID, humanPlayer.Color)
			if prettyPlayer == "" {
				fmt.Printf("✅ %s player connected!\n", humanPlayer.Color)
			}
		}

		// NOW start AI player after human has connected
		if prettyPlayer == "" {
			fmt.Printf("🤖 Starting AI player %s...\n", aiPlayers[0].Name)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			playAIGame(aiPlayers[0])
		}()

		// Wait for AI player to complete
		wg.Wait()

		if prettyPlayer == "" {
			fmt.Println("⏳ Waiting for human player to complete the game...")
		}

		// Monitor game completion
		waitForGameCompletion(gameID)
	} else {
		// Human vs Human
		if prettyPlayer == "" {
			fmt.Println("📋 Both players are human - no AI players to start.")
			fmt.Println("   Use the commands above to connect to the game in separate terminals.")
			fmt.Println("⏳ Waiting for game to complete...")
		}

		// Monitor game completion
		waitForGameCompletion(gameID)
	}
}

func isValidPlayerType(playerType string) bool {
	return playerType == "random" || playerType == "human" || playerType == "test" || playerType == "claude-1" || playerType == "claude-2" || playerType == "claude-3" || playerType == "claude-4"
}

func getPlayerName(color, playerType string) string {
	if playerType == "human" {
		return fmt.Sprintf("Human_%s", color)
	} else if playerType == "test" {
		return fmt.Sprintf("TestBot_%s", color)
	} else if playerType == "claude-1" {
		return fmt.Sprintf("Claude1_%s", color)
	} else if playerType == "claude-2" {
		return fmt.Sprintf("Claude2_%s", color)
	} else if playerType == "claude-3" {
		return fmt.Sprintf("Claude3_%s", color)
	} else if playerType == "claude-4" {
		return fmt.Sprintf("Claude4_%s", color)
	}
	return fmt.Sprintf("RandomBot_%s", color)
}

func createStrategy(playerType string) PlayerStrategy {
	switch playerType {
	case "random":
		return &RandomStrategy{}
	case "human":
		return &HumanStrategy{}
	case "test":
		return &TestStrategy{attackIndex: 0}
	case "claude-1":
		return &ClaudeStrategy{}
	case "claude-2":
		return &Claude2Strategy{}
	case "claude-3":
		return &Claude3Strategy{}
	case "claude-4":
		return &Claude4Strategy{}
	default:
		log.Fatalf("Unknown player type: %s", playerType)
		return nil
	}
}

func createNewGame() (string, error) {
	cmd := exec.Command("battleship", "new")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create game: %v", err)
	}

	// Parse game ID from output
	re := regexp.MustCompile(`New game created with ID: ([a-f0-9-]+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not parse game ID from output: %s", string(output))
	}

	return matches[1], nil
}

func createNewGameWithPlayers(redPlayerName, bluePlayerName string) (string, error) {
	cmd := exec.Command("battleship", "new", redPlayerName, bluePlayerName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create game: %v", err)
	}

	// Parse game ID from output
	re := regexp.MustCompile(`New game created with ID: ([a-f0-9-]+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not parse game ID from output: %s", string(output))
	}

	return matches[1], nil
}

func waitForPlayerToJoin(gameID, playerColor string) {
	for {
		// Try to run "battleship print" for the player
		cmd := exec.Command("battleship", "print", gameID, playerColor)
		output, err := cmd.Output()

		if err == nil {
			// If print succeeded, check the output
			outputStr := string(output)
			if strings.Contains(outputStr, "has not joined this game yet") {
				// Player hasn't joined yet, keep waiting
			} else if strings.Contains(outputStr, "Player's Board:") {
				// Player has joined and has a board - they're connected
				return
			}
		}

		// Wait 2 seconds before checking again
		time.Sleep(2 * time.Second)
	}
}

func waitForGameCompletion(gameID string) {
	if globalPrettyPlayer == "" {
		fmt.Printf("🔍 Monitoring game %s for completion...\n", gameID)
	}

	for {
		// Use the new status command to check game state
		cmd := exec.Command("battleship", "status", gameID)
		output, err := cmd.Output()

		if err != nil {
			if globalPrettyPlayer == "" {
				fmt.Printf("🔍 Error checking game status: %v\n", err)
			}
			time.Sleep(5 * time.Second)
			continue
		}

		status := strings.TrimSpace(string(output))

		// Check if game is completed
		if strings.HasPrefix(status, "COMPLETED:") {
			winner := strings.TrimPrefix(status, "COMPLETED:")
			if globalPrettyPlayer == "" {
				fmt.Printf("🎉 Game %s completed! Winner: %s\n", gameID, winner)
			}
			return
		}

		// Game is still in progress
		if globalPrettyPlayer == "" {
			if status == "IN_PROGRESS" {
				fmt.Printf("🔍 Game still in progress...\n")
			} else {
				fmt.Printf("🔍 Game status: %s\n", status)
			}
		}

		// Wait 3 seconds before checking again
		time.Sleep(3 * time.Second)
	}
}

func playAIvsAIGame(player1, player2 *Player) {
	if globalPrettyPlayer == "" {
		fmt.Printf("🤖 Starting simplified AI vs AI game between %s and %s\n", player1.Name, player2.Name)
	}

	// Place ships for both players
	for _, player := range []*Player{player1, player2} {
		if globalPrettyPlayer == "" {
			fmt.Printf("[%s] Placing ships...\n", player.Name)
		}

		shipPlacements := player.Strategy.PlaceShips()
		for _, placement := range shipPlacements {
			orientation := "v"
			if placement.IsHorizontal {
				orientation = "h"
			}

			placeCmd := exec.Command("battleship", "place", player.GameID, player.Color, placement.Position, orientation)
			if err := placeCmd.Run(); err != nil {
				log.Printf("Error placing ship for %s at %s: %v", player.Name, placement.Position, err)
				return
			}

			if globalPrettyPlayer == "" {
				fmt.Printf("[%s] Placed ship at %s (%s)\n", player.Name, placement.Position, orientation)
			}
		}
	}

	if globalPrettyPlayer == "" {
		fmt.Printf("🤖 Both players finished placing ships, starting combat!\n")
	}

	// Determine starting player by checking turn order
	currentPlayer := player1
	otherPlayer := player2

	// Simple alternating attack loop
	maxConsecutiveFailures := 10 // Limit consecutive failed attacks to prevent infinite loops
	consecutiveFailures := 0

	for {
		// Make attack with current player
		attackPos := currentPlayer.Strategy.GetNextMove("", false, "")
		attackCmd := exec.Command("battleship", "attack", currentPlayer.GameID, currentPlayer.Color, attackPos)
		attackOutput, err := attackCmd.Output()

		if err != nil {
			// If attack failed, it might not be this player's turn - try the other player
			if strings.Contains(err.Error(), "not your turn") {
				// Switch players and try again
				currentPlayer, otherPlayer = otherPlayer, currentPlayer
				continue
			}
			// If the coordinate was already attacked, just try again with the same player
			if strings.Contains(err.Error(), "already been attacked") {
				consecutiveFailures++
				if consecutiveFailures >= maxConsecutiveFailures {
					if globalPrettyPlayer == "" {
						fmt.Printf("⚠️  Too many consecutive failed attacks, checking game status...\n")
					}
					break // Exit loop and check game status
				}
				if globalPrettyPlayer == "" {
					fmt.Printf("[%s] Position %s already attacked, trying again... (%d/%d)\n", currentPlayer.Name, attackPos, consecutiveFailures, maxConsecutiveFailures)
				}
				continue
			}
			log.Printf("Error attacking for %s at %s: %v", currentPlayer.Name, attackPos, err)
			return
		}

		// Reset consecutive failures on successful attack
		consecutiveFailures = 0

		if globalPrettyPlayer == "" {
			result := strings.TrimSpace(string(attackOutput))
			fmt.Printf("[%s] Attacked %s: %s\n", currentPlayer.Name, attackPos, result)
		}

		// Check game status immediately after each attack
		statusCmd := exec.Command("battleship", "status", player1.GameID)
		statusOutput, err := statusCmd.Output()
		if err != nil {
			log.Printf("Error checking game status: %v", err)
			return
		}

		status := strings.TrimSpace(string(statusOutput))

		// Check if game is completed
		if strings.HasPrefix(status, "COMPLETED:") {
			if globalPrettyPlayer == "" {
				winner := strings.TrimPrefix(status, "COMPLETED:")
				fmt.Printf("🎉 AI vs AI game completed! Winner: %s\n", winner)
			}
			return
		}

		// Switch to other player for next turn
		currentPlayer, otherPlayer = otherPlayer, currentPlayer

		// Brief pause to avoid overwhelming the system
		time.Sleep(50 * time.Millisecond)
	}

	// Final status check in case we exited due to too many failures
	statusCmd := exec.Command("battleship", "status", player1.GameID)
	statusOutput, err := statusCmd.Output()
	if err == nil {
		status := strings.TrimSpace(string(statusOutput))
		if strings.HasPrefix(status, "COMPLETED:") {
			if globalPrettyPlayer == "" {
				winner := strings.TrimPrefix(status, "COMPLETED:")
				fmt.Printf("🎉 AI vs AI game completed! Winner: %s\n", winner)
			}
		} else {
			if globalPrettyPlayer == "" {
				fmt.Printf("⚠️  Game ended due to too many failed attacks. Final status: %s\n", status)
			}
		}
	}
}

func playAIGame(player *Player) {
	if globalPrettyPlayer == "" {
		fmt.Printf("Starting AI player %s (%s) for game %s\n", player.Name, player.Color, player.GameID)
	}

	// Place all ships (this handles joining automatically)
	shipPlacements := player.Strategy.PlaceShips()
	for _, placement := range shipPlacements {
		orientation := "v"
		if placement.IsHorizontal {
			orientation = "h"
		}

		placeCmd := exec.Command("battleship", "place", player.GameID, player.Color, placement.Position, orientation)
		if err := placeCmd.Run(); err != nil {
			log.Printf("Error placing ship for %s at %s: %v", player.Name, placement.Position, err)
			return
		}

		if globalPrettyPlayer == "" {
			fmt.Printf("[%s] Placed ship at %s (%s)\n", player.Name, placement.Position, orientation)
		}
	}

	if globalPrettyPlayer == "" {
		fmt.Printf("[%s] All ships placed, entering combat phase\n", player.Name)
	}

	// Main game loop - make attacks until game ends
	maxConsecutiveFailures := 10 // Limit consecutive failed attacks to prevent infinite loops
	consecutiveFailures := 0

	for {
		// Check if it's our turn and game status
		statusCmd := exec.Command("battleship", "status", player.GameID)
		statusOutput, err := statusCmd.Output()
		if err != nil {
			log.Printf("Error checking status for %s: %v", player.Name, err)
			return
		}

		status := strings.TrimSpace(string(statusOutput))

		// Check if game is completed
		if strings.HasPrefix(status, "COMPLETED:") {
			if globalPrettyPlayer == "" {
				winner := strings.TrimPrefix(status, "COMPLETED:")
				fmt.Printf("[%s] Game completed! Winner: %s\n", player.Name, winner)
			}
			return
		}

		// Check if game is still in setup phase (not IN_PROGRESS yet)
		if status != "IN_PROGRESS" {
			// Game is not ready for combat yet, wait for both players to finish placing ships
			time.Sleep(1000 * time.Millisecond)
			continue
		}

		// Game is in progress, wait for our turn using the new waitforturn command
		waitCmd := exec.Command("battleship", "waitforturn", player.GameID, player.Color)
		err = waitCmd.Run()
		if err != nil {
			// If waitforturn failed, the game might be over
			log.Printf("Wait for turn failed for %s: %v", player.Name, err)
			return
		}

		// It's our turn, get a move and attack
		attackPos := player.Strategy.GetNextMove("", false, "")
		attackCmd := exec.Command("battleship", "attack", player.GameID, player.Color, attackPos)
		attackOutput, err := attackCmd.Output()
		if err != nil {
			// If the coordinate was already attacked, just try again with the same player
			if strings.Contains(err.Error(), "already been attacked") {
				consecutiveFailures++
				if consecutiveFailures >= maxConsecutiveFailures {
					if globalPrettyPlayer == "" {
						fmt.Printf("⚠️  [%s] Too many consecutive failed attacks, checking game status...\n", player.Name)
					}
					break // Exit loop and check game status
				}
				// Silently try again - no need to log this noise
				continue
			}
			log.Printf("Error attacking for %s at %s: %v", player.Name, attackPos, err)
			return
		}

		// Reset consecutive failures on successful attack
		consecutiveFailures = 0

		if globalPrettyPlayer == "" {
			result := strings.TrimSpace(string(attackOutput))
			fmt.Printf("[%s] Attacked %s: %s\n", player.Name, attackPos, result)
		}

		// Brief pause between attacks
		time.Sleep(100 * time.Millisecond)
	}
}
