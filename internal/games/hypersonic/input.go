package hypersonic

import "github.com/adamadair/codingame-referees/internal/jregex"

// The player command patterns, translated from the Java ones with jregex so
// that they accept exactly the same lines. Java uses unbounded \d+, so a
// number too large for an int is caught later as an invalid action.
const (
	xy      = jregex.Space + `+(-?\d+)` + jregex.Space + `+(-?\d+)`
	message = `(?:` + jregex.Space + `+)?(?:` + jregex.Space + `+(.+))?`
)

var (
	playerMovePattern = jregex.Matches(jregex.Fold("MOVE") + xy + message)
	playerBombPattern = jregex.Matches(jregex.Fold("BOMB") + xy + message)
)

const expected = "MOVE x y | BOMB x y"
