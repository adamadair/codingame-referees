package cgref_test

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/adamadair/codingame-referees/internal/cgref"
	"github.com/adamadair/codingame-referees/internal/jprops"
	"github.com/adamadair/codingame-referees/internal/jrandom"
)

// toyGame is testdata/ToyGame.java ported to cgref. See that file for the
// rules.
type toyGame struct {
	cgref.Base

	seed            int64
	rounds          int
	winScore        int
	turnBased       bool
	random          *jrandom.Random
	playerCount     int
	scores, guesses []int
	guessed, dead   []bool
	lastTarget      int
}

var guessPattern = regexp.MustCompile(`(?i)^GUESS (-?\d{1,3})(?: (.+))?$`)

// newToy deliberately leaves lastTarget at 0 although ToyGame.java declares
// "lastTarget = -1": the Java game runs inside the AbstractReferee
// constructor, before the subclass's field initializers execute, so Java
// plays the whole game with the field's zero value.
func newToy() *toyGame { return &toyGame{} }

func (g *toyGame) InitReferee(playerCount int, p *jprops.Properties) error {
	var err error
	if v, ok := p.Get("seed"); ok {
		g.seed, err = strconv.ParseInt(v, 10, 64)
	} else {
		g.seed = jrandom.New(0).NextLong() // Java uses the clock; tests always pass a seed
	}
	if err != nil {
		return cgref.InvalidFormat("bad number")
	}
	r, err1 := strconv.ParseInt(p.GetProperty("rounds", "30"), 10, 32)
	w, err2 := strconv.ParseInt(p.GetProperty("winScore", "5"), 10, 32)
	if err1 != nil || err2 != nil {
		return cgref.InvalidFormat("bad number")
	}
	g.rounds, g.winScore = int(r), int(w)
	g.turnBased = strings.EqualFold(p.GetProperty("turnBased", "false"), "true")
	g.random = jrandom.New(g.seed)
	g.playerCount = playerCount
	g.scores = make([]int, playerCount)
	g.guesses = make([]int, playerCount)
	g.guessed = make([]bool, playerCount)
	g.dead = make([]bool, playerCount)
	return nil
}

func (g *toyGame) Configuration() *jprops.Properties {
	p := jprops.New()
	p.Set("seed", strconv.FormatInt(g.seed, 10))
	p.Set("rounds", strconv.Itoa(g.rounds))
	p.Set("winScore", strconv.Itoa(g.winScore))
	p.Set("turnBased", strconv.FormatBool(g.turnBased))
	return p
}

func (g *toyGame) PopulateMessages(m map[string]string) {
	m["quit"] = "$%d gave up"
	m["won"] = "$%d claimed victory"
	m["scoreReached"] = "$%d reached the target score"
}

func (g *toyGame) IsTurnBasedGame() bool           { return g.turnBased }
func (g *toyGame) MaxRoundCount(int) int           { return g.rounds }
func (g *toyGame) MillisTimeForRound() int         { return 50 }
func (g *toyGame) InitDataForView() []string       { return []string{"toy"} }
func (g *toyGame) ExpectedOutputLineCount(int) int { return 1 }
func (g *toyGame) GameName() string                { return "Toy" }
func (g *toyGame) MinimumPlayerCount() int         { return 2 }
func (g *toyGame) ShowTooltips() bool              { return true }
func (g *toyGame) IsPlayerDead(p int) bool         { return g.dead[p] }
func (g *toyGame) DeathReason(p int) string        { return fmt.Sprintf("$%d: out of range", p) }
func (g *toyGame) Score(p int) int                 { return g.scores[p] }
func (g *toyGame) GameSummary(round int) []string  { return []string{"round " + strconv.Itoa(round)} }
func (g *toyGame) SetPlayerTimeout(_, _, p int)    { g.scores[p]-- }

func (g *toyGame) FrameDataForView(round, frame int, keyFrame bool) []string {
	return []string{fmt.Sprintf("%d %d", round, g.lastTarget)}
}

func (g *toyGame) HandlePlayerOutput(frame, round, player int, output []string) error {
	line := output[0]
	if strings.EqualFold(line, "QUIT") {
		return cgref.Lost("quit", player)
	}
	if strings.EqualFold(line, "WIN") {
		if g.scores[player] >= 2 {
			return cgref.Win("won", player)
		}
		return cgref.InvalidInput("GUESS n", line)
	}
	m := guessPattern.FindStringSubmatch(line)
	if m == nil {
		return cgref.InvalidInput("GUESS n", line)
	}
	g.guesses[player], _ = strconv.Atoi(m[1])
	g.guessed[player] = true
	return nil
}

func (g *toyGame) InitInputForPlayer(p int) []string {
	return []string{fmt.Sprintf("%d %d %d", p, g.playerCount, g.rounds)}
}

func (g *toyGame) InputForPlayer(round, p int) []string {
	state := make([]string, g.playerCount)
	seen := make([]string, g.playerCount)
	for i := range g.playerCount {
		d := 0
		if g.dead[i] {
			d = 1
		}
		state[i] = fmt.Sprintf("%d:%d", g.scores[i], d)
		seen[i] = "-"
		if g.guessed[i] {
			seen[i] = strconv.Itoa(g.guesses[i])
		}
	}
	return []string{fmt.Sprintf("%d %d", round, g.lastTarget), strings.Join(state, " "), strings.Join(seen, " ")}
}

func (g *toyGame) UpdateGame(round int) error {
	target := int(g.random.NextIntN(100))
	g.lastTarget = target
	best := math.MaxInt32
	for i := range g.playerCount {
		if !g.dead[i] && g.guessed[i] && g.guesses[i] >= 0 && g.guesses[i] <= 100 {
			best = min(best, abs(g.guesses[i]-target))
		}
	}
	for i := range g.playerCount {
		if !g.guessed[i] || g.dead[i] {
			continue
		}
		if g.guesses[i] < 0 || g.guesses[i] > 100 {
			g.dead[i] = true
		} else if abs(g.guesses[i]-target) == best {
			g.scores[i]++
		}
	}
	if g.random.NextDouble() < 0.1 {
		g.scores[g.random.NextIntN(int32(g.playerCount))]++
	}
	clear(g.guessed)
	for i := range g.playerCount {
		if g.scores[i] >= g.winScore {
			return cgref.GameOver("scoreReached", i)
		}
	}
	return nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
