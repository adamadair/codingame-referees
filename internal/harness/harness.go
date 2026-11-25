// Package harness drives a referee the way the CodinGame runner (or
// cg-brutaltester) does, and records what happened so that a Java referee
// and its Go port can be compared.
//
// The harness sends INIT with the game parameters, then repeats
// GET_GAME_INFO, hands the requested player's input to a Bot, and sends the
// bot's answer back as SET_PLAYER_OUTPUT, or SET_PLAYER_TIMEOUT when the bot
// returns nil. The game ends when the referee prints SCORES.
package harness

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Bot plays one player. Play gets the lines the referee sent this turn and
// the number of lines the referee expects back. It returns nil to simulate
// a timeout. A bot must be deterministic for comparisons to make sense.
type Bot interface {
	Play(input []string, expectedLines int) []string
}

// BotFunc adapts a function to Bot.
type BotFunc func(input []string, expectedLines int) []string

func (f BotFunc) Play(input []string, expectedLines int) []string { return f(input, expectedLines) }

// Config describes one game.
type Config struct {
	PlayerCount int
	// Params are the game parameters, one properties line each, for example
	// "seed=42". They are sent as the INIT lines.
	Params []string
	// Bots has one bot per player.
	Bots []Bot
	// MaxTurns stops a runaway game. 0 means 10000.
	MaxTurns int
	// Timeout bounds the whole game. 0 means 60 seconds.
	Timeout time.Duration
}

// Block is one "[[COMMAND] n]" block of referee output.
type Block struct {
	Command string
	Lines   []string
}

// Event is one step of a game that matters for "same game per seed":
// what a player was told, what it answered, and how the game ended.
type Event struct {
	// Kind is INPUT, INFO, OUTPUT, TIMEOUT, UINPUT or SCORES.
	Kind   string
	Player int // -1 when not about one player
	Lines  []string
}

func (e Event) String() string {
	head := e.Kind
	if e.Player >= 0 {
		head += " player " + strconv.Itoa(e.Player)
	}
	if len(e.Lines) == 0 {
		return head
	}
	return head + ":\n    " + strings.Join(e.Lines, "\n    ")
}

// Transcript is the record of one game.
type Transcript struct {
	Events []Event
	// Blocks is every block the referee printed, including view-only ones.
	Blocks []Block
	// Stderr is whatever the referee printed on its error stream.
	Stderr string
	// Finished is true when the referee printed SCORES.
	Finished bool
}

// Scores returns the final SCORES lines, or nil if the game did not finish.
func (t *Transcript) Scores() []string {
	for i := len(t.Events) - 1; i >= 0; i-- {
		if t.Events[i].Kind == "SCORES" {
			return t.Events[i].Lines
		}
	}
	return nil
}

// RunCommand plays a game against a referee process, such as
// JavaCommand's result. The command must not have been started.
func RunCommand(ctx context.Context, cmd *exec.Cmd, cfg Config) (*Transcript, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &lockedWriter{w: &stderr}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	t, runErr := drive(ctx, stdin, stdout, cfg)
	stdin.Close()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
	t.Stderr = stderr.String()
	return t, runErr
}

// RunFunc plays a game against an in-process referee, such as a Go port
// calling cgref.Run.
func RunFunc(ctx context.Context, referee func(in io.Reader, out, errOut io.Writer), cfg Config) (*Transcript, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	var stderr strings.Builder
	errW := &lockedWriter{w: &stderr}

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer outW.Close()
		// Unblock our writes once the referee stops reading.
		defer inR.CloseWithError(io.ErrClosedPipe)
		defer func() {
			if p := recover(); p != nil {
				fmt.Fprintf(errW, "referee panicked: %v\n", p)
			}
		}()
		referee(inR, outW, errW)
	}()

	t, runErr := drive(ctx, inW, outR, cfg)
	inW.Close()
	outR.CloseWithError(io.ErrClosedPipe)
	<-finished
	t.Stderr = stderr.String()
	return t, runErr
}

func (c Config) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 60 * time.Second
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

var headerPattern = regexp.MustCompile(`^\[\[([A-Z_]+)\] ([0-9]+)\]$`)

