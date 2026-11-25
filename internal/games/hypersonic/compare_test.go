package hypersonic

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adamadair/codingame-referees/internal/harness"
	"github.com/adamadair/codingame-referees/internal/jrandom"
)

// javaClasses holds the compiled official referee, or "" when Java or the
// sources are unavailable.
var javaClasses string

func TestMain(m *testing.M) {
	code := 0
	cleanup := func() {}
	if src, ok := harness.JavaRefereesDir(); ok && harness.JavaAvailable() {
		dir, err := os.MkdirTemp("", "hypersonic-java")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cleanup = func() { os.RemoveAll(dir) }
		if err := harness.CompileJava(context.Background(), dir, filepath.Join(src, "Hyper Sonic", "Referee.java")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		} else {
			javaClasses = dir
		}
	}
	if code == 0 {
		code = m.Run()
	}
	cleanup()
	os.Exit(code)
}

func requireJava(t *testing.T) {
	t.Helper()
	if javaClasses == "" {
		t.Skipf("java/javac or the Java referees are unavailable (set %s)", harness.JavaRefereesEnv)
	}
}

// bomberBot plays one player. It reads the grid and entities each turn and
// answers with its strategy, unless the script overrides the turn with a
// literal line or "timeout".
type bomberBot struct {
	strategy string // "bomber", "runner" or "random"
	seed     int64
	script   map[int]string

	turn          int
	width, height int
	id            int
	rand          *jrandom.Random
	flee          *Coord
}

type botEntity struct{ typ, owner, x, y, p1, p2 int }

func (b *bomberBot) Play(input []string, expected int) []string {
	defer func() { b.turn++ }()
	if b.turn == 0 {
		f := strings.Fields(input[0])
		b.width, _ = strconv.Atoi(f[0])
		b.height, _ = strconv.Atoi(f[1])
		b.id, _ = strconv.Atoi(f[2])
		b.rand = jrandom.New(b.seed)
		input = input[1:]
	}
	if s, ok := b.script[b.turn]; ok {
		if s == "timeout" {
			return nil
		}
		return []string{s}
	}
	rows := input[:b.height]
	var me botEntity
	var bombs []botEntity
	for _, l := range input[b.height+1:] {
		v := make([]int, 6)
		for i, f := range strings.Fields(l) {
			v[i], _ = strconv.Atoi(f)
		}
		e := botEntity{v[0], v[1], v[2], v[3], v[4], v[5]}
		if e.typ == entityPlayer && e.owner == b.id {
			me = e
		}
		if e.typ == entityBomb {
			bombs = append(bombs, e)
		}
	}
	return []string{b.command(rows, me, bombs)}
}

func (b *bomberBot) cell(rows []string, x, y int) byte {
	if y < 0 || y >= len(rows) || x < 0 || x >= len(rows[y]) {
		return 'X'
	}
	return rows[y][x]
}

// threatened reports whether a bomb lies in the same row or column within
// its range (ignoring walls, to keep the bot simple).
func threatened(x, y int, bombs []botEntity) bool {
	for _, bomb := range bombs {
		if (bomb.x == x && abs(bomb.y-y) < bomb.p2) || (bomb.y == y && abs(bomb.x-x) < bomb.p2) {
			return true
		}
	}
	return false
}

