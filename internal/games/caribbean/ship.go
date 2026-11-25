package caribbean

import "math"

// action is Referee.Action. noAction stands for Java's null (WAIT, or a
// MOVE that keeps going straight).
type action int

const (
	noAction action = iota
	actFaster
	actSlower
	actPort
	actStarboard
	actFire
	actMine
)

func (a action) String() string {
	return [...]string{"WAIT", "FASTER", "SLOWER", "PORT", "STARBOARD", "FIRE", "MINE"}[a]
}

// ship is Referee.Ship. The new* fields are only read after moveShips or
// rotateShips has set them for every ship, so Java's null checks never
// matter and plain values are enough.
type ship struct {
	entity
	orientation    int
	speed          int
	health         int
	initialHealth  int
	owner          int
	message        string
	action         action
	mineCooldown   int
	cannonCooldown int
	target         Coord

	newOrientation     int
	newPosition        Coord
	newBowCoordinate   Coord
	newSternCoordinate Coord
}

func (g *Game) newShip(x, y, orientation, owner int) *ship {
	return &ship{
		entity:      g.newEntity(typeShip, x, y),
		orientation: orientation,
		health:      initialShipHealth,
		owner:       owner,
	}
}

func (s *ship) viewString() string {
	bow, stern := s.bow(), s.stern()
	return join(s.id, s.position.y, s.position.x, s.orientation, s.health, s.speed, s.action,
		bow.y, bow.x, stern.y, stern.x, " ;"+s.message)
}

func (s *ship) playerString(playerIdx int) string {
	mine := 0
	if s.owner == playerIdx {
		mine = 1
	}
	return s.toPlayerString(s.orientation, s.speed, s.health, mine)
}

// setMessage keeps the optional message of a command, cut to 50 UTF-16
// units. A missing message clears it.
func (s *ship) setMessage(message string) {
	s.message = truncateMessage(message)
}

// moveTo is the MOVE autopilot: pick the action that brings the ship
// toward (x, y). It is ported line by line, including the exact float
// comparisons, which is why Coord.angle must match Java bit for bit.
func (s *ship) moveTo(x, y int) {
	currentPosition := s.position
	targetPosition := Coord{x, y}

	if currentPosition == targetPosition {
		s.action = actSlower
		return
	}

	o := float64(s.orientation)
	switch s.speed {
	case 2:
		s.action = actSlower
	case 1:
		// Suppose we've moved first
		currentPosition = currentPosition.neighbor(s.orientation)
		if !currentPosition.isInsideMap() {
			s.action = actSlower
			break
		}

		// Target reached at next turn
		if currentPosition == targetPosition {
			s.action = noAction
			break
		}

		// For each neighbor cell, find the closest to target
		targetAngle := currentPosition.angle(targetPosition)
		angleStraight := math.Min(math.Abs(o-targetAngle), 6-math.Abs(o-targetAngle))
		anglePort := math.Min(math.Abs((o+1)-targetAngle), math.Abs((o-5)-targetAngle))
		angleStarboard := math.Min(math.Abs((o+5)-targetAngle), math.Abs((o-1)-targetAngle))

		centerAngle := currentPosition.angle(Coord{mapWidth / 2, mapHeight / 2})
		anglePortCenter := math.Min(math.Abs((o+1)-centerAngle), math.Abs((o-5)-centerAngle))
		angleStarboardCenter := math.Min(math.Abs((o+5)-centerAngle), math.Abs((o-1)-centerAngle))

		// Next to target with bad angle, slow down then rotate (avoid to turn around the target!)
		if currentPosition.distanceTo(targetPosition) == 1 && angleStraight > 1.5 {
			s.action = actSlower
			break
		}

		distanceMin, haveMin := 0, false

		// Test forward
		nextPosition := currentPosition.neighbor(s.orientation)
		if nextPosition.isInsideMap() {
			distanceMin, haveMin = nextPosition.distanceTo(targetPosition), true
			s.action = noAction
		}

		// Test port
		nextPosition = currentPosition.neighbor((s.orientation + 1) % 6)
		if nextPosition.isInsideMap() {
			distance := nextPosition.distanceTo(targetPosition)
			if !haveMin || distance < distanceMin || distance == distanceMin && anglePort < angleStraight-0.5 {
				distanceMin, haveMin = distance, true
				s.action = actPort
			}
		}

		// Test starboard
		nextPosition = currentPosition.neighbor((s.orientation + 5) % 6)
		if nextPosition.isInsideMap() {
			distance := nextPosition.distanceTo(targetPosition)
			if !haveMin || distance < distanceMin ||
				(distance == distanceMin && angleStarboard < anglePort-0.5 && s.action == actPort) ||
				(distance == distanceMin && angleStarboard < angleStraight-0.5 && s.action == noAction) ||
				(distance == distanceMin && s.action == actPort && angleStarboard == anglePort &&
					angleStarboardCenter < anglePortCenter) ||
				(distance == distanceMin && s.action == actPort && angleStarboard == anglePort &&
					angleStarboardCenter == anglePortCenter && (s.orientation == 1 || s.orientation == 4)) {
				s.action = actStarboard
			}
		}
	case 0:
		// Rotate ship towards target
		targetAngle := currentPosition.angle(targetPosition)
		angleStraight := math.Min(math.Abs(o-targetAngle), 6-math.Abs(o-targetAngle))
		anglePort := math.Min(math.Abs((o+1)-targetAngle), math.Abs((o-5)-targetAngle))
		angleStarboard := math.Min(math.Abs((o+5)-targetAngle), math.Abs((o-1)-targetAngle))

		centerAngle := currentPosition.angle(Coord{mapWidth / 2, mapHeight / 2})
		anglePortCenter := math.Min(math.Abs((o+1)-centerAngle), math.Abs((o-5)-centerAngle))
		angleStarboardCenter := math.Min(math.Abs((o+5)-centerAngle), math.Abs((o-1)-centerAngle))

		forwardPosition := currentPosition.neighbor(s.orientation)

		s.action = noAction

		if anglePort <= angleStarboard {
			s.action = actPort
		}

		if angleStarboard < anglePort || angleStarboard == anglePort && angleStarboardCenter < anglePortCenter ||
			angleStarboard == anglePort && angleStarboardCenter == anglePortCenter && (s.orientation == 1 || s.orientation == 4) {
			s.action = actStarboard
		}

		if forwardPosition.isInsideMap() && angleStraight <= anglePort && angleStraight <= angleStarboard {
			s.action = actFaster
		}
	}
}

