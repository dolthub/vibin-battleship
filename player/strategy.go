package main

type ShipPlacement struct {
	Position     string
	IsHorizontal bool
}

type PlayerStrategy interface {
	GetNextMove(gameState string) string
	PlaceShips() []ShipPlacement
}