func (b *bomberBot) command(rows []string, me botEntity, bombs []botEntity) string {
	switch b.strategy {
	case "random":
		x := int(b.rand.NextIntN(int32(b.width+6))) - 3
		y := int(b.rand.NextIntN(int32(b.height+6))) - 3
		if b.rand.NextIntN(3) == 0 {
			return fmt.Sprintf("BOMB %d %d", x, y)
		}
		return fmt.Sprintf("MOVE %d %d rnd", x, y)
	case "runner":
		corner := [4]Coord{{0, 0}, {b.width - 1, b.height - 1}, {b.width - 1, 0}, {0, b.height - 1}}[(b.turn/6)%4]
		if b.turn%6 == 0 && me.p1 > 0 {
			return fmt.Sprintf("BOMB %d %d", corner.x, corner.y)
		}
		return fmt.Sprintf("MOVE %d %d", corner.x, corner.y)
	default: // bomber
		if b.flee != nil && threatened(me.x, me.y, bombs) {
			return fmt.Sprintf("MOVE %d %d", b.flee.x, b.flee.y)
		}
		b.flee = nil
		nextToBox := false
		for _, d := range directions {
			if c := b.cell(rows, me.x+d.getX(), me.y+d.getY()); c >= '0' && c <= '9' {
				nextToBox = true
			}
		}
		if nextToBox && me.p1 > 0 {
			// Run to the nearest floor cell out of this bomb's lines.
			best, bestDist := Coord{me.x, me.y}, 1<<30
			for y, row := range rows {
				for x := range row {
					if row[x] != '.' || x == me.x || y == me.y {
						continue
					}
					if d := abs(x-me.x) + abs(y-me.y); d < bestDist {
						best, bestDist = Coord{x, y}, d
					}
				}
			}
			b.flee = &best
			return fmt.Sprintf("BOMB %d %d boom", best.x, best.y)
		}
		// Walk next to the nearest box.
		best, bestDist := Coord{me.x, me.y}, 1<<30
		for y, row := range rows {
			for x := range row {
				if row[x] < '0' || row[x] > '9' {
					continue
				}
				for _, d := range directions {
					nx, ny := x+d.getX(), y+d.getY()
					if b.cell(rows, nx, ny) != '.' {
						continue
					}
					if dist := abs(nx-me.x) + abs(ny-me.y); dist < bestDist {
						best, bestDist = Coord{nx, ny}, dist
					}
				}
			}
		}
		return fmt.Sprintf("MOVE %d %d", best.x, best.y)
	}
}

type scenario struct {
	name    string
	params  []string
	bots    []string
	scripts []map[int]string
}

func (s scenario) config(seed int64) harness.Config {
	bots := make([]harness.Bot, len(s.bots))
	for i, strategy := range s.bots {
		var script map[int]string
		if i < len(s.scripts) {
			script = s.scripts[i]
		}
		bots[i] = &bomberBot{strategy: strategy, seed: seed*31 + int64(i), script: script}
	}
	return harness.Config{PlayerCount: len(s.bots), Params: s.params, Bots: bots}
}

var edgeLines = []string{
	"move 3 4", "Bomb\t1\t1", "MOVE 2\x0b3", "BOMB 1 1 ", "MOVE 1 1  spaced out", "ſomething", "MOVE 1 1 " + strings.Repeat("🙂", 12),
	"MOVE 99999999999 1", "BOMB 1 -99999999999", "MOVE 1", "BOMB", "MOVE  1  1", "MOVE -0 -0", "WAIT", "move 1 1 x",
}

