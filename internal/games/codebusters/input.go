package codebusters

import "github.com/adamadair/codingame-referees/internal/jregex"

// The player command patterns, translated from the Java ones with jregex so
// that they accept exactly the same lines. Java uses unbounded \d+, so a
// number too large for an int is caught later as an invalid action.
const (
	xy      = jregex.Space + `+(-?\d+)` + jregex.Space + `+(-?\d+)`
	id      = jregex.Space + `+(\d+)`
	message = `(?:` + jregex.Space + `+)?(?:` + jregex.Space + `+(.+))?`
)

var (
	playerMovePattern    = jregex.Matches(jregex.Fold("MOVE") + xy + message)
	playerGrabPattern    = jregex.Matches(jregex.Fold("BUST") + id + message)
	playerReleasePattern = jregex.Matches(jregex.Fold("RELEASE") + message)
	playerStunPattern    = jregex.Matches(jregex.Fold("STUN") + id + message)
	playerRadarPattern   = jregex.Matches(jregex.Fold("RADAR") + message)
	playerEjectPattern   = jregex.Matches(jregex.Fold("EJECT") + xy + message)
)

// expected is the InvalidInputException text for the final league, where
// stun, radar and eject are all allowed. The Java source has a
// non-breaking space before RELEASE.
const expected = "MOVE x y | BUST id | RELEASE | STUN id | RADAR | EJECT"
