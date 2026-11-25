package gameofdrones

import (
	"math"
	"strconv"
	"strings"

	"github.com/adamadair/codingame-referees/internal/jrandom"
	"github.com/adamadair/codingame-referees/internal/jstrings"
)

// Board and rules, from the Java referee.
const (
	width           = 4000
	height          = 1800
	speed           = 100
	zoneRadius      = 100
	zoneUpMargin    = 250
	zoneRightMargin = 350
	maxRounds       = 200
	maxDrones       = 20
	maxZones        = 8
)

const (
	lostParsingReason     = "Failure: invalid input"
	outOfBounds           = "Failure: an input value is out of bounds"
	lostParsingReasonCode = "INPUT"
)

type playerData struct {
	score  int
	lost   bool
	info   string // "" is Java's null
	dx, dy []int  // rounded positions, updated at the end of each round
	angle  []float32
	rx, ry []float64 // exact positions
}

type point struct{ x, y int }

// distance is java.awt.Point.distance(x, y). The float64 conversions keep Go
// from fusing the multiply and add, which Java never does.
func (p point) distance(x, y int) float64 {
	px := float64(x) - float64(p.x)
	py := float64(y) - float64(p.y)
	return math.Sqrt(float64(px*px) + float64(py*py))
}

type zoneData struct {
	playerID int
	count    []int
	center   point
}

// initGame is Referee.initGame: read the parameters or pick defaults, then
// generate the map. Bad parameters don't fail here; they end the game at the
// next GET_GAME_INFO.
func (r *Referee) initGame(nbPlayers int, initLines []string) {
	r.nbPlayers = nbPlayers
	r.nbCurrentPlayers = nbPlayers
	if len(initLines) == 0 {
		seed := r.clock()
		r.rand = jrandom.New(seed)
		r.nbZones = nbPlayers * 2
		r.nbDronesPerPlayer = 3 + 2*int(r.rand.NextIntN(5))
		r.saveUserInput = "nbZones=" + strconv.Itoa(r.nbZones) + "\n" +
			"nbDrones=" + strconv.Itoa(r.nbDronesPerPlayer) + "\n" +
			"gameSeed=" + strconv.FormatInt(seed, 10) + "\n"
		r.generateInput()
		return
	}

	if !r.parseParams(initLines) {
		r.end = true
		if r.reason != outOfBounds {
			r.reason = lostParsingReason
		}
		r.reasonCode = lostParsingReasonCode
		r.initLines = initLines
		return
	}
	r.generateInput()
}

