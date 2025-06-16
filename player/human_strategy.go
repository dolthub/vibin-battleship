package main

type HumanStrategy struct{}

func (h *HumanStrategy) GetNextMove(gameState string) string {
	panic("HumanStrategy.GetNextMove should never be called - human players connect directly")
}

func (h *HumanStrategy) PlaceShips() []ShipPlacement {
	panic("HumanStrategy.PlaceShips should never be called - human players connect directly")
}