package main

import (
	"log"
	"os"

	"battleship/pkg/commands"
)

func main() {
	if err := commands.RunCommand(os.Args); err != nil {
		log.Fatal(err)
	}
}
