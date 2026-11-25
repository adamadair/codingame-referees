package caribbean

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adamadair/codingame-referees/internal/harness"
	"github.com/adamadair/codingame-referees/internal/jrandom"
)

// pirateBot plays one player. Each turn it reads its ships and the other
// entities and answers one command per ship with its strategy, unless the
// script overrides the turn.
type pirateBot struct {
	strategy string // "barrels", "mover" or "random"
	seed     int64
	// script maps a turn to a special answer: "timeout", "extra" (one line
	// too many) or a literal line used for every ship.
	script map[int]string

	turn int
	rand *jrandom.Random
}

type botEntity struct {
	id, x, y, a1, a2, a3, a4 int
	typ                      string
}

func (b *pirateBot) Play(input []string, expected int) []string {
	defer func() { b.turn++ }()
	if b.rand == nil {
		b.rand = jrandom.New(b.seed)
	}
	if s, ok := b.script[b.turn]; ok {
		switch s {
		case "timeout":
			return nil
		case "extra":
			out := make([]string, expected+1)
			for i := range out {
				out[i] = "WAIT"
			}
			return out
		default:
			out := make([]string, expected)
			for i := range out {
				out[i] = s
			}
			return out
		}
	}

	var mine, enemies, barrels []botEntity
	for _, l := range input[2:] {
		f := strings.Fields(l)
		e := botEntity{typ: f[1]}
		for i, p := range []*int{&e.id, nil, &e.x, &e.y, &e.a1, &e.a2, &e.a3, &e.a4} {
			if p != nil {
				*p, _ = strconv.Atoi(f[i])
			}
		}
		switch {
		case e.typ == "SHIP" && e.a4 == 1:
			mine = append(mine, e)
		case e.typ == "SHIP":
			enemies = append(enemies, e)
		case e.typ == "BARREL":
			barrels = append(barrels, e)
		}
	}

	out := make([]string, expected)
	for i := range out {
		var s botEntity
		if i < len(mine) {
			s = mine[i]
		}
		out[i] = b.command(i, s, enemies, barrels)
	}
	return out
}

func nearest(from botEntity, list []botEntity) (botEntity, bool) {
	best, bestDist := botEntity{}, -1
	for _, e := range list {
		d := (Coord{from.x, from.y}).distanceTo(Coord{e.x, e.y})
		if bestDist < 0 || d < bestDist {
			best, bestDist = e, d
		}
	}
	return best, bestDist >= 0
}

var moverTargets = []Coord{{0, 0}, {22, 20}, {11, 10}, {22, 0}, {0, 20}, {-5, 30}, {11, -3}, {40, 10}, {11, 3}, {5, 10}, {17, 10}}

func (b *pirateBot) command(i int, s botEntity, enemies, barrels []botEntity) string {
	switch b.strategy {
	case "mover":
		t := moverTargets[(b.turn/7+i*3)%len(moverTargets)]
		return fmt.Sprintf("MOVE %d %d", t.x, t.y)
	case "random":
		x := int(b.rand.NextIntN(31)) - 4
		y := int(b.rand.NextIntN(29)) - 4
		switch b.rand.NextIntN(12) {
		case 0:
			return "FASTER"
		case 1:
			return "slower"
		case 2:
			return "PORT turning"
		case 3:
			return "Starboard"
		case 4:
			return "WAIT"
		case 5:
			return "MINE"
		case 6, 7:
			return fmt.Sprintf("FIRE %d %d", x, y)
		case 8:
			return fmt.Sprintf("MOVE %d %d", int(b.rand.NextIntN(199999999))-99999999, int(b.rand.NextIntN(199999999))-99999999)
		default:
			return fmt.Sprintf("move %d %d arr", x, y)
		}
	default: // barrels
		if b.turn%3 == 1 {
			if e, ok := nearest(s, enemies); ok {
				return fmt.Sprintf("FIRE %d %d", e.x, e.y)
			}
		}
		if b.turn%9 == 4 {
			return "MINE"
		}
		if e, ok := nearest(s, barrels); ok {
			return fmt.Sprintf("MOVE %d %d", e.x, e.y)
		}
		if e, ok := nearest(s, enemies); ok {
			return fmt.Sprintf("MOVE %d %d", e.x, e.y)
		}
		return "WAIT"
	}
}

type scenario struct {
	name    string
	params  []string
	bots    []string
	scripts []map[int]string
	crash   bool // both referees stop without scores
}

func (s scenario) config(seed int64) harness.Config {
	bots := make([]harness.Bot, len(s.bots))
	for i, strategy := range s.bots {
		var script map[int]string
		if i < len(s.scripts) {
			script = s.scripts[i]
		}
		bots[i] = &pirateBot{strategy: strategy, seed: seed*31 + int64(i), script: script}
	}
	return harness.Config{PlayerCount: len(s.bots), Params: s.params, Bots: bots}
}

