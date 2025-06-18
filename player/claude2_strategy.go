package main

import (
	"fmt"
	"math/rand"
	"time"
)

type Claude2Strategy struct {
	playerColor       string
	attackedPositions map[string]bool // Track all attacked positions
	heatMap          map[string]int  // Heat map for target priority
	hits             []string        // All successful hits
	misses           []string        // All misses
	sunkShips        []string        // Ships we've confirmed sunk
	targetQueue      []string        // Priority queue of targets
	gamePhase        string          // "hunt", "target", "cleanup"
}

func (c *Claude2Strategy) GetNextMove(gameState string) string {
	// Initialize on first call
	if c.attackedPositions == nil {
		c.initializeStrategy()
	}

	// Update heat map based on current game state
	c.updateHeatMap()

	var target string

	switch c.gamePhase {
	case "hunt":
		// Early game: use heat map to find ships efficiently
		target = c.getHeatMapTarget()
		if len(c.hits) > 0 {
			c.gamePhase = "target"
		}
	case "target":
		// Mid game: systematically eliminate found ships
		target = c.getSystematicTarget()
		if len(c.targetQueue) == 0 && len(c.hits) < 3 {
			c.gamePhase = "cleanup"
		}
	case "cleanup":
		// End game: clean up remaining positions with smart targeting
		target = c.getCleanupTarget()
	}

	// Fallback if no target found
	if target == "" || c.attackedPositions[target] {
		target = c.getAnyValidTarget()
	}

	// Mark as attacked
	c.attackedPositions[target] = true

	return target
}

func (c *Claude2Strategy) initializeStrategy() {
	c.attackedPositions = make(map[string]bool)
	c.heatMap = make(map[string]int)
	c.hits = []string{}
	c.misses = []string{}
	c.sunkShips = []string{}
	c.targetQueue = []string{}
	c.gamePhase = "hunt"

	// Initialize heat map with strategic weights
	c.initializeHeatMap()
}

