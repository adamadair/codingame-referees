package jrandom

import (
	"bufio"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

// testdata/java_random.txt is produced by testdata/Gen.java:
//
//	javac -d /tmp testdata/Gen.java && java -cp /tmp Gen > testdata/java_random.txt
//
// For each seed it records a fixed sequence of calls on one java.util.Random.
// Floating-point values are stored as raw IEEE-754 bits (hex) so the
// comparison is bit-exact.

type javaCase struct {
	seed  int64
	lines map[string][]string
}

var callOrder = []string{"int", "int10", "int16", "intbig", "long", "bool", "double", "float", "gauss"}

func loadJavaCases(t *testing.T) []javaCase {
	t.Helper()
	f, err := os.Open("testdata/java_random.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var cases []javaCase
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "seed" {
			s, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			cases = append(cases, javaCase{seed: s, lines: map[string][]string{}})
			continue
		}
		cases[len(cases)-1].lines[fields[0]] = fields[1:]
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return cases
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func mustHex(t *testing.T, s string) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMatchesJava(t *testing.T) {
	cases := loadJavaCases(t)
	if len(cases) == 0 {
		t.Fatal("no test cases loaded")
	}
	for _, c := range cases {
		t.Run(strconv.FormatInt(c.seed, 10), func(t *testing.T) {
			r := New(c.seed)
			// Calls must happen in the same order as Gen.java, since they
			// share one generator.
			for _, kind := range callOrder {
				for i, want := range c.lines[kind] {
					var got, exp string
					switch kind {
					case "int":
						got, exp = strconv.FormatInt(int64(r.NextInt()), 10), want
					case "int10":
						got, exp = strconv.FormatInt(int64(r.NextIntN(10)), 10), want
					case "int16":
						got, exp = strconv.FormatInt(int64(r.NextIntN(16)), 10), want
					case "intbig":
						got, exp = strconv.FormatInt(int64(r.NextIntN(1_500_000_001)), 10), want
					case "long":
						got, exp = strconv.FormatInt(r.NextLong(), 10), strconv.FormatInt(mustInt(t, want), 10)
					case "bool":
						got, exp = strconv.FormatBool(r.NextBoolean()), want
					case "double":
						got = strconv.FormatUint(math.Float64bits(r.NextDouble()), 16)
						exp = strconv.FormatUint(mustHex(t, want), 16)
					case "float":
						got = strconv.FormatUint(uint64(math.Float32bits(r.NextFloat())), 16)
						exp = strconv.FormatUint(mustHex(t, want), 16)
					case "gauss":
						got = strconv.FormatUint(math.Float64bits(r.NextGaussian()), 16)
						exp = strconv.FormatUint(mustHex(t, want), 16)
					}
					if got != exp {
						t.Fatalf("%s[%d]: got %s, want %s", kind, i, got, exp)
					}
				}
			}
		})
	}
}

func TestSetSeedResets(t *testing.T) {
	r := New(7)
	first := r.NextInt()
	r.NextGaussian() // leaves a cached gaussian behind
	r.SetSeed(7)
	if got := r.NextInt(); got != first {
		t.Fatalf("after SetSeed: got %d, want %d", got, first)
	}
}

func TestNextIntNPanicsOnNonPositive(t *testing.T) {
	for _, b := range []int32{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NextIntN(%d) did not panic", b)
				}
			}()
			New(1).NextIntN(b)
		}()
	}
}
