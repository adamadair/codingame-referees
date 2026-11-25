package jstrings

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// testdata/expected.txt is produced by testdata/Gen.java from
// testdata/inputs.txt (hex-encoded UTF-8, one input per line):
//
//	javac -d /tmp testdata/Gen.java && java -cp /tmp Gen testdata/inputs.txt > testdata/expected.txt

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

func hexOf(s string) string { return "x" + hex.EncodeToString([]byte(s)) }

func splitLine(parts []string) string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(len(parts)))
	for _, p := range parts {
		b.WriteString(" " + hexOf(p))
	}
	return b.String()
}

func TestMatchesJava(t *testing.T) {
	inputs := readLines(t, "testdata/inputs.txt")
	var expected [][]string
	for _, l := range readLines(t, "testdata/expected.txt") {
		if strings.HasPrefix(l, "case ") {
			expected = append(expected, nil)
			continue
		}
		expected[len(expected)-1] = append(expected[len(expected)-1], l)
	}
	if len(expected) != len(inputs) {
		t.Fatalf("%d inputs but %d expected cases", len(inputs), len(expected))
	}

	for i, h := range inputs {
		b, err := hex.DecodeString(h)
		if err != nil {
			t.Fatal(err)
		}
		in := string(b)

		var got []string
		if v, err := ParseInt(in); err != nil {
			got = append(got, "int error")
		} else {
			got = append(got, fmt.Sprintf("int %d", v))
		}
		if v, err := ParseLong(in); err != nil {
			got = append(got, "long error")
		} else {
			got = append(got, fmt.Sprintf("long %d", v))
		}
		got = append(got, "split= "+splitLine(Split(in, "=")), "split_ "+splitLine(Split(in, " ")))

		for j := range got {
			if got[j] != expected[i][j] {
				t.Errorf("case %d %q: got %q, want %q", i, in, got[j], expected[i][j])
			}
		}
	}
}

// TestDigitRuns checks the assumption behind digit(): every run of Nd code
// points in the Basic Multilingual Plane starts at a zero and has a length
// that is a multiple of ten.
func TestDigitRuns(t *testing.T) {
	for _, r := range unicode.Nd.R16 {
		if r.Stride != 1 || (int(r.Hi)-int(r.Lo)+1)%10 != 0 {
			t.Errorf("unexpected Nd range %#x-%#x stride %d", r.Lo, r.Hi, r.Stride)
		}
	}
}
