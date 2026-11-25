package codebusters

import (
	"math"
	"strconv"

	"github.com/adamadair/codingame-referees/internal/jmath"
)

// Vector is the Java Vector class: an immutable pair of doubles. Only the
// methods the referee uses are ported. Products are wrapped in float64(...)
// so Go doesn't fuse them into FMA instructions, which Java never does.
type Vector struct {
	x, y float64
}

// newVectorFrom is new Vector(a, b): the vector from a to b.
func newVectorFrom(a, b Vector) Vector { return Vector{b.x - a.x, b.y - a.y} }

func (v Vector) equals(o Vector) bool { return o.x == v.x && o.y == v.y }

// round is new Vector((int) Math.round(x), (int) Math.round(y)).
func (v Vector) round() Vector {
	return Vector{float64(int32(jmath.Round(v.x))), float64(int32(jmath.Round(v.y)))}
}

func (v Vector) getX() float64 { return v.x }
func (v Vector) getY() float64 { return v.y }

func (v Vector) distance(o Vector) float64 {
	dx, dy := o.x-v.x, o.y-v.y
	return math.Sqrt(float64(dx*dx) + float64(dy*dy))
}

func (v Vector) add(o Vector) Vector { return Vector{v.x + o.x, v.y + o.y} }

func (v Vector) mult(factor float64) Vector { return Vector{v.x * factor, v.y * factor} }

func (v Vector) length() float64 { return math.Sqrt(float64(v.x*v.x) + float64(v.y*v.y)) }

func (v Vector) lengthSquared() float64 { return float64(v.x*v.x) + float64(v.y*v.y) }

func (v Vector) normalize() Vector {
	length := v.length()
	if length == 0 {
		return Vector{0, 0}
	}
	return Vector{v.x / length, v.y / length}
}

func (v Vector) angle() float64 { return jmath.Atan2(v.y, v.x) }

// toIntString is (int) x + " " + (int) y.
func (v Vector) toIntString() string {
	return strconv.Itoa(int(jmath.Int(v.x))) + " " + strconv.Itoa(int(jmath.Int(v.y)))
}

// translate is new Vector(o.x + x, o.y + y).
func (v Vector) translate(o Vector) Vector { return Vector{o.x + v.x, o.y + v.y} }