// readBlocks parses referee output into blocks and sends them on a channel
// until the output ends. It always drains the output, so the referee never
// blocks on a write.
func readBlocks(r io.Reader) <-chan blockOrErr {
	ch := make(chan blockOrErr, 64)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 1<<20), 1<<26)
		for sc.Scan() {
			line := sc.Text()
			m := headerPattern.FindStringSubmatch(line)
			if m == nil {
				ch <- blockOrErr{err: fmt.Errorf("unexpected referee output %q", line)}
				continue
			}
			n, _ := strconv.Atoi(m[2])
			b := Block{Command: m[1]}
			for len(b.Lines) < n && sc.Scan() {
				b.Lines = append(b.Lines, sc.Text())
			}
			ch <- blockOrErr{block: b}
		}
	}()
	return ch
}

type blockOrErr struct {
	block Block
	err   error
}

func drive(ctx context.Context, in io.Writer, out io.Reader, cfg Config) (*Transcript, error) {
	t := &Transcript{}
	if len(cfg.Bots) != cfg.PlayerCount {
		return t, fmt.Errorf("harness: %d bots for %d players", len(cfg.Bots), cfg.PlayerCount)
	}
	maxTurns := cfg.MaxTurns
	if maxTurns == 0 {
		maxTurns = 10000
	}
	blocks := readBlocks(out)
	defer func() {
		go func() {
			for range blocks { // let the reader finish instead of leaking
			}
		}()
	}()

	// Writes are ignored once the referee has exited: it may end the game
	// right after a SET_PLAYER_OUTPUT, before reading our next command.
	send := func(lines ...string) {
		_, _ = io.WriteString(in, strings.Join(lines, "\n")+"\n")
	}

	// next waits for the referee's answer to GET_GAME_INFO. It returns the
	// player to move, or -1 when the game is over.
	var pendingInput []string
	next := func() (player, expected int, err error) {
		for {
			var be blockOrErr
			var ok bool
			select {
			case be, ok = <-blocks:
			case <-ctx.Done():
				return 0, 0, ctx.Err()
			}
			if !ok {
				return 0, 0, errors.New("referee output ended before SCORES")
			}
			if be.err != nil {
				return 0, 0, be.err
			}
			b := be.block
			t.Blocks = append(t.Blocks, b)
			switch b.Command {
			case "NEXT_PLAYER_INPUT":
				pendingInput = b.Lines
			case "NEXT_PLAYER_INFO":
				if len(b.Lines) < 2 {
					return 0, 0, fmt.Errorf("short NEXT_PLAYER_INFO: %q", b.Lines)
				}
				p, err1 := strconv.Atoi(b.Lines[0])
				e, err2 := strconv.Atoi(b.Lines[1])
				if err := errors.Join(err1, err2); err != nil {
					return 0, 0, fmt.Errorf("bad NEXT_PLAYER_INFO %q: %w", b.Lines, err)
				}
				t.Events = append(t.Events,
					Event{Kind: "INPUT", Player: p, Lines: pendingInput},
					Event{Kind: "INFO", Player: p, Lines: b.Lines})
				return p, e, nil
			case "UINPUT":
				lines := append([]string(nil), b.Lines...)
				sort.Strings(lines) // Java prints these in Hashtable order
				t.Events = append(t.Events, Event{Kind: "UINPUT", Player: -1, Lines: lines})
			case "SCORES":
				t.Events = append(t.Events, Event{Kind: "SCORES", Player: -1, Lines: b.Lines})
				t.Finished = true
				return -1, 0, nil
			}
		}
	}

	init := []string{fmt.Sprintf("[[INIT] %d]", len(cfg.Params)+1), strconv.Itoa(cfg.PlayerCount)}
	send(append(init, cfg.Params...)...)

	for turn := 0; ; turn++ {
		if turn >= maxTurns {
			return t, fmt.Errorf("harness: no SCORES after %d turns", maxTurns)
		}
		send("[[GET_GAME_INFO] 0]")
		p, expected, err := next()
		if err != nil {
			return t, err
		}
		if p < 0 {
			return t, nil
		}
		if p >= len(cfg.Bots) {
			return t, fmt.Errorf("referee asked for player %d of %d", p, len(cfg.Bots))
		}
		output := cfg.Bots[p].Play(pendingInput, expected)
		if output == nil {
			t.Events = append(t.Events, Event{Kind: "TIMEOUT", Player: p})
			send("[[SET_PLAYER_TIMEOUT] 0]")
			continue
		}
		t.Events = append(t.Events, Event{Kind: "OUTPUT", Player: p, Lines: output})
		send(append([]string{fmt.Sprintf("[[SET_PLAYER_OUTPUT] %d]", len(output))}, output...)...)
	}
}
