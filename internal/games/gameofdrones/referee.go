// Package gameofdrones is a Go port of the official CodinGame Java referee
// for Game of Drones.
//
// Each player controls the same number of drones on a 4000x1800 board with
// a few circular zones. Every turn a player gives a target point for each of
// its drones, and drones move up to 100 units toward it. At the end of each
// round, a zone belongs to the player with strictly the most drones in it
// (a tie keeps the current owner), and every owned zone scores a point for
// its owner. The game lasts 200 rounds.
//
// Given the same parameters (or seed) and the same bot outputs, this port
// gives players the same input and the same scores as the Java referee.
// Unlike most referees, Game of Drones doesn't use the shared Java
// framework, so this package has its own protocol loop.
package gameofdrones

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/adamadair/codingame-referees/internal/jrandom"
	"github.com/adamadair/codingame-referees/internal/jscanner"
	"github.com/adamadair/codingame-referees/internal/jstrings"
)

// Referee plays one game. The zero value is ready to use.
type Referee struct {
	// Clock returns the seed used when the runner sends no parameters, in
	// milliseconds like Java's System.currentTimeMillis. Nil means the
	// current time.
	Clock func() int64

	rand              *jrandom.Random
	end               bool
	reason            string
	reasonCode        string
	nbPlayers         int
	nbZones           int
	nbDronesPerPlayer int
	saveUserInput     string
	players           []*playerData
	zones             []*zoneData
	initLines         []string

	currentPlayerID  int
	currentPlayer    *playerData
	nbRounds         int
	nbIntermediate   int
	nbGlobalRounds   int
	nbCurrentPlayers int

	out      *bufio.Writer
	finished bool
}

// PanicError reports a panic inside the referee, with its stack trace.
type PanicError struct {
	Value any
	Stack []byte
}

func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v\n%s", e.Value, e.Stack) }

// Run plays one game with a default Referee.
func Run(in io.Reader, out io.Writer) error {
	return (&Referee{}).Run(in, out)
}

// Run reads the runner's commands from in and writes the referee's answers
// to out. It returns nil once the scores are printed. It returns an error
// if the input is malformed, ends before the game is over, or the referee
// fails; the Java referee would have logged a stack trace in those cases.
func (r *Referee) Run(in io.Reader, out io.Writer) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = &PanicError{Value: p, Stack: debug.Stack()}
		}
	}()
	r.currentPlayerID = -1
	r.out = bufio.NewWriter(out)
	s := jscanner.New(in)
	for !r.finished {
		if err := r.command(s); err != nil {
			if errors.Is(err, jscanner.ErrNoSuchElement) {
				return errors.New("input ended before the game was over")
			}
			return err
		}
	}
	return nil
}

func (r *Referee) clock() int64 {
	if r.Clock != nil {
		return r.Clock()
	}
	return time.Now().UnixMilli()
}

func errMissingLines(got, want int) error {
	return fmt.Errorf("SET_PLAYER_OUTPUT has %d lines, expected %d", got, want)
}

