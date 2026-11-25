package hypersonic

import "strconv"

// Entity types, as printed to the players.
const (
	entityPlayer = 0
	entityBomb   = 1
	entityItem   = 2
)

// Item types.
const (
	itemFireUp = 1
	itemBombUp = 2
)

// Item is the Java Item class (FireUpItem and BombUpItem). Java numbers
// items and bombs with static counters (++ITEM_COUNT, ++BOMB_COUNT), kept on
// the Game here. The IDs only reach the viewer.
type Item struct {
	id        int
	position  Coord // set when the item's box is destroyed
	itemType  int
	removedBy []*Man
}

func (g *Game) newItem(itemType int) *Item {
	g.itemCount++
	return &Item{id: g.itemCount, itemType: itemType}
}

// instantiate is Item.instantiate.
func (g *Game) instantiate(itemType int) *Item {
	switch itemType {
	case itemBombUp, itemFireUp:
		return g.newItem(itemType)
	}
	return nil
}

func (g *Game) copyItem(i *Item) *Item { return g.newItem(i.itemType) }

func (g *Game) powerUp(i *Item, man *Man) {
	switch i.itemType {
	case itemBombUp:
		man.ammo = append(man.ammo, g.newBomb(man))
		man.maxAmmo++
	case itemFireUp:
		man.power++
	}
}

func (i *Item) toPlayerString() string {
	return entityString(entityItem, 0, i.position, i.itemType, i.itemType)
}

// entityString is GameEntity.toPlayerString.
func entityString(typ, owner int, position Coord, state, value int) string {
	return strconv.Itoa(typ) + " " + strconv.Itoa(owner) + " " + position.String() + " " +
		strconv.Itoa(state) + " " + strconv.Itoa(value)
}

// Bomb is the Java Bomb class.
type Bomb struct {
	id       int
	position Coord
	power    int
	fuse     int
	owner    *Man
}

func (g *Game) newBomb(owner *Man) *Bomb {
	g.bombCount++
	return &Bomb{id: g.bombCount, owner: owner}
}

func (b *Bomb) arm(c Coord, power, fuse int) {
	b.power = power
	b.fuse = fuse
	b.position = c
}

func (b *Bomb) getOwner() int { return b.owner.player.index }

func (b *Bomb) toPlayerString() string {
	return entityString(entityBomb, b.getOwner(), b.position, b.fuse, b.power)
}

// action is Man.Action; noAttempt stands for Java's null.
type action int

const (
	noAttempt action = iota
	attemptMove
	attemptBomb
)

// Man is the Java Man class: a player's character. Pointer fields stand
// for Java references that may be null.
type Man struct {
	id                   int
	position             Coord
	success, confused    bool
	dead, piercingBombs  bool
	killedBy             *Explosion
	nextMove             *Direction
	attempt              action
	attemptCoord         *Coord
	lastOccupiedPosition *Coord
	lastDestination      *Coord
	message              string
	direction            Direction
	power, maxAmmo       int
	player               *PlayerInfo
	ammo                 []*Bomb // a java.util.Stack: push and pop at the end
}

func (g *Game) newMan(c Coord, maxAmmo, power int, player *PlayerInfo) *Man {
	m := &Man{id: player.index, position: c, power: power, direction: dirDown, player: player, maxAmmo: maxAmmo}
	for range max(0, maxAmmo) {
		m.ammo = append(m.ammo, g.newBomb(m))
	}
	return m
}

func (m *Man) getX() int { return m.position.x }
func (m *Man) getY() int { return m.position.y }

func (m *Man) toPlayerString() string {
	return entityString(entityPlayer, m.player.index, m.position, len(m.ammo), m.power)
}

func (m *Man) reset() {
	m.message = ""
	m.attempt = noAttempt
	m.attemptCoord = nil
	m.nextMove = nil
	m.confused = false
	m.success = false
}

// PlayerInfo is the Java PlayerInfo class.
type PlayerInfo struct {
	invalidAction bool
	dead          bool
	index, deadAt int
	man           *Man
	boxesHit      int
}

func (g *Game) newPlayerInfo(index int, position Coord, bombs, power int) *PlayerInfo {
	p := &PlayerInfo{index: index}
	p.man = g.newMan(position, bombs, power, p)
	return p
}

// Explosion is the Java Explosion class: one bomb going off.
type Explosion struct {
	source     *Bomb
	flammables []*Cell
	bombs      []*Bomb
	kills      []*Man
	span       [4]int // by Direction
}

func (e *Explosion) toFrameString() string {
	return e.source.position.String() + " " + strconv.Itoa(e.span[dirLeft]) + " " + strconv.Itoa(e.span[dirRight]) + " " +
		strconv.Itoa(e.span[dirUp]) + " " + strconv.Itoa(e.span[dirDown]) + " " + strconv.Itoa(e.source.id)
}

type bombPlacement struct {
	owner *Man
	cell  *Cell
}
