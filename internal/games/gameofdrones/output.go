package gameofdrones

import (
	"bufio"
	"math"
	"strconv"
	"strings"
)

// dumpResponse writes one "[[id] n]" block. Each argument may hold several
// lines separated by '\n'. The count includes a final line without '\n'.
// Like the Java version there is no trimming, and an empty argument is an
// error (Java fails on charAt(-1)).
func dumpResponse(w *bufio.Writer, id string, output ...string) {
	nbLines := 0
	for _, s := range output {
		if s == "" {
			panic("java.lang.StringIndexOutOfBoundsException: empty " + id + " line")
		}
		nbLines += strings.Count(s, "\n")
		if !strings.HasSuffix(s, "\n") {
			nbLines++
		}
	}
	w.WriteString("[[" + id + "] " + strconv.Itoa(nbLines) + "]\n")
	for _, s := range output {
		w.WriteString(s)
		if !strings.HasSuffix(s, "\n") {
			w.WriteByte('\n')
		}
	}
	w.Flush()
}

// javaFloatString approximates Float.toString for the drone angles, which
// only reach the viewer: shortest digits, ".0" for whole numbers, and
// computerized scientific notation outside [1e-3, 1e7).
func javaFloatString(f float32) string {
	v := float64(f)
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 0) {
		if v > 0 {
			return "Infinity"
		}
		return "-Infinity"
	}
	if a := math.Abs(v); a == 0 || (a >= 1e-3 && a < 1e7) {
		s := strconv.FormatFloat(v, 'f', -1, 32)
		if !strings.Contains(s, ".") {
			s += ".0"
		}
		return s
	}
	s := strconv.FormatFloat(v, 'e', -1, 32) // e.g. 1.5e-04
	mant, exp, _ := strings.Cut(s, "e")
	if !strings.Contains(mant, ".") {
		mant += ".0"
	}
	e, _ := strconv.Atoi(exp)
	return mant + "E" + strconv.Itoa(e)
}