// edgeLines are command lines whose acceptance depends on regex details.
var edgeLines = []string{
	"move 11 10", "Move 3\t4 hi", "MOVE 3\x0b4", "MOVE 3 4\x0bmsg", "ſlower", "FASTER  ", "wait   msg", "PORT",
	"starboard\tx", "MINE", "fire 5 5 boom", "FIRE 123456789 1", "MOVE 1  2", "MOVE  1 2", "SLOWER\x0bmsg",
	"MOVE -0 -00000001", "MOVE 1 2 " + strings.Repeat("🏴‍☠️", 20), "WAIT " + strings.Repeat("a", 49) + "😀 tail",
	"MOVEx", "WAITING", "MINE!", "FIRE 1", "move 99999999 -99999999", "WAIT nbsp", "Kelvin K",
}

func scenarios() []scenario {
	pairs := [][]string{{"barrels", "mover"}, {"random", "barrels"}, {"mover", "random"}, {"barrels", "barrels"}, {"random", "random"}}
	var out []scenario
	for i := range 25 {
		out = append(out, scenario{
			name:   fmt.Sprintf("sweep %d", i),
			params: []string{"seed=" + strconv.Itoa(i*7919-40000)},
			bots:   pairs[i%len(pairs)],
		})
	}
	both := []string{"barrels", "random"}
	out = append(out,
		scenario{name: "1 ship", params: []string{"seed=5", "shipsPerPlayer=1"}, bots: both},
		scenario{name: "2 ships", params: []string{"seed=5", "shipsPerPlayer=2"}, bots: both},
		scenario{name: "3 ships", params: []string{"seed=5", "shipsPerPlayer=3"}, bots: both},
		scenario{name: "fewest mines and barrels", params: []string{"seed=6", "mineCount=5", "barrelCount=10"}, bots: both},
		scenario{name: "most mines and barrels", params: []string{"seed=6", "mineCount=10", "barrelCount=26"}, bots: both},
		scenario{name: "values out of range", params: []string{"seed=7", "shipsPerPlayer=-5", "mineCount=99", "barrelCount=0"}, bots: both},
		scenario{name: "values that wrap", params: []string{"seed=8", "shipsPerPlayer=4294967298", "barrelCount=99999999999"}, bots: both},
		scenario{name: "values not numbers", params: []string{"seed=9", "shipsPerPlayer=two", "mineCount=", "barrelCount=1.5"}, bots: both},
		// Java crashes drawing the init error frame: see Game.minesCreated.
		scenario{name: "one player", params: []string{"seed=10"}, bots: []string{"barrels"}, crash: true},
		scenario{name: "three players", params: []string{"seed=11"}, bots: []string{"barrels", "random", "mover"}},
		scenario{name: "invalid output", params: []string{"seed=12"}, bots: both, scripts: []map[int]string{nil, {5: "DANCE"}}},
		scenario{name: "timeout", params: []string{"seed=13"}, bots: both, scripts: []map[int]string{{4: "timeout"}}},
		scenario{name: "too many lines", params: []string{"seed=14"}, bots: both, scripts: []map[int]string{{3: "extra"}}, crash: true},
	)
	for i, line := range edgeLines {
		out = append(out, scenario{
			name:    fmt.Sprintf("edge line %d %q", i, line),
			params:  []string{"seed=" + strconv.Itoa(100+i)},
			bots:    []string{"barrels", "mover"},
			scripts: []map[int]string{{2: line, 3: line}},
		})
	}
	return out
}

func goReferee(clock int64) func(io.Reader, io.Writer, io.Writer) {
	return func(in io.Reader, out, errOut io.Writer) {
		g := &Game{Clock: func() int64 { return clock }}
		if err := g.Run(in, out); err != nil {
			fmt.Fprintln(errOut, err)
		}
	}
}

// sameEvents compares two games that both crashed (no SCORES).
func sameEvents(want, got *harness.Transcript) error {
	if want.Finished || got.Finished {
		return fmt.Errorf("expected both referees to stop without scores (java finished=%v, go finished=%v)", want.Finished, got.Finished)
	}
	if len(want.Events) != len(got.Events) {
		return fmt.Errorf("java had %d events, go %d", len(want.Events), len(got.Events))
	}
	for i := range want.Events {
		w, g := want.Events[i], got.Events[i]
		if w.Kind != g.Kind || w.Player != g.Player || !slices.Equal(w.Lines, g.Lines) {
			return fmt.Errorf("event %d differs:\nwant %s\ngot  %s", i, w, g)
		}
	}
	return nil
}

