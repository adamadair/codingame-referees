// Package codebusters is a Go port of the official CodinGame Java referee
// for Code Busters (the multiplayer puzzle version, final league).
//
// Two teams of 2 to 5 busters hunt ghosts on a 16001x9001 map. A buster
// can BUST a ghost from 900 to 1760 units away, which wears down its
// stamina; at zero stamina the ghost is trapped and can be carried back to
// base and released there. Busters can also STUN each other, use a RADAR
// once, and EJECT a carried ghost. The score is the number of ghosts in
// the player's base plus those its busters are carrying.
//
// Given the same parameters (or seed) and the same bot outputs, this port
// gives players the same input and the same scores as the Java referee.
package codebusters

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/adamadair/codingame-referees/internal/cgref"
	"github.com/adamadair/codingame-referees/internal/jprops"
	"github.com/adamadair/codingame-referees/internal/jrandom"
	"github.com/adamadair/codingame-referees/internal/jstrings"
)

// The Java referee's constants. GAME_VERSION is hardcoded to 5 in the
// puzzle referee (CodinGame recompiles it for the lower leagues), so stun,
// ghost stamina, radar and eject are always on.
const (
	gameVersion = 5

	width                    = 16001
	height                   = 9001
	minBusters               = 2
	maxBusters               = 5
	spawnRadius              = 1600
	hqRadius                 = 1600
	spaceBetweenBusters      = 600
	maxExtraGhosts           = 8
	maxGhosts                = 30
	radarRange               = 4400
	grabRange                = 1760
	grabSafetyDistance       = 900
	stunRange                = 1760
	ghostMinSeparation       = 1200
	ghostMargin              = 200
	stunDuration             = 10
	stunCooldown             = 20
	canGrabGhostsInEnemySpwn = false
	obstacleMode             = 0
	radarsPerBuster          = 1
	collisionSteps           = 10
)

var defaultGhostStamina = []int{3, 15, 40}

// corners are the four bases, in player order.
var corners = []Vector{{0, 0}, {width - 1, height - 1}, {width - 1, 0}, {0, height - 1}}

// config holds the Java static fields that initReferee reads from the game
// parameters. One game runs per process, so they act as per-game settings.
type config struct {
	busterSpeed              int32
	busterSpeedWhileCarrying int32
	viewRange                int32
	ghostViewRange           int32
	ejectRange               int32
	ghostSpeed               int32
	centralGhost             bool
}

var javaDefaults = config{
	busterSpeed:              800,
	busterSpeedWhileCarrying: 800,
	viewRange:                2200,
	ghostViewRange:           2200,
	ejectRange:               1760,
	ghostSpeed:               400,
	centralGhost:             true,
}

// Game is the Code Busters referee. The zero value is ready to use; run it
// with Run.
type Game struct {
	cgref.Base

	// Clock seeds the random default seed when the runner sends none, in
	// milliseconds like Java's System.currentTimeMillis. Nil means the
	// current time.
	Clock func() int64

	config
	obstacleCount int // Obstacle.obstacleCount, a static counter in Java

	players                      []*PlayerInfo
	allBusters                   []*Buster
	bustersPerPlayer, ghostCount int
	seed                         int64
	random                       *jrandom.Random
	ghosts                       []*Ghost
	gameOver                     bool
	gameMap                      []*Obstacle // "map" in Java

	// Java creates the ghost list at the end of initReferee. If init fails
	// before that, the framework's init-error frame reads a null map or
	// ghost list and the referee stops without scores.
	ghostsCreated bool
}

// Run plays one game with a default Game.
func Run(in io.Reader, out io.Writer) error {
	return (&Game{}).Run(in, out)
}

// Run plays one game on the runner protocol. See cgref.Run for the error.
func (g *Game) Run(in io.Reader, out io.Writer) error { return g.runAs(g, in, out) }

// runAs runs game, which is g or a wrapper around it (tests use one to
// observe turns), with this framework copy's options.
func (g *Game) runAs(game cgref.Game, in io.Reader, out io.Writer) error {
	return cgref.Run(game, in, out, io.Discard, cgref.Options{UpdateScoresEachRound: true})
}

func (g *Game) clock() int64 {
	if g.Clock != nil {
		return g.Clock()
	}
	return time.Now().UnixMilli()
}

