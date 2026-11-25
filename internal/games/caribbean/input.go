package caribbean

import (
	"regexp"

	"github.com/adamadair/codingame-referees/internal/jregex"
)

// The player command patterns, translated from the Java ones with jregex so
// that they accept exactly the same lines.
const message = `(?:` + jregex.Space + `+(.+))?`

func command(keyword string) *regexp.Regexp {
	return jregex.Matches(jregex.Fold(keyword) + message)
}

func commandXY(keyword string) *regexp.Regexp {
	return jregex.Matches(jregex.Fold(keyword) + ` (-?[0-9]{1,8})` + jregex.Space + `+(-?[0-9]{1,8})` + message)
}

var (
	playerInputMovePattern      = commandXY("MOVE")
	playerInputSlowerPattern    = command("SLOWER")
	playerInputFasterPattern    = command("FASTER")
	playerInputWaitPattern      = command("WAIT")
	playerInputPortPattern      = command("PORT")
	playerInputStarboardPattern = command("STARBOARD")
	playerInputFirePattern      = commandXY("FIRE")
	playerInputMinePattern      = command("MINE")
)