// command reads and runs one command, parsed like Referee.parseCommand:
// "[[NAME] count]".
func (r *Referee) command(s *jscanner.Scanner) error {
	line, err := s.NextLine()
	if err != nil {
		return err
	}
	firstClose := strings.IndexByte(line, ']')
	if firstClose < 2 || firstClose+2 > len(line)-1 {
		return fmt.Errorf("malformed command %q", line)
	}
	cmd := line[2:firstClose]
	nbLines, err := jstrings.ParseInt(line[firstClose+2 : len(line)-1])
	if err != nil {
		return fmt.Errorf("malformed command %q", line)
	}

	switch cmd {
	case "INIT":
		nbPlayers, err := s.NextInt()
		if err != nil {
			return err
		}
		if _, err := s.NextLine(); err != nil {
			return err
		}
		var initLines []string
		for range max(0, int(nbLines)-1) {
			l, err := s.NextLine()
			if err != nil {
				return err
			}
			initLines = append(initLines, l)
		}
		r.initGame(nbPlayers, initLines)
	case "GET_GAME_INFO":
		r.getGameInfo()
	case "SET_PLAYER_OUTPUT":
		output := make([]string, nbLines)
		for i := range output {
			if output[i], err = s.NextLine(); err != nil {
				return err
			}
		}
		if r.currentPlayer == nil {
			return errors.New("SET_PLAYER_OUTPUT before any GET_GAME_INFO")
		}
		return r.setPlayerOutput(output)
	case "SET_PLAYER_TIMEOUT":
		if r.currentPlayer == nil {
			return errors.New("SET_PLAYER_TIMEOUT before any GET_GAME_INFO")
		}
		r.setPlayerTimeout()
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
	return nil
}

// getGameInfo is Referee.getGameInfo: end the round if every active player
// has moved, pick the next player, and print the view, infos, summary and
// either the next player's input or the scores.
func (r *Referee) getGameInfo() {
	if r.end && r.reasonCode == lostParsingReasonCode {
		var inputError strings.Builder
		for _, l := range r.initLines {
			inputError.WriteString(l + "\n")
		}
		dumpResponse(r.out, "VIEW", "KEY_FRAME -1", r.reasonCode)
		dumpResponse(r.out, "INFOS", "¤RED¤"+r.reason+"§RED§", inputError.String())
		r.dumpScores()
		return
	}
	if r.nbIntermediate == r.nbCurrentPlayers {
		r.nbIntermediate = 0
		r.updateScores()
		r.nbCurrentPlayers = 0
		for _, p := range r.players {
			if !p.lost {
				r.nbCurrentPlayers++
			}
		}
		r.nbGlobalRounds++
	}

	key := r.nbIntermediate == 0
	frame := "INTERMEDIATE_FRAME " + strconv.Itoa(r.nbRounds)
	if key {
		frame = "KEY_FRAME " + strconv.Itoa(r.nbRounds)
	}
	scores := r.nbCurrentPlayers == 0 || r.nbGlobalRounds == maxRounds
	lastPlayerID := r.currentPlayerID

	if r.nbCurrentPlayers != 0 {
		for {
			r.currentPlayerID = (r.currentPlayerID + 1) % r.nbPlayers
			if !r.players[r.currentPlayerID].lost {
				break
			}
		}
		r.currentPlayer = r.players[r.currentPlayerID]
	}

	params := r.buildPlayerInput()
	viewParams := r.buildViewParams()
	infos := r.buildPlayerInfo(lastPlayerID)
	var summary []string
	if key {
		summary = r.buildSummary()
	}
	dumpSummary := func() {
		if summary != nil {
			dumpResponse(r.out, "SUMMARY", summary...)
		}
	}
	id := strconv.Itoa(r.currentPlayerID)
	drones := strconv.Itoa(r.nbDronesPerPlayer)

	switch {
	case r.nbGlobalRounds == 0:
		if r.nbRounds == 0 {
			dumpResponse(r.out, "VIEW", frame, "GOD", r.buildInitView(), r.buildInitInput(-1), viewParams)
		} else {
			dumpResponse(r.out, "VIEW", frame, viewParams)
		}
		dumpResponse(r.out, "INFOS", infos...)
		dumpSummary()
		dumpResponse(r.out, "NEXT_PLAYER_INPUT", r.buildInitInput(r.currentPlayerID), params)
		dumpResponse(r.out, "NEXT_PLAYER_INFO", id, drones, "1000")
	case scores:
		dumpResponse(r.out, "VIEW", frame, viewParams)
		dumpResponse(r.out, "INFOS", infos...)
		dumpSummary()
		r.dumpScores()
	default:
		dumpResponse(r.out, "VIEW", frame, viewParams)
		dumpResponse(r.out, "INFOS", infos...)
		dumpSummary()
		dumpResponse(r.out, "NEXT_PLAYER_INFO", id, drones, "150")
		dumpResponse(r.out, "NEXT_PLAYER_INPUT", params)
	}
	if key {
		r.nbRounds++
	}
}

// dumpScores prints UINPUT (the parameters that replay this game) and
// SCORES. After an init error, every score is 0 and there is no UINPUT.
func (r *Referee) dumpScores() {
	parseError := r.reasonCode == lostParsingReasonCode
	scores := make([]string, r.nbPlayers)
	for i := range scores {
		score := 0
		if !parseError {
			score = r.players[i].score
		}
		scores[i] = strconv.Itoa(i) + " " + strconv.Itoa(score)
	}
	if !parseError {
		dumpResponse(r.out, "UINPUT", r.saveUserInput)
	}
	dumpResponse(r.out, "SCORES", scores...)
	r.finished = true
}
