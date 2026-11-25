// Package caribbean is a Go port of the official CodinGame Java referee for
// Coders of the Caribbean, at league level 3 (the final league).
//
// Two players each command up to three pirate ships on a 23x21 hex grid.
// Ships lose a unit of rum every turn, refill from rum barrels, and fight
// with cannonballs and mines. A player whose ships have all sunk loses; after
// 200 rounds the most rum wins.
//
// Given the same parameters (or seed) and the same bot outputs, this port
// gives players the same input and the same scores as the Java referee.
package caribbean

import (
	"io"
	"strconv"
	"time"

	"github.com/adamadair/codingame-referees/internal/cgref"
	"github.com/adamadair/codingame-referees/internal/jmath"
	"github.com/adamadair/codingame-referees/internal/jprops"
	"github.com/adamadair/codingame-referees/internal/jrandom"
	"github.com/adamadair/codingame-referees/internal/jstrings"
)

const (
	mapWidth             = 23
	mapHeight            = 21
	cooldownCannon       = 2
	cooldownMine         = 5
	initialShipHealth    = 100
	maxShipHealth        = 100
	minShips             = 1
	minRumBarrels        = 10
	maxRumBarrels        = 26
	minRumBarrelValue    = 10
	maxRumBarrelValue    = 20
	rewardRumBarrelValue = 30
	mineVisibilityRange  = 5
	fireDistanceMax      = 10
	lowDamage            = 25
	highDamage           = 50
	mineDamage           = 25
	nearMineDamage       = 10
)

// League settings. The Java referee hard-codes LEAGUE_LEVEL = 3; this is
// its "default" case. Lower leagues would change only this block (and the
// LEAGUE_LEVEL == 0 check in gameIsOver).
const (
	leagueLevel    = 3
	maxShips       = 3
	cannonsEnabled = true
	minesEnabled   = true
	minMines       = 5
	maxMines       = 10
	maxShipSpeed   = 2
)

// player is Referee.Player.
type player struct {
	id         int
	ships      []*ship
	shipsAlive []*ship
}

func (p *player) setDead() {
	for _, s := range p.ships {
		s.health = 0
	}
}

func (p *player) score() int {
	score := 0
	for _, s := range p.ships {
		score += s.health
	}
	return score
}

func (p *player) viewStrings() []string {
	data := []string{strconv.Itoa(p.id)}
	for _, s := range p.ships {
		data = append(data, s.viewString())
	}
	return data
}

// Game is the Coders of the Caribbean referee. The zero value is ready to
// use; run it with Run.
type Game struct {
	cgref.Base

	// Clock seeds the random default seed when the runner sends none, in
	// milliseconds like Java's System.currentTimeMillis. Nil means the
	// current time.
	Clock func() int64

	nextEntityID int

	seed                 int64
	cannonballs          []*cannonball
	mines                []*mine
	barrels              []*rumBarrel
	players              []*player
	ships                []*ship
	damages              []damage
	cannonBallExplosions []Coord
	shipsPerPlayer       int
	mineCount            int
	barrelCount          int
	random               *jrandom.Random

	// Java's mines and barrels lists stay null until initReferee creates
	// them. If init fails first (fewer than 2 players), the framework's
	// error frame calls getFrameDataForView, which then fails with a
	// NullPointerException, and the referee stops without scores. These
	// flags let FrameDataForView fail the same way.
	minesCreated, barrelsCreated bool
}

// Run plays one game with a default Game.
func Run(in io.Reader, out io.Writer) error {
	return (&Game{}).Run(in, out)
}

// Run plays one game on the runner protocol. See cgref.Run for the error.
func (g *Game) Run(in io.Reader, out io.Writer) error {
	return cgref.Run(g, in, out, io.Discard, cgref.Options{})
}

func (g *Game) clock() int64 {
	if g.Clock != nil {
		return g.Clock()
	}
	return time.Now().UnixMilli()
}

