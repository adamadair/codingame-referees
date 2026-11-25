// Command cg-game-of-drones is a referee for the CodinGame game Game of
// Drones, ported from the official Java referee. Run it with -h for usage.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

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

Exit status
  0 when the game ends normally, 1 if the input is malformed, ends before
  the game is over, or the referee fails.

Options
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("cg-game-of-drones", flag.ContinueOnError)
	flags.SetOutput(stderr)
	logFile := flags.String("l", "", "append errors to `logfile` as well as stderr (by default no log file is written)")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "cg-game-of-drones: unexpected argument %q\n", flags.Arg(0))
		flags.Usage()
		return 2
	}

	err := gameofdrones.Run(stdin, stdout)
	if err == nil {
		return 0
	}
	fmt.Fprintln(stderr, "cg-game-of-drones:", err)
	if *logFile != "" {
		if logErr := appendLog(*logFile, err); logErr != nil {
			fmt.Fprintln(stderr, "cg-game-of-drones: cannot write log:", logErr)
		}
	}
	return 1
}

// appendLog adds a timestamped error report to the log file.
func appendLog(path string, err error) error {
	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if openErr != nil {
		return openErr
	}
	_, writeErr := fmt.Fprintf(f, "%s cg-game-of-drones: %v\n", time.Now().Format(time.RFC3339), err)
	return errors.Join(writeErr, f.Close())
}
