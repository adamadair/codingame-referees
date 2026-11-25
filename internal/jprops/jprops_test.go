package jprops

import (
	"bufio"
	"encoding/hex"
	"os"
	"strings"
	"testing"
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
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

func unhex(t *testing.T, s string) string {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(s, "x"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMatchesJava(t *testing.T) {
	inputs := readLines(t, "testdata/inputs.txt")

	// Group expected output by case.
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
		in := unhex(t, h)
		p := New()
		err := p.Load(in)

		var got []string
		if err != nil {
			got = append(got, "error "+err.Error())
		}
		for _, k := range p.Keys() {
			v, _ := p.Get(k)
			got = append(got, "x"+hex.EncodeToString([]byte(k))+" x"+hex.EncodeToString([]byte(v)))
		}

		want := expected[i]
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("case %d %q:\n got  %v\n want %v", i, in, decode(t, got), decode(t, want))
		}
	}
}

// decode turns "xKEY xVALUE" lines back into readable text for failures.
func decode(t *testing.T, lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.HasPrefix(l, "error ") {
			out[i] = l
			continue
		}
		k, v, _ := strings.Cut(l, " ")
		out[i] = unhex(t, k) + "=" + unhex(t, v)
	}
	return out
}

func TestGetPropertyAndOverride(t *testing.T) {
	p := New()
	if err := p.Load("seed=1"); err != nil {
		t.Fatal(err)
	}
	if err := p.Load("seed=2\nmap=a b"); err != nil {
		t.Fatal(err)
	}
	if got := p.GetProperty("seed", "x"); got != "2" {
		t.Errorf("seed = %q, want 2", got)
	}
	if got := p.GetProperty("missing", "def"); got != "def" {
		t.Errorf("missing = %q, want def", got)
	}
	if p.Len() != 2 {
		t.Errorf("Len = %d, want 2", p.Len())
	}
}