func clamp(val, lo, hi int) int { return max(lo, min(hi, val)) }

// parseProperty is Referee.parseProperty: a missing or unparsable value
// gives the default. The caller has already evaluated the default (and
// used up its random numbers), as Java does.
func parseProperty(prop *jprops.Properties, key string, defaultValue int64) int64 {
	if v, ok := prop.Get(key); ok {
		if n, err := jstrings.ParseLong(v); err == nil {
			return n
		}
	}
	return defaultValue
}

// intProperty is (int) parseProperty(...): Java's long-to-int cast wraps.
func intProperty(prop *jprops.Properties, key string, defaultValue int) int {
	return int(int32(parseProperty(prop, key, int64(defaultValue))))
}

func (g *Game) InitReferee(playerCount int, prop *jprops.Properties) error {
	g.seed = parseProperty(prop, "seed", jrandom.New(g.clock()).NextLong())
	g.random = jrandom.New(g.seed)

	g.shipsPerPlayer = clamp(intProperty(prop, "shipsPerPlayer", int(g.random.NextIntN(1+maxShips-minShips))+minShips), minShips, maxShips)

	if maxMines > minMines {
		g.mineCount = clamp(intProperty(prop, "mineCount", int(g.random.NextIntN(maxMines-minMines))+minMines), minMines, maxMines)
	} else {
		g.mineCount = minMines
	}

	g.barrelCount = clamp(intProperty(prop, "barrelCount", int(g.random.NextIntN(maxRumBarrels-minRumBarrels))+minRumBarrels),
		minRumBarrels, maxRumBarrels)

	g.cannonballs = nil
	g.cannonBallExplosions = nil
	g.damages = nil

	// Generate players
	g.players = make([]*player, playerCount)
	for i := range g.players {
		g.players[i] = &player{id: i}
	}
	// Generate ships. Like Java, this fails with fewer than 2 players.
	for j := range g.shipsPerPlayer {
		xMin := 1 + j*mapWidth/g.shipsPerPlayer
		xMax := (j+1)*mapWidth/g.shipsPerPlayer - 2

		y := 1 + int(g.random.NextIntN(mapHeight/2-2))
		x := xMin + int(g.random.NextIntN(int32(1+xMax-xMin)))
		orientation := int(g.random.NextIntN(6))

		ship0 := g.newShip(x, y, orientation, 0)
		ship1 := g.newShip(x, mapHeight-1-y, (6-orientation)%6, 1)

		g.players[0].ships = append(g.players[0].ships, ship0)
		g.players[1].ships = append(g.players[1].ships, ship1)
		g.players[0].shipsAlive = append(g.players[0].shipsAlive, ship0)
		g.players[1].shipsAlive = append(g.players[1].shipsAlive, ship1)
	}

	g.ships = nil
	for _, p := range g.players {
		g.ships = append(g.ships, p.ships...)
	}

	// Generate mines. Every candidate takes an entity ID, kept or not, and
	// before its mirrored copy.
	g.mines = nil
	g.minesCreated = true
	for len(g.mines) < g.mineCount {
		x := 1 + int(g.random.NextIntN(mapWidth-2))
		y := 1 + int(g.random.NextIntN(mapHeight/2))

		m := g.newMine(x, y)

		if g.cellIsFreeOfMines(m.position) && g.cellIsFreeOfShips(m.position, nil) {
			if y != mapHeight-1-y {
				g.mines = append(g.mines, g.newMine(x, mapHeight-1-y))
			}
			g.mines = append(g.mines, m)
		}
	}
	g.mineCount = len(g.mines)

	// Generate supplies
	g.barrels = nil
	g.barrelsCreated = true
	for len(g.barrels) < g.barrelCount {
		x := 1 + int(g.random.NextIntN(mapWidth-2))
		y := 1 + int(g.random.NextIntN(mapHeight/2))
		h := minRumBarrelValue + int(g.random.NextIntN(1+maxRumBarrelValue-minRumBarrelValue))

		m := g.newRumBarrel(x, y, h)

		if g.cellIsFreeOfShips(m.position, nil) && g.cellIsFreeOfMines(m.position) && g.cellIsFreeOfBarrels(m.position) {
			if y != mapHeight-1-y {
				g.barrels = append(g.barrels, g.newRumBarrel(x, mapHeight-1-y, h))
			}
			g.barrels = append(g.barrels, m)
		}
	}
	g.barrelCount = len(g.barrels)
	return nil
}