// TestMatchesJava plays every scenario on the official referee and on the
// port and requires the same game.
func TestMatchesJava(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	for i, s := range scenarios() {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			seed := int64(i)
			want, err := harness.RunCommand(ctx, harness.JavaCommand(ctx, javaClasses), s.config(seed))
			if err != nil && !s.crash {
				t.Fatalf("java: %v\nstderr:\n%s", err, want.Stderr)
			}
			got, err := harness.RunFunc(ctx, goReferee(0), s.config(seed))
			if err != nil && !s.crash {
				t.Fatalf("go: %v\nstderr:\n%s", err, got.Stderr)
			}
			if s.crash {
				if err := sameEvents(want, got); err != nil {
					t.Fatal(err)
				}
				if got.Stderr == "" {
					t.Error("go referee reported no error for the crash")
				}
				return
			}
			if err := harness.Compare(want, got); err != nil {
				t.Fatal(err)
			}
			t.Logf("%d events, scores %q", len(want.Events), want.Scores())
		})
	}
}

// TestSeedNotANumber checks the fallback to a random seed. Java seeds it
// from the clock, so this can't be compared with Java: instead the port's
// seed must come from its Clock the way Java's comes from
// System.currentTimeMillis.
func TestSeedNotANumber(t *testing.T) {
	const clock = 1700000000000
	s := scenario{params: []string{"seed=abc", "shipsPerPlayer=2"}, bots: []string{"barrels", "random"}}
	got, err := harness.RunFunc(context.Background(), goReferee(clock), s.config(1))
	if err != nil {
		t.Fatalf("go: %v\nstderr:\n%s", err, got.Stderr)
	}
	want := "seed=" + strconv.FormatInt(jrandom.New(clock).NextLong(), 10)
	uinput := got.Events[len(got.Events)-2]
	if !slices.Contains(uinput.Lines, want) || !slices.Contains(uinput.Lines, "shipsPerPlayer=2") {
		t.Errorf("UINPUT %q, want %s and shipsPerPlayer=2", uinput.Lines, want)
	}
}

// TestReplayFromUINPUT runs the port without parameters, so the map comes
// from its clock, then replays its UINPUT on the official referee.
func TestReplayFromUINPUT(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	for _, clock := range []int64{1732482000000, 1, 987654321} {
		t.Run(strconv.FormatInt(clock, 10), func(t *testing.T) {
			t.Parallel()
			s := scenario{bots: []string{"barrels", "random"}}
			got, err := harness.RunFunc(ctx, goReferee(clock), s.config(clock))
			if err != nil {
				t.Fatalf("go: %v\nstderr:\n%s", err, got.Stderr)
			}
			uinput := got.Events[len(got.Events)-2]
			if uinput.Kind != "UINPUT" || len(uinput.Lines) != 4 {
				t.Fatalf("unexpected UINPUT %v", uinput)
			}
			s.params = uinput.Lines
			want, err := harness.RunCommand(ctx, harness.JavaCommand(ctx, javaClasses), s.config(clock))
			if err != nil {
				t.Fatalf("java: %v\nstderr:\n%s", err, want.Stderr)
			}
			if err := harness.Compare(want, got); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestFullOutputMatchesJava compares every block, view included, for some
// scenarios. TOOLTIP blocks are compared as sets of (message, player)
// pairs, since Java prints them in HashSet identity order.
func TestFullOutputMatchesJava(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	picked := []string{"sweep 0", "sweep 1", "sweep 2", "sweep 3", "sweep 4", "3 ships", "three players",
		"invalid output", "timeout", "edge line 16 ", "edge line 17 "}
	for i, s := range scenarios() {
		if !slices.ContainsFunc(picked, func(p string) bool { return s.name == p || strings.HasPrefix(s.name, p) && strings.HasSuffix(p, " ") }) {
			continue
		}
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			want, err := harness.RunCommand(ctx, harness.JavaCommand(ctx, javaClasses), s.config(int64(i)))
			if err != nil {
				t.Fatal(err)
			}
			got, err := harness.RunFunc(ctx, goReferee(0), s.config(int64(i)))
			if err != nil {
				t.Fatal(err)
			}
			if len(want.Blocks) != len(got.Blocks) {
				t.Fatalf("want %d blocks, got %d", len(want.Blocks), len(got.Blocks))
			}
			for k := range want.Blocks {
				w, g := want.Blocks[k], got.Blocks[k]
				wl, gl := w.Lines, g.Lines
				if w.Command == "TOOLTIP" || w.Command == "UINPUT" {
					wl, gl = sortedPairs(w.Command, wl), sortedPairs(g.Command, gl)
				}
				if w.Command != g.Command || !slices.Equal(wl, gl) {
					t.Fatalf("block %d differs:\nwant [%s] %q\ngot  [%s] %q", k, w.Command, w.Lines, g.Command, g.Lines)
				}
			}
		})
	}
}

// sortedPairs sorts a TOOLTIP block's (message, player) pairs, or a
// UINPUT block's lines.
func sortedPairs(command string, lines []string) []string {
	var items []string
	if command == "TOOLTIP" {
		for i := 0; i+1 < len(lines); i += 2 {
			items = append(items, lines[i]+"\x00"+lines[i+1])
		}
	} else {
		items = append(items, lines...)
	}
	slices.Sort(items)
	return items
}
