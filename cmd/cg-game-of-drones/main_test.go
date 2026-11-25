package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	for _, arg := range []string{"-h", "-help", "--help"} {
		var out, errOut strings.Builder
		code := run([]string{arg}, strings.NewReader(""), &out, &errOut)
		if code != 0 {
			t.Errorf("%s: exit %d, want 0", arg, code)
		}
		help := errOut.String()
		for _, want := range []string{"Game of Drones", "nbZones=<1-8>", "gameSeed=", "UINPUT", "-l logfile", "cg-brutaltester"} {
			if !strings.Contains(help, want) {
				t.Errorf("%s: help text lacks %q", arg, want)
			}
		}
	}
}

func TestBadFlag(t *testing.T) {
	var out, errOut strings.Builder
	if code := run([]string{"-x"}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if code := run([]string{"extra"}, strings.NewReader(""), &out, &errOut); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

// TestLogFileOnError checks that -l receives the error when the input ends
// before the game is over, and that earlier entries are kept.
func TestLogFileOnError(t *testing.T) {
	log := filepath.Join(t.TempDir(), "referee.log")
	in := "[[INIT] 4]\n2\nnbZones=4\nnbDrones=3\ngameSeed=42\n[[GET_GAME_INFO] 0]\n"
	for range 2 {
		var out, errOut strings.Builder
		if code := run([]string{"-l", log}, strings.NewReader(in), &out, &errOut); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		if !strings.Contains(errOut.String(), "input ended before the game was over") {
			t.Errorf("stderr = %q", errOut.String())
		}
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "input ended before the game was over"); n != 2 {
		t.Errorf("log has %d entries, want 2:\n%s", n, data)
	}
}

// TestNoLogOnNormalGame plays a game that ends at once (bad parameters
// give every player 0) and checks that no log file appears.
func TestNoLogOnNormalGame(t *testing.T) {
	log := filepath.Join(t.TempDir(), "referee.log")
	in := "[[INIT] 4]\n2\nnbZones=99\nnbDrones=3\ngameSeed=42\n[[GET_GAME_INFO] 0]\n"
	var out, errOut strings.Builder
	if code := run([]string{"-l", log}, strings.NewReader(in), &out, &errOut); code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "[[SCORES] 2]\n0 0\n1 0\n") {
		t.Errorf("unexpected output:\n%s", out.String())
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Errorf("log file was written on a normal game")
	}
}

// TestNoLogFileByDefault checks that an error without -l only reaches
// stderr.
func TestNoLogFileByDefault(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out, errOut strings.Builder
	if code := run(nil, strings.NewReader("[[BOGUS] 0]\n"), &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), `unknown command "BOGUS"`) {
		t.Errorf("stderr = %q", errOut.String())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("files written without -l: %v", entries)
	}
}