func (c *Claude2Strategy) initializeHeatMap() {
	rows := "ABCDEFGHIJ"

	// Strategy: Ships are more likely to be placed in certain patterns
	// Weight center areas higher, edges lower
	// Weight intersections of common ship placement patterns
	
	for rowIdx, row := range rows {
		for col := 1; col <= 10; col++ {
			pos := fmt.Sprintf("%c%d", row, col)
			heat := 10 // Base heat

			// Center bias - ships often placed away from edges
			centerDistanceRow := abs(float64(rowIdx) - 4.5)
			centerDistanceCol := abs(float64(col) - 5.5)
			if centerDistanceRow <= 3 && centerDistanceCol <= 3 {
				heat += 5 // Center area bonus
			}

			// Edge penalty
			if rowIdx == 0 || rowIdx == 9 || col == 1 || col == 10 {
				heat -= 3
			}

			// Strategic intersection points (where ships commonly cross)
			if (rowIdx == 2 || rowIdx == 7) && (col == 3 || col == 8) {
				heat += 8 // High-value intersection
			}

			// Diagonal preferences (many players avoid pure horizontal/vertical lines)
			if rowIdx+col == 5 || rowIdx+col == 10 || rowIdx+col == 15 {
				heat += 3
			}

			c.heatMap[pos] = heat
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func (c *Claude2Strategy) updateHeatMap() {
	// Dynamically adjust heat map based on hits and misses
	
	// Reduce heat around misses
	for _, miss := range c.misses {
		adjacent := c.getAdjacentPositions(miss)
		for _, pos := range adjacent {
			if !c.attackedPositions[pos] {
				c.heatMap[pos] = max(0, c.heatMap[pos]-2)
			}
		}
	}

	// Increase heat around hits (but not too close to sunk ships)
	for _, hit := range c.hits {
		if !c.isPartOfSunkShip(hit) {
			adjacent := c.getAdjacentPositions(hit)
			for _, pos := range adjacent {
				if !c.attackedPositions[pos] {
					c.heatMap[pos] += 15 // Strong heat boost near unsunk hits
				}
			}
		}
	}

	// Update target queue with high-priority adjacent positions
	c.updateTargetQueue()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (c *Claude2Strategy) isPartOfSunkShip(position string) bool {
	// Simple heuristic: if we have many hits in a line and no more adjacent targets,
	// assume the ship is sunk (this is simplified - real implementation would track sunk ships)
	return false
}

func (c *Claude2Strategy) updateTargetQueue() {
	// Clear and rebuild target queue with high-priority positions
	c.targetQueue = []string{}
	
	// Add positions adjacent to recent hits with high heat
	for _, hit := range c.hits {
		if !c.isPartOfSunkShip(hit) {
			adjacent := c.getAdjacentPositions(hit)
			for _, pos := range adjacent {
				if !c.attackedPositions[pos] && c.heatMap[pos] > 15 {
					c.targetQueue = append(c.targetQueue, pos)
				}
			}
		}
	}
}

func (c *Claude2Strategy) getHeatMapTarget() string {
	// Find the position with highest heat that hasn't been attacked
	bestPos := ""
	maxHeat := -1

	rows := "ABCDEFGHIJ"
	for _, row := range rows {
		for col := 1; col <= 10; col++ {
			pos := fmt.Sprintf("%c%d", row, col)
			if !c.attackedPositions[pos] && c.heatMap[pos] > maxHeat {
				maxHeat = c.heatMap[pos]
				bestPos = pos
			}
		}
	}

	return bestPos
}

func (c *Claude2Strategy) getSystematicTarget() string {
	// Use target queue for systematic elimination
	for len(c.targetQueue) > 0 {
		target := c.targetQueue[0]
		c.targetQueue = c.targetQueue[1:]
		
		if !c.attackedPositions[target] {
			return target
		}
	}

	// If queue is empty, fall back to heat map
	return c.getHeatMapTarget()
}

func (c *Claude2Strategy) getCleanupTarget() string {
	// End game cleanup: target remaining high-probability areas
	// Use a modified checkerboard pattern to efficiently cover remaining space
	
	rows := "ABCDEFGHIJ"
	
	// Phase 1: Checkerboard pattern (every other square)
	for offset := 0; offset < 2; offset++ {
		for rowIdx, row := range rows {
			for col := 1; col <= 10; col++ {
				if (rowIdx+col+offset)%2 == 0 {
					pos := fmt.Sprintf("%c%d", row, col)
					if !c.attackedPositions[pos] && c.heatMap[pos] > 5 {
						return pos
					}
				}
			}
		}
	}

	// Phase 2: Any remaining position
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

func (c *Claude2Strategy) getAnyValidTarget() string {
	rows := "ABCDEFGHIJ"
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

func (c *Claude2Strategy) getAdjacentPositions(pos string) []string {
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

func (c *Claude2Strategy) PlaceShips() []ShipPlacement {
	rand.Seed(time.Now().UnixNano())

	shipLengths := []int{5, 4, 3, 3, 2} // Carrier, Battleship, Cruiser, Submarine, Destroyer

	var placements []ShipPlacement
	occupiedPositions := make(map[string]bool)

	// Claude-2 strategy: Clustered placement in one corner/region
	// This makes it harder for systematic searchers to find all ships quickly
	// Place ships in overlapping defensive clusters

	// Define preferred cluster regions (pick one randomly)
	clusters := []struct {
		startRow, endRow int
		startCol, endCol int
	}{
		{0, 4, 1, 5},   // Top-left cluster
		{5, 9, 6, 10},  // Bottom-right cluster
		{0, 4, 6, 10},  // Top-right cluster
		{5, 9, 1, 5},   // Bottom-left cluster
	}

	selectedCluster := clusters[rand.Intn(len(clusters))]

	// Try to place ships in the selected cluster first
	for _, ship := range shipLengths {
		placed := false

		// Try cluster placement first (80% of attempts)
		for attempts := 0; attempts < 800; attempts++ {
			rows := "ABCDEFGHIJ"
			rowIdx := selectedCluster.startRow + rand.Intn(selectedCluster.endRow-selectedCluster.startRow+1)
			row := rows[rowIdx]
			col := selectedCluster.startCol + rand.Intn(selectedCluster.endCol-selectedCluster.startCol+1)
			horizontal := rand.Intn(2) == 0
			position := fmt.Sprintf("%c%d", row, col)

			if isValidPlacement(position, horizontal, ship, occupiedPositions) {
				shipPositions := getShipPositions(position, horizontal, ship)
				
				// Check if placement stays mostly within cluster
				inCluster := 0
				for _, pos := range shipPositions {
					shipRow := int(pos[0] - 'A')
					shipCol := 0
					if len(pos) == 3 {
						shipCol = 10
					} else {
						shipCol = int(pos[1] - '0')
					}
					if shipRow >= selectedCluster.startRow && shipRow <= selectedCluster.endRow &&
						shipCol >= selectedCluster.startCol && shipCol <= selectedCluster.endCol {
						inCluster++
					}
				}

				// Accept if at least 70% of ship is in cluster
				if float64(inCluster)/float64(len(shipPositions)) >= 0.7 {
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

		// If cluster placement failed, fall back to random placement (20% of attempts)
		if !placed {
			for attempts := 0; attempts < 200; attempts++ {
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
		}

		// Final emergency fallback
		if !placed {
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

func (c *Claude2Strategy) SetPlayerColor(color string) {
	c.playerColor = color
}

// Hook for when we get hit feedback (this would need to be integrated with the game system)
func (c *Claude2Strategy) OnAttackResult(position string, hit bool, sunk bool, sunkShipType string) {
	if hit {
		c.hits = append(c.hits, position)
		if sunk {
			c.sunkShips = append(c.sunkShips, sunkShipType)
			// Remove sunk ship positions from consideration
			c.cleanupSunkShip(position, sunkShipType)
		}
	} else {
		c.misses = append(c.misses, position)
	}

	// Update heat map after each attack result
	c.updateHeatMap()
}

func (c *Claude2Strategy) cleanupSunkShip(hitPosition, shipType string) {
	// Mark ship area as cleared (simplified implementation)
	// In a full implementation, we'd track the exact ship positions
}