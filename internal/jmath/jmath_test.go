package jmath

import (
	"bufio"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

// testdata/expected.txt is produced by testdata/Gen.java:
//
//	javac -d /tmp testdata/Gen.java && java -cp /tmp Gen > testdata/expected.txt

func bits(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return math.Float64frombits(v)
}

func TestMatchesJava(t *testing.T) {
	f, err := os.Open("testdata/expected.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var atan2s, rounds, failures int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		switch fields[0] {
		case "atan2":
			atan2s++
			y, x := bits(t, fields[1]), bits(t, fields[2])
			want := fields[3]
			if got := strconv.FormatUint(math.Float64bits(Atan2(y, x)), 16); got != want {
				failures++
				if failures <= 20 {
					t.Errorf("Atan2(%v, %v) = %s, want %s", y, x, got, want)
				}
			}
		case "round":
			rounds++
			a := bits(t, fields[1])
			for i, got := range []int64{Round(a), Long(a), int64(Int(a))} {
				want, err := strconv.ParseInt(fields[2+i], 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					failures++
					if failures <= 20 {
						name := []string{"Round", "Long", "Int"}[i]
						t.Errorf("%s(%v) = %d, want %d", name, a, got, want)
					}
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if atan2s < 5000 || rounds < 1000 {
		t.Fatalf("too few cases: %d atan2, %d round", atan2s, rounds)
	}
	if failures > 0 {
		t.Fatalf("%d mismatches", failures)
	}
}

// TestGoAtan2Differs documents why the port exists: Go's math.Atan2 does
// not always match Java.
func TestGoAtan2Differs(t *testing.T) {
	f, err := os.Open("testdata/expected.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	differ := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if fields[0] != "atan2" {
			continue
		}
		y, x := bits(t, fields[1]), bits(t, fields[2])
		if strconv.FormatUint(math.Float64bits(math.Atan2(y, x)), 16) != fields[3] {
			differ++
		}
	}
	t.Logf("Go's math.Atan2 differs from Java on %d inputs", differ)
}
