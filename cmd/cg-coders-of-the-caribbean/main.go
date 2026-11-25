// Command cg-coders-of-the-caribbean is a referee for the CodinGame game
// Coders of the Caribbean, ported from the official Java referee. Run it
// with -h for usage.
package main

import (
	"github.com/adamadair/codingame-referees/internal/cli"
	"github.com/adamadair/codingame-referees/internal/games/caribbean"
)

const usage = `cg-coders-of-the-caribbean: referee for the CodinGame game "Coders of the Caribbean"

Usage:
  cg-coders-of-the-caribbean [-l logfile]

The referee speaks the CodinGame runner protocol on stdin and stdout, so it
can be used by local game runners such as cg-brutaltester:

  java -jar cg-brutaltester.jar -r "cg-coders-of-the-caribbean" -p1 "./bot1" -p2 "./bot2"

It is a Go port of the official Java referee at league level 3 (the final
league: up to 3 ships, mines, cannons, speed 2). Given the same parameters
and the same bot outputs, it plays exactly the same game. The game is for
2 players.

Game parameters
  The runner may send these as properties lines (key=value) after the
  player count in [[INIT]], in any order. Each is optional:

    seed=<long>              random seed for the map (default: random)
    shipsPerPlayer=<1-3>     ships per player (default: random)
    mineCount=<5-10>         mines to place (default: random)
    barrelCount=<10-26>      rum barrels to place (default: random)

  Values out of range are clamped, and values that aren't whole numbers are
  replaced by the random default. Mines and barrels are placed in mirrored
  pairs, so the final counts can be one higher than asked.

Replaying a game
  At the end, the referee prints the parameters it actually used in an
  [[UINPUT]] block. Sending them back as parameters replays the same map.
`

func main() {
	cli.Main("cg-coders-of-the-caribbean", usage, caribbean.Run)
}
