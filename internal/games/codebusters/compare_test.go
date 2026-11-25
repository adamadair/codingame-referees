package codebusters

import (
	"context"
	"fmt"
	"io"
	"math"
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
		dir, err := os.MkdirTemp("", "codebusters-java")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cleanup = func() { os.RemoveAll(dir) }
		if err := harness.CompileJava(context.Background(), dir, filepath.Join(src, "Code Busters", "puzzle", "Referee.java")); err != nil {
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

// busterBot plays one player. Each turn it reads the visible entities and
// gives every buster a command from its strategy, unless the script
// overrides the turn: "timeout" sends nothing, and "i:line" replaces
// buster i's command.
type busterBot struct {
	strategy string // "hunter", "fighter" or "random"
	seed     int64
	script   map[int]string

	turn                    int
	busters, ghostCount, id int
	rand                    *jrandom.Random
	lastStun                map[int]int
	radarUsed               bool
}

type botEntity struct{ id, x, y, typ, state, value int }

func dist(ax, ay, bx, by int) float64 {
	dx, dy := float64(ax-bx), float64(ay-by)
	return math.Sqrt(dx*dx + dy*dy)
}

func (b *busterBot) Play(input []string, expected int) []string {
	defer func() { b.turn++ }()
	if b.turn == 0 {
		b.busters, _ = strconv.Atoi(input[0])
		b.ghostCount, _ = strconv.Atoi(input[1])
		b.id, _ = strconv.Atoi(input[2])
		b.rand = jrandom.New(b.seed)
		b.lastStun = map[int]int{}
		input = input[3:]
	}
	s, scripted := b.script[b.turn]
	if scripted && s == "timeout" {
		return nil
	}
	var mine, enemies, ghosts []botEntity
	for _, l := range input[1:] {
		v := make([]int, 6)
		for i, f := range strings.Fields(l) {
			v[i], _ = strconv.Atoi(f)
		}
		e := botEntity{v[0], v[1], v[2], v[3], v[4], v[5]}
		switch {
		case e.typ == b.id:
			mine = append(mine, e)
		case e.typ >= 0:
			enemies = append(enemies, e)
		case e.typ == ghostNormal:
			ghosts = append(ghosts, e)
		}
	}
	out := make([]string, len(mine))
	for i, me := range mine {
		out[i] = b.command(i, me, enemies, ghosts)
	}
	if scripted {
		k, line, _ := strings.Cut(s, ":")
		i, _ := strconv.Atoi(k)
		out[i] = line
	}
	return out
}

func (b *busterBot) command(i int, me botEntity, enemies, ghosts []botEntity) string {
	base := corners[b.id]
	bx, by := int(base.x), int(base.y)
	if b.strategy == "random" {
		switch b.rand.NextIntN(7) {
		case 0:
			return fmt.Sprintf("BUST %d", b.rand.NextIntN(int32(b.ghostCount+3)))
		case 1:
			return fmt.Sprintf("STUN %d rnd", b.rand.NextIntN(int32(4*b.busters+2)))
		case 2:
			return "RELEASE"
		case 3:
			return "RADAR"
		case 4:
			return fmt.Sprintf("EJECT %d %d", b.rand.NextIntN(20000)-2000, b.rand.NextIntN(13000)-2000)
		default:
			return fmt.Sprintf("MOVE %d %d", b.rand.NextIntN(20000)-2000, b.rand.NextIntN(13000)-2000)
		}
	}
	fighter := b.strategy == "fighter"
	if fighter && me.state != busterStunned {
		if last, ok := b.lastStun[me.id]; !ok || b.turn-last >= stunCooldown {
			for _, e := range enemies {
				if e.state != busterStunned && dist(me.x, me.y, e.x, e.y) <= stunRange {
					b.lastStun[me.id] = b.turn
					return fmt.Sprintf("STUN %d zap", e.id)
				}
			}
		}
		if !b.radarUsed && b.turn >= 6 && i == 0 {
			b.radarUsed = true
			return "RADAR"
		}
	}
	if me.state == busterCarrying {
		if dist(me.x, me.y, bx, by) <= hqRadius-50 {
			return "RELEASE home"
		}
		if fighter {
			for _, e := range enemies {
				if dist(me.x, me.y, e.x, e.y) <= 2200 {
					return fmt.Sprintf("EJECT %d %d", bx, by)
				}
			}
		}
		return fmt.Sprintf("MOVE %d %d", bx, by)
	}
	var target *botEntity
	best := math.Inf(1)
	for k, g := range ghosts {
		if d := dist(me.x, me.y, g.x, g.y); d < best {
			best, target = d, &ghosts[k]
		}
	}
	if target != nil {
		switch {
		case best >= grabSafetyDistance && best <= grabRange:
			return fmt.Sprintf("BUST %d", target.id)
		case best < grabSafetyDistance:
			return fmt.Sprintf("MOVE %d %d back", 2*me.x-target.x, 2*me.y-target.y)
		}
		return fmt.Sprintf("MOVE %d %d", target.x, target.y)
	}
	// Explore: each buster cycles through its own waypoints.
	wp := (b.turn/12 + i*3 + b.id) % 9
	return fmt.Sprintf("MOVE %d %d", 1500+(wp%3)*6500, 1200+(wp/3)*3300)
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
		bots[i] = &busterBot{strategy: strategy, seed: seed*31 + int64(i), script: script}
	}
	return harness.Config{PlayerCount: len(s.bots), Params: s.params, Bots: bots}
}

var edgeLines = []string{
	"move 3 4", "Bust\t1", "MOVE 2\x0b3", "RELEASE ", "RELEASE  spaced out", "ſtun 1", "radar", "RADARx", "RADAR x",
	"MOVE 1 1 " + strings.Repeat("🙂", 12), "MOVE 99999999999 1", "BUST 99999999999", "STUN 99999999999",
	"STUN -1", "EJECT 99999999999 0", "EJECT 1", "EJECT 0 0 out", "MOVE -0 -0", "WAIT", "BUST 0 hi there",
}

func scenarios() []scenario {
	groups := [][]string{
		{"hunter", "hunter"}, {"fighter", "hunter"}, {"fighter", "fighter"}, {"random", "fighter"}, {"hunter", "random"},
	}
	var out []scenario
	for i := range 30 {
		out = append(out, scenario{
			name:   fmt.Sprintf("sweep %d", i),
			params: []string{"seed=" + strconv.Itoa(i*7919-60000)},
			bots:   groups[i%len(groups)],
		})
	}
	two := []string{"fighter", "hunter"}
	duel := []string{"fighter", "fighter"}
	out = append(out,
		scenario{name: "three players", params: []string{"seed=40"}, bots: []string{"fighter", "hunter", "random"}},
		scenario{name: "four players", params: []string{"seed=41"}, bots: []string{"fighter", "fighter", "hunter", "hunter"}},
		scenario{name: "three players, two time out", params: []string{"seed=42"}, bots: []string{"hunter", "hunter", "hunter"},
			scripts: []map[int]string{nil, {3: "timeout"}, {4: "timeout"}}},
		scenario{name: "four players, one times out", params: []string{"seed=43"}, bots: []string{"hunter", "hunter", "hunter", "fighter"},
			scripts: []map[int]string{nil, nil, {5: "timeout"}}},
		scenario{name: "busters 0", params: []string{"seed=44", "busters=0"}, bots: two},
		scenario{name: "busters 1", params: []string{"seed=44", "busters=1"}, bots: two},
		scenario{name: "busters 5", params: []string{"seed=45", "busters=5"}, bots: duel},
		scenario{name: "busters 9", params: []string{"seed=45", "busters=9"}, bots: two},
		scenario{name: "ghosts 0", params: []string{"seed=46", "ghosts=0"}, bots: two},
		scenario{name: "ghosts 1", params: []string{"seed=46", "ghosts=1", "CENTRAL_GHOST=false"}, bots: two},
		scenario{name: "ghosts 2", params: []string{"seed=46", "ghosts=2"}, bots: two},
		scenario{name: "ghosts 500", params: []string{"seed=47", "ghosts=500"}, bots: duel},
		scenario{name: "no central ghost", params: []string{"seed=48", "CENTRAL_GHOST=false"}, bots: duel},
		scenario{name: "central ghost garbage", params: []string{"seed=48", "CENTRAL_GHOST=yes"}, bots: duel},
		scenario{name: "central ghost TRUE", params: []string{"seed=48", "CENTRAL_GHOST=TRUE"}, bots: duel},
		scenario{name: "speed overflow", params: []string{"seed=49", "BUSTER_SPEED=50000", "BUSTER_SPEED_WHILE_CARRYING=70000"}, bots: duel},
		scenario{name: "eject overflow", params: []string{"seed=50", "EJECT_RANGE=50000"}, bots: duel},
		scenario{name: "negative speeds", params: []string{"seed=51", "BUSTER_SPEED=-800", "GHOST_SPEED=-400"}, bots: two},
		scenario{name: "fast ghosts", params: []string{"seed=52", "GHOST_SPEED=2000", "GHOST_VIEW_RANGE=5000"}, bots: duel},
		scenario{name: "blind", params: []string{"seed=53", "VIEW_RANGE=0"}, bots: duel},
		scenario{name: "far sight", params: []string{"seed=53", "VIEW_RANGE=9000", "BUSTER_SPEED_WHILE_CARRYING=300"}, bots: duel},
		scenario{name: "bad seed", params: []string{"seed=abc"}, bots: two},
		scenario{name: "bad busters", params: []string{"seed=1", "busters=x"}, bots: two},
		scenario{name: "bad ghosts", params: []string{"seed=1", "ghosts=99999999999"}, bots: two},
		scenario{name: "bad speed", params: []string{"seed=1", "BUSTER_SPEED=fast"}, bots: two},
		scenario{name: "bad eject range", params: []string{"seed=1", "EJECT_RANGE=far"}, bots: two},
		scenario{name: "map triple", params: []string{"seed=1", "map=1 2 3"}, bots: two},
		scenario{name: "map six", params: []string{"seed=1", "map=1000 2000 300 4000 5000 600"}, bots: two},
		scenario{name: "map empty", params: []string{"seed=1", "map="}, bots: two},
		scenario{name: "map word", params: []string{"seed=1", "map=x"}, bots: two},
		scenario{name: "one player", params: []string{"seed=54"}, bots: []string{"hunter"}},
		scenario{name: "five players", params: []string{"seed=55"}, bots: []string{"hunter", "hunter", "hunter", "hunter", "hunter"}},
		scenario{name: "invalid first line", params: []string{"seed=56"}, bots: duel, scripts: []map[int]string{nil, {7: "0:JUMP"}}},
		scenario{name: "invalid later line", params: []string{"seed=57", "busters=4"}, bots: duel, scripts: []map[int]string{{9: "2:JUMP"}}},
		scenario{name: "invalid while carrying", params: []string{"seed=58", "busters=3"}, bots: two, scripts: []map[int]string{{40: "0:EJECT 99999999999 1"}}},
		scenario{name: "timeout", params: []string{"seed=59"}, bots: duel, scripts: []map[int]string{nil, {5: "timeout"}}},
		scenario{name: "self stun", params: []string{"seed=60"}, bots: duel, scripts: []map[int]string{{3: "0:STUN 0 ouch", 30: "1:STUN 0"}}},
	)
	for i, line := range edgeLines {
		out = append(out, scenario{
			name:    fmt.Sprintf("edge line %d %q", i, line),
			params:  []string{"seed=" + strconv.Itoa(100+i)},
			bots:    two,
			scripts: []map[int]string{{2: "0:" + line, 3: "1:" + line}},
		})
	}
	return out
}

// stats counts the events the scenarios are meant to exercise.
type stats struct {
	stuns, ejects, radars, captures, grabs, overflowMoves atomic.Int64
}

// statsGame counts events after each update.
type statsGame struct {
	*Game
	s *stats
}

func (s statsGame) UpdateGame(round int) error {
	err := s.Game.UpdateGame(round)
	for _, b := range s.allBusters {
		switch {
		case b.stunSucceeded():
			s.s.stuns.Add(1)
		case b.attempt == actionEject && b.success:
			s.s.ejects.Add(1)
		case b.radarSucceeded():
			s.s.radars.Add(1)
		case b.trappedGhost():
			s.s.grabs.Add(1)
		}
		// A vector longer than the map's diagonal can only come from the
		// branch an int-overflowed speed*speed or range*range sends it to.
		if b.attemptedVector != nil && b.attemptedVector.length() > 20000 {
			s.s.overflowMoves.Add(1)
		}
	}
	for _, g := range s.ghosts {
		if g.isCaptured() {
			s.s.captures.Add(1)
		}
	}
	return err
}

func goReferee(clock int64, st *stats) func(io.Reader, io.Writer, io.Writer) {
	return func(in io.Reader, out, errOut io.Writer) {
		g := statsGame{&Game{Clock: func() int64 { return clock }}, st}
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
		return fmt.Errorf("java had %d events, go %d\njava stderr:\n%s\ngo stderr:\n%s", len(want.Events), len(got.Events), want.Stderr, got.Stderr)
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
	var st stats
	t.Run("scenarios", func(t *testing.T) {
		for i, s := range scenarios() {
			t.Run(s.name, func(t *testing.T) {
				t.Parallel()
				want, _ := harness.RunCommand(ctx, harness.JavaCommand(ctx, javaClasses), s.config(int64(i)))
				got, _ := harness.RunFunc(ctx, goReferee(0, &st), s.config(int64(i)))
				if err := compare(want, got); err != nil {
					t.Fatal(err)
				}
				t.Logf("finished=%v, %d events, scores %q", want.Finished, len(want.Events), want.Scores())
			})
		}
	})
	t.Logf("stuns %d, ejects %d, radars %d, trapped %d, ghost-turns captured %d, overflowed moves %d",
		st.stuns.Load(), st.ejects.Load(), st.radars.Load(), st.grabs.Load(), st.captures.Load(), st.overflowMoves.Load())
	for name, n := range map[string]int64{"stun": st.stuns.Load(), "eject": st.ejects.Load(), "radar": st.radars.Load(),
		"trap": st.grabs.Load(), "capture": st.captures.Load(), "overflowed move": st.overflowMoves.Load()} {
		if n == 0 {
			t.Errorf("no %s happened; the scenarios don't exercise it", name)
		}
	}
}

// TestReplayFromUINPUT runs the port without a seed, so the map comes from
// its clock, then replays its UINPUT on the official referee.
func TestReplayFromUINPUT(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	var st stats
	for _, clock := range []int64{1732482000000, 1, 987654321} {
		t.Run(strconv.FormatInt(clock, 10), func(t *testing.T) {
			t.Parallel()
			s := scenario{bots: []string{"fighter", "hunter"}}
			got, err := harness.RunFunc(ctx, goReferee(clock, &st), s.config(clock))
			if err != nil {
				t.Fatalf("go: %v\nstderr:\n%s", err, got.Stderr)
			}
			uinput := got.Events[len(got.Events)-2]
			if uinput.Kind != "UINPUT" || len(uinput.Lines) != 3 {
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

// TestFullOutputMatchesJava compares every block for some scenarios,
// viewer output included. TOOLTIP blocks are compared as sets, because
// Java keeps tooltips in an identity-hashed HashSet, and UINPUT lines are
// sorted, because Java prints them in Hashtable order.
func TestFullOutputMatchesJava(t *testing.T) {
	requireJava(t)
	ctx := context.Background()
	var st stats
	picked := []string{"sweep 0", "sweep 1", "sweep 2", "sweep 3", "sweep 4", "three players", "four players",
		"busters 5", "ghosts 500", "eject overflow", "invalid first line", "invalid later line", "invalid while carrying",
		"timeout", "one player", "self stun", "bad seed", "map triple"}
	for i, s := range scenarios() {
		if !slices.Contains(picked, s.name) {
			continue
		}
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()
			want, _ := harness.RunCommand(ctx, harness.JavaCommand(ctx, javaClasses), s.config(int64(i)))
			got, _ := harness.RunFunc(ctx, goReferee(0, &st), s.config(int64(i)))
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
				}
				if w.Command != g.Command || !slices.Equal(wl, gl) {
					t.Fatalf("block %d differs:\nwant [%s] %q\ngot  [%s] %q", k, w.Command, w.Lines, g.Command, g.Lines)
				}
			}
		})
	}
}

// TestDefaultSeedReplays checks, without Java, that a game with no seed
// comes from the clock and that its UINPUT replays it.
func TestDefaultSeedReplays(t *testing.T) {
	ctx := context.Background()
	var st stats
	s := scenario{bots: []string{"fighter", "hunter"}}
	first, err := harness.RunFunc(ctx, goReferee(42, &st), s.config(1))
	if err != nil {
		t.Fatal(err)
	}
	s.params = first.Events[len(first.Events)-2].Lines
	again, err := harness.RunFunc(ctx, goReferee(7, &st), s.config(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := harness.Compare(first, again); err != nil {
		t.Fatal(err)
	}
	if want := "seed=" + strconv.FormatInt(jrandom.New(42).NextLong(), 10); !slices.Contains(s.params, want) {
		t.Errorf("UINPUT %q lacks %q", s.params, want)
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
