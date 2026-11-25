package jcollections

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// testdata/expected.txt is produced by testdata/Gen.java:
//
//	javac -d /tmp testdata/Gen.java && java -cp /tmp Gen > testdata/expected.txt

type point struct{ x, y int }

func pointHash(p point) int32 { return int32(31*(31+p.x) + p.y) }

func TestMatchesJava(t *testing.T) {
	f, err := os.Open("testdata/expected.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var set *HashSet[point]
	var caseName string
	orders, maxSize := 0, 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		switch f[0] {
		case "case":
			caseName = f[1]
			set = NewHashSet(pointHash)
		case "a":
			x, _ := strconv.Atoi(f[1])
			y, _ := strconv.Atoi(f[2])
			set.Add(point{x, y})
			maxSize = max(maxSize, set.Len())
		case "c":
			set.Clear()
		case "o":
			orders++
			var got []string
			for _, p := range set.All() {
				got = append(got, strconv.Itoa(p.x)+","+strconv.Itoa(p.y))
			}
			if strings.Join(got, " ") != strings.Join(f[1:], " ") {
				t.Fatalf("case %s: order\n got  %v\n want %v", caseName, got, f[1:])
			}
			if set.Len() != len(f)-1 {
				t.Fatalf("case %s: Len %d, want %d", caseName, set.Len(), len(f)-1)
			}
		}
	}
	if orders < 100 || maxSize < 100 {
		t.Fatalf("weak test data: %d orders, largest set %d", orders, maxSize)
	}
	t.Logf("%d orders match; largest set %d", orders, maxSize)
}

func TestContainsAndDuplicates(t *testing.T) {
	s := NewHashSet(pointHash)
	if !s.Add(point{1, 2}) || s.Add(point{1, 2}) {
		t.Error("Add should report only the first insertion")
	}
	if !s.Contains(point{1, 2}) || s.Contains(point{2, 1}) {
		t.Error("Contains is wrong")
	}
	s.Clear()
	if s.Len() != 0 || s.Contains(point{1, 2}) || len(s.All()) != 0 {
		t.Error("Clear left elements behind")
	}
}
