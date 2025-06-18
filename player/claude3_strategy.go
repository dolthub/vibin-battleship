package main

import (
	"fmt"
	"math/rand"
	"time"
)

type Claude3Strategy struct {
	playerColor         string
	attackedPositions   map[string]bool // All positions we've attacked
	hitPositions        []string        // Successful hits we haven't finished hunting
	targetQueue         []string        // Adjacent positions to hit next
	huntMode            bool            // Are we hunting around a hit?
	currentHitIndex     int             // Index in hitPositions we're currently hunting
}

func (c *Claude3Strategy) GetNextMove(gameState string) string {
	// Initialize on first call
	if c.attackedPositions == nil {
		c.initializeStrategy()
	}

	var target string

	if c.huntMode && len(c.targetQueue) > 0 {
		// Hunt mode: attack positions around known hits
		target = c.getNextHuntTarget()
	} else {
		// Search mode: random attack until we hit something
		target = c.getRandomTarget()
		// If we got a random target, we're no longer in hunt mode
		c.huntMode = false
	}

	// Mark position as attacked
	c.attackedPositions[target] = true

	return target
}

func (c *Claude3Strategy) initializeStrategy() {
	c.attackedPositions = make(map[string]bool)
	c.hitPositions = []string{}
	c.targetQueue = []string{}
	c.huntMode = false
	c.currentHitIndex = 0
	
	rand.Seed(time.Now().UnixNano())
}

func (c *Claude3Strategy) getRandomTarget() string {
	rows := "ABCDEFGHIJ"
	
	// Generate random attacks until we find an unattacked position
	for attempts := 0; attempts < 1000; attempts++ {
		row := rows[rand.Intn(10)]
		col := rand.Intn(10) + 1
		pos := fmt.Sprintf("%c%d", row, col)
		
		if !c.attackedPositions[pos] {
			return pos
		}
	}
	
	// Fallback: systematic search if random fails
	for _, row := range rows {
		for col := 1; col <= 10; col++ {
			pos := fmt.Sprintf("%c%d", row, col)
			if !c.attackedPositions[pos] {
				return pos
			}
		}
	}
	
	return "A1" // Should never reach here
}

func (c *Claude3Strategy) getNextHuntTarget() string {
	// Try to get next target from queue
	for len(c.targetQueue) > 0 {
		target := c.targetQueue[0]
		c.targetQueue = c.targetQueue[1:] // Remove first element
		
		if !c.attackedPositions[target] {
			return target
		}
	}
	
	// If queue is empty, exit hunt mode
	c.huntMode = false
	return c.getRandomTarget()
}

func (c *Claude3Strategy) getAdjacentPositions(pos string) []string {
	if len(pos) < 2 {
		return []string{}
	}

	row := int(pos[0] - 'A')
	col := 0
	if len(pos) == 3 && pos[1:] == "10" {
		col = 10
	} else {
		col = int(pos[1] - '0')
	}

	var adjacent []string
	directions := []struct{ dr, dc int }{
		{-1, 0}, {1, 0}, {0, -1}, {0, 1}, // up, down, left, right
	}

	for _, dir := range directions {
		newRow := row + dir.dr
		newCol := col + dir.dc

		if newRow >= 0 && newRow <= 9 && newCol >= 1 && newCol <= 10 {
			newPos := fmt.Sprintf("%c%d", 'A'+newRow, newCol)
			adjacent = append(adjacent, newPos)
		}
	}

	return adjacent
}

