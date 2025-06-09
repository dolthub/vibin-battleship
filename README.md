# Battleship Game

> Read about the development process: [Vibe Coding with Claude](https://www.dolthub.com/blog/2025-06-09-vibin-coding-with-claude/)

A terminal-based Battleship game built with Go and Dolt database, featuring Git-style branching for game isolation.

## Prerequisites

- [Dolt](https://github.com/dolthub/dolt) - SQL database with Git-style versioning
- Go 1.19 or later

## Setup

### 1. Install Dolt

```bash
sudo bash -c 'curl -L https://github.com/dolthub/dolt/releases/latest/download/install.sh | bash'
```

### 2. Clone the Repository

```bash
git clone https://github.com/dolthub/vibin-battleship
cd vibin-battleship
git checkout claude
```

### 3. Build the Game

From the project root, build the battleship binary:

```bash
cd src
go build -o ../bin/battleship .
```

### 4. Start Dolt SQL Server

Navigate to the `db` directory and start the Dolt SQL server:

```bash
cd db
dolt sql-server
```

The server will start on `127.0.0.1:3306` by default.

## How to Play

### 1. Create a New Game

```bash
./bin/battleship new
```

This will output a game ID that players can use to join.

### 2. Join a Game

Each player joins using the game ID and chooses red or blue:

```bash
./bin/battleship join <game_id> red
./bin/battleship join <game_id> blue
```

Players will be prompted to place their 5 ships:
- Carrier (5 spaces)
- Battleship (4 spaces)
- Cruiser (3 spaces)
- Submarine (3 spaces)
- Destroyer (2 spaces)

### 3. Play the Game

Start the interactive game session:

```bash
./bin/battleship play <game_id> red
# In another terminal:
./bin/battleship play <game_id> blue
```

Players take turns attacking coordinates (e.g., A5, J10) until one player sinks all of the opponent's ships.

## Additional Commands

### Print Current Board State

```bash
./bin/battleship print <game_id> <red|blue>
```

### Attack a Specific Coordinate

```bash
./bin/battleship attack <game_id> <red|blue> <coordinate>
```

### List All Games

```bash
./bin/battleship list
```

## How It Works

This game leverages Dolt's unique branching capabilities:

- The `main` branch contains a `games` table with game metadata and results
- Each game creates its own branch with three tables:
  - `turn` - Manages turn order with random values
  - `red_board` and `blue_board` - Store ship positions and attack results
- When a game completes, temporary tables are dropped and the branch is merged back to `main`

This approach provides complete game isolation and a full audit trail of every move through Dolt's commit history.

## Game Rules

- Standard Battleship rules apply
- 10x10 grid using coordinates A1-J10
- First player is determined by random roll when both players join
- Hit all enemy ships to win
- Ships cannot overlap or extend off the board

## Development

### Running Tests

```bash
cd src
go test -v
```

### Database Schema

The game uses a branching strategy where each game gets its own isolated branch:

**Main Branch:**
- `games` table - Game metadata and final results

**Game Branches:**
- `turn` table - Turn management with player values
- `red_board` / `blue_board` tables - Ship placements and attack results

This design ensures games are completely isolated and provides full versioning of game state.