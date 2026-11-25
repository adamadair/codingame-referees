// Package hypersonic is a Go port of the official CodinGame Java referee
// for Hypersonic.
//
// Two to four players walk a 13x11 grid of floors, blocks and boxes, drop
// bombs that blow up after 8 turns in a cross, and collect the items that
// destroyed boxes leave behind. Bombs set off other bombs. The score is the
// number of boxes destroyed, and staying alive beats everything.
//
// Given the same parameters (or seed) and the same bot outputs, this port
// gives players the same input and the same scores as the Java referee.
package hypersonic

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"time"

	"github.com/adamadair/codingame-referees/internal/cgref"
	"github.com/adamadair/codingame-referees/internal/jcollections"
	"github.com/adamadair/codingame-referees/internal/jprops"
	"github.com/adamadair/codingame-referees/internal/jrandom"
	"github.com/adamadair/codingame-referees/internal/jstrings"
)

const (
	symmetryMapOnly     = 0
	symmetryMapAndItems = 1
	symmetryNone        = 2
	defaultTimeLimit    = 200

	moveModeStupid = 0
	moveModeBFS    = 1

	noProgressTimeout       = 20
	extraRoundsAfterNoBoxes = 20

	moveWithBomb = true
)

// config holds the Java referee's static fields. Java changes them in
// initReferee from the game parameters; one game runs per process, so they
// act as per-game settings.
type config struct {
	gameVersion           int
	items                 bool
	visibleItems          bool
	friendlyFire          bool
	endOnNoBoxes          bool
	countdownOnNoBoxes    bool
	practiceMode          bool
	width, height         int
	defaultBombPower      int
	defaultBombsPerPlayer int
	bombFuse              int
	symmetry              int
	suddenDeath           int
	suddenDeathSlowness   int
	moveMode              int
}

// javaDefaults are the static initializers.
var javaDefaults = config{
	gameVersion:           4,
	visibleItems:          true,
	endOnNoBoxes:          true,
	practiceMode:          true,
	width:                 13,
	height:                11,
	defaultBombPower:      3,
	defaultBombsPerPlayer: 1,
	bombFuse:              8,
	symmetry:              symmetryMapAndItems,
	suddenDeath:           -1,
	suddenDeathSlowness:   10,
	moveMode:              moveModeBFS,
}

