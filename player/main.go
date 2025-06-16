package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Player struct {
	Name     string
	Color    string
	GameID   string
	Strategy PlayerStrategy
}

func main() {
	// Parse command line arguments
	redPlayerType := flag.String("red-player", "random", "Type of red player: random or human")
	bluePlayerType := flag.String("blue-player", "random", "Type of blue player: random or human")
	flag.Parse()

	// Validate player types
	if !isValidPlayerType(*redPlayerType) {
		fmt.Printf("Invalid red player type: %s. Must be 'random' or 'human'\n", *redPlayerType)
		os.Exit(1)
	}
	if !isValidPlayerType(*bluePlayerType) {
		fmt.Printf("Invalid blue player type: %s. Must be 'random' or 'human'\n", *bluePlayerType)
		os.Exit(1)
	}

	fmt.Println("Starting Battleship Game Orchestrator...")
	fmt.Printf("Red player: %s, Blue player: %s\n", *redPlayerType, *bluePlayerType)

	// Create a new game
	gameID, err := createNewGame()
	if err != nil {
		log.Fatalf("Failed to create new game: %v", err)
	}

	fmt.Printf("Created game: %s\n", gameID)

	// Create players based on specified types
	player1 := &Player{
		Name:     getPlayerName("red", *redPlayerType),
		Color:    "red",
		GameID:   gameID,
		Strategy: createStrategy(*redPlayerType),
	}

	player2 := &Player{
		Name:     getPlayerName("blue", *bluePlayerType),
		Color:    "blue",
		GameID:   gameID,
		Strategy: createStrategy(*bluePlayerType),
	}

	// Start only AI players in goroutines
	var wg sync.WaitGroup
	aiPlayers := []*Player{}

	// Check which players are AI and need to be started
	if _, isHuman := player1.Strategy.(*HumanStrategy); !isHuman {
		aiPlayers = append(aiPlayers, player1)
	} else {
		fmt.Printf("\n🎮 Human player %s (%s) instructions:\n", player1.Name, player1.Color)
		fmt.Printf("   Open a new terminal and run:\n")
		fmt.Printf("   battleship play %s %s\n", gameID, player1.Color)
	}

	if _, isHuman := player2.Strategy.(*HumanStrategy); !isHuman {
		aiPlayers = append(aiPlayers, player2)
	} else {
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

	// Wait for human players to join
	for _, humanPlayer := range humanPlayers {
		fmt.Printf("⏳ Waiting for %s player to connect...\n", humanPlayer.Color)
		waitForPlayerToJoin(gameID, humanPlayer.Color)
		fmt.Printf("✅ %s player connected!\n", humanPlayer.Color)
	}

	// Start AI players
	if len(aiPlayers) > 0 {
		fmt.Printf("🤖 Starting %d AI player(s)...\n", len(aiPlayers))
		wg.Add(len(aiPlayers))

		for _, player := range aiPlayers {
			go func(p *Player) {
				defer wg.Done()
				playAIGame(p)
			}(player)
		}

		wg.Wait()
	}

	// Wait for game completion regardless of player types
	if len(aiPlayers) == 0 {
		fmt.Println("📋 Both players are human - no AI players to start.")
		fmt.Println("   Use the commands above to connect to the game in separate terminals.")
		fmt.Println("⏳ Waiting for game to complete...")
	} else {
		fmt.Printf("🤖 AI players completed!\n")
		if len(aiPlayers) < 2 {
			fmt.Println("⏳ Waiting for human player to complete the game...")
		}
	}
	
	// Monitor game completion
	waitForGameCompletion(gameID)
}

func isValidPlayerType(playerType string) bool {
	return playerType == "random" || playerType == "human"
}

func getPlayerName(color, playerType string) string {
	if playerType == "human" {
		return fmt.Sprintf("Human_%s", color)
	}
	return fmt.Sprintf("RandomBot_%s", color)
}

func createStrategy(playerType string) PlayerStrategy {
	switch playerType {
	case "random":
		return &RandomStrategy{}
	case "human":
		return &HumanStrategy{}
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
	fmt.Printf("🔍 Monitoring game %s for completion...\n", gameID)
	
	for {
		// Use the new status command to check game state
		cmd := exec.Command("battleship", "status", gameID)
		output, err := cmd.Output()
		
		if err != nil {
			fmt.Printf("🔍 Error checking game status: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		
		status := strings.TrimSpace(string(output))
		
		// Check if game is completed
		if strings.HasPrefix(status, "COMPLETED:") {
			winner := strings.TrimPrefix(status, "COMPLETED:")
			fmt.Printf("🎉 Game %s completed! Winner: %s\n", gameID, winner)
			return
		}
		
		// Game is still in progress
		if status == "IN_PROGRESS" {
			fmt.Printf("🔍 Game still in progress...\n")
		} else {
			fmt.Printf("🔍 Game status: %s\n", status)
		}
		
		// Wait 3 seconds before checking again
		time.Sleep(3 * time.Second)
	}
}

func playAIGame(player *Player) {
	fmt.Printf("Starting AI player %s (%s) for game %s\n", player.Name, player.Color, player.GameID)
	// Start the interactive battleship play command
	cmd := exec.Command("battleship", "play", player.GameID, player.Color)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("Error creating stdin pipe for %s: %v", player.Name, err)
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Error creating stdout pipe for %s: %v", player.Name, err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("Error creating stderr pipe for %s: %v", player.Name, err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Error starting command for %s: %v", player.Name, err)
		return
	}

	// Handle the interactive game
	go handlePlayerIO(player, stdin, stdout, stderr)

	if err := cmd.Wait(); err != nil {
		log.Printf("AI player %s finished with error: %v", player.Name, err)
	} else {
		fmt.Printf("AI player %s finished successfully\n", player.Name)
	}
}

func handlePlayerIO(player *Player, stdin io.WriteCloser, stdout, stderr io.ReadCloser) {
	defer stdin.Close()

	// Read from stdout and stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Printf("[%s ERROR] %s\n", player.Name, line)
		}
	}()

	reader := bufio.NewReader(stdout)
	gameState := ""
	shipPlacements := player.Strategy.PlaceShips()
	shipIndex := 0
	shipPlacementComplete := false
	buffer := ""

	for {
		// Read with a timeout to detect prompts that don't end with newlines
		b := make([]byte, 1)
		n, err := reader.Read(b)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading from %s: %v", player.Name, err)
			return
		}
		if n > 0 {
			buffer += string(b[0])
			
			// If we see a newline, process the complete line
			if b[0] == '\n' {
				line := strings.TrimSpace(buffer)
				if line != "" {
					fmt.Printf("[%s] %s\n", player.Name, line)
					gameState += line + "\n"
					
					// Check if ship placement is complete
					if strings.Contains(line, "player has completed ship placement") || strings.Contains(line, "Both players have placed ships") {
						shipPlacementComplete = true
					}
					
					// Check if we need to make an attack move
					if strings.Contains(line, "Enter attack coordinate") || strings.Contains(line, "Your turn!") {
						shipPlacementComplete = true // We're now in the game phase
						move := player.Strategy.GetNextMove(gameState)
						fmt.Printf("[%s] Making attack: %s\n", player.Name, move)
						stdin.Write([]byte(move + "\n"))
					}
					
					// Only check for game endings during actual gameplay, not ship placement
					if shipPlacementComplete && (strings.Contains(line, "Game Over") || strings.Contains(line, "wins!")) {
						fmt.Printf("[%s] Game ended - detected line: '%s'\n", player.Name, line)
						return
					}
					
					// AI should fail if any ship placement is rejected
					if strings.Contains(line, "Please try again") {
						fmt.Printf("[%s] AI placement failed - terminating\n", player.Name)
						return
					}
				}
				buffer = ""
			} else if strings.HasSuffix(buffer, ": ") {
				// This looks like a prompt - handle it
				line := strings.TrimSpace(buffer)
				fmt.Printf("[%s] %s\n", player.Name, line)
				
				if strings.Contains(line, "Enter starting position") {
					if shipIndex < len(shipPlacements) {
						placement := shipPlacements[shipIndex]
						fmt.Printf("[%s] Placing ship at: %s\n", player.Name, placement.Position)
						stdin.Write([]byte(placement.Position + "\n"))
					}
				} else if strings.Contains(line, "Place horizontally") {
					if shipIndex < len(shipPlacements) {
						placement := shipPlacements[shipIndex]
						orientation := "n"
						if placement.IsHorizontal {
							orientation = "y"
						}
						fmt.Printf("[%s] Orientation: %s\n", player.Name, orientation)
						stdin.Write([]byte(orientation + "\n"))
						shipIndex++
					}
				}
				buffer = ""
			}
		}
	}
}
