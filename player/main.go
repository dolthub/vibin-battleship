package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

type Player struct {
	Name     string
	Color    string
	GameID   string
	Strategy PlayerStrategy
}


func main() {
	fmt.Println("Starting Battleship Game Orchestrator...")
	
	// Create a new game
	gameID, err := createNewGame()
	if err != nil {
		log.Fatalf("Failed to create new game: %v", err)
	}
	
	fmt.Printf("Created game: %s\n", gameID)
	
	// Create two players
	player1 := &Player{
		Name:     "RandomBot1",
		Color:    "red",
		GameID:   gameID,
		Strategy: &RandomStrategy{},
	}
	
	player2 := &Player{
		Name:     "RandomBot2", 
		Color:    "blue",
		GameID:   gameID,
		Strategy: &RandomStrategy{},
	}
	
	// Start both players in goroutines
	var wg sync.WaitGroup
	wg.Add(2)
	
	go func() {
		defer wg.Done()
		playGame(player1)
	}()
	
	go func() {
		defer wg.Done()
		playGame(player2)
	}()
	
	wg.Wait()
	fmt.Println("Game completed!")
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

func playGame(player *Player) {
	fmt.Printf("Starting player %s (%s) for game %s\n", player.Name, player.Color, player.GameID)
	
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
		log.Printf("Player %s finished with error: %v", player.Name, err)
	} else {
		fmt.Printf("Player %s finished successfully\n", player.Name)
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
	
	scanner := bufio.NewScanner(stdout)
	gameState := ""
	shipPlacements := player.Strategy.PlaceShips()
	shipIndex := 0
	
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Printf("[%s] %s\n", player.Name, line)
		gameState += line + "\n"
		
		// Handle ship placement
		if strings.Contains(line, "Enter starting position") {
			if shipIndex < len(shipPlacements) {
				placement := shipPlacements[shipIndex]
				fmt.Printf("[%s] Placing ship at: %s\n", player.Name, placement.Position)
				
				_, err := stdin.Write([]byte(placement.Position + "\n"))
				if err != nil {
					log.Printf("Error writing ship position for %s: %v", player.Name, err)
					return
				}
			}
		}
		
		// Handle orientation
		if strings.Contains(line, "Place horizontally?") {
			if shipIndex < len(shipPlacements) {
				placement := shipPlacements[shipIndex]
				orientation := "n"
				if placement.IsHorizontal {
					orientation = "y"
				}
				fmt.Printf("[%s] Orientation: %s\n", player.Name, orientation)
				
				_, err := stdin.Write([]byte(orientation + "\n"))
				if err != nil {
					log.Printf("Error writing orientation for %s: %v", player.Name, err)
					return
				}
				shipIndex++
			}
		}
		
		// Check if we need to make an attack move
		if strings.Contains(line, "Enter attack coordinate") || strings.Contains(line, "Your turn!") {
			move := player.Strategy.GetNextMove(gameState)
			fmt.Printf("[%s] Making attack: %s\n", player.Name, move)
			
			_, err := stdin.Write([]byte(move + "\n"))
			if err != nil {
				log.Printf("Error writing attack for %s: %v", player.Name, err)
				return
			}
		}
		
		// Check if game is over
		if strings.Contains(line, "Game Over") || strings.Contains(line, "wins!") {
			fmt.Printf("[%s] Game ended\n", player.Name)
			return
		}
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from %s: %v", player.Name, err)
	}
}