package caribbean

import (
	"regexp"
	"strings"
	"unicode"
)

// The player command patterns, translated from the Java ones so that they
// accept exactly the same lines:
//
//   - Java calls matches(), so the whole line must match: ^...$.
//   - Java's CASE_INSENSITIVE folds ASCII letters only. Go's (?i) would
//     also accept 'ſ' for 's' and the Kelvin sign for 'k', so each keyword
//     letter is spelled out as [Xx] instead.
//   - Java's \s includes the vertical tab \x0B and Go's does not, so
//     whitespace is the explicit class [ \t\n\x0B\f\r].
//   - Java's . excludes line terminators (\n, \r, \u0085,  ,  ).
//     The runner's lines are split on exactly those characters, so a line
//     can't contain them and Go's . is equivalent here.
const ws = `[ \t\n\x0B\f\r]`

// message is the optional free text after a command.
const message = `(?:` + ws + `+(.+))?`

// fold spells a keyword case-insensitively for ASCII letters only.
func fold(keyword string) string {
	var b strings.Builder
	for _, r := range keyword {
		b.WriteString("[" + string(unicode.ToUpper(r)) + string(unicode.ToLower(r)) + "]")
	}
	return b.String()
}

func command(keyword string) *regexp.Regexp {
	return regexp.MustCompile(`^` + fold(keyword) + message + `$`)
}

func commandXY(keyword string) *regexp.Regexp {
	return regexp.MustCompile(`^` + fold(keyword) + ` (-?[0-9]{1,8})` + ws + `+(-?[0-9]{1,8})` + message + `$`)
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