// parseInt is Integer.valueOf(prop.getProperty(key, String.valueOf(def))).
func parseInt(prop *jprops.Properties, key string, def int32) (int32, error) {
	return jstrings.ParseInt(prop.GetProperty(key, strconv.Itoa(int(def))))
}

// aioobe is the message of Java's ArrayIndexOutOfBoundsException.
func aioobe(index, length int) string {
	return fmt.Sprintf("Index %d out of bounds for length %d", index, length)
}

func (g *Game) InitReferee(playerCount int, prop *jprops.Properties) error {
	g.config = javaDefaults

	seed, err := jstrings.ParseLong(prop.GetProperty("seed", strconv.FormatInt(jrandom.New(g.clock()).NextLong(), 10)))
	if err != nil {
		return err
	}
	g.seed = seed
	g.random = jrandom.New(g.seed)

	ghostStamina := defaultGhostStamina // GAME_VERSION >= 3

	// Both defaults draw a random number even when the parameter is given.
	busters, err := parseInt(prop, "busters", g.random.NextIntN(maxBusters-minBusters)+minBusters)
	if err != nil {
		return err
	}
	g.bustersPerPlayer = max(minBusters, min(maxBusters, int(busters)))
	ghosts, err := parseInt(prop, "ghosts", int32(min(maxGhosts, g.bustersPerPlayer*4+int(g.random.NextIntN(maxExtraGhosts+1)))))
	if err != nil {
		return err
	}
	g.ghostCount = max(1, int(ghosts))
	for _, f := range []struct {
		field *int32
		key   string
	}{
		{&g.busterSpeed, "BUSTER_SPEED"},
		{&g.busterSpeedWhileCarrying, "BUSTER_SPEED_WHILE_CARRYING"},
		{&g.viewRange, "VIEW_RANGE"},
		{&g.ghostViewRange, "GHOST_VIEW_RANGE"},
		{&g.ghostSpeed, "GHOST_SPEED"},
	} {
		v, err := parseInt(prop, f.key, *f.field)
		if err != nil {
			return err
		}
		*f.field = v
	}
	// Boolean.valueOf: anything but "true" (ignoring case) is false.
	g.centralGhost = strings.EqualFold(prop.GetProperty("CENTRAL_GHOST", strconv.FormatBool(g.centralGhost)), "true")
	ejectRange, err := parseInt(prop, "EJECT_RANGE", g.ejectRange)
	if err != nil {
		return err
	}
	g.ejectRange = ejectRange

	g.gameMap = nil
	if mapCoords, ok := prop.Get("map"); !ok {
		g.gameMap = g.chooseMap()
	} else if err := g.parseMap(mapCoords); err != nil {
		return err
	}

	g.players = nil
	g.allBusters = nil

	busterID := 0
	directions := []Vector{{1, 1}, {-1, -1}, {-1, 1}, {1, -1}}
	spawnOffset := 2 * spawnRadius / 2

	for i := range playerCount {
		playerInfo := &PlayerInfo{index: i}
		g.players = append(g.players, playerInfo)

		vector := Vector{1, 1}
		if i < 2 {
			vector = Vector{1, -1}
		}
		vector = vector.normalize()
		if i%2 == 1 {
			vector = vector.mult(-1)
		}

		if i >= len(corners) {
			return errors.New(aioobe(i, len(corners)))
		}
		startPoint := corners[i]
		for j := range g.bustersPerPlayer {
			sign := 1
			if j%2 == 0 {
				sign = -1
			}
			offset := float64(sign*(j/2*2+1) + g.bustersPerPlayer%2)
			position := vector.mult(offset * spaceBetweenBusters).translate(startPoint).
				translate(directions[i].mult(float64(spawnOffset))).round()
			// Java calls snapToGameZone(position) here and discards the
			// result, so with 5 busters some start off the map.
			b := newBuster(busterID, j, position, playerInfo, directions[i].angle())
			busterID++
			playerInfo.busters = append(playerInfo.busters, b)
			g.allBusters = append(g.allBusters, b)
		}
	}

	// Generate ghosts
	iterations := 0
	g.ghosts = nil
	g.ghostsCreated = true
	ghostID := 0

	if g.centralGhost {
		stamina := 0
		if len(ghostStamina) > 0 {
			stamina = ghostStamina[g.random.NextIntN(int32(len(ghostStamina)))]
		}
		g.ghosts = append(g.ghosts, newGhost(ghostID, Vector{(width - 1) / 2, (height - 1) / 2}, ghostNormal, stamina))
		ghostID++
	}

	for ghostID < g.ghostCount {
		iterations++
		if iterations > 1000 {
			break
		}

		x := ghostMargin + int(g.random.NextIntN(width-ghostMargin*2))
		y := ghostMargin + int(g.random.NextIntN(height-ghostMargin*2))
		pos := Vector{float64(x), float64(y)}

		ok := true
		for _, ghost := range g.ghosts {
			if ghost.position.distance(pos) <= ghostMinSeparation {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		for k := range playerCount {
			// Java has VIEW_RANGE commented out here in favour of 2200.
			if corners[k].distance(pos) <= spawnRadius+2200 {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		for _, c := range g.gameMap {
			if c.position.distance(pos) <= float64(c.getRadius()) {
				ok = false
				break
			}
		}

		if ok {
			stamina := 0
			if len(ghostStamina) > 0 {
				stamina = ghostStamina[g.random.NextIntN(int32(len(ghostStamina)))]
			}
			g.ghosts = append(g.ghosts, newGhost(ghostID, pos, ghostNormal, stamina))
			ghostID++
			g.ghosts = append(g.ghosts, newGhost(ghostID, Vector{width - 1 - pos.getX(), height - 1 - pos.getY()}, ghostNormal, stamina))
			ghostID++
		}
	}
	g.ghostCount = ghostID
	return nil
}

// chooseMap returns no obstacles: OBSTACLE_MODE is 0. Java first builds
// its 13 fixed maps, 58 obstacles in all, which advances the static
// obstacle counter; the maps themselves are never used, so they aren't
// copied here.
func (g *Game) chooseMap() []*Obstacle {
	g.obstacleCount += 58
	if obstacleMode == 0 {
		return []*Obstacle{}
	}
	panic("obstacle maps are not ported")
}

// parseMap is Java's parsing of the "map" parameter. It always fails: the
// loop runs over every token, not every third one, and reads tab[i*2]
// where tab[i*3] was meant, so it either parses a bad token or reads past
// the end of the array. Java assigns the new array before the loop, which
// is mirrored for the init-error frame.
func (g *Game) parseMap(mapCoords string) error {
	tab := jstrings.Split(mapCoords, " ")
	g.gameMap = make([]*Obstacle, len(tab)/3)
	at := func(i int) (int, error) {
		if i >= len(tab) {
			return 0, errors.New(aioobe(i, len(tab)))
		}
		v, err := jstrings.ParseInt(tab[i])
		return int(v), err
	}
	for i := 0; i < len(tab); i++ {
		x, err := at(i * 2)
		if err != nil {
			return err
		}
		y, err := at(i*3 + 1)
		if err != nil {
			return err
		}
		r, err := at(i*3 + 2)
		if err != nil {
			return err
		}
		o := g.newObstacle(x, y, r)
		if i >= len(g.gameMap) {
			return errors.New(aioobe(i, len(g.gameMap)))
		}
		g.gameMap[i] = o
	}
	return nil
}

func (g *Game) Configuration() *jprops.Properties {
	p := jprops.New()
	p.Set("seed", strconv.FormatInt(g.seed, 10))
	p.Set("busters", strconv.Itoa(g.bustersPerPlayer))
	p.Set("ghosts", strconv.Itoa(g.ghostCount))
	return p
}

func (g *Game) InitInputForPlayer(playerIdx int) []string {
	return []string{strconv.Itoa(g.bustersPerPlayer), strconv.Itoa(g.ghostCount), strconv.Itoa(playerIdx)}
}

func (g *Game) Prepare(round int) {
	for _, player := range g.players {
		for _, buster := range player.busters {
			buster.reset()
		}
	}
}

// sightRange is how far a buster sees: farther for the round after a radar.
func (g *Game) sightRange(b *Buster) float64 {
	if b.radarTimeout > 0 {
		return radarRange
	}
	return float64(g.viewRange)
}

func (g *Game) InputForPlayer(round, playerIdx int) []string {
	player := g.players[playerIdx]
	var visibleEnemies []*Buster
	var visibleObstacles []*Obstacle
	var visibleGhosts []*Ghost

	for j := range g.players {
		if j == playerIdx {
			continue
		}
		for _, enemy := range g.players[j].busters {
			for _, b := range player.busters {
				if enemy.position.distance(b.position) <= g.sightRange(b) {
					visibleEnemies = append(visibleEnemies, enemy)
					break
				}
			}
		}
	}

	for _, ghost := range g.ghosts {
		for _, b := range player.busters {
			if !ghost.inHq && !ghost.isGrabbed() && ghost.position.distance(b.position) <= g.sightRange(b) {
				visibleGhosts = append(visibleGhosts, ghost)
				break
			}
		}
	}

	for _, o := range g.gameMap {
		for _, b := range player.busters {
			if o.position.distance(b.position) <= g.sightRange(b)+float64(o.getRadius()) {
				visibleObstacles = append(visibleObstacles, o)
				break
			}
		}
	}

	lines := []string{strconv.Itoa(g.bustersPerPlayer + len(visibleEnemies) + len(visibleGhosts) + len(visibleObstacles))}
	for _, b := range player.busters {
		lines = append(lines, b.toPlayerString())
	}
	for _, enemy := range visibleEnemies {
		lines = append(lines, enemy.toPlayerString())
	}
	for _, ghost := range visibleGhosts {
		lines = append(lines, ghost.toPlayerString())
	}
	for _, o := range visibleObstacles {
		lines = append(lines, o.toPlayerString())
	}
	return lines
}

func (g *Game) ExpectedOutputLineCount(int) int { return g.bustersPerPlayer }

// HandlePlayerOutput reads one command per buster. The first bad line,
// including one with a number too large for an int, kills the player; the
// busters before it keep their commands.
func (g *Game) HandlePlayerOutput(frame, round, playerIdx int, outputs []string) error {
	player := g.players[playerIdx]
	for i, line := range outputs {
		buster := player.busters[i]
		if !g.handleLine(buster, line) {
			// Java's catch (Exception) also catches its own "no pattern
			// matched" exception and treats every failure alike, even
			// setting a stunned buster to IDLE.
			buster.letGo()
			buster.invalidMove = true
			buster.confused = true
			buster.state = busterIdle
			player.dead = true
			return cgref.InvalidInput(expected, line)
		}
	}
	return nil
}

// limitTo is the MOVE and EJECT vector: the way to the target, cut to
// limit. limit*limit is an int product in Java and wraps for large limits.
func limitTo(from Vector, x, y int32, limit int32) *Vector {
	v := newVectorFrom(from, Vector{float64(x), float64(y)})
	if v.lengthSquared() <= float64(limit*limit) {
		return &v
	}
	v = v.normalize().mult(float64(limit))
	return &v
}

// handleLine applies one command and reports whether it was valid.
func (g *Game) handleLine(buster *Buster, line string) bool {
	if m := playerMovePattern.FindStringSubmatch(line); m != nil {
		x, errX := jstrings.ParseInt(m[1])
		if errX != nil {
			return false
		}
		y, errY := jstrings.ParseInt(m[2])
		if errY != nil {
			return false
		}
		if buster.position.getX() != float64(x) || buster.position.getY() != float64(y) {
			speed := g.busterSpeed
			if buster.state == busterCarrying {
				speed = g.busterSpeedWhileCarrying
			}
			buster.attemptedVector = limitTo(buster.position, x, y, speed)
			buster.attempt = actionMove
		}
		matchMessage(buster, m[3])
	} else if m := playerGrabPattern.FindStringSubmatch(line); m != nil {
		id, err := jstrings.ParseInt(m[1])
		if err != nil {
			return false
		}
		buster.attemptTargetId, buster.hasAttemptTargetId = int(id), true
		buster.attempt = actionGrab
		matchMessage(buster, m[2])
	} else if m := playerReleasePattern.FindStringSubmatch(line); m != nil {
		buster.attempt = actionRelease
		matchMessage(buster, m[1])
	} else if m := playerStunPattern.FindStringSubmatch(line); m != nil {
		id, err := jstrings.ParseInt(m[1])
		if err != nil {
			return false
		}
		buster.attempt = actionStun
		buster.attemptTargetId, buster.hasAttemptTargetId = int(id), true
		matchMessage(buster, m[2])
	} else if m := playerRadarPattern.FindStringSubmatch(line); m != nil {
		buster.attempt = actionRadar
		matchMessage(buster, m[1])
	} else if m := playerEjectPattern.FindStringSubmatch(line); m != nil {
		// Set before the coordinates are parsed, so it stays when they
		// overflow.
		buster.attempt = actionEject
		x, errX := jstrings.ParseInt(m[1])
		if errX != nil {
			return false
		}
		y, errY := jstrings.ParseInt(m[2])
		if errY != nil {
			return false
		}
		buster.attemptedVector = limitTo(buster.position, x, y, g.ejectRange)
		matchMessage(buster, m[3])
	} else {
		return false
	}
	return true
}

// matchMessage keeps a command's message, cut to 17 UTF-16 units plus "..."
// when longer than 19.
func matchMessage(buster *Buster, message string) {
	if jstrings.Len16(message) > 19 {
		message = jstrings.Substring16(message, 0, 17) + "..."
	}
	buster.message = message
}

// computeCollisions is Referee.computeCollisions. Java never calls it (the
// call is commented out and there are no obstacles); it is kept so the
// port matches the source.
func (g *Game) computeCollisions(buster *Buster, newPos Vector) Vector {
	var collidesWith []*Obstacle
	for _, o := range g.gameMap {
		if o.position.distance(newPos) <= float64(o.getRadius()) {
			collidesWith = append(collidesWith, o)
		}
	}
	if len(collidesWith) > 0 {
		v := newVectorFrom(buster.position, newPos)
		smallest := 1.0
		for _, o := range collidesWith {
			a, b, best := 0.0, 1.0, 0.0
			for range collisionSteps {
				next := (a + b) / 2
				if buster.position.add(v.mult(next)).distance(o.position) <= float64(o.getRadius()) {
					b = next
				} else {
					a = next
					best = a
				}
			}
			if best < smallest {
				smallest = best
			}
		}
		return buster.position.add(v.mult(smallest))
	}
	return newPos
}

type stun struct{ stunner, stunned *Buster }

func (g *Game) UpdateGame(round int) error {
	if g.gameOver {
		// Unreachable: the framework ends the game as soon as gameOver is
		// set. In Java, translating the unknown "end" code would throw.
		return cgref.GameOver("end")
	}

	grabDemand := map[*Ghost][]*Buster{}
	// A HashMap keyed by identity-hashed busters in Java. Each stunner is
	// one key and the stuns don't depend on each other, so iterating in
	// buster order gives the same game; only the count of "letting go of
	// nothing" debug lines could be interleaved differently.
	var stunMap []stun

	for _, buster := range g.allBusters {
		if buster.state == busterStunned {
			continue
		}
		switch buster.attempt {
		case actionGrab:
			buster.letGo()
			var target *Ghost
			if buster.attemptTargetId >= 0 && buster.attemptTargetId < len(g.ghosts) {
				target = g.ghosts[buster.attemptTargetId]
				dist := target.position.distance(buster.position)
				if dist < grabSafetyDistance || dist > grabRange {
					if grabSafetyDistance > dist {
						buster.tooClose = true
					}
					target = nil
				} else if !canGrabGhostsInEnemySpwn {
					// Check if in hq
					if target.isCaptured() {
						target = nil
					}
				}
			}

			if target == nil {
				buster.confused = true
			} else {
				grabDemand[target] = append(grabDemand[target], buster)
				buster.rotation = newVectorFrom(buster.position, target.position).angle()
				buster.success = true
				buster.ghost = target
			}
		case actionMove:
			newPos := snapToGameZone(buster.position.add(*buster.attemptedVector)).round()
			buster.nextPosition = &newPos
		case actionEject:
			if buster.state == busterCarrying {
				target := snapToGameZone(buster.position.add(*buster.attemptedVector)).round()
				buster.success = true
				buster.ghost.position = target
				buster.ghost.inHq = false
				buster.letGo()
			} else {
				buster.confused = true
			}
		case actionRadar:
			if buster.radars > 0 {
				buster.success = true
				buster.radars--
				buster.radarTimeout = 2
			} else {
				buster.confused = true
			}
		case actionRelease:
			if buster.state == busterCarrying {
				buster.success = true
				buster.letGo()
			} else {
				buster.confused = true
			}
		case actionStun:
			if buster.state == busterGrabbing {
				buster.state = busterIdle
			}
			var target *Buster
			if buster.stunRecharge == 0 && buster.attemptTargetId >= 0 && buster.attemptTargetId < len(g.allBusters) {
				target = g.allBusters[buster.attemptTargetId]
				if target.position.distance(buster.position) > stunRange {
					target = nil
				}
			}
			if target == nil {
				buster.confused = true
			} else {
				buster.success = true
				buster.stunRecharge = stunCooldown
				buster.letGo()
				stunMap = append(stunMap, stun{buster, target})
			}
		}
	}
	for _, buster := range g.allBusters {
		if buster.nextPosition != nil {
			buster.rotation = newVectorFrom(buster.position, *buster.nextPosition).angle()
			buster.position = *buster.nextPosition
		}
		if buster.stunCountDown > 0 {
			buster.stunCountDown--
			if buster.stunCountDown == 0 {
				buster.state = busterIdle
			}
		}
		if buster.stunRecharge > 0 {
			buster.stunRecharge--
		}
		if buster.state == busterGrabbing && buster.attempt != actionGrab {
			buster.state = busterIdle
		}
	}

	for _, ghost := range g.ghosts {
		ghost.wasSuffering = ghost.suffering
		ghost.suffering = false

		ghost.attemptedGrabs = 0

		if !ghost.isGrabbed() && !ghost.escaping {
			grabbers := grabDemand[ghost]
			if len(grabbers) > 0 {
				ghost.decreaseStamina(len(grabbers))
				ghost.attemptedGrabs = len(grabbers)
				ghost.suffering = true
				if ghost.getStamina() == 0 {
					winningTeam := -1 // null
					maxGrabs := 0
					teamGrabs := make([]int, len(g.players))
					for _, b := range grabbers {
						x := teamGrabs[b.owner.index] + 1
						teamGrabs[b.owner.index] = x
						if x > maxGrabs {
							maxGrabs = x
							winningTeam = b.owner.index
						} else if x == maxGrabs {
							winningTeam = -1
						}
					}

					if winningTeam != -1 {
						minDist := math.MaxFloat64
						var closest *Buster
						for _, b := range grabbers {
							if b.owner.index != winningTeam {
								continue
							}
							dist := b.position.distance(ghost.position)
							if dist < minDist {
								minDist = dist
								closest = b
							}
						}
						for _, b := range grabbers {
							b.state = busterIdle
							b.finishGrab = true
						}
						closest.state = busterCarrying
						ghost.grabbedBy = closest
					} else {
						for _, b := range grabbers {
							b.state = busterGrabbing
						}
					}
				} else {
					for _, b := range grabbers {
						b.state = busterGrabbing
					}
				}
			}
		}

		if ghost.isGrabbed() {
			ghost.position = ghost.grabbedBy.position

			inHqRange := false
			for i := range g.players {
				if ghost.position.distance(corners[i]) <= hqRadius {
					inHqRange = true
					break
				}
			}
			ghost.inHq = inHqRange
			ghost.flagUpdated()
		} else if ghost.suffering != ghost.wasSuffering {
			ghost.flagUpdated()
		}
	}

	for _, s := range stunMap {
		stunned := s.stunned
		if stunned.state == busterCarrying {
			if stunned.ghost != nil && stunned.ghost.grabbedBy == stunned {
				stunned.ghost.grabbedBy = nil
				stunned.ghost.flagUpdated()
			} else {
				g.Ref().PrintError("Error: letting go of nothing")
			}
		}
		stunned.stun(stunDuration)
		s.stunner.rotation = newVectorFrom(s.stunner.position, s.stunned.position).angle()
	}

	// Frightened ghosts
	for _, ghost := range g.ghosts {
		ghost.escaping = false
		if !ghost.isGrabbed() && !ghost.isCaptured() && !ghost.suffering {
			if ghost.frightenedBy != nil {
				move := newVectorFrom(*ghost.frightenedBy, ghost.position).normalize().mult(float64(g.ghostSpeed))
				ghost.position = snapToGameZone(ghost.position.add(move)).round()
				ghost.flagUpdated()
			}

			var closest []*Buster
			minDist := 0.0
			for _, b := range g.allBusters {
				dist := b.position.distance(ghost.position)
				if len(closest) == 0 {
					closest = append(closest, b)
					minDist = dist
				} else if dist == minDist {
					closest = append(closest, b)
				} else if dist < minDist {
					closest = append(closest[:0], b)
					minDist = dist
				}
			}
			if minDist <= float64(g.ghostViewRange) {
				x, y := 0.0, 0.0
				for _, b := range closest {
					x += b.position.getX()
					y += b.position.getY()
				}
				x /= float64(len(closest))
				y /= float64(len(closest))

				ghost.frightenedBy = &Vector{x, y}
				if ghost.position.round().equals(Vector{x, y}.round()) {
					ghost.frightenedBy = nil
				}
			} else {
				ghost.frightenedBy = nil
			}
		} else {
			ghost.frightenedBy = nil
		}
	}

	if !canGrabGhostsInEnemySpwn && g.allGhostsCaptured() {
		g.gameOver = true
	}

	// Java sizes these for two players, although it accepts up to four.
	scores := []int{g.Score(0), g.Score(1)}
	captures := []int{g.getGhostCount(0, false), g.getGhostCount(1, false)}
	score := func(i int) int {
		if i >= len(scores) {
			panic("java.lang.ArrayIndexOutOfBoundsException: " + aioobe(i, len(scores)))
		}
		return scores[i]
	}

	deads := 0
	var alive *PlayerInfo
	for _, player := range g.players {
		if player.dead {
			deads++
		} else {
			alive = player
		}
	}
	if deads == len(g.players)-1 {
		capturedByDead := 0
		for _, other := range g.players {
			if alive == other {
				continue
			}
			capturedByDead += score(other.index)
		}
		if score(alive.index) > capturedByDead {
			g.gameOver = true
		}
	} else if deads == len(g.players) {
		g.gameOver = true
	}

	// Early stop once a player has more than half the ghosts.
	if captures[0] > len(g.ghosts)-captures[0] || captures[1] > len(g.ghosts)-captures[1] {
		g.gameOver = true
	}
	return nil
}

func (g *Game) allGhostsCaptured() bool {
	for _, ghost := range g.ghosts {
		if !ghost.isCaptured() {
			return false
		}
	}
	return true
}

func snapToGameZone(v Vector) Vector {
	snapX, snapY := v.getX(), v.getY()
	if snapX < 0 {
		snapX = 0
	}
	if snapX >= width {
		snapX = width - 1
	}
	if snapY < 0 {
		snapY = 0
	}
	if snapY >= height {
		snapY = height - 1
	}
	return Vector{snapX, snapY}
}

func (g *Game) PopulateMessages(p map[string]string) {
	p["invalid"] = "Buster %d of player $%d was given an invalid command"
	p["stunned"] = "Buster %d of player $%d is stunned"
	p["move"] = "Buster %d of player $%d moves to (%d,%d)"
	p["moveAndBust"] = "Buster %d of player $%d moves to (%d,%d) while carrying ghost %d"
	p["bustError"] = "Buster %d of player $%d wants to bust ghost %d but can't see it."
	p["bustTooClose"] = "Buster %d of player $%d wants to bust ghost %d but is too close."
	p["bust"] = "Buster %d of player $%d trapped ghost %d."
	p["busting"] = "Buster %d of player $%d is busting ghost %d."
	p["stun"] = "Buster %d of player $%d stuns buster %d of player $%d."
	p["autostun"] = "Buster %d of player $%d stuns himself! What a clutz!."
	p["stunError"] = "Buster %d of player $%d wants to stun buster %d but cannot."
	p["stunErrorRecharge"] = "Buster %d of player $%d wants to stun buster %d but needs more time."
	p["release"] = "Buster %d of player $%d released ghost %d."
	p["radar"] = "Buster %d of player $%d used his radar."
}

func (g *Game) InitDataForView() []string {
	if !g.ghostsCreated {
		panic("java.lang.NullPointerException: map or ghosts is null")
	}
	lines := []string{fmt.Sprintf("%d %d %d %d %d %d %d %d", width, height, hqRadius, g.bustersPerPlayer, g.ghostCount,
		g.viewRange, grabRange, len(g.gameMap))}
	for _, ghost := range g.ghosts {
		lines = append(lines, ghost.toInitFrameString())
	}
	for _, o := range g.gameMap {
		lines = append(lines, o.toInitFrameString())
	}
	return append([]string{strconv.Itoa(len(lines) + 1)}, lines...)
}

func (g *Game) FrameDataForView(round, frame int, keyFrame bool) []string {
	if !keyFrame {
		// This will never ever happen. Not in a million years.
		return []string{"X"}
	}
	var lines []string
	for _, player := range g.players {
		lines = append(lines, strconv.Itoa(g.getGhostCount(player.index, false)), strconv.Itoa(g.Score(player.index)))
		for _, b := range player.busters {
			lines = append(lines, b.toFrameString())
		}
	}
	var updatedGhosts []*Ghost
	for _, ghost := range g.ghosts {
		if ghost.hasMoved(round) {
			updatedGhosts = append(updatedGhosts, ghost)
		}
	}
	lines = append(lines, strconv.Itoa(len(updatedGhosts)))
	for _, ghost := range updatedGhosts {
		lines = append(lines, ghost.toFrameString())
	}
	return lines
}

func (g *Game) GameName() string            { return "CodeBusters" }
func (g *Game) HeadlineAtGameStart() string { return "Who you gonna call... ?" }
func (g *Game) MinimumPlayerCount() int     { return 2 }
func (g *Game) ShowTooltips() bool          { return true }
func (g *Game) MillisTimeForRound() int     { return 100 }
func (g *Game) MaxRoundCount(int) int       { return 250 }

// GameOver is the game's gameOver() override, which replaces the
// framework's alive-player check.
func (g *Game) GameOver() bool { return g.gameOver }

func (g *Game) GameSummary(round int) []string {
	var lines []string
	for i := range g.players {
		lines = append(lines, g.playerSummary(i)...)
	}
	return lines
}

func (g *Game) playerSummary(playerIdx int) []string {
	var lines []string
	player := g.players[playerIdx]
	tr := g.Ref().Translate
	for _, b := range player.busters {
		px, py := int(b.position.getX()), int(b.position.getY())
		switch {
		case b.invalidMove:
			lines = append(lines, tr("invalid", b.id, player.index))
		case b.state == busterStunned:
			lines = append(lines, tr("stunned", b.id, player.index))
		case b.attemptedMove() && b.attemptedVector != nil:
			if b.state == busterCarrying {
				if b.ghost != nil {
					lines = append(lines, tr("moveAndBust", b.id, player.index, px, py, b.ghost.id))
				} else {
					g.Ref().PrintError("Error: carrying no ghost?")
				}
			} else {
				lines = append(lines, tr("move", b.id, player.index, px, py))
			}
		case b.attempt == actionStun:
			if b.success {
				if b.attemptTargetId == b.id {
					lines = append(lines, tr("autostun", b.id, player.index))
				} else {
					lines = append(lines, tr("stun", b.id, player.index, b.attemptTargetId, g.allBusters[b.attemptTargetId].owner.index))
				}
			} else if b.stunRecharge > 0 {
				lines = append(lines, tr("stunErrorRecharge", b.id, player.index, b.attemptTargetId))
			} else {
				lines = append(lines, tr("stunError", b.id, player.index, b.attemptTargetId))
			}
		case b.state == busterGrabbing:
			lines = append(lines, tr("busting", b.id, player.index, b.ghost.id))
		case b.trappedGhost():
			lines = append(lines, tr("bust", b.id, player.index, b.ghost.id))
		case b.attempt == actionRelease && b.success:
			lines = append(lines, tr("release", b.id, player.index, b.ghost.id))
		case b.attemptedGrab() && !b.success && b.hasAttemptTargetId:
			if b.tooClose {
				lines = append(lines, tr("bustTooClose", b.id, player.index, b.attemptTargetId))
			} else {
				lines = append(lines, tr("bustError", b.id, player.index, b.attemptTargetId))
			}
		case b.radarSucceeded():
			lines = append(lines, tr("radar", b.id, player.index))
		}
	}
	return lines
}

func (g *Game) IsPlayerDead(playerIdx int) bool { return g.players[playerIdx].dead }

func (g *Game) DeathReason(int) string { return "Timeout!" }

func (g *Game) getGhostCount(playerIdx int, countTrapped bool) int {
	count := 0
	for _, ghost := range g.ghosts {
		if ghost.position.distance(corners[playerIdx]) <= hqRadius && ghost.isCaptured() ||
			countTrapped && ghost.isTrappedBy(g.players[playerIdx]) {
			count++
		}
	}
	return count
}

// Score is the ghosts in the player's base plus those its busters carry.
func (g *Game) Score(playerIdx int) int { return g.getGhostCount(playerIdx, true) }

func (g *Game) SetPlayerTimeout(_, _, playerIdx int) { g.players[playerIdx].dead = true }