func (g *Game) cellIsFreeOfMines(c Coord) bool {
	for _, m := range g.mines {
		if m.position == c {
			return false
		}
	}
	return true
}

func (g *Game) cellIsFreeOfBarrels(c Coord) bool {
	for _, b := range g.barrels {
		if b.position == c {
			return false
		}
	}
	return true
}

// cellIsFreeOfShips ignores the ship except, if any.
func (g *Game) cellIsFreeOfShips(c Coord, except *ship) bool {
	for _, s := range g.ships {
		if s != except && s.at(c) {
			return false
		}
	}
	return true
}

func (g *Game) Configuration() *jprops.Properties {
	p := jprops.New()
	p.Set("seed", strconv.FormatInt(g.seed, 10))
	p.Set("shipsPerPlayer", strconv.Itoa(g.shipsPerPlayer))
	p.Set("barrelCount", strconv.Itoa(g.barrelCount))
	p.Set("mineCount", strconv.Itoa(g.mineCount))
	return p
}

func (g *Game) Prepare(round int) {
	for _, p := range g.players {
		for _, s := range p.ships {
			s.action = noAction
			s.message = ""
		}
	}
	g.cannonBallExplosions = g.cannonBallExplosions[:0]
	g.damages = g.damages[:0]
}

func (g *Game) ExpectedOutputLineCount(playerIdx int) int {
	return len(g.players[playerIdx].shipsAlive)
}

// HandlePlayerOutput reads one command per living ship. Like Java, more
// lines than ships crashes the referee, and an invalid line kills every
// ship of the player before the framework marks it as lost.
func (g *Game) HandlePlayerOutput(frame, round, playerIdx int, outputs []string) error {
	p := g.players[playerIdx]
	for i, line := range outputs {
		s := p.shipsAlive[i]
		if m := playerInputMovePattern.FindStringSubmatch(line); m != nil {
			x, _ := jstrings.ParseInt(m[1])
			y, _ := jstrings.ParseInt(m[2])
			s.setMessage(m[3])
			s.moveTo(int(x), int(y))
		} else if m := playerInputFasterPattern.FindStringSubmatch(line); m != nil {
			s.setMessage(m[1])
			s.faster()
		} else if m := playerInputSlowerPattern.FindStringSubmatch(line); m != nil {
			s.setMessage(m[1])
			s.slower()
		} else if m := playerInputPortPattern.FindStringSubmatch(line); m != nil {
			s.setMessage(m[1])
			s.port()
		} else if m := playerInputStarboardPattern.FindStringSubmatch(line); m != nil {
			s.setMessage(m[1])
			s.starboard()
		} else if m := playerInputWaitPattern.FindStringSubmatch(line); m != nil {
			s.setMessage(m[1])
		} else if m := playerInputMinePattern.FindStringSubmatch(line); m != nil {
			s.setMessage(m[1])
			s.placeMine()
		} else if m := playerInputFirePattern.FindStringSubmatch(line); m != nil {
			x, _ := jstrings.ParseInt(m[1])
			y, _ := jstrings.ParseInt(m[2])
			s.setMessage(m[3])
			s.fire(int(x), int(y))
		} else {
			p.setDead()
			return cgref.InvalidInput("A valid action", line)
		}
	}
	return nil
}

func (g *Game) decrementRum() {
	for _, s := range g.ships {
		s.damage(1)
	}
}

func (g *Game) updateInitialRum() {
	for _, s := range g.ships {
		s.initialHealth = s.health
	}
}