func scenarios() []scenario {
	groups := [][]string{
		{"bomber", "bomber"}, {"bomber", "runner", "random"}, {"bomber", "bomber", "runner", "bomber"},
		{"random", "bomber"}, {"bomber", "random", "bomber", "runner"},
	}
	var out []scenario
	for i := range 25 {
		out = append(out, scenario{
			name:   fmt.Sprintf("sweep %d", i),
			params: []string{"seed=" + strconv.Itoa(i*7919-30000)},
			bots:   groups[i%len(groups)],
		})
	}
	four := []string{"bomber", "bomber", "bomber", "runner"}
	two := []string{"bomber", "runner"}
	for v := 1; v <= 4; v++ {
		out = append(out, scenario{name: fmt.Sprintf("version %d", v), params: []string{"seed=17", "GAME_VERSION=" + strconv.Itoa(v)}, bots: four})
	}
	out = append(out,
		scenario{name: "wider grid", params: []string{"seed=18", "WIDTH=17", "HEIGHT=15"}, bots: four},
		scenario{name: "even grid", params: []string{"seed=18", "WIDTH=14", "HEIGHT=12"}, bots: two},
		scenario{name: "smaller grid", params: []string{"seed=19", "WIDTH=7", "HEIGHT=5"}, bots: two},
		scenario{name: "few boxes", params: []string{"seed=20", "boxes=4"}, bots: two},
		scenario{name: "many boxes", params: []string{"seed=20", "boxes=200"}, bots: four},
		scenario{name: "short fuse and big blasts", params: []string{"seed=21", "BOMB_FUSE=2", "DEFAULT_BOMB_POWER=9", "DEFAULT_BOMBS_PER_PLAYER=4"}, bots: four},
		scenario{name: "no bombs", params: []string{"seed=22", "DEFAULT_BOMBS_PER_PLAYER=0"}, bots: two},
		scenario{name: "bad seed", params: []string{"seed=abc"}, bots: two},
		scenario{name: "bad version", params: []string{"seed=1", "GAME_VERSION=x"}, bots: two},
		scenario{name: "bad width", params: []string{"seed=1", "WIDTH=wide"}, bots: two},
		scenario{name: "zero width", params: []string{"seed=1", "WIDTH=0"}, bots: two},
		scenario{name: "one player", params: []string{"seed=23"}, bots: []string{"bomber"}},
		scenario{name: "five players", params: []string{"seed=24"}, bots: []string{"bomber", "bomber", "bomber", "bomber", "bomber"}},
		scenario{name: "invalid output", params: []string{"seed=25"}, bots: four, scripts: []map[int]string{nil, {7: "JUMP"}}},
		scenario{name: "timeout", params: []string{"seed=26"}, bots: four, scripts: []map[int]string{nil, nil, {5: "timeout"}}},
	)
	for i, line := range edgeLines {
		out = append(out, scenario{
			name:    fmt.Sprintf("edge line %d %q", i, line),
			params:  []string{"seed=" + strconv.Itoa(100+i)},
			bots:    two,
			scripts: []map[int]string{{2: line, 3: line}},
		})
	}
	return out
}

// statsGame counts turns where several items popped at once and Java's
// HashSet order put them in a different order than they were destroyed:
// the case jcollections exists for.
type statsGame struct {
	*Game
	reordered *atomic.Int64
}

func (s statsGame) UpdateGame(round int) error {
	err := s.Game.UpdateGame(round)
	var destroyed []*Cell
	for _, group := range s.explosionGroups {
		for _, e := range group {
			for _, c := range e.flammables {
				if s.explodedBoxes.Contains(c) && !slices.Contains(destroyed, c) {
					destroyed = append(destroyed, c)
				}
			}
		}
	}
	popped := func(cells []*Cell) []*Cell {
		var out []*Cell
		for _, c := range cells {
			if slices.ContainsFunc(s.activeItems, func(i *Item) bool { return i.position == c.Coord }) {
				out = append(out, c)
			}
		}
		return out
	}
	if a, b := popped(destroyed), popped(s.explodedBoxes.All()); len(a) > 1 && !slices.Equal(a, b) {
		s.reordered.Add(1)
	}
	return err
}

func goReferee(clock int64, reordered *atomic.Int64) func(io.Reader, io.Writer, io.Writer) {
	return func(in io.Reader, out, errOut io.Writer) {
		g := statsGame{&Game{Clock: func() int64 { return clock }}, reordered}
		if err := g.Game.runAs(g, in, out); err != nil {
			fmt.Fprintln(errOut, err)
		}
	}
}

