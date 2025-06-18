package main

import (
	"fmt"
	"math/rand"
	"time"
)

type Claude5Strategy struct {
	playerColor       string
	attackedPositions map[string]bool    // All positions we've attacked
	hitPositions      []string           // All successful hits (unsunk ships only)
	sunkShips         []string           // Types of ships we've sunk
	huntTargets       []string           // Priority targets around hits
	shipOrientations  map[string]string  // Track ship orientations: "horizontal", "vertical", "unknown"
	shipSegments      map[string][]string // Track which hits belong to which ship
	probabilityGrid   map[string]float64 // Probability density for each position
	gamePhase         string             // "search", "hunt", "finish"
	lastTarget        string             // Last position we attacked
	consecutiveHits   int                // Number of consecutive hits on current ship
}

func (c *Claude5Strategy) GetNextMove(gameState string, lastMoveHit bool, sunkShipType string) string {
	// Initialize on first call
	if c.attackedPositions == nil {
		c.initializeStrategy()
	}

	// Process feedback from last move
	c.processMoveResult(lastMoveHit, sunkShipType)

	// Update probability grid based on current knowledge
	c.updateProbabilityGrid()

	var target string
	switch c.gamePhase {
	case "hunt":
		target = c.getHuntTarget()
		if target == "" {
			c.gamePhase = "search"
			target = c.getSearchTarget()
		}
	case "finish":
		target = c.getFinishingTarget()
		if target == "" {
			c.gamePhase = "search"
			target = c.getSearchTarget()
		}
	default: // "search"
		target = c.getSearchTarget()
	}

	// Mark position as attacked and update last target
	c.attackedPositions[target] = true
	c.lastTarget = target

	return target
}

func (c *Claude5Strategy) initializeStrategy() {
	c.attackedPositions = make(map[string]bool)
	c.hitPositions = []string{}
	c.sunkShips = []string{}
	c.huntTargets = []string{}
	c.shipOrientations = make(map[string]string)
	c.shipSegments = make(map[string][]string)
	c.probabilityGrid = make(map[string]float64)
	c.gamePhase = "search"
	c.consecutiveHits = 0

	rand.Seed(time.Now().UnixNano())

	// Initialize probability grid with ship placement analysis
	c.calculateInitialProbabilities()
}

func (c *Claude5Strategy) processMoveResult(hit bool, sunkShipType string) {
	if c.lastTarget == "" {
		return
	}

	if hit {
		c.hitPositions = append(c.hitPositions, c.lastTarget)
		c.consecutiveHits++

		if sunkShipType != "" {
			// Ship was sunk - clean up hunt state
			c.sunkShips = append(c.sunkShips, sunkShipType)
			c.cleanupSunkShip(c.lastTarget, sunkShipType)
			c.consecutiveHits = 0

			// If we still have unsunk hits, continue hunting, otherwise search
			if len(c.hitPositions) > 0 {
				c.gamePhase = "hunt"
			} else {
				c.gamePhase = "search"
			}
		} else {
			// Hit but ship not sunk - enter hunt mode
			c.gamePhase = "hunt"
			c.addIntelligentHuntTargets(c.lastTarget)
		}
	} else {
		// Miss - reset consecutive hits if we were hunting
		if c.gamePhase == "hunt" {
			c.consecutiveHits = 0
		}
	}
}

func (c *Claude5Strategy) addIntelligentHuntTargets(hitPos string) {
	adjacent := c.getAdjacentPositions(hitPos)

	// If this is our first hit, add all adjacent positions
	if c.consecutiveHits == 1 {
		for _, pos := range adjacent {
			if !c.attackedPositions[pos] && !c.contains(c.huntTargets, pos) {
				c.huntTargets = append(c.huntTargets, pos)
			}
		}
		c.shipOrientations[hitPos] = "unknown"
		return
	}

	// If we have multiple hits, determine ship orientation
	orientation := c.determineShipOrientation()
	if orientation != "unknown" {
		// Focus on continuing in the discovered direction
		c.focusHuntDirection(hitPos, orientation)
	}
}

