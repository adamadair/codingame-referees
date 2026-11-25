// Package jregex helps write Go regular expressions that accept exactly
// the lines a referee's Java Pattern accepts.
//
// The differences that matter for referee commands:
//
//   - Java's Matcher.matches() must match the whole input: wrap with
//     Matches, which anchors with ^...$.
//   - Java's CASE_INSENSITIVE (without UNICODE_CASE) folds ASCII letters
//     only. Go's (?i) also accepts 'ſ' for 's' and the Kelvin sign for 'k',
//     so spell keywords with Fold instead.
//   - Java's \s includes the vertical tab \x0B and Go's doesn't: use Space.
//   - Java's . excludes line terminators (\n, \r, \u0085,  ,  ).
//     The runner's lines are split on exactly those characters, so a line
//     can't contain them and Go's . is equivalent.
//   - Java's \d is ASCII [0-9] unless UNICODE_CHARACTER_CLASS is set, the
//     same as Go's \d.
package jregex

import (
	"regexp"
	"strings"
	"unicode"
)

// Space is Java's \s: [ \t\n\x0B\f\r].
const Space = `[ \t\n\x0B\f\r]`

// Fold spells a keyword so that it matches with ASCII-only case folding,
// like Java's CASE_INSENSITIVE: "MOVE" becomes "[Mm][Oo][Vv][Ee]".
func Fold(keyword string) string {
	var b strings.Builder
	for _, r := range keyword {
		if r < unicode.MaxASCII && unicode.IsLetter(r) {
			b.WriteString("[" + string(unicode.ToUpper(r)) + string(unicode.ToLower(r)) + "]")
		} else {
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	return b.String()
}

// Matches compiles a pattern that must match the whole line, like
// Matcher.matches().
func Matches(pattern string) *regexp.Regexp {
	return regexp.MustCompile(`^(?:` + pattern + `)$`)
}