func (g *Game) moveCannonballs() {
	kept := g.cannonballs[:0]
	for _, ball := range g.cannonballs {
		if ball.remainingTurns == 0 {
			continue
		} else if ball.remainingTurns > 0 {
			ball.remainingTurns--
		}
		if ball.remainingTurns == 0 {
			g.cannonBallExplosions = append(g.cannonBallExplosions, ball.position)
		}
		kept = append(kept, ball)
	}
	g.cannonballs = kept
}

func (g *Game) applyActions() {
	for _, p := range g.players {
		for _, s := range p.shipsAlive {
			if s.mineCooldown > 0 {
				s.mineCooldown--
			}
			if s.cannonCooldown > 0 {
				s.cannonCooldown--
			}

			s.newOrientation = s.orientation

			switch s.action {
			case actFaster:
				if s.speed < maxShipSpeed {
					s.speed++
				}
			case actSlower:
				if s.speed > 0 {
					s.speed--
				}
			case actPort:
				s.newOrientation = (s.orientation + 1) % 6
			case actStarboard:
				s.newOrientation = (s.orientation + 5) % 6
			case actMine:
				if s.mineCooldown == 0 {
					target := s.stern().neighbor((s.orientation + 3) % 6)
					if target.isInsideMap() {
						if g.cellIsFreeOfBarrels(target) && g.cellIsFreeOfShips(target, s) && g.cellIsFreeOfMines(target) {
							s.mineCooldown = cooldownMine
							g.mines = append(g.mines, g.newMine(target.x, target.y))
						}
					}
				}
			case actFire:
				distance := s.bow().distanceTo(s.target)
				if s.target.isInsideMap() && distance <= fireDistanceMax && s.cannonCooldown == 0 {
					travelTime := int(int32(1 + jmath.Round(float64(s.bow().distanceTo(s.target))/3.0)))
					bow := s.bow()
					g.cannonballs = append(g.cannonballs, g.newCannonball(s.target.x, s.target.y, s.id, bow.x, bow.y, travelTime))
					s.cannonCooldown = cooldownCannon
				}
			}
		}
	}
}

func (g *Game) checkBarrelCollisions(s *ship) {
	bow, stern, center := s.bow(), s.stern(), s.position
	kept := g.barrels[:0]
	for _, barrel := range g.barrels {
		if barrel.position == bow || barrel.position == stern || barrel.position == center {
			s.heal(barrel.health)
			continue
		}
		kept = append(kept, barrel)
	}
	g.barrels = kept
}

func (g *Game) checkMineCollisions() {
	kept := g.mines[:0]
	for _, m := range g.mines {
		if mineDamage := m.explode(g.ships, false); len(mineDamage) > 0 {
			g.damages = append(g.damages, mineDamage...)
			continue
		}
		kept = append(kept, m)
	}
	g.mines = kept
}

func (g *Game) checkCollisions() {
	// Check collisions with barrels
	for _, s := range g.ships {
		g.checkBarrelCollisions(s)
	}
	// Check collisions with mines
	g.checkMineCollisions()
}

func (g *Game) moveShips() {
	for i := 1; i <= maxShipSpeed; i++ {
		// Go forward
		for _, p := range g.players {
			for _, s := range p.shipsAlive {
				s.newPosition = s.position
				s.newBowCoordinate = s.bow()
				s.newSternCoordinate = s.stern()

				if i > s.speed {
					continue
				}

				newCoordinate := s.position.neighbor(s.orientation)
				if newCoordinate.isInsideMap() {
					// Set new coordinate.
					s.newPosition = newCoordinate
					s.newBowCoordinate = newCoordinate.neighbor(s.orientation)
					s.newSternCoordinate = newCoordinate.neighbor((s.orientation + 3) % 6)
				} else {
					// Stop ship!
					s.speed = 0
				}
			}
		}

		// Check ship and obstacles collisions
		for collisionDetected := true; collisionDetected; {
			collisionDetected = false
			var collisions []*ship
			for _, s := range g.ships {
				if s.newBowIntersectAny(g.ships) {
					collisions = append(collisions, s)
				}
			}
			for _, s := range collisions {
				// Revert last move
				s.newPosition = s.position
				s.newBowCoordinate = s.bow()
				s.newSternCoordinate = s.stern()
				// Stop ships
				s.speed = 0
				collisionDetected = true
			}
		}

		// Move ships to their new location
		for _, s := range g.ships {
			s.position = s.newPosition
		}

		g.checkCollisions()
	}
}

