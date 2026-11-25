// Command cg-code-busters is a referee for the CodinGame game Code
// Busters, ported from the official Java referee. Run it with -h for usage.
package main

import (
	"github.com/adamadair/codingame-referees/internal/cli"
	"github.com/adamadair/codingame-referees/internal/games/codebusters"
)

const usage = `cg-code-busters: referee for the CodinGame game "Code Busters"

Usage:
  cg-code-busters [-l logfile]

The referee speaks the CodinGame runner protocol on stdin and stdout, so it
can be used by local game runners such as cg-brutaltester:

  java -jar cg-brutaltester.jar -r "cg-code-busters" -p1 "./bot1" -p2 "./bot2"

It is a Go port of the official Java referee (the multiplayer puzzle, final
league: MOVE, BUST, RELEASE, STUN, RADAR and EJECT). Given the same
parameters and the same bot outputs, it plays exactly the same game.

The game is for 2 players. Like the Java referee it accepts 3 or 4, but
only scores players 0 and 1 for its early end, and stops without scores
when all but one player of such a game are dead.

Game parameters
  The runner may send these as properties lines (key=value) after the
  player count in [[INIT]], in any order. Each is optional, and a value
  that isn't a whole number is an init error, after which the referee
  stops without scores, as the Java one does:

    seed=<long>                       random seed for the map (default: random)
    busters=<n>                       busters per player, clamped to 2-5
                                      (default: 2-4, random)
    ghosts=<n>                        ghosts to place, at least 1; they come in
                                      mirrored pairs plus a central ghost, so
                                      one more may be placed (default:
                                      4 per buster + 0-8, at most 30)
    CENTRAL_GHOST=<true|false>        place a ghost at the centre (default: true)
    BUSTER_SPEED=<n>                  move distance per turn (default: 800)
    BUSTER_SPEED_WHILE_CARRYING=<n>   the same while carrying a ghost (default: 800)
    VIEW_RANGE=<n>                    busters' sight (default: 2200)
    GHOST_VIEW_RANGE=<n>              distance at which ghosts flee (default: 2200)
    GHOST_SPEED=<n>                   ghosts' flee distance per turn (default: 400)
    EJECT_RANGE=<n>                   how far EJECT throws a ghost (default: 1760)
    map=<...>                         obstacles; any value is an init error, as
                                      in the Java referee

Replaying a game
  At the end, the referee prints the seed, busters and ghosts in an
  [[UINPUT]] block. Sending them back as parameters, together with any
  other parameters the game used, replays the same game.
`

func main() {
	cli.Main("cg-code-busters", usage, codebusters.Run)
}
