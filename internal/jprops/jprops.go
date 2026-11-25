// Package jprops replicates the parsing rules of java.util.Properties.load.
//
// The CodinGame referees receive their game parameters (seed, map, ...) as
// lines of Java properties text, so parsing has to match Java exactly: the
// same separators, whitespace handling, comments, line continuations and
// escape sequences.
package jprops

import (
	"errors"
	"sort"
	"unicode/utf16"
)

// ErrMalformedUnicode matches the IllegalArgumentException Java throws for a
// bad \uXXXX escape.
var ErrMalformedUnicode = errors.New("Malformed \\uxxxx encoding.")

// Properties is a string-to-string map with Java Properties semantics.
//
// Java's Properties iterates in Hashtable order. That order isn't reproduced
// here: Keys returns keys sorted instead.
type Properties struct {
	m map[string]string
}

// New returns an empty Properties.
func New() *Properties { return &Properties{m: map[string]string{}} }

// Get returns the value for key and whether it was present.
func (p *Properties) Get(key string) (string, bool) {
	v, ok := p.m[key]
	return v, ok
}

// GetProperty mirrors Properties.getProperty(key, defaultValue).
func (p *Properties) GetProperty(key, def string) string {
	if v, ok := p.m[key]; ok {
		return v
	}
	return def
}

// Set mirrors Properties.setProperty.
func (p *Properties) Set(key, value string) { p.m[key] = value }

// Len returns the number of entries.
func (p *Properties) Len() int { return len(p.m) }

// Keys returns all keys in sorted order.
func (p *Properties) Keys() []string {
	keys := make([]string, 0, len(p.m))
	for k := range p.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Load parses text the way Properties.load(new StringReader(text)) does and
// adds the entries to p, replacing existing keys. On error, entries parsed
// before the bad line are kept, as in Java.
func (p *Properties) Load(text string) error {
	lr := lineReader{buf: utf16.Encode([]rune(text))}
	for {
		line, ok := lr.readLine()
		if !ok {
			return nil
		}
		key, value, err := splitLine(line)
		if err != nil {
			return err
		}
		p.m[key] = value
	}
}

// lineReader is a port of Properties.LineReader. It works on UTF-16 code
// units like Java does, so that \u escapes that form surrogate pairs decode
// the same way.
type lineReader struct {
	buf []uint16
	pos int
}

func isBlank(c uint16) bool { return c == ' ' || c == '\t' || c == '\f' }

// readLine returns the next logical line with leading whitespace, comments
// and continuations already handled. ok is false at end of input.
func (r *lineReader) readLine() (line []uint16, ok bool) {
	var out []uint16
	skipWhiteSpace := true
	appendedLineBegin := false
	isNewLine := true
	isCommentLine := false
	precedingBackslash := false
	skipLF := false

	for {
		if r.pos >= len(r.buf) {
			if len(out) == 0 || isCommentLine {
				return nil, false
			}
			if precedingBackslash {
				out = out[:len(out)-1]
			}
			return out, true
		}
		c := r.buf[r.pos]
		r.pos++

		if skipLF {
			skipLF = false
			if c == '\n' {
				continue
			}
		}
		if skipWhiteSpace {
			if isBlank(c) {
				continue
			}
			if !appendedLineBegin && (c == '\r' || c == '\n') {
				continue
			}
			skipWhiteSpace = false
			appendedLineBegin = false
		}
		if isNewLine {
			isNewLine = false
			if c == '#' || c == '!' {
				isCommentLine = true
				continue
			}
		}

		if c != '\n' && c != '\r' {
			out = append(out, c)
			if c == '\\' {
				precedingBackslash = !precedingBackslash
			} else {
				precedingBackslash = false
			}
			continue
		}

		// End of a natural line.
		if isCommentLine || len(out) == 0 {
			isCommentLine = false
			isNewLine = true
			skipWhiteSpace = true
			out = out[:0]
			continue
		}
		if precedingBackslash {
			out = out[:len(out)-1]
			skipWhiteSpace = true
			appendedLineBegin = true
			precedingBackslash = false
			if c == '\r' {
				skipLF = true
			}
			continue
		}
		return out, true
	}
}

// splitLine separates a logical line into key and value, as Properties.load0
// does, and decodes escapes in both.
func splitLine(line []uint16) (key, value string, err error) {
	limit := len(line)
	keyLen := 0
	valueStart := limit
	hasSep := false
	precedingBackslash := false

	for keyLen < limit {
		c := line[keyLen]
		if (c == '=' || c == ':') && !precedingBackslash {
			valueStart = keyLen + 1
			hasSep = true
			break
		} else if isBlank(c) && !precedingBackslash {
			valueStart = keyLen + 1
			break
		}
		if c == '\\' {
			precedingBackslash = !precedingBackslash
		} else {
			precedingBackslash = false
		}
		keyLen++
	}
	for valueStart < limit {
		c := line[valueStart]
		if !isBlank(c) {
			if !hasSep && (c == '=' || c == ':') {
				hasSep = true
			} else {
				break
			}
		}
		valueStart++
	}

	if key, err = loadConvert(line[:keyLen]); err != nil {
		return "", "", err
	}
	if value, err = loadConvert(line[valueStart:]); err != nil {
		return "", "", err
	}
	return key, value, nil
}

// loadConvert decodes backslash escapes, as Properties.loadConvert does.
func loadConvert(in []uint16) (string, error) {
	out := make([]uint16, 0, len(in))
	for i := 0; i < len(in); {
		c := in[i]
		i++
		if c != '\\' || i >= len(in) {
			out = append(out, c)
			continue
		}
		c = in[i]
		i++
		switch c {
		case 'u':
			var v uint16
			for range 4 {
				if i >= len(in) {
					return "", ErrMalformedUnicode
				}
				h := in[i]
				i++
				switch {
				case h >= '0' && h <= '9':
					v = v<<4 + h - '0'
				case h >= 'a' && h <= 'f':
					v = v<<4 + 10 + h - 'a'
				case h >= 'A' && h <= 'F':
					v = v<<4 + 10 + h - 'A'
				default:
					return "", ErrMalformedUnicode
				}
			}
			out = append(out, v)
		case 't':
			out = append(out, '\t')
		case 'r':
			out = append(out, '\r')
		case 'n':
			out = append(out, '\n')
		case 'f':
			out = append(out, '\f')
		default:
			out = append(out, c)
		}
	}
	return string(utf16.Decode(out)), nil
}
