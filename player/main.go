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
	"time"
)

type Player struct {
	Name     string
	Color    string
	GameID   string
	Strategy PlayerStrategy
}

type PlayerStrategy interface {
	GetNextMove(gameState string) string
	PlaceShips() []string
}

type RandomStrategy struct{}

func (r *RandomStrategy) GetNextMove(gameState string) string {
	// Simple random strategy - just pick coordinates A1-J10
	rows := "ABCDEFGHIJ"
	cols := "12345678910"
	
	// For now, just return a random coordinate
	// In a real implementation, we'd parse the game state and avoid already hit positions
	row := string(rows[time.Now().UnixNano()%10])
	col := string(cols[time.Now().UnixNano()%10])
	return row + col
}

func (r *RandomStrategy) PlaceShips() []string {
	// For now, return empty - the battleship game handles ship placement automatically
	return []string{}
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
	
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Printf("[%s] %s\n", player.Name, line)
		gameState += line + "\n"
		
		// Check if we need to make a move
		if strings.Contains(line, "Enter coordinate to attack") {
			move := player.Strategy.GetNextMove(gameState)
			fmt.Printf("[%s] Making move: %s\n", player.Name, move)
			
			_, err := stdin.Write([]byte(move + "\n"))
			if err != nil {
				log.Printf("Error writing move for %s: %v", player.Name, err)
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