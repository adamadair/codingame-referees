package cgref_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adamadair/codingame-referees/internal/cgref"
	"github.com/adamadair/codingame-referees/internal/harness"
)

// frameworkVariants pairs a Java framework copy (taken from an official
// referee) with the Options that should reproduce it.
var frameworkVariants = []struct {
	source string
	opts   cgref.Options
}{
	{"Coders of the Caribbean", cgref.Options{}},
	{"Coders Strike Back", cgref.Options{UpdateScoresEachRound: true, InputsOnDemand: true}},
}

// buildToyJava writes ToyGame.java followed by the framework copy from the
// given referee, compiles it and returns the class folder.
func buildToyJava(t *testing.T, src, game string) string {
	t.Helper()
	toy, err := os.ReadFile("testdata/ToyGame.java")
	if err != nil {
		t.Fatal(err)
	}
	framework, err := extractFramework(filepath.Join(src, game, "Referee.java"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "Referee.java")
	if err := os.WriteFile(file, append(toy, framework...), 0o644); err != nil {
		t.Fatal(err)
	}
	classes := filepath.Join(dir, "classes")
	if err := harness.CompileJava(context.Background(), classes, file); err != nil {
		t.Fatal(err)
	}
	return classes
}

// extractFramework returns the referee source from "abstract class
// MultiReferee" to the end of the file.
func extractFramework(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	i := strings.Index(string(data), "abstract class MultiReferee")
	if i < 0 {
		return nil, fmt.Errorf("%s: no MultiReferee", path)
	}
	return data[i:], nil
}

// toyBot is a deterministic bot for the toy game. Each turn it looks at the
// last three input lines ("round lastTarget", scores, guesses seen) and
// answers according to its script for that turn.
type toyBot struct {
	player int
	turn   int
	script map[int]string // turn -> special answer; "" means timeout
}

func (b *toyBot) Play(input []string, _ int) []string {
	defer func() { b.turn++ }()
	if s, ok := b.script[b.turn]; ok {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	last := input[len(input)-3:]
	round, target := 0, 0
	fmt.Sscanf(last[0], "%d %d", &round, &target)
	seen := 0
	for _, f := range strings.Fields(last[2]) {
		if v, err := strconv.Atoi(f); err == nil {
			seen += v
		}
	}
	guess := (target*7 + round*13 + b.player*29 + seen) % 101
	if guess < 0 {
		guess = -guess
	}
	return []string{"GUESS " + strconv.Itoa(guess)}
}

type scenario struct {
	name    string
	players int
	params  []string
	scripts []map[int]string // per player
}

var scenarios = []scenario{
	{name: "plain", players: 2, params: []string{"seed=1"}},
	{name: "three players", players: 3, params: []string{"seed=99", "rounds=40", "winScore=8"}},
	{name: "turn based", players: 3, params: []string{"seed=5", "turnBased=true", "winScore=6"}},
	{name: "max rounds", players: 2, params: []string{"seed=-77", "rounds=6"}},
	{name: "out of range dies", players: 3, params: []string{"seed=3", "winScore=20"},
		scripts: []map[int]string{nil, {2: "GUESS 150"}, nil}},
	{name: "quit", players: 2, params: []string{"seed=11"},
		scripts: []map[int]string{{3: "QUIT"}, nil}},
	{name: "quit turn based", players: 2, params: []string{"seed=11", "turnBased=true"},
		scripts: []map[int]string{{3: "QUIT"}, nil}},
	{name: "invalid input", players: 3, params: []string{"seed=12"},
		scripts: []map[int]string{nil, {1: "HELLO"}, nil}},
	{name: "early win is invalid", players: 2, params: []string{"seed=13"},
		scripts: []map[int]string{{0: "WIN"}, nil}},
	{name: "win", players: 3, params: []string{"seed=21", "winScore=50", "rounds=60"},
		scripts: []map[int]string{{40: "WIN"}, nil, {45: "WIN"}}},
	{name: "timeout", players: 3, params: []string{"seed=14"},
		scripts: []map[int]string{nil, nil, {2: ""}}},
	{name: "timeout turn based", players: 2, params: []string{"seed=14", "turnBased=true"},
		scripts: []map[int]string{{2: ""}, nil}},
	{name: "guess with message", players: 2, params: []string{"seed=15"},
		scripts: []map[int]string{{1: "guess 50 hello there"}, nil}},
	{name: "not enough players", players: 1, params: []string{"seed=16"}},
	{name: "bad parameter", players: 2, params: []string{"seed=1", "rounds=abc"}},
	{name: "malformed escape", players: 2, params: []string{"seed=1", "x=\\u12"}},
	{name: "separator variants", players: 2, params: []string{"seed : 8", "rounds 12", "# comment"}},
}

func (s scenario) config() harness.Config {
	bots := make([]harness.Bot, s.players)
	for i := range bots {
		var script map[int]string
		if i < len(s.scripts) {
			script = s.scripts[i]
		}
		bots[i] = &toyBot{player: i, script: script}
	}
	return harness.Config{PlayerCount: s.players, Params: s.params, Bots: bots}
}

func goToy(opts cgref.Options) func(io.Reader, io.Writer, io.Writer) {
	return func(in io.Reader, out, errOut io.Writer) {
		if err := cgref.Run(newToy(), in, out, errOut, opts); err != nil {
			fmt.Fprintln(errOut, err)
		}
	}
}

func requireJava(t *testing.T) string {
	t.Helper()
	if !harness.JavaAvailable() {
		t.Skip("java/javac not on PATH")
	}
	src, ok := harness.JavaRefereesDir()
	if !ok {
		t.Skipf("Java referees not found; set %s", harness.JavaRefereesEnv)
	}
	return src
}

// TestToyMatchesJava plays every scenario on the Java toy referee and on
// the Go port, for each framework variant, and requires the same game.
func TestToyMatchesJava(t *testing.T) {
	src := requireJava(t)
	ctx := context.Background()
	for _, v := range frameworkVariants {
		t.Run(v.source, func(t *testing.T) {
			classes := buildToyJava(t, src, v.source)
			for _, s := range scenarios {
				t.Run(s.name, func(t *testing.T) {
					want, err := harness.RunCommand(ctx, harness.JavaCommand(ctx, classes), s.config())
					if err != nil {
						t.Fatalf("java: %v\nstderr:\n%s", err, want.Stderr)
					}
					got, err := harness.RunFunc(ctx, goToy(v.opts), s.config())
					if err != nil {
						t.Fatalf("go: %v\nstderr:\n%s", err, got.Stderr)
					}
					if err := harness.Compare(want, got); err != nil {
						t.Fatal(err)
					}
					if testing.Verbose() {
						t.Logf("%d events, scores %q", len(want.Events), want.Scores())
					}
				})
			}
		})
	}
}

// TestOptionsMatter checks that the comparison would catch a wrong Options
// choice: the older framework variant must not match the default options.
func TestOptionsMatter(t *testing.T) {
	src := requireJava(t)
	ctx := context.Background()
	classes := buildToyJava(t, src, "Coders Strike Back")
	s := scenarios[1]
	want, err := harness.RunCommand(ctx, harness.JavaCommand(ctx, classes), s.config())
	if err != nil {
		t.Fatal(err)
	}
	got, err := harness.RunFunc(ctx, goToy(cgref.Options{}), s.config())
	if err != nil {
		t.Fatal(err)
	}
	if harness.Compare(want, got) == nil {
		t.Fatal("default options matched the Coders Strike Back framework; the comparison is not sensitive enough")
	}
}

// TestFullOutputMatchesJava compares every output line, view blocks
// included, for the newest framework copy. Only the order of UINPUT keys
// may differ, since Java prints them in Hashtable order.
func TestFullOutputMatchesJava(t *testing.T) {
	src := requireJava(t)
	ctx := context.Background()
	classes := buildToyJava(t, src, "Coders of the Caribbean")
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			want, err := harness.RunCommand(ctx, harness.JavaCommand(ctx, classes), s.config())
			if err != nil {
				t.Fatal(err)
			}
			got, err := harness.RunFunc(ctx, goToy(cgref.Options{}), s.config())
			if err != nil {
				t.Fatal(err)
			}
			w, g := flatten(want.Blocks), flatten(got.Blocks)
			for i := range min(len(w), len(g)) {
				if w[i] != g[i] {
					t.Fatalf("line %d differs:\n want %q\n got  %q", i, w[i], g[i])
				}
			}
			if len(w) != len(g) {
				t.Fatalf("want %d lines, got %d", len(w), len(g))
			}
		})
	}
}

func flatten(blocks []harness.Block) []string {
	var out []string
	for _, b := range blocks {
		lines := b.Lines
		if b.Command == "UINPUT" {
			lines = sortedCopy(lines)
		}
		out = append(out, "["+b.Command+"]")
		out = append(out, lines...)
	}
	return out
}

func sortedCopy(lines []string) []string {
	s := append([]string(nil), lines...)
	sc := bufio.NewScanner(strings.NewReader(strings.Join(s, "\n")))
	s = s[:0]
	for sc.Scan() {
		s = append(s, sc.Text())
	}
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return s
}
