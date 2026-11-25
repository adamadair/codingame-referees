package jregex

import "testing"

func TestFoldAndSpace(t *testing.T) {
	re := Matches(Fold("MOVE") + Space + `+(-?\d+)` + `(?:` + Space + `+(.+))?`)
	for line, want := range map[string]bool{
		"MOVE 1":      true,
		"move 1":      true,
		"MoVe\t-2 hi": true,
		"MOVE\x0b3":   true,  // Java's \s includes \x0B
		"MOVE 1 ":     false, // a message needs at least one character after the space
		"xMOVE 1":     false, // matches() needs the whole line
		"MOVE 1x":     false,
		"MOVE ١":      false, // \d is ASCII only
		"ſTOP 1":      false, // ſ is not folded
	} {
		if got := re.MatchString(line); got != want {
			t.Errorf("%q: got %v, want %v", line, got, want)
		}
	}
	if got := Matches(Fold("STOP")).MatchString("ſTOP"); got {
		t.Error("Fold must not accept ſ for s")
	}
	if got := Matches(Fold("WALK")).MatchString("WALK"); got {
		t.Error("Fold must not accept the Kelvin sign for k")
	}
}
