package harness

import (
	"fmt"
	"slices"
	"strings"
)

// Compare checks that two transcripts describe the same game: every player
// was given the same input, answered the same way, and the game ended with
// the same parameters and scores. View-only blocks (VIEW, INFOS, SUMMARY,
// TOOLTIP) are not compared.
//
// It returns nil when the games match. Otherwise the error describes the
// first difference, with the events leading up to it.
func Compare(want, got *Transcript) error {
	if !want.Finished || !got.Finished {
		return fmt.Errorf("unfinished game (want finished=%v, got finished=%v)\nwant stderr:\n%s\ngot stderr:\n%s",
			want.Finished, got.Finished, want.Stderr, got.Stderr)
	}
	n := min(len(want.Events), len(got.Events))
	for i := range n {
		w, g := want.Events[i], got.Events[i]
		if w.Kind == g.Kind && w.Player == g.Player && slices.Equal(w.Lines, g.Lines) {
			continue
		}
		return fmt.Errorf("games differ at event %d\n%s\nwant: %s\ngot:  %s%s",
			i, precedingEvents(want.Events, i), w, g, lineDiff(w.Lines, g.Lines))
	}
	if len(want.Events) != len(got.Events) {
		return fmt.Errorf("games differ in length: want %d events, got %d\n%s",
			len(want.Events), len(got.Events), precedingEvents(want.Events, n))
	}
	return nil
}

// precedingEvents shows the few events before index i.
func precedingEvents(events []Event, i int) string {
	var b strings.Builder
	b.WriteString("preceding events:\n")
	for j := max(0, i-4); j < i; j++ {
		fmt.Fprintf(&b, "  [%d] %s\n", j, events[j])
	}
	return b.String()
}

// lineDiff points at the first differing line of two blocks.
func lineDiff(want, got []string) string {
	n := min(len(want), len(got))
	for i := range n {
		if want[i] != got[i] {
			return fmt.Sprintf("\nfirst different line %d:\n  want %q\n  got  %q", i, want[i], got[i])
		}
	}
	if len(want) != len(got) {
		return fmt.Sprintf("\nline counts differ: want %d, got %d", len(want), len(got))
	}
	return ""
}
