package gameofdrones_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adamadair/codingame-referees/internal/games/gameofdrones"
	"github.com/adamadair/codingame-referees/internal/harness"
	"github.com/adamadair/codingame-referees/internal/jrandom"
)

// javaClasses holds the compiled Java referee, or "" when Java or the
// sources are unavailable.
var javaClasses string

func TestMain(m *testing.M) {
	code, cleanup := setupJava()
	if code == 0 {
		code = m.Run()
	}
	cleanup()
	os.Exit(code)
}

// setupJava compiles the official referee once. Its crash log path is
// hard-coded to /tmp/referee.log, and it "crashes" at the end of every game
// when its input closes, so the path is redirected to a temp folder first.
// Nothing else in the source is changed.
func setupJava() (int, func()) {
	src, ok := harness.JavaRefereesDir()
	if !ok || !harness.JavaAvailable() {
		return 0, func() {}
	}
	data, err := os.ReadFile(filepath.Join(src, "Game of Drones", "Referee.java"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, func() {}
	}
	dir, err := os.MkdirTemp("", "gameofdrones-java")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, func() {}
	}
	cleanup := func() { os.RemoveAll(dir) }
	const logPath = `"/tmp/referee.log"`
	if strings.Count(string(data), logPath) != 1 {
		fmt.Fprintln(os.Stderr, "expected exactly one", logPath, "in the Java referee")
		return 1, cleanup
	}
	patched := strings.Replace(string(data), logPath, strconv.Quote(filepath.Join(dir, "referee.log")), 1)
	file := filepath.Join(dir, "Referee.java")
	if err := os.WriteFile(file, []byte(patched), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, cleanup
	}
	classes := filepath.Join(dir, "classes")
	if err := harness.CompileJava(context.Background(), classes, file); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, cleanup
	}
	javaClasses = classes
	return 0, cleanup
}

func requireJava(t *testing.T) {
	t.Helper()
	if javaClasses == "" {
		t.Skipf("java/javac or the Java referees are unavailable (set %s)", harness.JavaRefereesEnv)
	}
}

// droneBot plays one player. It reads the zones on its first turn and then
// picks targets with its strategy. script overrides a turn: "oob", "nan",
// "space", "double", "extra" or "timeout".
type droneBot struct {
	strategy string // "nearest", "spread" or "random"
	script   map[int]string
	seed     int64

	turn            int
	players, id     int
	drones, nbZones int
	zones           [][2]int
	rand            *jrandom.Random
}

func (b *droneBot) Play(input []string, expected int) []string {
	defer func() { b.turn++ }()
	if b.turn == 0 {
		f := strings.Fields(input[0])
		b.players, _ = strconv.Atoi(f[0])
		b.id, _ = strconv.Atoi(f[1])
		b.drones, _ = strconv.Atoi(f[2])
		b.nbZones, _ = strconv.Atoi(f[3])
		for i := range b.nbZones {
			var x, y int
			fmt.Sscanf(input[1+i], "%d %d", &x, &y)
			b.zones = append(b.zones, [2]int{x, y})
		}
		b.rand = jrandom.New(b.seed*7919 + int64(b.id))
	}
	// The last players*drones lines are every drone's position, ours at
	// index id*drones.
	positions := input[len(input)-b.players*b.drones:]

	out := make([]string, expected)
	for d := range out {
		var x, y int
		fmt.Sscanf(positions[b.id*b.drones+d], "%d %d", &x, &y)
		tx, ty := b.target(d, x, y)
		out[d] = fmt.Sprintf("%d %d", tx, ty)
	}
	switch b.script[b.turn] {
	case "oob":
		out[expected-1] = "4000 10"
	case "nan":
		out[0] = "abc def"
	case "space":
		out[0] = " " + out[0]
	case "double":
		out[0] = strings.Replace(out[0], " ", "  ", 1)
	case "extra":
		for d := range out {
			out[d] += " going places"
		}
	case "timeout":
		return nil
	}
	return out
}

