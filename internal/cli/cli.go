// Package cli holds what every referee command shares: the -l and -h
// flags, running one game on stdin and stdout, logging errors and exit
// codes.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// Play runs one game, reading the runner's commands from in and writing
// the referee's answers to out. It returns an error when the referee stops
// without printing scores.
type Play func(in io.Reader, out io.Writer) error

const commonUsage = `
Exit status
  0 when the game ends normally, 1 if the input is malformed, ends before
  the game is over, or the referee fails, 2 for bad command-line arguments.

Options
`

// Main runs a referee command and exits. usage is the command's own help
// text; the exit status and options sections are appended to it.
func Main(name, usage string, play Play) {
	os.Exit(Run(name, usage, play, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// Run is Main without the exit, for tests. It returns the exit status.
func Run(name, usage string, play Play, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	logFile := flags.String("l", "", "append errors to `logfile` as well as stderr (by default no log file is written)")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usage, commonUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "%s: unexpected argument %q\n", name, flags.Arg(0))
		flags.Usage()
		return 2
	}

	err := play(stdin, stdout)
	if err == nil {
		return 0
	}
	fmt.Fprintf(stderr, "%s: %v\n", name, err)
	if *logFile != "" {
		if logErr := appendLog(*logFile, name, err); logErr != nil {
			fmt.Fprintf(stderr, "%s: cannot write log: %v\n", name, logErr)
		}
	}
	return 1
}

// appendLog adds a timestamped error report to the log file.
func appendLog(path, name string, err error) error {
	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if openErr != nil {
		return openErr
	}
	_, writeErr := fmt.Fprintf(f, "%s %s: %v\n", time.Now().Format(time.RFC3339), name, err)
	return errors.Join(writeErr, f.Close())
}
