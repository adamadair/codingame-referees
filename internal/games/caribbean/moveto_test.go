package caribbean

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// runDriver runs testdata/MoveToDriver.java against the official referee
// classes and returns its output lines.
func runDriver(t *testing.T, args ...string) []string {
	t.Helper()
	out, err := exec.Command("java", append([]string{"-cp", javaClasses, "MoveToDriver"}, args...)...).Output()
	if err != nil {
		t.Fatalf("driver %v: %v", args, err)
	}
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

func actionChar(a action) byte {
	if a == noAction {
		return '.'
	}
	return 'A' + byte(a-actFaster) // Java's Action.ordinal()
}

func goAction(x, y, o, speed, tx, ty int) byte {
	s := &ship{entity: entity{position: Coord{x, y}}, orientation: o, speed: speed}
	s.moveTo(tx, ty)
	return actionChar(s.action)
}

// TestMoveToExhaustive compares the autopilot with Java for every ship
// position, orientation and speed and every target on the map.
func TestMoveToExhaustive(t *testing.T) {
	requireJava(t)
	lines := runDriver(t, "moveto")
	if want := mapWidth * mapHeight * 6 * 3; len(lines) != want {
		t.Fatalf("driver printed %d lines, want %d", len(lines), want)
	}
	i, failures, cases := 0, 0, 0
	for y := range mapHeight {
		for x := range mapWidth {
			for o := range 6 {
				for speed := range 3 {
					line := lines[i]
					i++
					k := 0
					for ty := range mapHeight {
						for tx := range mapWidth {
							cases++
							if got, want := goAction(x, y, o, speed, tx, ty), line[k]; got != want {
								failures++
								if failures <= 10 {
									t.Errorf("ship (%d,%d) o=%d speed=%d to (%d,%d): got %c, want %c", x, y, o, speed, tx, ty, got, want)
								}
							}
							k++
						}
					}
				}
			}
		}
	}
	if failures > 0 {
		t.Fatalf("%d of %d cases differ", failures, cases)
	}
	t.Logf("%d cases match", cases)
}

// TestMoveToOffMap compares random cases with targets off the map, up to
// the 8-digit coordinates the MOVE command accepts.
func TestMoveToOffMap(t *testing.T) {
	requireJava(t)
	failures := 0
	lines := runDriver(t, "offmap", "20261125")
	for _, line := range lines {
		f := strings.Fields(line)
		v := make([]int, 6)
		for i := range v {
			v[i], _ = strconv.Atoi(f[i])
		}
		if got := goAction(v[0], v[1], v[2], v[3], v[4], v[5]); got != f[6][0] {
			failures++
			if failures <= 10 {
				t.Errorf("%s: got %c", line, got)
			}
		}
	}
	if len(lines) != 20000 || failures > 0 {
		t.Fatalf("%d lines, %d differ", len(lines), failures)
	}
}

// TestAngleExhaustive compares Coord.angle bit for bit for every pair of
// cells on the map.
func TestAngleExhaustive(t *testing.T) {
	requireJava(t)
	lines := runDriver(t, "angle")
	if len(lines) != mapWidth*mapHeight {
		t.Fatalf("driver printed %d lines", len(lines))
	}
	failures, goMathDiffers, row := 0, 0, 0
	for y := range mapHeight {
		for x := range mapWidth {
			want := strings.Fields(lines[row])
			row++
			k := 0
			for ty := range mapHeight {
				for tx := range mapWidth {
					if strconv.FormatUint(math.Float64bits(angleWithGoMath(Coord{x, y}, Coord{tx, ty})), 16) != want[k] {
						goMathDiffers++
					}
					got := strconv.FormatUint(math.Float64bits(Coord{x, y}.angle(Coord{tx, ty})), 16)
					if got != want[k] {
						failures++
						if failures <= 10 {
							t.Errorf("angle (%d,%d)->(%d,%d): got %s, want %s", x, y, tx, ty, got, want[k])
						}
					}
					k++
				}
			}
		}
	}
	if failures > 0 {
		t.Fatal(fmt.Sprintf("%d angles differ", failures))
	}
	t.Logf("with Go's math.Atan2, %d of %d angles would differ from Java", goMathDiffers, len(lines)*len(lines))
}

// angleWithGoMath is Coord.angle using Go's math.Atan2, to show that the
// test can tell the difference.
func angleWithGoMath(c, target Coord) float64 {
	dy := float64(target.y-c.y) * math.Sqrt(3) / 2
	dx := float64(target.x-c.x) + float64(float64((c.y-target.y)&1)*0.5)
	angle := -math.Atan2(dy, dx) * 3 / math.Pi
	if angle < 0 {
		angle += 6
	} else if angle >= 6 {
		angle -= 6
	}
	return angle
}