func (g *Game) rotateShips() {
	// Rotate
	for _, p := range g.players {
		for _, s := range p.shipsAlive {
			s.newPosition = s.position
			s.newBowCoordinate = s.newBow()
			s.newSternCoordinate = s.newStern()
		}
	}

	// Check collisions
	for collisionDetected := true; collisionDetected; {
		collisionDetected = false
		var collisions []*ship
		for _, s := range g.ships {
			if s.newPositionsIntersectAny(g.ships) {
				collisions = append(collisions, s)
			}
		}
		for _, s := range collisions {
			s.newOrientation = s.orientation
			s.newBowCoordinate = s.newBow()
			s.newSternCoordinate = s.newStern()
			s.speed = 0
			collisionDetected = true
		}
	}

	// Apply rotation
	for _, s := range g.ships {
		s.orientation = s.newOrientation
	}

	g.checkCollisions()
}

func (g *Game) gameIsOver() bool {
	for _, p := range g.players {
		if len(p.shipsAlive) == 0 {
			return true
		}
	}
	return len(g.barrels) == 0 && leagueLevel == 0
}

// explodeShips applies each exploding cannonball to the first ship it hits
// (in ship order) and drops that explosion.
func (g *Game) explodeShips() {
	kept := g.cannonBallExplosions[:0]
	for _, position := range g.cannonBallExplosions {
		hit := false
		for _, s := range g.ships {
			if position == s.bow() || position == s.stern() {
				g.damages = append(g.damages, damage{position, lowDamage, true})
				s.damage(lowDamage)
				hit = true
				break
			} else if position == s.position {
				g.damages = append(g.damages, damage{position, highDamage, true})
				s.damage(highDamage)
				hit = true
				break
			}
		}
		if !hit {
			kept = append(kept, position)
		}
	}
	g.cannonBallExplosions = kept
}

func (g *Game) explodeMines() {
	kept := g.cannonBallExplosions[:0]
	for _, position := range g.cannonBallExplosions {
		hit := false
		for i, m := range g.mines {
			if m.position == position {
				g.damages = append(g.damages, m.explode(g.ships, true)...)
				g.mines = append(g.mines[:i], g.mines[i+1:]...)
				hit = true
				break
			}
		}
		if !hit {
			kept = append(kept, position)
		}
	}
	g.cannonBallExplosions = kept
}

func (g *Game) explodeBarrels() {
	kept := g.cannonBallExplosions[:0]
	for _, position := range g.cannonBallExplosions {
		hit := false
		for i, b := range g.barrels {
			if b.position == position {
				g.damages = append(g.damages, damage{position, 0, true})
				g.barrels = append(g.barrels[:i], g.barrels[i+1:]...)
				hit = true
				break
			}
		}
		if !hit {
			kept = append(kept, position)
		}
	}
	g.cannonBallExplosions = kept
}

