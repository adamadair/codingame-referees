package hypersonic

import "strconv"

// Coord is the Java Coord class: a grid position, compared by value.
type Coord struct {
	x, y int
}

func (c Coord) add(x, y int) Coord { return Coord{c.x + x, c.y + y} }

func (c Coord) String() string { return strconv.Itoa(c.x) + " " + strconv.Itoa(c.y) }

// hashCode is Coord.hashCode(), in Java's 32-bit arithmetic. It decides the
// iteration order of explodedBoxes, which reaches the players.
func (c Coord) hashCode() int32 {
	const prime = 31
	result := int32(1)
	result = prime*result + int32(c.x)
	result = prime*result + int32(c.y)
	return result
}

func (c Coord) manhattanTo(t Coord) int { return abs(t.x-c.x) + abs(t.y-c.y) }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Cell types.
const (
	cellBlock = 0
	cellFloor = 1
	cellBox   = 2
)

// Cell is the Java Cell class, a grid square. It extends Coord in Java.
type Cell struct {
	Coord
	bombs []*Bomb
	typ   int
	item  *Item
	boxID int
}

func newCell(x, y, typ int) *Cell { return &Cell{Coord: Coord{x, y}, typ: typ} }

func (c *Cell) hasItem() bool { return c.item != nil }
func (c *Cell) isBox() bool   { return c.typ == cellBox }
func (c *Cell) hasBomb() bool { return len(c.bombs) > 0 }
func (c *Cell) isFree() bool  { return c.typ == cellFloor && !c.hasBomb() }

func (c *Cell) toPlayerString(visibleItems bool) string {
	switch c.typ {
	case cellBlock:
		return "X"
	case cellFloor:
		return "."
	case cellBox:
		if visibleItems && c.hasItem() {
			return strconv.Itoa(c.item.itemType)
		}
		return "0"
	default:
		return "X"
	}
}

// Direction is the Java Direction enum. The order of directions (LEFT,
// RIGHT, UP, DOWN) matters: explosions and the BFS visit them in it.
type Direction int

const (
	dirLeft Direction = iota
	dirRight
	dirUp
	dirDown
)

var directions = [4]Direction{dirLeft, dirRight, dirUp, dirDown}

func (d Direction) getX() int { return [...]int{-1, 1, 0, 0}[d] }
func (d Direction) getY() int { return [...]int{0, 0, -1, 1}[d] }

func (d Direction) String() string { return [...]string{"LEFT", "RIGHT", "UP", "DOWN"}[d] }
