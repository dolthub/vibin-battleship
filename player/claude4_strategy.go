package main

import (
	"fmt"
	"math/rand"
	"time"
)

type Claude4Strategy struct {
	playerColor         string
	attackedPositions   map[string]bool         // All positions we've attacked
	hitPositions        []string                // Successful hits we haven't finished hunting
	probabilityGrid     map[string]float64      // Probability density function for each position
	remainingShips      []int                   // Ship lengths still alive
	gameState           map[string]string       // Current known state of opponent's board
	lastHit             string                  // Most recent hit position
	huntMode            bool                    // Are we in focused hunt mode?
	targetQueue         []string                // Priority targets around hits
}

func (c *Claude4Strategy) GetNextMove(gameState string) string {
	// Initialize on first call
	if c.attackedPositions == nil {
		c.initializeStrategy()
	}

	// Update our probability grid based on current game state
	c.updateProbabilityGrid()

	var target string

	if c.huntMode && len(c.targetQueue) > 0 {
		// Hunt mode: prioritize targets around known hits
		target = c.getHighestPriorityTarget()
	} else {
		// Probability mode: use superposition algorithm
		target = c.getProbabilityBasedTarget()
	}

	// Mark position as attacked and update game state
	c.attackedPositions[target] = true
	
	return target
}

func (c *Claude4Strategy) initializeStrategy() {
	c.attackedPositions = make(map[string]bool)
	c.hitPositions = []string{}
	c.probabilityGrid = make(map[string]float64)
	c.gameState = make(map[string]string)
	c.remainingShips = []int{5, 4, 3, 3, 2} // Carrier, Battleship, Cruiser, Submarine, Destroyer
	c.huntMode = false
	c.targetQueue = []string{}
	
	rand.Seed(time.Now().UnixNano())
	
	// Initialize probability grid
	c.calculateInitialProbabilities()
}

func (c *Claude4Strategy) calculateInitialProbabilities() {
	// Reset probability grid
	rows := "ABCDEFGHIJ"
	for _, row := range rows {
		for col := 1; col <= 10; col++ {
			pos := fmt.Sprintf("%c%d", row, col)
			c.probabilityGrid[pos] = 0.0
		}
	}
	
	// Superposition algorithm: calculate probability based on all possible ship placements
	for _, shipLength := range c.remainingShips {
		c.addShipProbabilities(shipLength)
	}
}

func (c *Claude4Strategy) addShipProbabilities(shipLength int) {
	rows := "ABCDEFGHIJ"
	
	// Try all possible horizontal placements
	for _, row := range rows {
		for col := 1; col <= 10-shipLength+1; col++ {
			if c.canPlaceShipHorizontally(row, col, shipLength) {
				// Add probability to each position this ship would occupy
				for i := 0; i < shipLength; i++ {
					pos := fmt.Sprintf("%c%d", row, col+i)
					c.probabilityGrid[pos] += 1.0
				}
			}
		}
	}
	
	// Try all possible vertical placements
	for rowIdx := 0; rowIdx <= 10-shipLength; rowIdx++ {
		for col := 1; col <= 10; col++ {
			if c.canPlaceShipVertically(rowIdx, col, shipLength) {
				// Add probability to each position this ship would occupy
				for i := 0; i < shipLength; i++ {
					row := rune('A' + rowIdx + i)
					pos := fmt.Sprintf("%c%d", row, col)
					c.probabilityGrid[pos] += 1.0
				}
			}
		}
	}
}

func (c *Claude4Strategy) canPlaceShipHorizontally(row rune, startCol, length int) bool {
	for i := 0; i < length; i++ {
		pos := fmt.Sprintf("%c%d", row, startCol+i)
		if c.attackedPositions[pos] {
			// Can't place ship through known positions
			if state, exists := c.gameState[pos]; exists && state == "miss" {
				return false
			}
		}
	}
	return true
}

func (c *Claude4Strategy) canPlaceShipVertically(startRow, col, length int) bool {
	for i := 0; i < length; i++ {
		row := rune('A' + startRow + i)
		pos := fmt.Sprintf("%c%d", row, col)
		if c.attackedPositions[pos] {
			// Can't place ship through known positions
			if state, exists := c.gameState[pos]; exists && state == "miss" {
				return false
			}
		}
	}
	return true
}

func (c *Claude4Strategy) updateProbabilityGrid() {
	// Recalculate probabilities based on current knowledge
	c.calculateInitialProbabilities()
	
	// Apply constraints based on known hits and misses
	for pos, state := range c.gameState {
		if state == "miss" {
			c.probabilityGrid[pos] = 0.0
		} else if state == "hit" {
			// Boost probability of adjacent positions for unsunk ships
			c.boostAdjacentProbabilities(pos)
		}
	}
}

func (c *Claude4Strategy) boostAdjacentProbabilities(hitPos string) {
	if len(hitPos) < 2 {
		return
	}
	
	row := int(hitPos[0] - 'A')
	col := 0
	if len(hitPos) == 3 && hitPos[1:] == "10" {
		col = 10
	} else {
		col = int(hitPos[1] - '0')
	}
	
	// Boost adjacent positions
	directions := []struct{ dr, dc int }{
		{-1, 0}, {1, 0}, {0, -1}, {0, 1}, // up, down, left, right
	}
	
	for _, dir := range directions {
		newRow := row + dir.dr
		newCol := col + dir.dc
		
		if newRow >= 0 && newRow <= 9 && newCol >= 1 && newCol <= 10 {
			newPos := fmt.Sprintf("%c%d", 'A'+newRow, newCol)
			if !c.attackedPositions[newPos] {
				c.probabilityGrid[newPos] *= 2.5 // Significant boost for hunt mode
			}
		}
	}
}

