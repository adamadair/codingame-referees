package caribbean

import (
	"math"

	"github.com/adamadair/codingame-referees/internal/jmath"
)

// Coord is Referee.Coord: a cell of the hex grid in offset coordinates
// (odd rows shifted right). It is a value type; Java compares Coords with
// equals, which is ==.
type Coord struct {
	x, y int
}

var (
	directionsEven = [6][2]int{{1, 0}, {0, -1}, {-1, -1}, {-1, 0}, {-1, 1}, {0, 1}}
	directionsOdd  = [6][2]int{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {0, 1}, {1, 1}}
)

// angle returns the direction from c to target in sixths of a turn, in
// [0, 6). The ship autopilot compares these values exactly, so this uses
// Java's atan2 (jmath.Atan2), not Go's.
func (c Coord) angle(target Coord) float64 {
	dy := float64(target.y-c.y) * math.Sqrt(3) / 2
	dx := float64(target.x-c.x) + float64(float64((c.y-target.y)&1)*0.5)
	angle := -jmath.Atan2(dy, dx) * 3 / math.Pi
	if angle < 0 {
		angle += 6
	} else if angle >= 6 {
		angle -= 6
	}
	return angle
}

func (c Coord) toCubeCoordinate() cubeCoordinate {
	xp := c.x - (c.y-(c.y&1))/2
	zp := c.y
	yp := -(xp + zp)
	return cubeCoordinate{xp, yp, zp}
}

// neighbor returns the adjacent cell in a direction (0 is east, then
// counter-clockwise). Like Java, a negative odd row (y % 2 == -1) uses the
// even-row offsets.
func (c Coord) neighbor(orientation int) Coord {
	if c.y%2 == 1 {
		return Coord{c.x + directionsOdd[orientation][0], c.y + directionsOdd[orientation][1]}
	}
	return Coord{c.x + directionsEven[orientation][0], c.y + directionsEven[orientation][1]}
}

func (c Coord) isInsideMap() bool {
	return c.x >= 0 && c.x < mapWidth && c.y >= 0 && c.y < mapHeight
}

func (c Coord) distanceTo(dst Coord) int {
	return c.toCubeCoordinate().distanceTo(dst.toCubeCoordinate())
}

func (c Coord) String() string { return join(c.x, c.y) }

// cubeCoordinate is Referee.CubeCoordinate.
type cubeCoordinate struct {
	x, y, z int
}

func (c cubeCoordinate) distanceTo(dst cubeCoordinate) int {
	return (abs(c.x-dst.x) + abs(c.y-dst.y) + abs(c.z-dst.z)) / 2
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
