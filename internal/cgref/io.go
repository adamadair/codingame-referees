package cgref

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// scanner reproduces the two java.util.Scanner calls the framework makes:
// nextLine and nextInt.
type scanner struct {
	r *bufio.Reader
}

var errNoSuchElement = errors.New("java.util.NoSuchElementException")

func newScanner(r io.Reader) *scanner { return &scanner{r: bufio.NewReader(r)} }

// isLineSep matches Scanner's line separators: \n, \r,  ,  , \u0085
// (\r\n is handled by the caller).
func isLineSep(c rune) bool {
	return c == '\n' || c == '\r' || c == ' ' || c == ' ' || c == '\u0085'
}

// nextLine returns the rest of the current line without its terminator. At
// end of input with nothing left, it fails like Scanner.nextLine.
func (s *scanner) nextLine() (string, error) {
	var b strings.Builder
	read := false
	for {
		c, _, err := s.r.ReadRune()
		if err != nil {
			if read {
				return b.String(), nil
			}
			return "", errNoSuchElement
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

// nextInt skips whitespace (including line breaks), reads one token and
// parses it as an int. The rest of the line stays unread.
func (s *scanner) nextInt() (int, error) {
	var b strings.Builder
	for {
		c, _, err := s.r.ReadRune()
		if err != nil {
			if b.Len() == 0 {
				return 0, errNoSuchElement
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

// outputData is AbstractReferee.OutputData: a block of lines under a
// "[[COMMAND] count]" header.
type outputData struct {
	command string
	lines   []string
}

func newOutput(command string) *outputData { return &outputData{command: command} }

func (o *outputData) add(s string) { o.lines = append(o.lines, s) }

func (o *outputData) addAll(lines []string) { o.lines = append(o.lines, lines...) }

// String matches OutputData.toString, including the final trim(). Java's
// trim strips leading and trailing characters <= ' ', so trailing empty
// lines (or trailing spaces on the last line) disappear while the header
// count still includes them. Players see exactly this text, so the quirk
// is kept.
func (o *outputData) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[[%s] %d]\n", o.command, len(o.lines))
	for _, l := range o.lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return javaTrim(b.String())
}

func javaTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return r <= ' ' })
}

// javaFormat approximates String.format for the framework's message
// templates. Java's %s and %d both print an Integer as a number, while Go's
// %s does not, so %s becomes %v here. A nil argument prints as "null", and
// extra arguments are ignored as in Java. Messages only reach the viewer,
// not the players.
func javaFormat(format string, args ...any) string {
	var b strings.Builder
	next := 0
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		// Find the end of the specifier: flags, width, precision, verb.
		j := i + 1
		for j < len(format) && strings.IndexByte("-#+ 0,(123456789.$", format[j]) >= 0 {
			j++
		}
		if j >= len(format) {
			b.WriteString(format[i:])
			break
		}
		verb := format[j]
		spec := format[i+1 : j]
		i = j
		switch verb {
		case '%':
			b.WriteByte('%')
			continue
		case 'n':
			b.WriteByte('\n')
			continue
		}
		// Explicit argument index: %2$s.
		argIdx := next
		if k := strings.IndexByte(spec, '$'); k >= 0 {
			if n, err := strconv.Atoi(spec[:k]); err == nil {
				argIdx = n - 1
			}
			spec = spec[k+1:]
		} else {
			next++
		}
		if argIdx < 0 || argIdx >= len(args) {
			b.WriteString("%!" + string(verb) + "(MISSING)")
			continue
		}
		arg := args[argIdx]
		if arg == nil {
			b.WriteString("null")
			continue
		}
		spec = strings.ReplaceAll(spec, ",", "")
		switch verb {
		case 's', 'S':
			s := fmt.Sprintf("%"+spec+"v", arg)
			if verb == 'S' {
				s = strings.ToUpper(s)
			}
			b.WriteString(s)
		default:
			b.WriteString(fmt.Sprintf("%"+spec+string(verb), arg))
		}
	}
	return b.String()
}
