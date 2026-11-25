// Package jstrings reproduces Java string helpers whose edge cases differ
// from Go's: String.split with a literal separator, Integer.parseInt and
// Long.parseLong.
package jstrings

import (
	"errors"
	"math"
	"strings"
	"unicode"
)

// ErrNumberFormat matches Java's NumberFormatException.
var ErrNumberFormat = errors.New("java.lang.NumberFormatException")

// Split behaves like Java's s.split(sep) for a separator with no regex
// meaning (such as "=" or " "). Unlike strings.Split, trailing empty strings
// are removed, so "a=" gives ["a"] and "=" gives []. A string without the
// separator, including "", gives [s].
func Split(s, sep string) []string {
	if !strings.Contains(s, sep) {
		return []string{s}
	}
	parts := strings.Split(s, sep)
	n := len(parts)
	for n > 0 && parts[n-1] == "" {
		n--
	}
	return parts[:n]
}

// ParseInt behaves like Integer.parseInt(s): an optional sign followed by
// decimal digits, which may be any Unicode decimal digit in the Basic
// Multilingual Plane (Java reads UTF-16 code units). No spaces,
// underscores or prefixes are allowed, and overflow is an error.
func ParseInt(s string) (int32, error) {
	v, err := parse(s, math.MinInt32, math.MaxInt32)
	return int32(v), err
}

// ParseLong behaves like Long.parseLong(s).
func ParseLong(s string) (int64, error) {
	return parse(s, math.MinInt64, math.MaxInt64)
}

// parse ports Java's algorithm, which accumulates negatively so that the
// minimum value parses without overflow.
func parse(s string, minVal, maxVal int64) (int64, error) {
	runes := []rune(s)
	if len(runes) == 0 {
		return 0, ErrNumberFormat
	}
	i := 0
	negative := false
	limit := -maxVal
	if runes[0] < '0' {
		switch runes[0] {
		case '-':
			negative = true
			limit = minVal
		case '+':
		default:
			return 0, ErrNumberFormat
		}
		if len(runes) == 1 {
			return 0, ErrNumberFormat
		}
		i++
	}
	multmin := limit / 10
	var result int64
	for ; i < len(runes); i++ {
		d := digit(runes[i])
		if d < 0 || result < multmin {
			return 0, ErrNumberFormat
		}
		result *= 10
		if result < limit+int64(d) {
			return 0, ErrNumberFormat
		}
		result -= int64(d)
	}
	if negative {
		return result, nil
	}
	return -result, nil
}

// digit is Character.digit(c, 10) for a UTF-16 code unit: the value of a
// Unicode decimal digit (category Nd), or -1.
func digit(c rune) int {
	if c > 0xFFFF || !unicode.IsDigit(c) {
		return -1
	}
	if c <= '9' {
		return int(c - '0')
	}
	// Decimal digits come in runs of ten starting at zero.
	for _, r := range unicode.Nd.R16 {
		if uint16(c) >= r.Lo && uint16(c) <= r.Hi {
			return int(uint16(c)-r.Lo) % 10
		}
	}
	return -1
}
