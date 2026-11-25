package cgref

import (
	"fmt"
	"strconv"
	"strings"
)

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