func (c *Claude5Strategy) determineShipOrientation() string {
	if len(c.hitPositions) < 2 {
		return "unknown"
	}

	// Check if recent hits are aligned horizontally or vertically
	lastTwo := c.hitPositions[len(c.hitPositions)-2:]
	pos1, pos2 := lastTwo[0], lastTwo[1]

	row1, col1 := c.parseCoordinate(pos1)
	row2, col2 := c.parseCoordinate(pos2)

	if row1 == row2 {
		return "horizontal"
	} else if col1 == col2 {
		return "vertical"
	}
	return "unknown"
}

func (c *Claude5Strategy) focusHuntDirection(hitPos string, orientation string) {
	row, col := c.parseCoordinate(hitPos)

	// Clear old hunt targets and add focused ones
	c.huntTargets = []string{}

	if orientation == "horizontal" {
		// Add positions to the left and right
		leftPos := c.formatCoordinate(row, col-1)
		rightPos := c.formatCoordinate(row, col+1)

		for _, pos := range []string{leftPos, rightPos} {
			if c.isValidCoordinate(pos) && !c.attackedPositions[pos] {
				c.huntTargets = append(c.huntTargets, pos)
			}
		}
	} else if orientation == "vertical" {
		// Add positions above and below
		upPos := c.formatCoordinate(row-1, col)
		downPos := c.formatCoordinate(row+1, col)

		for _, pos := range []string{upPos, downPos} {
			if c.isValidCoordinate(pos) && !c.attackedPositions[pos] {
				c.huntTargets = append(c.huntTargets, pos)
			}
		}
	}
}

func (c *Claude5Strategy) getHuntTarget() string {
	// Prioritize targets in the direction of known ship orientation
	for len(c.huntTargets) > 0 {
		target := c.huntTargets[0]
		c.huntTargets = c.huntTargets[1:]

		if !c.attackedPositions[target] {
			return target
		}
	}

	// If no hunt targets, try to find adjacent positions to any unsunk hits
	for _, hit := range c.hitPositions {
		adjacent := c.getAdjacentPositions(hit)
		for _, pos := range adjacent {
			if !c.attackedPositions[pos] {
				return pos
			}
		}
	}

	return ""
}

func (c *Claude5Strategy) getSearchTarget() string {
	// Use probability-based targeting for search mode
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
		// Fallback to any unattacked position
		return c.getRandomTarget()
	}

	return bestPos
}

func (c *Claude5Strategy) getFinishingTarget() string {
	// In finish mode, systematically clear remaining positions
	// Use a checkerboard pattern for efficiency
	rows := "ABCDEFGHIJ"

	for offset := 0; offset < 2; offset++ {
		for rowIdx, row := range rows {
			for col := 1; col <= 10; col++ {
				if (rowIdx+col+offset)%2 == 0 {
					pos := fmt.Sprintf("%c%d", row, col)
					if !c.attackedPositions[pos] {
						return pos
					}
				}
			}
		}
	}

	// Fallback to any position
	return c.getRandomTarget()
}

func (c *Claude5Strategy) calculateInitialProbabilities() {
	rows := "ABCDEFGHIJ"

	// Reset grid
	for _, row := range rows {
		for col := 1; col <= 10; col++ {
			pos := fmt.Sprintf("%c%d", row, col)
			c.probabilityGrid[pos] = 0.0
		}
	}

	// Calculate based on remaining ships
	remainingShips := c.getRemainingShips()
	for _, shipLength := range remainingShips {
		c.addShipProbability(shipLength)
	}
}