func (s *ship) faster()    { s.action = actFaster }
func (s *ship) slower()    { s.action = actSlower }
func (s *ship) port()      { s.action = actPort }
func (s *ship) starboard() { s.action = actStarboard }

func (s *ship) placeMine() {
	if minesEnabled {
		s.action = actMine
	}
}

func (s *ship) fire(x, y int) {
	if cannonsEnabled {
		s.target = Coord{x, y}
		s.action = actFire
	}
}

func (s *ship) stern() Coord    { return s.position.neighbor((s.orientation + 3) % 6) }
func (s *ship) bow() Coord      { return s.position.neighbor(s.orientation) }
func (s *ship) newStern() Coord { return s.position.neighbor((s.newOrientation + 3) % 6) }
func (s *ship) newBow() Coord   { return s.position.neighbor(s.newOrientation) }

// at reports whether the ship covers the cell.
func (s *ship) at(c Coord) bool {
	return s.stern() == c || s.bow() == c || s.position == c
}

func (s *ship) newBowIntersect(other *ship) bool {
	return s.newBowCoordinate == other.newBowCoordinate || s.newBowCoordinate == other.newPosition ||
		s.newBowCoordinate == other.newSternCoordinate
}

func (s *ship) newBowIntersectAny(ships []*ship) bool {
	for _, other := range ships {
		if s != other && s.newBowIntersect(other) {
			return true
		}
	}
	return false
}

func (s *ship) newPositionsIntersect(other *ship) bool {
	sternCollision := s.newSternCoordinate == other.newBowCoordinate || s.newSternCoordinate == other.newPosition ||
		s.newSternCoordinate == other.newSternCoordinate
	centerCollision := s.newPosition == other.newBowCoordinate || s.newPosition == other.newPosition ||
		s.newPosition == other.newSternCoordinate
	return s.newBowIntersect(other) || sternCollision || centerCollision
}

func (s *ship) newPositionsIntersectAny(ships []*ship) bool {
	for _, other := range ships {
		if s != other && s.newPositionsIntersect(other) {
			return true
		}
	}
	return false
}

func (s *ship) damage(health int) {
	s.health -= health
	if s.health <= 0 {
		s.health = 0
	}
}

func (s *ship) heal(health int) {
	s.health += health
	if s.health > maxShipHealth {
		s.health = maxShipHealth
	}
}
