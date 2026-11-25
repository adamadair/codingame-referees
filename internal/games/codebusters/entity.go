package codebusters

import (
	"math"
	"strconv"

	"github.com/adamadair/codingame-referees/internal/jmath"
)

// Entity types in the player input (GameEntity.type). Busters use their
// owner's index.
const (
	ghostNormal = -1
	obstacle    = -2
)

// gameEntity is GameEntity: the fields every entity prints to players.
type gameEntity struct {
	id       int
	position Vector
	typ      int
	state    int
}

func (e *gameEntity) toPlayerString(value int) string {
	return strconv.Itoa(e.id) + " " + e.position.toIntString() + " " + strconv.Itoa(e.typ) + " " +
		strconv.Itoa(e.state) + " " + strconv.Itoa(value)
}

// Ghost is the Java Ghost class. Its state is its stamina.
type Ghost struct {
	gameEntity
	frightenedBy           *Vector
	grabbedBy              *Buster
	moved                  bool // never reset: once moved, a ghost is in every view frame
	inHq, suffering        bool
	wasSuffering, escaping bool
	attemptedGrabs         int
}

func newGhost(id int, position Vector, typ, stamina int) *Ghost {
	return &Ghost{gameEntity: gameEntity{id, position, typ, stamina}}
}

func (g *Ghost) isTrappedBy(player *PlayerInfo) bool {
	return g.isGrabbed() && g.grabbedBy.ghost == g && g.grabbedBy.state == busterCarrying && g.grabbedBy.owner == player
}

func (g *Ghost) getStamina() int { return g.state }

func (g *Ghost) isGrabbed() bool { return g.grabbedBy != nil }

func (g *Ghost) toInitFrameString() string { return strconv.Itoa(g.getStamina()) }

func (g *Ghost) hasMoved(round int) bool { return round == 0 || g.moved }

func (g *Ghost) toFrameString() string {
	grabber := -1
	if g.isGrabbed() {
		grabber = g.grabbedBy.id
	}
	return strconv.Itoa(g.id) + " " + g.position.toIntString() + " " + strconv.Itoa(grabber) + " " +
		strconv.Itoa(b2i(g.suffering)) + " " + strconv.Itoa(g.getStamina()) + " " + strconv.Itoa(b2i(g.isCaptured()))
}

func (g *Ghost) flagUpdated() { g.moved = true }

func (g *Ghost) decreaseStamina(amount int) {
	g.state = max(0, g.getStamina()-amount)
	g.attemptedGrabs = amount
}

func (g *Ghost) isCaptured() bool { return !g.isGrabbed() && g.inHq }

func (g *Ghost) getValue() int {
	if g.suffering {
		return g.attemptedGrabs
	}
	return 0
}

func (g *Ghost) toPlayerString() string { return g.gameEntity.toPlayerString(g.getValue()) }

// Buster states.
const (
	busterIdle     = 0
	busterCarrying = 1
	busterStunned  = 2
	busterGrabbing = 3
)

// action is Buster.Action. actionNone stands for Java's null.
type action int

const (
	actionNone action = iota
	actionMove
	actionGrab
	actionStun
	actionRadar
	actionRelease
	actionEject
)

// Buster is the Java Buster class.
type Buster struct {
	gameEntity
	index, stunCountDown, stunRecharge, radars, radarTimeout int
	owner                                                    *PlayerInfo
	message                                                  string // "" is Java's null
	attemptedVector                                          *Vector
	nextPosition                                             *Vector
	rotation                                                 float64
	confused, finishGrab, tooClose                           bool
	invalidMove                                              bool
	doStun                                                   bool

	attempt action
	success bool
	// attemptTargetId is an Integer in Java. reset() doesn't clear it, and
	// it is only read in rounds where it was set.
	attemptTargetId    int
	hasAttemptTargetId bool

	ghost *Ghost
}

func newBuster(id, index int, position Vector, owner *PlayerInfo, rotation float64) *Buster {
	return &Buster{
		gameEntity: gameEntity{id, position, owner.index, busterIdle},
		index:      index,
		owner:      owner,
		rotation:   rotation,
		radars:     radarsPerBuster,
	}
}

func (b *Buster) attemptedMove() bool  { return b.attempt == actionMove }
func (b *Buster) radarSucceeded() bool { return b.attempt == actionRadar && b.success }
func (b *Buster) attemptedGrab() bool  { return b.attempt == actionGrab }
func (b *Buster) stunSucceeded() bool  { return b.attempt == actionStun && b.success }

func (b *Buster) reset() {
	b.attempt = actionNone
	b.success = false
	b.nextPosition = nil
	b.attemptedVector = nil
	b.invalidMove = false
	b.message = ""
	b.confused = false
	b.finishGrab = false
	b.tooClose = false
	if b.radarTimeout > 0 {
		b.radarTimeout--
	}
	if b.doStun {
		b.applyStun()
	}
}

// toFrameString panics like Java's NullPointerException if a grabbing
// buster has no ghost, which can't happen.
func (b *Buster) toFrameString() string {
	stunTarget := -1
	if b.stunSucceeded() {
		stunTarget = b.attemptTargetId
	}
	grabbed := -1
	if b.state == busterGrabbing || b.finishGrab {
		grabbed = b.ghost.id
	}
	return b.position.toIntString() + " " + strconv.Itoa(b.state) + " " +
		strconv.Itoa(int(jmath.Int(b.rotation*180/math.Pi))) + " " + strconv.Itoa(b2i(b.confused)) + " " +
		strconv.Itoa(stunTarget) + " " + strconv.Itoa(grabbed) + " " + strconv.Itoa(b2i(b.radarSucceeded())) + " " + b.message
}

func (b *Buster) applyStun() {
	b.state = busterStunned
	b.doStun = false
}

func (b *Buster) stun(duration int) {
	b.doStun = true
	b.stunCountDown = duration
}

func (b *Buster) trappedGhost() bool { return b.attemptedGrab() && b.state == busterCarrying }

func (b *Buster) letGo() {
	if b.state == busterCarrying {
		b.state = busterIdle
		if b.ghost != nil && b.ghost.grabbedBy == b {
			b.ghost.grabbedBy = nil
			b.ghost.flagUpdated()
			b.ghost.escaping = true
		}
	}
}

func (b *Buster) getValue() int {
	switch b.state {
	case busterCarrying, busterGrabbing:
		return b.ghost.id
	case busterStunned:
		return b.stunCountDown
	}
	return -1
}

// toPlayerString is the GAME_VERSION >= 3 form, the only one this port
// plays.
func (b *Buster) toPlayerString() string { return b.gameEntity.toPlayerString(b.getValue()) }

// Obstacle is the Java Obstacle class. Its state is its radius.
type Obstacle struct {
	gameEntity
}

// newObstacle numbers obstacles with a counter that Java keeps in a static
// field (obstacleCount), so the IDs keep growing across chooseMap calls.
func (g *Game) newObstacle(x, y, radius int) *Obstacle {
	o := &Obstacle{gameEntity{g.obstacleCount, Vector{float64(x), float64(y)}, obstacle, radius}}
	g.obstacleCount++
	return o
}

func (o *Obstacle) getRadius() int { return o.state }

func (o *Obstacle) toPlayerString() string { return o.gameEntity.toPlayerString(0) }

func (o *Obstacle) toInitFrameString() string {
	return strconv.Itoa(o.id) + " " + o.position.toIntString() + " " + strconv.Itoa(o.getRadius())
}

// PlayerInfo is the Java PlayerInfo class.
type PlayerInfo struct {
	busters []*Buster
	dead    bool
	index   int
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
