package main

import (
	"strings"
	"testing"

	"github.com/adamadair/codingame-referees/internal/cli"
	"github.com/adamadair/codingame-referees/internal/games/caribbean"
)

func TestHelp(t *testing.T) {
	var out, errOut strings.Builder
	code := cli.Run("cg-coders-of-the-caribbean", usage, caribbean.Run, []string{"-h"}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	for _, want := range []string{"Coders of the Caribbean", "league level 3", "shipsPerPlayer=<1-3>", "mineCount=<5-10>",
		"barrelCount=<10-26>", "seed=", "UINPUT", "Exit status", "-l logfile", "cg-brutaltester"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("help text lacks %q", want)
		}
	}
}

// TestCrashExitsWithError checks a one-player game: like Java, the
// referee fails while reporting the init error and prints no scores.
func TestCrashExitsWithError(t *testing.T) {
	in := "[[INIT] 2]\n1\nseed=1\n[[GET_GAME_INFO] 0]\n"
	var out, errOut strings.Builder
	if code := cli.Run("cg-coders-of-the-caribbean", usage, caribbean.Run, nil, strings.NewReader(in), &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if strings.Contains(out.String(), "[[SCORES]") || !strings.Contains(errOut.String(), "mines is null") {
		t.Errorf("stdout %q\nstderr %q", out.String(), errOut.String())
	}
}
