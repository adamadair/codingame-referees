package main

import (
	"strings"
	"testing"

	"github.com/adamadair/codingame-referees/internal/cli"
	"github.com/adamadair/codingame-referees/internal/games/gameofdrones"
)

func TestHelp(t *testing.T) {
	var out, errOut strings.Builder
	code := cli.Run("cg-game-of-drones", usage, gameofdrones.Run, []string{"-h"}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	for _, want := range []string{"Game of Drones", "nbZones=<1-8>", "gameSeed=", "UINPUT", "Exit status", "-l logfile", "cg-brutaltester"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("help text lacks %q", want)
		}
	}
}

// TestGame plays a game that ends at once (bad parameters give every
// player 0) through the command.
func TestGame(t *testing.T) {
	in := "[[INIT] 4]\n2\nnbZones=99\nnbDrones=3\ngameSeed=42\n[[GET_GAME_INFO] 0]\n"
	var out, errOut strings.Builder
	if code := cli.Run("cg-game-of-drones", usage, gameofdrones.Run, nil, strings.NewReader(in), &out, &errOut); code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "[[SCORES] 2]\n0 0\n1 0\n") {
		t.Errorf("unexpected output:\n%s", out.String())
	}
}