// compare requires the same game, or, when the Java referee stopped
// without scores, the same events and an error from the port.
func compare(want, got *harness.Transcript) error {
	if want.Finished {
		return harness.Compare(want, got)
	}
	if got.Finished {
		return fmt.Errorf("java stopped without scores but go finished\njava stderr:\n%s", want.Stderr)
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
	if got.Stderr == "" {
		return fmt.Errorf("go stopped without scores but reported no error")
	}
	return nil
}

// TestMatchesJava plays every scenario on the official referee and on the
// port and requires the same game.
func TestMatchesJava(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	var reordered atomic.Int64
	t.Run("scenarios", func(t *testing.T) {
		for i, s := range scenarios() {
			t.Run(s.name, func(t *testing.T) {
				t.Parallel()
				want, _ := harness.RunCommand(ctx, harness.JavaCommand(ctx, javaClasses), s.config(int64(i)))
				got, _ := harness.RunFunc(ctx, goReferee(0, &reordered), s.config(int64(i)))
				if err := compare(want, got); err != nil {
					t.Fatal(err)
				}
				t.Logf("finished=%v, %d events, scores %q", want.Finished, len(want.Events), want.Scores())
			})
		}
	})
	if reordered.Load() == 0 {
		t.Error("no turn exercised the Java HashSet item order; the scenarios need more chain reactions")
	}
	t.Logf("%d turns popped items in Java HashSet order rather than destruction order", reordered.Load())
}

// TestReplayFromUINPUT runs the port without a seed, so the map comes from
// its clock, then replays its UINPUT on the official referee.
func TestReplayFromUINPUT(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	var reordered atomic.Int64
	for _, clock := range []int64{1732482000000, 1, 987654321} {
		t.Run(strconv.FormatInt(clock, 10), func(t *testing.T) {
			t.Parallel()
			s := scenario{bots: []string{"bomber", "bomber", "runner"}}
			got, err := harness.RunFunc(ctx, goReferee(clock, &reordered), s.config(clock))
			if err != nil {
				t.Fatalf("go: %v\nstderr:\n%s", err, got.Stderr)
			}
			uinput := got.Events[len(got.Events)-2]
			if uinput.Kind != "UINPUT" || len(uinput.Lines) != 2 {
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

// TestFullOutputMatchesJava compares every block for some scenarios.
// TOOLTIP and UINPUT blocks are compared as sets. VIEW blocks are compared
// as multisets of lines with bomb IDs masked, because Java orders
// simultaneous explosions and removed items by identity hash, and the
// order explosions happen in decides which bomb a player gets back.
func TestFullOutputMatchesJava(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	var reordered atomic.Int64
	picked := []string{"sweep 0", "sweep 1", "sweep 2", "sweep 3", "version 1", "version 2", "version 3",
		"short fuse and big blasts", "invalid output", "timeout", "one player", "five players", "bad width"}
	for i, s := range scenarios() {
		if !slices.Contains(picked, s.name) {
			continue
		}
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			want, _ := harness.RunCommand(ctx, harness.JavaCommand(ctx, javaClasses), s.config(int64(i)))
			got, _ := harness.RunFunc(ctx, goReferee(0, &reordered), s.config(int64(i)))
			if len(want.Blocks) != len(got.Blocks) {
				t.Fatalf("want %d blocks, got %d", len(want.Blocks), len(got.Blocks))
			}
			for k := range want.Blocks {
				w, g := want.Blocks[k], got.Blocks[k]
				wl, gl := w.Lines, g.Lines
				switch w.Command {
				case "TOOLTIP":
					wl, gl = pairs(wl), pairs(gl)
				case "UINPUT":
					wl, gl = sorted(wl), sorted(gl)
				case "VIEW":
					wl, gl = maskView(wl), maskView(gl)
				}
				if w.Command != g.Command || !slices.Equal(wl, gl) {
					t.Fatalf("block %d differs:\nwant [%s] %q\ngot  [%s] %q", k, w.Command, w.Lines, g.Command, g.Lines)
				}
			}
		})
	}
}

func sorted(lines []string) []string {
	s := slices.Clone(lines)
	slices.Sort(s)
	return s
}

func pairs(lines []string) []string {
	var items []string
	for i := 0; i+1 < len(lines); i += 2 {
		items = append(items, lines[i]+"\x00"+lines[i+1])
	}
	return sorted(items)
}

// maskView hides bomb IDs: the 5th field of a bomb line (x y power fuse id
// owner) and the 7th of an explosion line (x y left right up down id).
func maskView(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		f := strings.Fields(l)
		allNumbers := len(f) > 0 && !slices.ContainsFunc(f, func(s string) bool { _, err := strconv.Atoi(s); return err != nil })
		switch {
		case allNumbers && len(f) == 6:
			f[4] = "#"
		case allNumbers && len(f) == 7:
			f[6] = "#"
		}
		out[i] = strings.Join(f, " ")
	}
	return sorted(out)
}
