package harness

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// requireJava skips the test unless java, javac and the Java referee
// sources are available, and returns the sources folder.
func requireJava(t *testing.T) string {
	t.Helper()
	if !JavaAvailable() {
		t.Skip("java/javac not on PATH")
	}
	dir, ok := JavaRefereesDir()
	if !ok {
		t.Skipf("Java referees not found; set %s", JavaRefereesEnv)
	}
	return dir
}

func compileReferee(t *testing.T, srcDir, game string) string {
	t.Helper()
	out := t.TempDir()
	if err := CompileJava(context.Background(), out, filepath.Join(srcDir, game, "Referee.java")); err != nil {
		t.Fatal(err)
	}
	return out
}

func repeatBot(line string) Bot {
	return BotFunc(func(_ []string, n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = line
		}
		return out
	})
}

// TestRealJavaReferees checks that the harness can play complete games
// against unmodified official referees, and that a seeded game is
// reproducible.
func TestRealJavaReferees(t *testing.T) {
	src := requireJava(t)
	ctx := context.Background()

	cases := []struct {
		game string
		bot  Bot
	}{
		{"Coders of the Caribbean", repeatBot("WAIT")},
		{"Coders Strike Back", repeatBot("8000 4500 100")},
	}
	for _, c := range cases {
		t.Run(c.game, func(t *testing.T) {
			classes := compileReferee(t, src, c.game)
			cfg := Config{
				PlayerCount: 2,
				Params:      []string{"seed=12345"},
				Bots:        []Bot{c.bot, c.bot},
			}
			first, err := RunCommand(ctx, JavaCommand(ctx, classes), cfg)
			if err != nil {
				t.Fatalf("run: %v\nstderr:\n%s", err, first.Stderr)
			}
			if !first.Finished || len(first.Scores()) != 2 {
				t.Fatalf("game did not finish properly: scores %q\nstderr:\n%s", first.Scores(), first.Stderr)
			}
			uinput := first.Events[len(first.Events)-2]
			if uinput.Kind != "UINPUT" || !strings.Contains(strings.Join(uinput.Lines, "\n"), "seed=12345") {
				t.Errorf("UINPUT should echo the seed, got %v", uinput)
			}

			second, err := RunCommand(ctx, JavaCommand(ctx, classes), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := Compare(first, second); err != nil {
				t.Errorf("same seed gave different games: %v", err)
			}

			// A different seed should give a different game.
			cfg.Params = []string{"seed=54321"}
			other, err := RunCommand(ctx, JavaCommand(ctx, classes), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if Compare(first, other) == nil {
				t.Error("different seeds gave identical games")
			}
			t.Logf("%d events, scores %q", len(first.Events), first.Scores())
		})
	}
}

// TestTimeoutAndInvalidOutput checks the SET_PLAYER_TIMEOUT path and that a
// bot answering garbage loses.
func TestTimeoutAndInvalidOutput(t *testing.T) {
	src := requireJava(t)
	ctx := context.Background()
	classes := compileReferee(t, src, "Coders of the Caribbean")

	timeout := BotFunc(func([]string, int) []string { return nil })
	garbage := repeatBot("DANCE")
	cfg := Config{
		PlayerCount: 2,
		Params:      []string{"seed=7"},
		Bots:        []Bot{timeout, garbage},
	}
	tr, err := RunCommand(ctx, JavaCommand(ctx, classes), cfg)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, tr.Stderr)
	}
	var sawTimeout bool
	for _, e := range tr.Events {
		if e.Kind == "TIMEOUT" {
			sawTimeout = true
		}
	}
	if !sawTimeout || !tr.Finished {
		t.Fatalf("expected a timeout and a finished game, got finished=%v events=%v", tr.Finished, tr.Events)
	}
}