func (b *droneBot) target(d, x, y int) (int, int) {
	switch b.strategy {
	case "spread":
		z := b.zones[(d+b.id)%b.nbZones]
		return z[0], z[1]
	case "random":
		if b.rand.NextIntN(4) == 0 {
			return x, y
		}
		return int(b.rand.NextIntN(4000)), int(b.rand.NextIntN(1800))
	default: // nearest
		best, bestDist := 0, -1
		for i, z := range b.zones {
			dist := (z[0]-x)*(z[0]-x) + (z[1]-y)*(z[1]-y)
			if bestDist < 0 || dist < bestDist {
				best, bestDist = i, dist
			}
		}
		return b.zones[best][0], b.zones[best][1]
	}
}

type scenario struct {
	name    string
	params  []string
	bots    []string         // strategy per player
	scripts []map[int]string // per player, may be shorter than bots
}

func (s scenario) config(seed int64) harness.Config {
	bots := make([]harness.Bot, len(s.bots))
	for i, strategy := range s.bots {
		var script map[int]string
		if i < len(s.scripts) {
			script = s.scripts[i]
		}
		bots[i] = &droneBot{strategy: strategy, script: script, seed: seed}
	}
	return harness.Config{PlayerCount: len(s.bots), Params: s.params, Bots: bots}
}

func params(zones, drones int, seed int64) []string {
	return []string{"nbZones=" + strconv.Itoa(zones), "nbDrones=" + strconv.Itoa(drones), "gameSeed=" + strconv.FormatInt(seed, 10)}
}

func scenarios() []scenario {
	strategies := [][]string{
		{"nearest", "spread"},
		{"random", "nearest", "spread"},
		{"spread", "random", "nearest", "random"},
	}
	var out []scenario
	for i := range 21 {
		bots := strategies[i%3]
		seed := int64(i*104729 - 50000)
		zones := 1 + i%8
		drones := 1 + (i*7)%20
		out = append(out, scenario{
			name:   fmt.Sprintf("sweep %d: %dp %dz %dd seed %d", i, len(bots), zones, drones, seed),
			params: params(zones, drones, seed),
			bots:   bots,
		})
	}
	out = append(out,
		scenario{name: "max zones and drones", params: params(8, 20, 99), bots: []string{"random", "spread", "nearest", "random"}},
		scenario{name: "min zones and drones", params: params(1, 1, 98), bots: []string{"nearest", "nearest"}},
		scenario{name: "keys are ignored", params: []string{"foo=3", "bar=4", "baz=5"}, bots: []string{"nearest", "spread"}},
		scenario{name: "extra parameter lines ignored", params: append(params(3, 4, 5), "nbZones=8"), bots: []string{"nearest", "spread"}},
		scenario{name: "value after second =", params: []string{"a=3=9", "b=4", "c=5"}, bots: []string{"nearest", "spread"}},
		scenario{name: "zones out of range", params: params(9, 4, 1), bots: []string{"nearest", "spread"}},
		scenario{name: "drones out of range", params: params(4, 0, 1), bots: []string{"nearest", "spread"}},
		scenario{name: "missing =", params: []string{"nbZones", "nbDrones=4", "gameSeed=1"}, bots: []string{"nearest", "spread"}},
		scenario{name: "empty value", params: []string{"nbZones=", "nbDrones=4", "gameSeed=1"}, bots: []string{"nearest", "spread"}},
		scenario{name: "only two lines", params: []string{"nbZones=3", "nbDrones=4"}, bots: []string{"nearest", "spread"}},
		scenario{name: "seed not a number", params: []string{"nbZones=3", "nbDrones=4", "gameSeed=x"}, bots: []string{"nearest", "spread"}},
		scenario{name: "arabic digits", params: []string{"nbZones=٣", "nbDrones=٤", "gameSeed=٧"}, bots: []string{"nearest", "spread"}},
		scenario{name: "mistakes", params: params(4, 5, 7), bots: []string{"nearest", "spread", "random", "nearest"},
			scripts: []map[int]string{{3: "oob"}, {5: "nan"}, {8: "space"}, {10: "double"}}},
		scenario{name: "extra tokens are fine", params: params(4, 5, 8), bots: []string{"nearest", "spread"},
			scripts: []map[int]string{{2: "extra", 3: "extra"}}},
		scenario{name: "timeout", params: params(4, 5, 9), bots: []string{"nearest", "spread", "random"},
			scripts: []map[int]string{nil, {4: "timeout"}}},
		scenario{name: "everyone lost", params: params(4, 5, 10), bots: []string{"nearest", "spread", "random"},
			scripts: []map[int]string{{6: "timeout"}, {6: "nan"}, {7: "oob"}}},
		scenario{name: "one player left keeps playing", params: params(3, 3, 11), bots: []string{"nearest", "spread"},
			scripts: []map[int]string{{1: "oob"}}},
	)
	return out
}

