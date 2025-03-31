package terminal

import (
	"fmt"
	"os"
	"strings"
)

// Colors for terminal output
const (
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Reset  = "\033[0m"
)

// Coordinate represents a position on the battleship board
type Coordinate struct {
	X int
	Y int
}

// Terminal handles colored output to the terminal
type Terminal struct {
	output *os.File
}

// New creates a new Terminal instance
func New() *Terminal {
	return &Terminal{
		output: os.Stdout,
	}
}

// PrintWelcome displays the welcome message
func (t *Terminal) PrintWelcome() {
	fmt.Fprintf(t.output, "%sWelcome to Battleship!%s\n", Blue, Reset)
	fmt.Fprintf(t.output, "%sPrepare for battle!%s\n", Yellow, Reset)
}

// PrintError displays an error message in red
func (t *Terminal) PrintError(msg string) {
	fmt.Fprintf(t.output, "%sError: %s%s\n", Red, msg, Reset)
}

// PrintSuccess displays a success message in green
func (t *Terminal) PrintSuccess(msg string) {
	fmt.Fprintf(t.output, "%s%s%s\n", Green, msg, Reset)
}

// PrintBoards displays both the player's board and the opponent's board side by side
func (t *Terminal) PrintBoards(myShips, opponentShots, myShots map[Coordinate]string, team string) {
	spaceWidth := strings.Repeat(" ", 10)

	// Print board labels based on team
	var leftLabel, rightLabel string
	switch team {
	case "red":
		leftLabel = "Blue Shots/Red Ships"
		rightLabel = "Red Shots"
	case "blue":
		leftLabel = "Red Shots/Blue Ships"
		rightLabel = "Blue Shots"
	default:
		leftLabel = "Their Shots/Your Ships"
		rightLabel = "Your Shots"
	}
	fmt.Fprintf(t.output, "%s%s%s\n", leftLabel, spaceWidth, rightLabel)

	// Print column headers for both boards
	fmt.Fprintf(t.output, "  ")
	for col := 'A'; col <= 'J'; col++ {
		fmt.Fprintf(t.output, "%c ", col)
	}
	fmt.Fprintf(t.output, "            ")
	for col := 'A'; col <= 'J'; col++ {
		fmt.Fprintf(t.output, "%c ", col)
	}
	fmt.Fprintln(t.output)

	// Print top borders
	fmt.Fprintf(t.output, "  %s%s  %s\n", strings.Repeat("-", 20), spaceWidth, strings.Repeat("-", 20))

	// Print rows for both boards
	for row := 0; row < 10; row++ {
		// Print row number and first board
		fmt.Fprintf(t.output, "%d|", row)
		for col := 0; col < 10; col++ {
			coord := Coordinate{X: col, Y: row}
			if value, exists := myShips[coord]; exists {
				switch value {
				case "H":
					fmt.Fprintf(t.output, "%s●%s|", Red, Reset)
				case "S":
					fmt.Fprintf(t.output, "●|")
				default:
					fmt.Fprintf(t.output, "●|")
				}
			} else if value, exists := opponentShots[coord]; exists {
				switch value {
				case "H":
					fmt.Fprintf(t.output, "%s●%s|", Red, Reset)
				case "M":
					fmt.Fprintf(t.output, "%s●%s|", Blue, Reset)
				default:
					fmt.Fprintf(t.output, "●|")
				}
			} else {
				fmt.Fprintf(t.output, " |")
			}
		}

		// Print separator between boards
		fmt.Fprintf(t.output, spaceWidth)

		// Print row number and second board
		fmt.Fprintf(t.output, "%d|", row)
		for col := 0; col < 10; col++ {
			coord := Coordinate{X: col, Y: row}
			if value, exists := myShots[coord]; exists {
				switch value {
				case "H":
					fmt.Fprintf(t.output, "%s●%s|", Red, Reset)
				case "M":
					fmt.Fprintf(t.output, "%s●%s|", Blue, Reset)
				default:
					fmt.Fprintf(t.output, "●|")
				}
			} else {
				fmt.Fprintf(t.output, " |")
			}
		}
		fmt.Fprintln(t.output)

		// Print bottom borders
		fmt.Fprintf(t.output, "  %s  %s%s\n", strings.Repeat("-", 20), spaceWidth, strings.Repeat("-", 20))
	}
}
