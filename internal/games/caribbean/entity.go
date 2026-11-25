package caribbean

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

// join is Referee.join: String.valueOf of each value, separated by spaces.
func join(values ...any) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprint(v)
	}
	return strings.Join(parts, " ")
}

type entityType int

const (
	typeShip entityType = iota
	typeBarrel
	typeMine
	typeCannonball
)

func (t entityType) String() string {
	return [...]string{"SHIP", "BARREL", "MINE", "CANNONBALL"}[t]
}

// entity is Referee.Entity. Java numbers entities with a static counter
// that every constructor call advances, even for candidates the map
// generator throws away. The IDs reach the players, so Game.newEntity must
// be called exactly where Java constructs an entity.
type entity struct {
	id       int
	typ      entityType
	position Coord
}

func (g *Game) newEntity(t entityType, x, y int) entity {
	e := entity{id: g.nextEntityID, typ: t, position: Coord{x, y}}
	g.nextEntityID++
	return e
}

func (e *entity) toViewString() string { return join(e.id, e.position.y, e.position.x) }

func (e *entity) toPlayerString(arg1, arg2, arg3, arg4 int) string {
	return join(e.id, e.typ, e.position.x, e.position.y, arg1, arg2, arg3, arg4)
}

// mine is Referee.Mine.
type mine struct{ entity }

func (g *Game) newMine(x, y int) *mine { return &mine{g.newEntity(typeMine, x, y)} }

func (m *mine) playerString() string { return m.toPlayerString(0, 0, 0, 0) }

// explode damages the ships on the mine, and with force (or when a ship hit
// it) the ships next to it. The last ship on the mine is the victim and
// isn't hit twice.
func (m *mine) explode(ships []*ship, force bool) []damage {
	var dmg []damage
	var victim *ship
	for _, s := range ships {
		if m.position == s.bow() || m.position == s.stern() || m.position == s.position {
			dmg = append(dmg, damage{m.position, mineDamage, true})
			s.damage(mineDamage)
			victim = s
		}
	}
	if force || victim != nil {
		if victim == nil {
			dmg = append(dmg, damage{m.position, mineDamage, true})
		}
		for _, s := range ships {
			if s == victim {
				continue
			}
			var impact *Coord
			if stern := s.stern(); stern.distanceTo(m.position) <= 1 {
				impact = &stern
			}
			if bow := s.bow(); bow.distanceTo(m.position) <= 1 {
				impact = &bow
			}
			if pos := s.position; pos.distanceTo(m.position) <= 1 {
				impact = &pos
			}
			if impact != nil {
				s.damage(nearMineDamage)
				dmg = append(dmg, damage{*impact, nearMineDamage, true})
			}
		}
	}
	return dmg
}

// cannonball is Referee.Cannonball.
type cannonball struct {
	entity
	ownerEntityID         int
	srcX, srcY            int
	initialRemainingTurns int
	remainingTurns        int
}

func (g *Game) newCannonball(x, y, ownerEntityID, srcX, srcY, remainingTurns int) *cannonball {
	return &cannonball{
		entity:                g.newEntity(typeCannonball, x, y),
		ownerEntityID:         ownerEntityID,
		srcX:                  srcX,
		srcY:                  srcY,
		initialRemainingTurns: remainingTurns,
		remainingTurns:        remainingTurns,
	}
}

func (c *cannonball) viewString() string {
	return join(c.id, c.position.y, c.position.x, c.srcY, c.srcX, c.initialRemainingTurns, c.remainingTurns, c.ownerEntityID)
}

func (c *cannonball) playerString() string {
	return c.toPlayerString(c.ownerEntityID, c.remainingTurns, 0, 0)
}

// rumBarrel is Referee.RumBarrel.
type rumBarrel struct {
	entity
	health int
}

func (g *Game) newRumBarrel(x, y, health int) *rumBarrel {
	return &rumBarrel{g.newEntity(typeBarrel, x, y), health}
}

func (b *rumBarrel) viewString() string {
	return join(b.id, b.position.y, b.position.x, b.health)
}

func (b *rumBarrel) playerString() string { return b.toPlayerString(b.health, 0, 0, 0) }

// damage is Referee.Damage, shown by the viewer only.
type damage struct {
	position Coord
	health   int
	hit      bool
}

func (d damage) viewString() string {
	hit := 0
	if d.hit {
		hit = 1
	}
	return join(d.position.y, d.position.x, d.health, hit)
}

// truncateMessage is Ship.setMessage's cut: messages longer than 50 UTF-16
// code units keep the first 50 and get "...". A surrogate pair split by
// the cut leaves a lone surrogate, which Java prints as '?'.
func truncateMessage(message string) string {
	units := utf16.Encode([]rune(message))
	if len(units) <= 50 {
		return message
	}
	head := units[:50]
	suffix := "..."
	if head[49] >= 0xD800 && head[49] < 0xDC00 { // high surrogate cut from its pair
		head = head[:49]
		suffix = "?..."
	}
	return string(utf16.Decode(head)) + suffix
}
