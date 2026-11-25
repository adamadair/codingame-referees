// Command cg-hypersonic is a referee for the CodinGame game Hypersonic,
// ported from the official Java referee. Run it with -h for usage.
package main

import (
	"github.com/adamadair/codingame-referees/internal/cli"
	"github.com/adamadair/codingame-referees/internal/games/hypersonic"
)

const usage = `cg-hypersonic: referee for the CodinGame game "Hypersonic"

Usage:
  cg-hypersonic [-l logfile]

The referee speaks the CodinGame runner protocol on stdin and stdout, so it
can be used by local game runners such as cg-brutaltester:

  java -jar cg-brutaltester.jar -r "cg-hypersonic" -p1 "./bot1" -p2 "./bot2"

It is a Go port of the official Java referee. Given the same parameters and
the same bot outputs, it plays exactly the same game. The game is for 2 to 4
players, who start in the corners.

Game parameters
  The runner may send these as properties lines (key=value) after the
  player count in [[INIT]], in any order. Each is optional, and a value
  that isn't a whole number is an init error:

    seed=<long>                      random seed for the map (default: random)
    boxes=<n>                        boxes to place, mirrored four ways
                                     (default: 30-65, random)
    GAME_VERSION=<1-4>               rules (default: 4, the final league):
                                       1  practice: no blocks, 30 boxes, no deaths
                                       2  adds items under boxes
                                       3  adds blocks and deaths; the game no
                                          longer ends when the boxes run out
                                       4  adds friendly fire, and the game ends
                                          20 rounds after the last box
    WIDTH=<n>, HEIGHT=<n>            grid size (default: 13 and 11)
    BOMB_FUSE=<n>                    turns before a bomb explodes (default: 8)
    DEFAULT_BOMB_POWER=<n>           starting blast range (default: 3)
    DEFAULT_BOMBS_PER_PLAYER=<n>     starting bombs (default: 1)

  As in the Java referee, players always start in the corners of a 13x11
  grid, even when WIDTH or HEIGHT is changed.

Replaying a game
  At the end, the referee prints the seed and box count in an [[UINPUT]]
  block. Sending them back as parameters, together with any other
  parameters the game used, replays the same map.
`

func main() {
	cli.Main("cg-hypersonic", usage, hypersonic.Run)
}