func goReferee(clock int64) func(io.Reader, io.Writer, io.Writer) {
	return func(in io.Reader, out, errOut io.Writer) {
		r := &gameofdrones.Referee{Clock: func() int64 { return clock }}
		if err := r.Run(in, out); err != nil {
			fmt.Fprintln(errOut, err)
		}
	}
}

// TestMatchesJava plays every scenario on the Java referee and on the port
// and requires the same game.
func TestMatchesJava(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	for i, s := range scenarios() {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			seed := int64(i)
			want, err := harness.RunCommand(ctx, harness.JavaCommand(ctx, javaClasses), s.config(seed))
			if err != nil {
				t.Fatalf("java: %v\nstderr:\n%s", err, want.Stderr)
			}
			got, err := harness.RunFunc(ctx, goReferee(0), s.config(seed))
			if err != nil {
				t.Fatalf("go: %v\nstderr:\n%s", err, got.Stderr)
			}
			if err := harness.Compare(want, got); err != nil {
				t.Fatal(err)
			}
			if got.Stderr != "" {
				t.Errorf("go referee wrote to stderr:\n%s", got.Stderr)
			}
			t.Logf("%d events, scores %q", len(want.Events), want.Scores())
		})
	}
}

// TestReplayFromUINPUT runs the port without parameters, so it picks the
// map from its clock, then replays the UINPUT it printed on the Java
// referee. Both must play the same game.
func TestReplayFromUINPUT(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	for _, clock := range []int64{1732482000000, 1, 987654321} {
		t.Run(strconv.FormatInt(clock, 10), func(t *testing.T) {
			t.Parallel()
			s := scenario{bots: []string{"nearest", "spread", "random"}}
			got, err := harness.RunFunc(ctx, goReferee(clock), s.config(clock))
			if err != nil {
				t.Fatalf("go: %v\nstderr:\n%s", err, got.Stderr)
			}
			var uinput []string
			for _, b := range got.Blocks {
				if b.Command == "UINPUT" {
					uinput = b.Lines // raw order: the referee reads them by position
				}
			}
			if len(uinput) != 3 || !strings.HasPrefix(uinput[2], "gameSeed="+strconv.FormatInt(clock, 10)) {
				t.Fatalf("unexpected UINPUT %q", uinput)
			}

			s.params = uinput
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

// TestFullOutputMatchesJava compares every output line, view blocks
// included, for a few scenarios. The drone angles in the view are the only
// floating-point output, so this also checks the Float.toString port.
func TestFullOutputMatchesJava(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	all := scenarios()
	for _, i := range []int{0, 1, 2, 21, 33, 35, 36} {
		s := all[i]
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			want, err := harness.RunCommand(ctx, harness.JavaCommand(ctx, javaClasses), s.config(1))
			if err != nil {
				t.Fatal(err)
			}
			got, err := harness.RunFunc(ctx, goReferee(0), s.config(1))
			if err != nil {
				t.Fatal(err)
			}
			if len(want.Blocks) != len(got.Blocks) {
				t.Fatalf("want %d blocks, got %d", len(want.Blocks), len(got.Blocks))
			}
			for i := range want.Blocks {
				w, g := want.Blocks[i], got.Blocks[i]
				if w.Command != g.Command || strings.Join(w.Lines, "\n") != strings.Join(g.Lines, "\n") {
					t.Fatalf("block %d differs:\nwant [%s] %q\ngot  [%s] %q", i, w.Command, w.Lines, g.Command, g.Lines)
				}
			}
		})
	}
}
