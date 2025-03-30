# Battleship Game

A terminal-based battleship game written in Go that uses colored text output and Dolt for state management.

## Project Structure

```
.
├── src/           # Source code
│   ├── pkg/       # Internal packages
│   │   ├── terminal/  # Terminal output handling
│   │   └── database/  # Dolt database operations
│   └── main.go    # Application entry point
└── data/          # Dolt database files
```

## Prerequisites

- Go 1.16 or later
- Dolt database

## Setup

1. Install Dolt:
   ```bash
   brew install dolt
   ```

2. Install Go dependencies:
   ```bash
   cd src
   go mod tidy
   ```

3. Initialize the Dolt database:
   ```bash
   cd ../data
   dolt init
   ```

## Running the Game

```bash
cd src
go run main.go
```
