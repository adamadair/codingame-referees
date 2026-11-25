// Command cg-game-of-drones is a referee for the CodinGame game Game of
// Drones, ported from the official Java referee. Run it with -h for usage.
package main

import (
	"github.com/adamadair/codingame-referees/internal/cli"
	"github.com/adamadair/codingame-referees/internal/games/gameofdrones"
)

const usage = `cg-game-of-drones: referee for the CodinGame game "Game of Drones"

Usage:
  cg-game-of-drones [-l logfile]

The referee speaks the CodinGame runner protocol on stdin and stdout, so it
can be used by local game runners such as cg-brutaltester:

  java -jar cg-brutaltester.jar -r "cg-game-of-drones" -p1 "./bot1" -p2 "./bot2"

It is a Go port of the official Java referee. Given the same parameters and
the same bot outputs, it plays exactly the same game.

Game parameters
  The runner may send up to three parameter lines after the player count in
  [[INIT]]. They are read by position and only the part after "=" matters:

    nbZones=<1-8>      number of zones (default: 2 per player)
    nbDrones=<1-20>    drones per player (default: 3, 5, 7, 9 or 11, random)
    gameSeed=<long>    random seed for the map (default: current time)

  Send all three or none. Any other parameters, or values out of range, end
  the game at once with every score at 0.

Replaying a game
  At the end, the referee prints the three lines above in an [[UINPUT]]
  block. Sending them back as parameters replays the same map.
`

func main() {
	cli.Main("cg-game-of-drones", usage, gameofdrones.Run)
}