func (c *Claude4Strategy) getProbabilityBasedTarget() string {
	bestPos := ""
	maxProbability := -1.0
	
	rows := "ABCDEFGHIJ"
	for _, row := range rows {
		for col := 1; col <= 10; col++ {
			pos := fmt.Sprintf("%c%d", row, col)
			if !c.attackedPositions[pos] && c.probabilityGrid[pos] > maxProbability {
				maxProbability = c.probabilityGrid[pos]
				bestPos = pos
			}
		}
	}
	
	if bestPos == "" {
		// Fallback to random if no position found
		return c.getRandomTarget()
	}
	
	return bestPos
}

func (c *Claude4Strategy) getHighestPriorityTarget() string {
	// Use target queue for systematic elimination around hits
	for len(c.targetQueue) > 0 {
		target := c.targetQueue[0]
		c.targetQueue = c.targetQueue[1:]
		
		if !c.attackedPositions[target] {
			return target
		}
	}
	
	// If queue is empty, exit hunt mode
	c.huntMode = false
	return c.getProbabilityBasedTarget()
}

func (c *Claude4Strategy) getRandomTarget() string {
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
	
	// Fallback: systematic search
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

func (c *Claude4Strategy) getAdjacentPositions(pos string) []string {
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

func (c *Claude4Strategy) PlaceShips() []ShipPlacement {
	rand.Seed(time.Now().UnixNano())
	
	shipLengths := []int{5, 4, 3, 3, 2} // Carrier, Battleship, Cruiser, Submarine, Destroyer
	
	var placements []ShipPlacement
	occupiedPositions := make(map[string]bool)
	
	// Claude-4 strategy: Distributed defensive placement
	// Spread ships across the board to minimize clustering and avoid predictable patterns
	
	for _, ship := range shipLengths {
		placed := false
		
		// Try placement in less obvious areas first
		for attempts := 0; attempts < 1000; attempts++ {
			rows := "ABCDEFGHIJ"
			
			// Prefer middle areas but avoid complete clustering
			var row rune
			var col int
			var horizontal bool
			
			// 60% chance for non-edge placement to avoid predictable edge avoidance
			if rand.Float64() < 0.6 {
				row = rune(rows[2+rand.Intn(6)]) // C-H
				col = 3 + rand.Intn(6)           // 3-8
			} else {
				row = rune(rows[rand.Intn(10)])
				col = rand.Intn(10) + 1
			}
			
			horizontal = rand.Intn(2) == 0
			position := fmt.Sprintf("%c%d", row, col)
			
			if isValidPlacement(position, horizontal, ship, occupiedPositions) {
				// Additional check: avoid tight clustering
				shipPositions := getShipPositions(position, horizontal, ship)
				if c.hasGoodSpacing(shipPositions, occupiedPositions) {
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

func (c *Claude4Strategy) hasGoodSpacing(newShipPositions []string, occupied map[string]bool) bool {
	// Check that new ship doesn't touch existing ships
	for _, pos := range newShipPositions {
		adjacent := c.getAdjacentPositions(pos)
		for _, adjPos := range adjacent {
			if occupied[adjPos] {
				return false // Too close to existing ship
			}
		}
	}
	return true
}

func (c *Claude4Strategy) SetPlayerColor(color string) {
	c.playerColor = color
}

// Advanced feedback processing for probability updates
func (c *Claude4Strategy) OnAttackResult(position string, hit bool, sunk bool, sunkShipType string) {
	if hit {
		c.gameState[position] = "hit"
		c.hitPositions = append(c.hitPositions, position)
		c.lastHit = position
		
		if !sunk {
			// Ship not sunk - enter hunt mode and add strategic targets
			c.huntMode = true
			c.addStrategicTargets(position)
		} else {
			// Ship sunk - remove related hits and update remaining ships
			c.removeShipFromTracking(sunkShipType)
			c.cleanupSunkShipTargets(position)
			
			// Exit hunt mode if no more unsunk hits
			if len(c.hitPositions) == 0 {
				c.huntMode = false
				c.targetQueue = []string{}
			}
		}
	} else {
		c.gameState[position] = "miss"
	}
	
	// Update probability grid after each result
	c.updateProbabilityGrid()
}

func (c *Claude4Strategy) addStrategicTargets(hitPos string) {
	adjacent := c.getAdjacentPositions(hitPos)
	
	// Add adjacent positions to target queue with priority
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
}

func (c *Claude4Strategy) removeShipFromTracking(shipType string) {
	// Remove appropriate ship length from remaining ships
	shipLengths := map[string]int{
		"Carrier":    5,
		"Battleship": 4,
		"Cruiser":    3,
		"Submarine":  3,
		"Destroyer":  2,
	}
	
	if length, exists := shipLengths[shipType]; exists {
		for i, ship := range c.remainingShips {
			if ship == length {
				c.remainingShips = append(c.remainingShips[:i], c.remainingShips[i+1:]...)
				break
			}
		}
	}
}

func (c *Claude4Strategy) cleanupSunkShipTargets(sunkPos string) {
	// Remove targets that were likely part of the sunk ship
	adjacent := c.getAdjacentPositions(sunkPos)
	newTargetQueue := []string{}
	
	for _, target := range c.targetQueue {
		keep := true
		for _, adj := range adjacent {
			if target == adj {
				keep = false
				break
			}
		}
		if keep {
			newTargetQueue = append(newTargetQueue, target)
		}
	}
	
	c.targetQueue = newTargetQueue
	
	// Remove sunk position from hit tracking
	newHitPositions := []string{}
	for _, hit := range c.hitPositions {
		if hit != sunkPos {
			newHitPositions = append(newHitPositions, hit)
		}
	}
	c.hitPositions = newHitPositions
}