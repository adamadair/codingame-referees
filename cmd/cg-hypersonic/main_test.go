package main

import (
	"strings"
	"testing"

	"github.com/adamadair/codingame-referees/internal/cli"
	"github.com/adamadair/codingame-referees/internal/games/hypersonic"
)

func TestHelp(t *testing.T) {
	var out, errOut strings.Builder
	code := cli.Run("cg-hypersonic", usage, hypersonic.Run, []string{"-h"}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	for _, want := range []string{"Hypersonic", "2 to 4", "GAME_VERSION=<1-4>", "WIDTH=<n>", "BOMB_FUSE=<n>",
		"13x11", "UINPUT", "Exit status", "-l logfile", "cg-brutaltester"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("help text lacks %q", want)
		}
	}
}

// TestBadSeedExitsWithError checks a bad seed: like Java, the referee
// fails while reporting the init error and prints no scores.
func TestBadSeedExitsWithError(t *testing.T) {
	in := "[[INIT] 2]\n2\nseed=abc\n[[GET_GAME_INFO] 0]\n"
	var out, errOut strings.Builder
	if code := cli.Run("cg-hypersonic", usage, hypersonic.Run, nil, strings.NewReader(in), &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if strings.Contains(out.String(), "[[SCORES]") || !strings.Contains(errOut.String(), "players is null") {
		t.Errorf("stdout %q\nstderr %q", out.String(), errOut.String())
	}
}