func (g *Game) UpdateGame(round int) error {
	g.moveCannonballs()
	g.decrementRum()
	g.updateInitialRum()

	g.applyActions()
	g.moveShips()
	g.rotateShips()

	g.explodeShips()
	g.explodeMines()
	g.explodeBarrels()

	// For each sunk ship, create a new rum barrel with the amount of rum the
	// ship had at the beginning of the turn (up to 30).
	for _, s := range g.ships {
		if s.health <= 0 {
			if reward := min(rewardRumBarrelValue, s.initialHealth); reward > 0 {
				g.barrels = append(g.barrels, g.newRumBarrel(s.position.x, s.position.y, reward))
			}
		}
	}

	for _, position := range g.cannonBallExplosions {
		g.damages = append(g.damages, damage{position, 0, false})
	}

	kept := g.ships[:0]
	for _, s := range g.ships {
		if s.health <= 0 {
			g.players[s.owner].removeAlive(s)
			continue
		}
		kept = append(kept, s)
	}
	g.ships = kept

	if g.gameIsOver() {
		return cgref.GameOver("endReached")
	}
	return nil
}

// removeAlive is shipsAlive.remove(ship): the first match, by identity.
func (p *player) removeAlive(s *ship) {
	for i, alive := range p.shipsAlive {
		if alive == s {
			p.shipsAlive = append(p.shipsAlive[:i], p.shipsAlive[i+1:]...)
			return
		}
	}
}

func (g *Game) PopulateMessages(m map[string]string) {
	m["endReached"] = "End reached"
}

func (g *Game) InitInputForPlayer(playerIdx int) []string { return nil }

// InputForPlayer lists the player's ships, then the opponent's, the mines
// near the player's ships, the cannonballs and the barrels, after the
// player's ship count and the entity count.
func (g *Game) InputForPlayer(round, playerIdx int) []string {
	own := g.players[playerIdx].shipsAlive
	var entities []string
	for _, s := range own {
		entities = append(entities, s.playerString(playerIdx))
	}
	for _, s := range g.players[(playerIdx+1)%2].shipsAlive {
		entities = append(entities, s.playerString(playerIdx))
	}
	for _, m := range g.mines {
		for _, s := range own {
			if s.position.distanceTo(m.position) <= mineVisibilityRange {
				entities = append(entities, m.playerString())
				break
			}
		}
	}
	for _, ball := range g.cannonballs {
		entities = append(entities, ball.playerString())
	}
	for _, b := range g.barrels {
		entities = append(entities, b.playerString())
	}
	return append([]string{strconv.Itoa(len(own)), strconv.Itoa(len(entities))}, entities...)
}

func (g *Game) InitDataForView() []string {
	return []string{"2", join(mapWidth, mapHeight, len(g.players[0].ships), mineVisibilityRange, g.seed)}
}

func (g *Game) FrameDataForView(round, frame int, keyFrame bool) []string {
	var data []string
	for _, p := range g.players {
		data = append(data, p.viewStrings()...)
	}
	data = append(data, strconv.Itoa(len(g.cannonballs)))
	for _, ball := range g.cannonballs {
		data = append(data, ball.viewString())
	}
	if !g.minesCreated {
		panic("java.lang.NullPointerException: mines is null")
	}
	data = append(data, strconv.Itoa(len(g.mines)))
	for _, m := range g.mines {
		data = append(data, m.toViewString())
	}
	if !g.barrelsCreated {
		panic("java.lang.NullPointerException: barrels is null")
	}
	data = append(data, strconv.Itoa(len(g.barrels)))
	for _, b := range g.barrels {
		data = append(data, b.viewString())
	}
	data = append(data, strconv.Itoa(len(g.damages)))
	for _, d := range g.damages {
		data = append(data, d.viewString())
	}
	return data
}

func (g *Game) GameName() string        { return "CodersOfTheCaribbean" }
func (g *Game) MinimumPlayerCount() int { return 2 }
func (g *Game) ShowTooltips() bool      { return true }
func (g *Game) DeathReason(playerIdx int) string {
	return "$" + strconv.Itoa(playerIdx) + ": Eliminated!"
}
func (g *Game) Score(playerIdx int) int              { return g.players[playerIdx].score() }
func (g *Game) SetPlayerTimeout(_, _, playerIdx int) { g.players[playerIdx].setDead() }
func (g *Game) MaxRoundCount(int) int                { return 200 }
func (g *Game) MillisTimeForRound() int              { return 50 }
