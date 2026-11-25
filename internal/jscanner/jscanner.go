// Package jscanner reproduces the java.util.Scanner calls the CodinGame
// referees use to read the runner's commands: nextLine and nextInt.
package jscanner

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// ErrNoSuchElement matches Scanner's NoSuchElementException: the input
// ended.
var ErrNoSuchElement = errors.New("java.util.NoSuchElementException")

// Scanner reads lines and integer tokens like java.util.Scanner.
type Scanner struct {
	r *bufio.Reader
}

// New returns a Scanner reading from r.
func New(r io.Reader) *Scanner { return &Scanner{r: bufio.NewReader(r)} }

// isLineSep matches Scanner's line separators: \n, \r,  ,   and
// \u0085 (\r\n is handled by the caller).
func isLineSep(c rune) bool {
	return c == '\n' || c == '\r' || c == ' ' || c == ' ' || c == '\u0085'
}

// NextLine returns the rest of the current line without its terminator. At
// end of input with nothing left, it returns ErrNoSuchElement.
func (s *Scanner) NextLine() (string, error) {
	var b strings.Builder
	read := false
	for {
		c, _, err := s.r.ReadRune()
		if err != nil {
			if read {
				return b.String(), nil
			}
			return "", ErrNoSuchElement
		}
		read = true
		if isLineSep(c) {
			if c == '\r' {
				if n, _, err := s.r.ReadRune(); err == nil && n != '\n' {
					_ = s.r.UnreadRune()
				}
			}
			return b.String(), nil
		}
		b.WriteRune(c)
	}
}

// NextInt skips whitespace (including line breaks), reads one token and
// parses it as a 32-bit int. The rest of the line stays unread.
func (s *Scanner) NextInt() (int, error) {
	var b strings.Builder
	for {
		c, _, err := s.r.ReadRune()
		if err != nil {
			if b.Len() == 0 {
				return 0, ErrNoSuchElement
			}
			break
		}
		if unicode.IsSpace(c) {
			if b.Len() == 0 {
				continue
			}
			_ = s.r.UnreadRune()
			break
		}
		b.WriteRune(c)
	}
	v, err := strconv.ParseInt(b.String(), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("java.util.InputMismatchException: %q", b.String())
	}
	return int(v), nil
}