func (c *Claude5Strategy) addShipProbability(shipLength int) {
	rows := "ABCDEFGHIJ"

	// Try all possible horizontal placements
	for _, row := range rows {
		for col := 1; col <= 10-shipLength+1; col++ {
			if c.canPlaceShip(row, col, true, shipLength) {
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
			row := rune('A' + rowIdx)
			if c.canPlaceShip(row, col, false, shipLength) {
				for i := 0; i < shipLength; i++ {
					pos := fmt.Sprintf("%c%d", 'A'+rowIdx+i, col)
					c.probabilityGrid[pos] += 1.0
				}
			}
		}
	}
}

func (c *Claude5Strategy) canPlaceShip(startRow rune, startCol int, horizontal bool, length int) bool {
	for i := 0; i < length; i++ {
		var pos string
		if horizontal {
			pos = fmt.Sprintf("%c%d", startRow, startCol+i)
		} else {
			pos = fmt.Sprintf("%c%d", startRow+rune(i), startCol)
		}

		if c.attackedPositions[pos] {
			return false // Can't place through known positions
		}
	}
	return true
}

func (c *Claude5Strategy) updateProbabilityGrid() {
	c.calculateInitialProbabilities()

	// Boost probability around known hits
	for _, hit := range c.hitPositions {
		adjacent := c.getAdjacentPositions(hit)
		for _, pos := range adjacent {
			if !c.attackedPositions[pos] {
				c.probabilityGrid[pos] *= 3.0 // Strong boost
			}
		}
	}

	// Reduce probability in areas where ships can't fit
	c.applyShipConstraints()
}

func (c *Claude5Strategy) applyShipConstraints() {
	remainingShips := c.getRemainingShips()
	if len(remainingShips) == 0 {
		return
	}

	// For each position, check if any remaining ship can be placed there
	rows := "ABCDEFGHIJ"
	for _, row := range rows {
		for col := 1; col <= 10; col++ {
			pos := fmt.Sprintf("%c%d", row, col)
			if c.attackedPositions[pos] {
				continue
			}

			canFitShip := false
			for _, shipLength := range remainingShips {
				// Check horizontal placement
				if c.canPlaceShip(row, col, true, shipLength) {
					canFitShip = true
					break
				}
				// Check vertical placement
				if c.canPlaceShip(row, col, false, shipLength) {
					canFitShip = true
					break
				}
			}

			if !canFitShip {
				c.probabilityGrid[pos] *= 0.1 // Heavily reduce probability
			}
		}
	}
}

func (c *Claude5Strategy) getRemainingShips() []int {
	remaining := []int{}

	shipCounts := map[int]int{5: 1, 4: 1, 3: 2, 2: 1}

	// Remove sunk ships
	for _, sunkType := range c.sunkShips {
		var length int
		switch sunkType {
		case "Carrier":
			length = 5
		case "Battleship":
			length = 4
		case "Cruiser", "Submarine":
			length = 3
		case "Destroyer":
			length = 2
		}

		if shipCounts[length] > 0 {
			shipCounts[length]--
		}
	}

	// Build remaining ships list
	for length, count := range shipCounts {
		for i := 0; i < count; i++ {
			remaining = append(remaining, length)
		}
	}

	return remaining
}

func (c *Claude5Strategy) cleanupSunkShip(sunkPos string, shipType string) {
	// Remove all hits that were part of this ship
	newHitPositions := []string{}
	for _, hit := range c.hitPositions {
		// Simple heuristic: remove hits that are adjacent to the sunk position
		if !c.areAdjacent(hit, sunkPos) && hit != sunkPos {
			newHitPositions = append(newHitPositions, hit)
		}
	}
	c.hitPositions = newHitPositions

	// Clear hunt targets in the area of the sunk ship
	newHuntTargets := []string{}
	for _, target := range c.huntTargets {
		if !c.areAdjacent(target, sunkPos) {
			newHuntTargets = append(newHuntTargets, target)
		}
	}
	c.huntTargets = newHuntTargets
}

func (c *Claude5Strategy) areAdjacent(pos1, pos2 string) bool {
	adjacent := c.getAdjacentPositions(pos1)
	return c.contains(adjacent, pos2)
}

func (c *Claude5Strategy) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (c *Claude5Strategy) parseCoordinate(pos string) (int, int) {
	if len(pos) < 2 {
		return -1, -1
	}

	row := int(pos[0] - 'A')
	col := 0
	if len(pos) == 3 && pos[1:] == "10" {
		col = 10
	} else {
		col = int(pos[1] - '0')
	}

	return row, col
}

func (c *Claude5Strategy) formatCoordinate(row, col int) string {
	if row < 0 || row > 9 || col < 1 || col > 10 {
		return ""
	}
	return fmt.Sprintf("%c%d", 'A'+row, col)
}

func (c *Claude5Strategy) isValidCoordinate(pos string) bool {
	if pos == "" {
		return false
	}
	row, col := c.parseCoordinate(pos)
	return row >= 0 && row <= 9 && col >= 1 && col <= 10
}

func (c *Claude5Strategy) getAdjacentPositions(pos string) []string {
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

func (c *Claude5Strategy) getRandomTarget() string {
	rows := "ABCDEFGHIJ"

	for attempts := 0; attempts < 1000; attempts++ {
		row := rows[rand.Intn(10)]
		col := rand.Intn(10) + 1
		pos := fmt.Sprintf("%c%d", row, col)

		if !c.attackedPositions[pos] {
			return pos
		}
	}

	// Systematic fallback
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

func (c *Claude5Strategy) PlaceShips() []ShipPlacement {
	rand.Seed(time.Now().UnixNano())

	shipLengths := []int{5, 4, 3, 3, 2} // Carrier, Battleship, Cruiser, Submarine, Destroyer

	var placements []ShipPlacement
	occupiedPositions := make(map[string]bool)

	// Claude-5 strategy: Adaptive placement with anti-pattern spacing
	// Mix defensive clusters with isolated ships to confuse systematic searchers

	for _, shipLength := range shipLengths {
		placed := false

		// Strategy: Larger ships get defensive placement, smaller ships spread out
		isLargeShip := shipLength >= 4

		for attempts := 0; attempts < 1000; attempts++ {
			rows := "ABCDEFGHIJ"
			var row rune
			var col int
			var horizontal bool

			if isLargeShip {
				// Large ships: prefer corners and edges for defense
				if rand.Float64() < 0.7 {
					// Edge placement
					edge := rand.Intn(4)
					switch edge {
					case 0: // Top edge
						row = 'A'
						col = rand.Intn(8) + 2 // B-I
					case 1: // Bottom edge
						row = 'J'
						col = rand.Intn(8) + 2
					case 2: // Left edge
						row = rune('A' + rand.Intn(8) + 1) // B-I
						col = 1
					case 3: // Right edge
						row = rune('A' + rand.Intn(8) + 1)
						col = 10
					}
				} else {
					// Corner placement
					corners := []struct{ r rune; c int }{
						{'A', 1}, {'A', 10}, {'J', 1}, {'J', 10},
					}
					corner := corners[rand.Intn(4)]
					row, col = corner.r, corner.c
				}
			} else {
				// Small ships: spread out randomly, avoid clustering
				row = rune(rows[rand.Intn(10)])
				col = rand.Intn(10) + 1
			}

			horizontal = rand.Intn(2) == 0
			position := fmt.Sprintf("%c%d", row, col)

			if isValidPlacement(position, horizontal, shipLength, occupiedPositions) {
				shipPositions := getShipPositions(position, horizontal, shipLength)

				// For small ships, ensure they're not too close to large ships
				if !isLargeShip {
					tooClose := false
					for _, pos := range shipPositions {
						nearbyPositions := c.getAdjacentPositions(pos)
						for _, nearby := range nearbyPositions {
							if occupiedPositions[nearby] {
								tooClose = true
								break
							}
						}
						if tooClose {
							break
						}
					}
					// Allow some clustering, but prefer spacing
					if tooClose && rand.Float64() < 0.6 {
						continue
					}
				}

				// Valid placement
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
			// Emergency systematic fallback
			rows := "ABCDEFGHIJ"
			for _, row := range rows {
				for col := 1; col <= 10; col++ {
					for _, horizontal := range []bool{true, false} {
						position := fmt.Sprintf("%c%d", row, col)
						if isValidPlacement(position, horizontal, shipLength, occupiedPositions) {
							shipPositions := getShipPositions(position, horizontal, shipLength)
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

func (c *Claude5Strategy) SetPlayerColor(color string) {
	c.playerColor = color
}