// parseParams reads the three parameter lines. Java matches them by
// position and ignores the keys: zones, drones, then seed.
func (r *Referee) parseParams(lines []string) bool {
	if len(lines) < 3 {
		return false
	}
	var values [3]string
	for i := range values {
		if lines[i] == "" { // Scanner.nextLine fails on an empty line
			return false
		}
		parts := jstrings.Split(lines[i], "=")
		if len(parts) < 2 {
			return false
		}
		values[i] = parts[1]
	}
	zones, err1 := jstrings.ParseInt(values[0])
	drones, err2 := jstrings.ParseInt(values[1])
	seed, err3 := jstrings.ParseLong(values[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	r.nbZones, r.nbDronesPerPlayer = int(zones), int(drones)
	r.saveUserInput = "nbZones=" + strconv.Itoa(r.nbZones) + "\n" +
		"nbDrones=" + strconv.Itoa(r.nbDronesPerPlayer) + "\n" +
		"gameSeed=" + strconv.FormatInt(seed, 10) + "\n"
	if r.nbDronesPerPlayer > maxDrones || r.nbDronesPerPlayer < 1 || r.nbZones > maxZones || r.nbZones < 1 {
		r.reason = outOfBounds
		return false
	}
	r.rand = jrandom.New(seed)
	r.rand.NextIntN(5) // keeps the sequence aligned with the default path
	return true
}

// generateInput places the zones and the drones' starting points.
func (r *Referee) generateInput() {
	r.players = make([]*playerData, r.nbPlayers)
	for p := range r.players {
		r.players[p] = &playerData{
			dx:    make([]int, r.nbDronesPerPlayer),
			dy:    make([]int, r.nbDronesPerPlayer),
			angle: make([]float32, r.nbDronesPerPlayer),
			rx:    make([]float64, r.nbDronesPerPlayer),
			ry:    make([]float64, r.nbDronesPerPlayer),
		}
	}
	r.zones = make([]*zoneData, r.nbZones)
	for z := range r.zones {
		r.zones[z] = &zoneData{playerID: -1, count: make([]int, r.nbPlayers)}
	}

	for i := range r.nbZones {
		for {
			x := int(r.rand.NextIntN((width-zoneRightMargin)-2*zoneRadius)) + zoneRadius
			y := int(r.rand.NextIntN((height-zoneUpMargin)-2*zoneRadius)) + zoneRadius + zoneUpMargin
			intersect := false
			for j := range i {
				if r.zones[j].center.distance(x, y) < zoneRadius*7 {
					intersect = true
					break
				}
			}
			if !intersect {
				r.zones[i].center = point{x, y}
				break
			}
		}
	}

	for i := range r.nbDronesPerPlayer {
		for {
			x := int(r.rand.NextIntN(width))
			y := int(r.rand.NextIntN(height))
			intersect := false
			for _, z := range r.zones {
				if z.center.distance(x, y) <= zoneRadius {
					intersect = true
					break
				}
			}
			if !intersect {
				for _, p := range r.players {
					p.dx[i], p.dy[i] = x, y
					p.rx[i], p.ry[i] = float64(x), float64(y)
				}
				break
			}
		}
	}
}

// setPlayerOutput moves the current player's drones toward the requested
// points. An invalid line makes the player lose; drones before it keep
// their moves.
func (r *Referee) setPlayerOutput(output []string) error {
	r.nbIntermediate++
	p := r.currentPlayer
	for i := range r.nbDronesPerPlayer {
		if i >= len(output) {
			return errMissingLines(len(output), r.nbDronesPerPlayer)
		}
		x, y, ok := parseTarget(output[i])
		if !ok {
			p.lost = true
			p.info = "¤RED¤Failure: invalid input§RED§. At line " + strconv.Itoa(i) +
				": expected two integers with values between [0 and " + strconv.Itoa(width-1) +
				"] and [0 and " + strconv.Itoa(height-1) + "]"
			return nil
		}

		if x == p.dx[i] && y == p.dy[i] {
			continue
		}
		curX, curY := p.rx[i], p.ry[i]
		diffX := float64(x) - curX
		diffY := float64(y) - curY
		square := float64(diffX*diffX) + float64(diffY*diffY)
		if square <= speed*speed {
			p.rx[i], p.ry[i] = float64(x), float64(y)
		} else {
			norm := math.Sqrt(square)
			p.rx[i] = curX + float64(speed*diffX)/norm
			p.ry[i] = curY + float64(speed*diffY)/norm
		}
		w := p.rx[i] - curX
		h := p.ry[i] - curY
		if w != 0 || h != 0 {
			p.angle[i] = float32(math.Atan2(h, w))
		}
	}
	return nil
}

// parseTarget reads "x y" like the Java code: split on single spaces, parse
// the first two tokens, ignore the rest, and require a point on the board.
func parseTarget(line string) (x, y int, ok bool) {
	coords := jstrings.Split(line, " ")
	if len(coords) < 2 {
		return 0, 0, false
	}
	cx, err1 := jstrings.ParseInt(coords[0])
	cy, err2 := jstrings.ParseInt(coords[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	x, y = int(cx), int(cy)
	if x < 0 || x >= width || y < 0 || y >= height {
		return 0, 0, false
	}
	return x, y, true
}

// updateScores ends a round: round the drones' positions, count drones in
// each zone, give each zone to the single player with the most drones (a
// tie keeps the current owner), and give a point per owned zone. Players
// who have lost still have drones on the board.
func (r *Referee) updateScores() {
	for p, player := range r.players {
		for _, z := range r.zones {
			z.count[p] = 0
		}
		for d := range r.nbDronesPerPlayer {
			player.dx[d] = int(player.rx[d] + 0.5)
			player.dy[d] = int(player.ry[d] + 0.5)
			for _, z := range r.zones {
				if z.center.distance(player.dx[d], player.dy[d]) <= zoneRadius {
					z.count[p]++
				}
			}
		}
	}
	for _, z := range r.zones {
		maxCount := 0
		owner := -1
		for i, c := range z.count {
			if c > maxCount {
				maxCount = c
				owner = i
			} else if c == maxCount {
				owner = -1
			}
		}
		if owner != -1 {
			z.playerID = owner
		}
		if z.playerID != -1 {
			r.players[z.playerID].score++
		}
	}
}

func (r *Referee) setPlayerTimeout() {
	r.currentPlayer.lost = true
	r.currentPlayer.info = "¤RED¤Timeout: the program did not provide " + strconv.Itoa(r.nbDronesPerPlayer) +
		" input lines in due time...§RED§ $" + strconv.Itoa(r.currentPlayerID) +
		" will no longer be active in this game."
	r.nbIntermediate++
}

func (r *Referee) buildInitView() string {
	return strconv.Itoa(width) + " " + strconv.Itoa(height) + " " + strconv.Itoa(zoneRadius)
}

func (r *Referee) buildInitInput(playerID int) string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(r.nbPlayers) + " " + strconv.Itoa(playerID) + " " +
		strconv.Itoa(r.nbDronesPerPlayer) + " " + strconv.Itoa(r.nbZones) + "\n")
	for _, z := range r.zones {
		b.WriteString(strconv.Itoa(z.center.x) + " " + strconv.Itoa(z.center.y) + "\n")
	}
	return b.String()
}

func (r *Referee) buildPlayerInput() string {
	var b strings.Builder
	for _, z := range r.zones {
		b.WriteString(strconv.Itoa(z.playerID) + "\n")
	}
	for _, p := range r.players {
		for d := range r.nbDronesPerPlayer {
			b.WriteString(strconv.Itoa(p.dx[d]) + " " + strconv.Itoa(p.dy[d]) + "\n")
		}
	}
	return b.String()
}

// buildPlayerInfo describes the previous player's turn. It clears that
// player's info message, as Java does.
func (r *Referee) buildPlayerInfo(playerID int) []string {
	if r.nbRounds == 0 {
		return []string{"Game starting..."}
	}
	p := r.players[playerID]
	if p.info != "" {
		info := p.info
		p.info = ""
		return []string{info}
	}
	return []string{"$" + strconv.Itoa(playerID) + " moved their drones"}
}

func (r *Referee) buildViewParams() string {
	var b strings.Builder
	for _, p := range r.players {
		b.WriteString(strconv.Itoa(p.score) + "\n")
	}
	for _, z := range r.zones {
		b.WriteString(strconv.Itoa(z.playerID) + "\n")
		for _, c := range z.count {
			b.WriteString(strconv.Itoa(c) + "\n")
		}
	}
	for _, p := range r.players {
		for d := range r.nbDronesPerPlayer {
			b.WriteString(strconv.Itoa(p.dx[d]) + " " + strconv.Itoa(p.dy[d]) + " " + javaFloatString(p.angle[d]) + "\n")
		}
	}
	return b.String()
}

func (r *Referee) buildSummary() []string {
	var summary []string
	for p, player := range r.players {
		if player.lost {
			summary = append(summary, "$"+strconv.Itoa(p)+" no longer plays in this game. (current Score = "+strconv.Itoa(player.score)+")")
		} else {
			summary = append(summary, "$"+strconv.Itoa(p)+" score: "+strconv.Itoa(player.score))
		}
	}
	for z, zone := range r.zones {
		if zone.playerID == -1 {
			summary = append(summary, "Zone "+strconv.Itoa(z)+" is neutral")
		} else {
			summary = append(summary, "Zone "+strconv.Itoa(z)+" belongs to $"+strconv.Itoa(zone.playerID))
		}
	}
	return summary
}