// Game is the Hypersonic referee. The zero value is ready to use; run it
// with Run.
type Game struct {
	cgref.Base

	// Clock seeds the random default seed when the runner sends none, in
	// milliseconds like Java's System.currentTimeMillis. Nil means the
	// current time.
	Clock func() int64

	config
	itemCount, bombCount int

	seed            int64
	boxCount        int
	random          *jrandom.Random
	grid            map[Coord]*Cell
	players         []*PlayerInfo
	activeBombs     []*Bomb
	activeItems     []*Item
	removedItems    []*Item // a HashSet of identity-hashed items in Java; view only
	boxes           []*Cell
	explodedBoxes   *jcollections.HashSet[*Cell]
	activeMen       []*Man
	explosionGroups [][]*Explosion

	countDown, noProgressCountDown, extraCountDown int

	// Java creates its lists partway through initReferee. If a bad seed or
	// GAME_VERSION fails init before that, the framework's init-error frame
	// reads players == null and the referee stops without scores.
	listsCreated bool
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

// setInt is field = Integer.valueOf(prop.getProperty(key, String.valueOf(def))).
// Like Java, the field keeps its value when parsing fails, which matters
// because the init-error frame still shows WIDTH and HEIGHT.
func setInt(field *int, prop *jprops.Properties, key string, def int) error {
	v, err := jstrings.ParseInt(prop.GetProperty(key, strconv.Itoa(def)))
	if err != nil {
		return err
	}
	*field = int(v)
	return nil
}

func cellHash(c *Cell) int32 { return c.hashCode() }

func (g *Game) InitReferee(playerCount int, prop *jprops.Properties) error {
	g.config = javaDefaults

	seed, err := jstrings.ParseLong(prop.GetProperty("seed", strconv.FormatInt(jrandom.New(g.clock()).NextLong(), 10)))
	if err != nil {
		return err
	}
	g.seed = seed
	g.random = jrandom.New(g.seed)

	if err := setInt(&g.gameVersion, prop, "GAME_VERSION", g.gameVersion); err != nil {
		return err
	}
	if g.gameVersion >= 2 {
		g.items = true
	}
	if g.gameVersion >= 3 {
		g.practiceMode = false
		g.endOnNoBoxes = false
	}
	if g.gameVersion >= 4 {
		g.friendlyFire = true
		g.countdownOnNoBoxes = true
	}

	g.grid = map[Coord]*Cell{}
	g.players = nil
	g.explodedBoxes = jcollections.NewHashSet(cellHash)
	g.boxes = nil
	g.activeMen = nil
	g.activeBombs = nil
	g.activeItems = nil
	g.removedItems = nil
	g.explosionGroups = nil
	g.listsCreated = true

	// Computed before WIDTH and HEIGHT are read, so they always use 13x11.
	startingPositions := []Coord{{0, 0}, {g.width - 1, g.height - 1}, {g.width - 1, 0}, {0, g.height - 1}}
	allDirections := []Coord{{0, -1}, {-1, 0}, {0, 1}, {1, 0}, {-1, -1}, {-1, 1}, {1, -1}, {1, 1}}

	if err := setInt(&g.width, prop, "WIDTH", g.width); err != nil {
		return err
	}
	if err := setInt(&g.height, prop, "HEIGHT", g.height); err != nil {
		return err
	}
	// The default draws a random number even when "boxes" is given.
	if err := setInt(&g.boxCount, prop, "boxes", 30+int(g.random.NextIntN(36))); err != nil {
		return err
	}
	if err := setInt(&g.bombFuse, prop, "BOMB_FUSE", g.bombFuse); err != nil {
		return err
	}
	if err := setInt(&g.defaultBombPower, prop, "DEFAULT_BOMB_POWER", g.defaultBombPower); err != nil {
		return err
	}
	if err := setInt(&g.defaultBombsPerPlayer, prop, "DEFAULT_BOMBS_PER_PLAYER", g.defaultBombsPerPlayer); err != nil {
		return err
	}

	g.countDown = g.suddenDeath
	g.noProgressCountDown = noProgressTimeout
	g.extraCountDown = extraRoundsAfterNoBoxes

	for y := 0; y < g.height; y++ {
		for x := 0; x < g.width; x++ {
			cellType := cellFloor
			if x%2 == 1 && y%2 == 1 {
				cellType = cellBlock
			}
			if g.practiceMode {
				cellType = cellFloor
			}
			cell := newCell(x, y, cellType)
			g.grid[cell.Coord] = cell
		}
	}

	if g.practiceMode {
		g.boxCount = 30
	}

	// A LinkedHashMap in Java: the loop below takes the last type with
	// items left, so fire-ups are placed before bomb-ups.
	toDistribute := []struct{ itemType, count int }{{itemBombUp, 0}, {itemFireUp, 0}}
	if g.items {
		toDistribute[0].count = g.boxCount / 3
		toDistribute[1].count = g.boxCount / 3
	}
	decrement := func(itemType int) {
		for i := range toDistribute {
			if toDistribute[i].itemType == itemType {
				toDistribute[i].count--
			}
		}
	}

	iterations := 0
	boxID := 0
	for boxID < g.boxCount {
		iterations++
		if iterations > 1000 {
			break
		}

		x := int(g.random.NextIntN(int32(g.width)))
		y := int(g.random.NextIntN(int32(g.height)))

		ok := true
		for _, p := range startingPositions {
			if p.manhattanTo(Coord{x, y}) < 2 {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		cell := g.grid[Coord{x, y}]
		if cell.typ != cellFloor {
			continue
		}
		if g.practiceMode {
			nextToBox := false
			for _, d := range allDirections {
				if neighb := g.grid[Coord{x + d.x, y + d.y}]; neighb != nil && neighb.isBox() {
					nextToBox = true
					break
				}
			}
			if nextToBox {
				continue
			}
		}

		cell.typ = cellBox

		itemType := 0 // Java's null
		for _, d := range toDistribute {
			if d.count > 0 {
				itemType = d.itemType
			}
		}
		if itemType != 0 {
			decrement(itemType)
			cell.item = g.instantiate(itemType)
		}

		cell.boxID = boxID
		boxID++
		g.boxes = append(g.boxes, cell)
		if g.symmetry != symmetryNone {
			otherCells := []*Cell{
				g.grid[Coord{g.width - x - 1, y}],
				g.grid[Coord{g.width - x - 1, g.height - y - 1}],
				g.grid[Coord{x, g.height - y - 1}],
			}
			for _, c := range otherCells {
				if c.typ != cellBox {
					c.typ = cellBox
					if g.symmetry == symmetryMapAndItems && cell.hasItem() {
						c.item = g.copyItem(cell.item)
						decrement(itemType)
					}
					c.boxID = boxID
					boxID++
					g.boxes = append(g.boxes, c)
				}
			}
		}
	}
	g.boxCount = boxID

	for i := range playerCount {
		if i >= len(startingPositions) { // ArrayIndexOutOfBoundsException in Java
			return fmt.Errorf("Index %d out of bounds for length %d", i, len(startingPositions))
		}
		player := g.newPlayerInfo(i, startingPositions[i], g.defaultBombsPerPlayer, g.defaultBombPower)
		g.players = append(g.players, player)
		g.activeMen = append(g.activeMen, player.man)
	}
	return nil
}

func (g *Game) Configuration() *jprops.Properties {
	p := jprops.New()
	p.Set("seed", strconv.FormatInt(g.seed, 10))
	p.Set("boxes", strconv.Itoa(g.boxCount))
	return p
}

func (g *Game) Prepare(round int) {
	for _, p := range g.players {
		p.man.reset()
	}
}

func (g *Game) InitInputForPlayer(playerIdx int) []string {
	return []string{strconv.Itoa(g.width) + " " + strconv.Itoa(g.height) + " " + strconv.Itoa(playerIdx)}
}

func (g *Game) InputForPlayer(round, playerIdx int) []string {
	var lines []string
	for y := 0; y < g.height; y++ {
		row := make([]byte, 0, g.width)
		for x := 0; x < g.width; x++ {
			row = append(row, g.grid[Coord{x, y}].toPlayerString(g.visibleItems)...)
		}
		lines = append(lines, string(row))
	}

	var entities []string
	for _, p := range g.players {
		if !p.dead {
			entities = append(entities, p.man.toPlayerString())
		}
	}
	for _, b := range g.activeBombs {
		entities = append(entities, b.toPlayerString())
	}
	for _, i := range g.activeItems {
		entities = append(entities, i.toPlayerString())
	}
	lines = append(lines, strconv.Itoa(len(entities)))
	return append(lines, entities...)
}

func (g *Game) ExpectedOutputLineCount(int) int { return 1 }

// HandlePlayerOutput reads "MOVE x y" or "BOMB x y". Any failure, including
// a number too large for an int, kills the player.
func (g *Game) HandlePlayerOutput(frame, round, playerIdx int, outputs []string) error {
	player := g.players[playerIdx]
	man := player.man
	line := outputs[0]

	invalid := func() error {
		player.dead = true
		player.deadAt = round
		player.invalidAction = true
		return cgref.InvalidInput(expected, line)
	}

	if m := playerMovePattern.FindStringSubmatch(line); m != nil {
		x, errX := jstrings.ParseInt(m[1])
		if errX != nil {
			return invalid()
		}
		y, errY := jstrings.ParseInt(m[2])
		if errY != nil {
			return invalid()
		}
		if man.getX() != int(x) || man.getY() != int(y) {
			man.attempt = attemptMove
			man.attemptCoord = &Coord{int(x), int(y)}
		}
		matchMessage(man, m[3])
	} else if m := playerBombPattern.FindStringSubmatch(line); m != nil {
		man.attempt = attemptBomb
		x, errX := jstrings.ParseInt(m[1])
		if errX != nil {
			return invalid()
		}
		y, errY := jstrings.ParseInt(m[2])
		if errY != nil {
			return invalid()
		}
		if man.getX() != int(x) || man.getY() != int(y) {
			man.attemptCoord = &Coord{int(x), int(y)}
		}
		matchMessage(man, m[3])
	} else {
		return invalid()
	}
	return nil
}

// matchMessage keeps a command's message, cut to 17 UTF-16 units plus "..."
// when longer than 19.
func matchMessage(man *Man, message string) {
	if jstrings.Len16(message) > 19 {
		message = jstrings.Substring16(message, 0, 17) + "..."
	}
	man.message = message
}

// moveStupid is Referee.moveStupid. Java never calls it (MOVE_MODE is BFS);
// it is kept so the port matches the source.
func (g *Game) moveStupid(man *Man) bool {
	if man.lastDestination == nil || *man.lastDestination != *man.attemptCoord {
		man.lastOccupiedPosition = nil
		man.lastDestination = man.attemptCoord
	}
	var best, lastResort *Direction
	by := 0
	for _, d := range directions {
		neighb := man.position.add(d.getX(), d.getY())
		c := g.grid[neighb]
		if c != nil && c.isFree() {
			if neighb == *man.attemptCoord {
				best = &d
				break
			} else if man.lastOccupiedPosition != nil && neighb == *man.lastOccupiedPosition {
				lastResort = &d
			} else {
				dist := c.manhattanTo(*man.attemptCoord)
				if best == nil || dist < by {
					by = dist
					best = &d
				}
			}
		}
	}
	if best == nil {
		best = lastResort
	}
	if best != nil {
		man.nextMove = best
		return true
	}
	man.lastOccupiedPosition = nil
	return false
}

// moveBFS walks a breadth-first search from the man toward his target,
// through free cells, and steps toward the reached cell closest to the
// target (by Manhattan distance, the first found on ties).
func (g *Game) moveBFS(man *Man) bool {
	type link struct {
		from  Coord
		start bool
	}
	fifo := []Coord{man.position}
	route := map[Coord]link{man.position: {start: true}}
	closest := man.position
	by := man.position.manhattanTo(*man.attemptCoord)
	foundTarget := false
	for !foundTarget && len(fifo) > 0 {
		e := fifo[0]
		fifo = fifo[1:]
		for _, d := range directions {
			n := e.add(d.getX(), d.getY())
			c := g.grid[n]
			if c == nil || !c.isFree() {
				continue
			}
			if _, seen := route[c.Coord]; seen {
				continue
			}
			if *man.attemptCoord == n {
				foundTarget = true
			}
			route[c.Coord] = link{from: e}
			fifo = append(fifo, c.Coord)
			if distance := c.manhattanTo(*man.attemptCoord); distance < by {
				by = distance
				closest = c.Coord
			}
		}
	}

	cur, prev := closest, closest
	for cur != man.position {
		prev = cur
		cur = route[cur].from
	}
	var next Direction
	switch {
	case man.getX() < prev.x:
		next = dirRight
	case man.getX() > prev.x:
		next = dirLeft
	case man.getY() < prev.y:
		next = dirDown
	case man.getY() > prev.y:
		next = dirUp
	default:
		return true // already at the closest cell: no step
	}
	man.nextMove = &next
	return true
}

func (g *Game) move(man *Man) bool {
	switch g.moveMode {
	case moveModeStupid:
		return g.moveStupid(man)
	case moveModeBFS:
		return g.moveBFS(man)
	}
	return false
}

// orderedBombs is the Java nextBombs HashSet<Bomb>. Bomb has no hashCode,
// so Java iterates it in identity-hash order, which isn't reproducible.
// The order only changes what the viewer sees (the order of explosions in
// a wave, and which bomb object a player gets back first), so insertion
// order is used.
type orderedBombs struct {
	list []*Bomb
	seen map[*Bomb]bool
}

func (o *orderedBombs) add(b *Bomb) {
	if o.seen == nil {
		o.seen = map[*Bomb]bool{}
	}
	if !o.seen[b] {
		o.seen[b] = true
		o.list = append(o.list, b)
	}
}

func (g *Game) UpdateGame(round int) error {
	progress := false

	g.explosionGroups = g.explosionGroups[:0]

	// Compute explosions
	var nextBombs orderedBombs
	for _, bomb := range g.activeBombs {
		progress = true
		bomb.fuse--
		if bomb.fuse <= 0 {
			nextBombs.add(bomb)
		}
	}
	blownUp := map[Coord]bool{}
	for len(nextBombs.list) > 0 {
		var currentExplosions []*Explosion
		for _, bomb := range nextBombs.list {
			e := &Explosion{source: bomb}
			for _, d := range directions {
				distance := bomb.power
				cur := bomb.position
				blocked := false
				for !blocked && distance > 0 {
					c := g.grid[cur]
					if c != nil && (c.typ != cellBlock && !blownUp[c.Coord] || c.hasBomb()) {
						if cur != bomb.position && (c.isBox() || c.hasBomb() || c.hasItem()) {
							if !blownUp[c.Coord] {
								if c.isBox() || c.hasItem() {
									e.flammables = append(e.flammables, c)
								} else if c.hasBomb() {
									for _, b := range c.bombs {
										if b.fuse > 0 {
											e.bombs = append(e.bombs, b)
										}
									}
								}
							}
							if !bomb.owner.piercingBombs {
								blocked = true
							}
						}
						if !g.practiceMode {
							for _, man := range g.menOn(c) {
								if g.friendlyFire || bomb.owner != man {
									man.dead = true
									if man.killedBy == nil {
										man.killedBy = e
									}
									e.kills = append(e.kills, man)
								}
							}
						}
						cur = cur.add(d.getX(), d.getY())
						distance--
					} else {
						blocked = true
					}
				}
				e.span[d] = bomb.power - distance
			}
			currentExplosions = append(currentExplosions, e)
		}
		nextBombs = orderedBombs{}

		if len(currentExplosions) > 0 {
			for _, e := range currentExplosions {
				for _, b := range e.bombs {
					blownUp[b.position] = true
				}
				blownUp[e.source.position] = true
				for _, b := range e.bombs {
					nextBombs.add(b)
				}
			}
			g.explosionGroups = append(g.explosionGroups, currentExplosions)
		}
	}

	// Sudden death. Unreachable: SUDDEN_DEATH is -1 and no parameter sets
	// it. Java iterates grid.values() in HashMap order here; since this
	// never runs, row order is used.
	if g.suddenDeath > 0 {
		g.countDown--
		if g.countDown < 0 {
			depth := g.getDepth()
			var deathZones []*Cell
			for y := 0; y < g.height; y++ {
				for x := 0; x < g.width; x++ {
					c := g.grid[Coord{x, y}]
					if c.typ != cellBlock && (c.x == depth-1 || c.x == g.width-depth || c.y == depth-1 || c.y == g.height-depth) {
						deathZones = append(deathZones, c)
					}
				}
			}
			for _, c := range deathZones {
				if c.hasItem() {
					g.activeItems = removeItem(g.activeItems, c.item)
					c.item = nil
				}
				for _, b := range c.bombs {
					b.fuse = 1
				}
				for _, man := range g.menOn(c) {
					man.dead = true
				}
				c.typ = cellBlock
			}
		}
	}

	// Compute intentions
	var bombPlacements []bombPlacement
	for _, player := range g.players {
		if player.dead {
			continue
		}
		man := player.man
		if man.dead {
			player.dead = true
			player.deadAt = round
			continue
		}
		switch man.attempt {
		case attemptBomb:
			c := g.grid[man.position]
			if len(man.ammo) > 0 && c != nil && c.isFree() {
				bombPlacements = append(bombPlacements, bombPlacement{man, c})
				man.success = true
				progress = true
			} else {
				man.confused = true
			}
			if moveWithBomb && man.attemptCoord != nil {
				g.move(man)
			}
		case attemptMove:
			man.success = g.move(man)
		}
	}

	// Place bombs
	for _, bp := range bombPlacements {
		b := bp.owner.ammo[len(bp.owner.ammo)-1]
		bp.owner.ammo = bp.owner.ammo[:len(bp.owner.ammo)-1]
		b.arm(bp.owner.position, bp.owner.power, g.bombFuse)
		bp.cell.bombs = append(bp.cell.bombs, b)
		g.activeBombs = append(g.activeBombs, b)
	}

	// Apply explosions (and retrieve ammo)
	boxHitMap := map[*PlayerInfo]map[*Cell]bool{}
	for _, player := range g.players {
		boxHitMap[player] = map[*Cell]bool{}
	}

	g.explodedBoxes.Clear()
	for _, group := range g.explosionGroups {
		for _, explosion := range group {
			g.unspawnBomb(explosion.source)
			for _, bomb := range explosion.bombs {
				g.unspawnBomb(bomb)
			}
			for _, cell := range explosion.flammables {
				if cell.isBox() {
					g.explodedBoxes.Add(cell)
					if i := slices.Index(g.boxes, cell); i >= 0 {
						g.boxes = slices.Delete(g.boxes, i, i+1)
					}
					responsible := explosion.source.owner
					boxHitMap[responsible.player][cell] = true
				} else if cell.hasItem() {
					if i := slices.Index(g.activeItems, cell.item); i >= 0 {
						g.activeItems = slices.Delete(g.activeItems, i, i+1)
						g.addRemovedItem(cell.item)
					}
					cell.item = nil
				}
			}
		}
	}

	for _, player := range g.players {
		player.boxesHit += len(boxHitMap[player])
	}

	// Perform actions. Java collects items in a HashSet of identity-hashed
	// items; removing them from activeItems doesn't depend on the order.
	var collectedItems []*Item
	for _, man := range g.activeMen {
		if man.nextMove == nil {
			continue
		}
		progress = true
		last := man.position
		man.lastOccupiedPosition = &last
		man.position = man.position.add(man.nextMove.getX(), man.nextMove.getY())
		man.direction = *man.nextMove
		location := g.grid[man.position]
		if location.item != nil {
			if !slices.Contains(collectedItems, location.item) {
				collectedItems = append(collectedItems, location.item)
			}
			location.item.removedBy = append(location.item.removedBy, man)
			g.powerUp(location.item, man)
		}
	}
	for _, item := range collectedItems {
		location := g.grid[item.position]
		if i := slices.Index(g.activeItems, location.item); i >= 0 {
			g.activeItems = slices.Delete(g.activeItems, i, i+1)
			g.addRemovedItem(location.item)
		}
		location.item = nil
	}

	// Pop items, in Java HashSet order: this order reaches the players.
	//
	// One case can't be reproduced, in Java either: boxes that share a hash
	// bucket keep their insertion order, and when two of them are destroyed
	// by different bombs of the same chain-reaction wave, that order comes
	// from nextBombs, which Java iterates in identity-hash order. The port
	// then follows activeBombs order.
	for _, box := range g.explodedBoxes.All() {
		if item := box.item; item != nil {
			g.activeItems = append(g.activeItems, item)
			item.position = box.Coord
		}
		box.typ = cellFloor
	}

	if progress {
		g.noProgressCountDown = noProgressTimeout
	} else {
		g.noProgressCountDown--
	}

	if len(g.boxes) == 0 {
		g.extraCountDown--
	}
	return nil
}

func removeItem(items []*Item, item *Item) []*Item {
	if i := slices.Index(items, item); i >= 0 {
		return slices.Delete(items, i, i+1)
	}
	return items
}

// addRemovedItem is removedItems.add: a set, in insertion order here (Java
// uses identity-hash order, which only the viewer sees).
func (g *Game) addRemovedItem(item *Item) {
	if !slices.Contains(g.removedItems, item) {
		g.removedItems = append(g.removedItems, item)
	}
}

func (g *Game) getDepth() int {
	if g.countDown < 0 {
		return -g.countDown / g.suddenDeathSlowness
	}
	return 0
}

func (g *Game) unspawnBomb(bomb *Bomb) {
	i := slices.Index(g.activeBombs, bomb)
	if i < 0 {
		return
	}
	g.activeBombs = slices.Delete(g.activeBombs, i, i+1)
	bomb.owner.ammo = append(bomb.owner.ammo, bomb)
	cell := g.grid[bomb.position]
	if j := slices.Index(cell.bombs, bomb); j >= 0 {
		cell.bombs = slices.Delete(cell.bombs, j, j+1)
	}
}

func (g *Game) menOn(c *Cell) []*Man {
	var res []*Man
	for _, man := range g.activeMen {
		if !man.dead && man.position == c.Coord {
			res = append(res, man)
		}
	}
	return res
}

func (g *Game) PopulateMessages(p map[string]string) {
	p["move"] = "$%d moves to %d,%d"
	p["bomb"] = "$%d places a bomb"
	p["bombError"] = "$%d wants to place a bomb but cannot"
}

func (g *Game) InitDataForView() []string {
	if !g.listsCreated {
		panic("java.lang.NullPointerException: players is null")
	}
	timeLimit := g.suddenDeath
	if g.suddenDeath < 0 {
		timeLimit = defaultTimeLimit
	}
	practice := 0
	if g.practiceMode {
		practice = 1
	}
	lines := []string{
		strconv.Itoa(g.width) + " " + strconv.Itoa(g.height) + " " + strconv.Itoa(len(g.players)) + " " +
			strconv.Itoa(timeLimit) + " " + strconv.Itoa(practice),
		strconv.Itoa(len(g.boxes)),
	}
	for _, box := range g.boxes {
		lines = append(lines, strconv.Itoa(box.boxID)+" "+strconv.Itoa(box.x)+" "+strconv.Itoa(box.y)+" "+box.toPlayerString(true))
	}
	return append([]string{strconv.Itoa(len(lines) + 1)}, lines...)
}

func (g *Game) FrameDataForView(round, frame int, keyFrame bool) []string {
	lines := []string{strconv.Itoa(round)}
	for _, player := range g.players {
		alive := 1
		if player.dead {
			alive = 0
		}
		man := player.man
		lines = append(lines, strconv.Itoa(man.getX())+" "+strconv.Itoa(man.getY())+" "+man.direction.String()+" "+
			strconv.Itoa(player.boxesHit)+" "+strconv.Itoa(alive)+" "+strconv.Itoa(len(man.ammo))+" "+
			strconv.Itoa(man.maxAmmo)+" "+strconv.Itoa(g.Score(player.index))+" "+man.message)
	}
	lines = append(lines, strconv.Itoa(len(g.activeBombs)))
	for _, b := range g.activeBombs {
		lines = append(lines, b.position.String()+" "+strconv.Itoa(b.power)+" "+strconv.Itoa(b.fuse)+" "+
			strconv.Itoa(b.id)+" "+strconv.Itoa(b.getOwner()))
	}
	lines = append(lines, strconv.Itoa(len(g.explosionGroups)))
	for _, group := range g.explosionGroups {
		lines = append(lines, strconv.Itoa(len(group)))
		for _, explosion := range group {
			lines = append(lines, explosion.toFrameString())
		}
	}

	lines = append(lines, strconv.Itoa(g.explodedBoxes.Len()))
	for _, box := range g.explodedBoxes.All() {
		lines = append(lines, strconv.Itoa(box.boxID))
	}

	lines = append(lines, strconv.Itoa(len(g.activeItems)))
	for _, item := range g.activeItems {
		lines = append(lines, item.position.String()+" "+strconv.Itoa(item.itemType)+" "+strconv.Itoa(item.id))
	}
	lines = append(lines, strconv.Itoa(len(g.removedItems)))
	for _, item := range g.removedItems {
		lines = append(lines, strconv.Itoa(item.id)+" 1") // removedBy is never null
	}

	return append(lines, strconv.Itoa(g.getDepth()))
}

func (g *Game) GameName() string        { return "Hypersonic" }
func (g *Game) MinimumPlayerCount() int { return 2 }
func (g *Game) ShowTooltips() bool      { return true }
func (g *Game) MillisTimeForRound() int { return 100 } // this framework copy's default

func (g *Game) IsPlayerDead(playerIdx int) bool { return g.players[playerIdx].dead }

func (g *Game) DeathReason(playerIdx int) string {
	if g.players[playerIdx].man.killedBy != nil {
		return "$" + strconv.Itoa(playerIdx) + ": Disintegrated!"
	}
	return "$" + strconv.Itoa(playerIdx) + ": Eliminated!"
}

// Score is the boxes destroyed, plus a big bonus for staying alive or 100
// per round survived.
func (g *Game) Score(playerIdx int) int {
	player := g.players[playerIdx]
	if player.dead {
		return player.boxesHit + player.deadAt*100
	}
	return player.boxesHit + 1000000
}

func (g *Game) GameSummary(round int) []string {
	var lines []string
	for i := range g.players {
		lines = append(lines, g.playerSummary(i, round)...)
	}
	return lines
}

func (g *Game) playerSummary(playerIdx, round int) []string {
	var lines []string
	player := g.players[playerIdx]
	man := player.man
	tr := g.Ref().Translate
	if man.attempt == attemptMove && man.success {
		lines = append(lines, tr("move", player.index, man.position.x, man.position.y))
	} else if man.attempt == attemptBomb {
		if man.success {
			lines = append(lines, tr("bomb", player.index))
		} else {
			lines = append(lines, tr("bombError", player.index))
		}
		if man.attemptCoord != nil && man.nextMove != nil {
			lines = append(lines, tr("move", player.index, man.position.x, man.position.y))
		}
	}
	if man.dead && man.player.deadAt == round {
		lines = append(lines, g.DeathReason(playerIdx))
	}
	return lines
}

func (g *Game) SetPlayerTimeout(_, round, playerIdx int) {
	player := g.players[playerIdx]
	player.dead = true
	player.deadAt = round
	player.man.dead = true
}

func (g *Game) MaxRoundCount(int) int {
	if g.suddenDeath < 0 {
		return defaultTimeLimit
	}
	return g.suddenDeath + g.suddenDeathSlowness*(max(g.width, g.height)/2+1)
}

// GameOver is the game's gameOver() override, which this framework copy
// calls after every round.
func (g *Game) GameOver() bool {
	return g.noProgressCountDown <= 0 || g.countdownOnNoBoxes && g.extraCountDown <= 0 ||
		g.endOnNoBoxes && len(g.boxes) == 0 || g.Ref().AlivePlayerCount() < g.MinimumPlayerCount()
}