func (c *Claude3Strategy) PlaceShips() []ShipPlacement {
	rand.Seed(time.Now().UnixNano())

	shipLengths := []int{5, 4, 3, 3, 2} // Carrier, Battleship, Cruiser, Submarine, Destroyer

	var placements []ShipPlacement
	occupiedPositions := make(map[string]bool)

	// Claude-3 strategy: Pure random placement
	// Keep it simple - random placement is hard to predict and counter
	for _, ship := range shipLengths {
		placed := false

		for attempts := 0; attempts < 1000; attempts++ {
			rows := "ABCDEFGHIJ"
			row := rows[rand.Intn(10)]
			col := rand.Intn(10) + 1
			horizontal := rand.Intn(2) == 0
			position := fmt.Sprintf("%c%d", row, col)

			if isValidPlacement(position, horizontal, ship, occupiedPositions) {
				shipPositions := getShipPositions(position, horizontal, ship)
				for _, pos := range shipPositions {
					occupiedPositions[pos] = true
				}

				placements = append(placements, ShipPlacement{
					Position:     position,
					IsHorizontal: horizontal,
				})
				placed = true
				break
			}
		}

		if !placed {
			// Emergency fallback: systematic placement
			rows := "ABCDEFGHIJ"
			for _, row := range rows {
				for col := 1; col <= 10; col++ {
					for _, horizontal := range []bool{true, false} {
						position := fmt.Sprintf("%c%d", row, col)
						if isValidPlacement(position, horizontal, ship, occupiedPositions) {
							shipPositions := getShipPositions(position, horizontal, ship)
							for _, pos := range shipPositions {
								occupiedPositions[pos] = true
							}

							placements = append(placements, ShipPlacement{
								Position:     position,
								IsHorizontal: horizontal,
							})
							placed = true
							break
						}
					}
					if placed {
						break
					}
				}
				if placed {
					break
				}
			}
		}
	}

	return placements
}

func (c *Claude3Strategy) SetPlayerColor(color string) {
	c.playerColor = color
}

// Hook for when we get hit feedback (this would need to be integrated with the game system)
func (c *Claude3Strategy) OnAttackResult(position string, hit bool, sunk bool, sunkShipType string) {
	if hit {
		// Add to hit positions for hunting
		c.hitPositions = append(c.hitPositions, position)
		
		if !sunk {
			// Ship not sunk yet - enter hunt mode and add adjacent positions to queue
			c.huntMode = true
			adjacent := c.getAdjacentPositions(position)
			
			// Add unattacked adjacent positions to target queue
			for _, pos := range adjacent {
				if !c.attackedPositions[pos] {
					// Check if already in queue to avoid duplicates
					inQueue := false
					for _, queuePos := range c.targetQueue {
						if queuePos == pos {
							inQueue = true
							break
						}
					}
					if !inQueue {
						c.targetQueue = append(c.targetQueue, pos)
					}
				}
			}
		} else {
			// Ship sunk - remove related hits from our tracking and continue hunting other hits
			c.removeHitsForSunkShip(position, sunkShipType)
			
			// If we still have unsunk hits, continue hunt mode, otherwise exit hunt mode
			if len(c.hitPositions) == 0 {
				c.huntMode = false
				c.targetQueue = []string{}
			}
		}
	}
}

func (c *Claude3Strategy) removeHitsForSunkShip(sunkPosition, shipType string) {
	// In a simple implementation, we could remove just the sunk position
	// In a more sophisticated version, we'd track which hits belong to which ships
	// For now, we'll use a simple heuristic: remove hits that are likely part of the sunk ship
	
	// Remove the sunk position from hit positions
	newHitPositions := []string{}
	for _, hit := range c.hitPositions {
		if hit != sunkPosition {
			newHitPositions = append(newHitPositions, hit)
		}
	}
	c.hitPositions = newHitPositions
	
	// Remove adjacent positions from target queue if they're likely part of the sunk ship
	// This is a simplification - a more advanced version would track ship orientations
	adjacent := c.getAdjacentPositions(sunkPosition)
	newTargetQueue := []string{}
	for _, target := range c.targetQueue {
		keep := true
		for _, adj := range adjacent {
			if target == adj {
				// This target is adjacent to the sunk ship, so it might be part of it
				// We'll remove it to avoid wasting attacks on a sunk ship
				// This is an approximation that works reasonably well
				keep = false
				break
			}
		}
		if keep {
			newTargetQueue = append(newTargetQueue, target)
		}
	}
	c.targetQueue = newTargetQueue
}