// Package cgref is a Go port of the framework embedded in the official
// CodinGame Java referees (AbstractReferee and MultiReferee).
//
// The referee talks to the game runner over stdin and stdout with the
// legacy protocol. The runner sends "[[INIT] n]", "[[GET_GAME_INFO] 0]",
// "[[SET_PLAYER_OUTPUT] n]" and "[[SET_PLAYER_TIMEOUT] 0]". The referee
// answers with VIEW, INFOS, SUMMARY, TOOLTIP, NEXT_PLAYER_INPUT,
// NEXT_PLAYER_INFO and, at the end, UINPUT and SCORES blocks.
//
// The port follows the newest copy of the framework (Coders of the
// Caribbean). Options covers the older copies' differences that change how
// a game plays out.
package cgref

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime/debug"
	"strconv"

	"github.com/adamadair/codingame-referees/internal/jprops"
	"github.com/adamadair/codingame-referees/internal/jscanner"
)

// Options selects framework behaviours that differ between the Java copies.
type Options struct {
	// UpdateScoresEachRound calls updateScores() right after each
	// updateGame(), as the Coders Strike Back, Fantastic Bits, Code
	// Busters, Hyper Sonic and SmashTheCode copies do. The newest copy only
	// updates scores at game over.
	UpdateScoresEachRound bool

	// InputsOnDemand builds each player's input when that player is about
	// to move, as the Back to the Code, Coders Strike Back, Platinum Rift,
	// Poker Chip Race and The Great Escape copies do. The newer copies build
	// every player's input once at the start of a round in simultaneous
	// games, so later players can't see earlier players' moves.
	InputsOnDemand bool
}

const (
	lostParsingReasonCode = "INPUT"
	lostParsingReason     = "Failure: invalid input"
)

var headerPattern = regexp.MustCompile(`^\[\[(.+)\] ?([0-9]+)\]$`)

// playerStatus is AbstractReferee.PlayerStatus. Empty strings stand for
// Java's null.
type playerStatus struct {
	id         int
	score      int
	lost, win  bool
	info       string
	reasonCode string
	nextInput  []string
}

type tooltip struct {
	player  int
	message string
}

// Referee runs one game. Games reach it through Base.Ref.
type Referee struct {
	game Game
	opts Options

	in  *jscanner.Scanner
	out *bufio.Writer
	err io.Writer

	messages   map[string]string
	properties *jprops.Properties

	// Java keeps tooltips in a HashSet of objects without hashCode, so its
	// order isn't reproducible. Insertion order is used here.
	tooltips []tooltip

	playerCount, alivePlayerCount int
	currentPlayer, nextPlayerIdx  int
	lastPlayer, status            *playerStatus
	frame, round                  int
	players                       []*playerStatus
	initLines                     []string
	newRound                      bool
	reasonCode, reason            string
}

// Main runs game on the process's standard streams.
func Main(game Game, opts Options) {
	Run(game, os.Stdin, os.Stdout, os.Stderr, opts)
}

// Run plays one game, reading runner commands from in and writing referee
// output to out. Errors and stack traces go to errOut, as Java's
// printStackTrace does.
func Run(game Game, in io.Reader, out, errOut io.Writer, opts Options) {
	r := &Referee{
		game:     game,
		opts:     opts,
		in:       jscanner.New(in),
		out:      bufio.NewWriter(out),
		err:      errOut,
		messages: map[string]string{},
	}
	if s, ok := game.(refereeSetter); ok {
		s.SetReferee(r)
	}
	r.start()
}

// Translate formats the message registered under code, or returns the code
// itself when there is no such message.
func (r *Referee) Translate(code string, values ...any) string {
	tmpl, ok := r.messages[code]
	if !ok {
		return code
	}
	return javaFormat(tmpl, values...)
}

// AddToolTip queues a tooltip for the viewer, if the game shows tooltips.
func (r *Referee) AddToolTip(player int, message string) {
	if r.game.ShowTooltips() {
		r.tooltips = append(r.tooltips, tooltip{player, message})
	}
}

// PrintError writes a debug message to the referee's error stream.
func (r *Referee) PrintError(message any) {
	fmt.Fprintln(r.err, message)
}

// ColoredReason wraps a reason in the viewer's colour markers.
func (r *Referee) ColoredReason(isError bool, reason string) string {
	if isError {
		return "¤RED¤" + reason + "§RED§"
	}
	return "¤GREEN¤" + reason + "§GREEN§"
}

// PlayerCount returns the number of players in the game.
func (r *Referee) PlayerCount() int { return r.playerCount }

// AlivePlayerCount returns the number of players still in the game.
func (r *Referee) AlivePlayerCount() int { return r.alivePlayerCount }

func (r *Referee) println(s fmt.Stringer) {
	r.out.WriteString(s.String())
	r.out.WriteByte('\n')
	r.out.Flush()
}

func (r *Referee) start() {
	r.messages["InvalidInput"] = "invalid input. Expected '%s' but found '%s'"
	r.messages["playerTimeoutMulti"] = "Timeout: the program did not provide %d input lines in due time... $%d will no longer be active in this game."
	r.messages["playerTimeoutSolo"] = "Timeout: the program did not provide %d input lines in due time..."
	r.messages["maxRoundsCountReached"] = "Max rounds reached"
	r.messages["notEnoughPlayers"] = "Not enough players (expected > %d, found %d)"
	r.messages["InvalidActionTooltip"] = "$%d: invalid action"
	r.messages["TimeoutTooltip"] = "$%d: timeout!"
	r.messages["WinTooltip"] = "$%d: victory!"
	r.messages["LostTooltip"] = "$%d lost: %s"
	r.game.PopulateMessages(r.messages)

	err := r.protect(r.loop)

	var over *GameOverError
	switch {
	case errors.As(err, &over):
		r.newRound = true
		r.reasonCode = over.Code
		r.reason = r.reasonFor(over.Code, over.Values)
		if err := r.protect(func() error {
			r.dumpView()
			r.dumpInfos()
			r.game.Prepare(r.round)
			r.updateScores()
			return nil
		}); err != nil {
			r.crash(err)
			return
		}
	case errors.Is(err, errGameError{}):
		// Init error: already reported, scores still follow.
	default:
		// Any other failure ends the referee without scores, like the
		// catch (Throwable) in Java.
		r.crash(err)
		return
	}

	scores := newOutput("SCORES")
	for i := range r.playerCount {
		scores.add(strconv.Itoa(i) + " " + strconv.Itoa(r.players[i].score))
	}
	r.appendDataToEnd()
	r.println(scores)
}

// protect runs f, turning a panic into an error the way Java turns a
// RuntimeException into a caught Throwable.
func (r *Referee) protect(f func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = &panicError{value: p, stack: debug.Stack()}
		}
	}()
	return f()
}

type panicError struct {
	value any
	stack []byte
}

func (e *panicError) Error() string { return fmt.Sprint(e.value) }

func (r *Referee) crash(err error) {
	var p *panicError
	if errors.As(err, &p) {
		fmt.Fprintf(r.err, "panic: %v\n%s", p.value, p.stack)
		return
	}
	fmt.Fprintln(r.err, err)
}

func (r *Referee) loop() error {
	for {
		line, err := r.in.NextLine()
		if err != nil {
			return err
		}
		m := headerPattern.FindStringSubmatch(line)
		if m == nil {
			return errors.New("Error in data sent to referee")
		}
		lineCount, err := strconv.ParseInt(m[2], 10, 32)
		if err != nil {
			return fmt.Errorf("java.lang.NumberFormatException: %q", m[2])
		}

		switch m[1] {
		case "INIT":
			err = r.handleInit(int(lineCount))
		case "GET_GAME_INFO":
			err = r.handleGetGameInfo()
		case "SET_PLAYER_OUTPUT":
			err = r.handlePlayerOutput(int(lineCount))
		case "SET_PLAYER_TIMEOUT":
			err = r.handlePlayerTimeout()
		default:
			err = fmt.Errorf("java.lang.IllegalArgumentException: No enum constant %s", m[1])
		}
		if err != nil {
			return err
		}
	}
}

func (r *Referee) handleInit(lineCount int) error {
	n, err := r.in.NextInt()
	if err != nil {
		return err
	}
	r.playerCount, r.alivePlayerCount = n, n
	r.players = make([]*playerStatus, n)
	for i := range r.players {
		r.players[i] = &playerStatus{id: i}
	}
	r.status = r.players[0]
	r.currentPlayer, r.nextPlayerIdx = n-1, n-1
	r.frame = 0
	r.round = -1
	r.newRound = true
	if _, err := r.in.NextLine(); err != nil {
		return err
	}

	// Java leaves initLines null when the count is 0, which then fails
	// inside handleInitInputForReferee.
	r.initLines = nil
	if lineCount > 0 {
		r.initLines = make([]string, lineCount-1)
		for i := range r.initLines {
			if r.initLines[i], err = r.in.NextLine(); err != nil {
				return err
			}
		}
	}

	if err := r.protect(func() error { return r.initReferee(lineCount > 0) }); err != nil {
		var p *panicError
		var msg string
		if errors.As(err, &p) {
			msg = fmt.Sprint(p.value)
		} else {
			msg = err.Error()
		}
		r.reason = "Init error: " + msg
		view := newOutput("VIEW")
		view.add("-1")
		view.add(lostParsingReasonCode)
		r.println(view)
		infos := newOutput("INFOS")
		infos.add(r.ColoredReason(true, lostParsingReason))
		infos.add(r.ColoredReason(true, msg))
		infos.addAll(r.initLines)
		r.println(infos)
		r.dumpView()
		r.dumpInfos()
		return errGameError{}
	}

	if r.playerCount < r.game.MinimumPlayerCount() {
		return GameOver("notEnoughPlayers", r.game.MinimumPlayerCount(), r.playerCount)
	}
	return nil
}

// initReferee is MultiReferee.handleInitInputForReferee.
func (r *Referee) initReferee(haveLines bool) error {
	r.properties = jprops.New()
	if !haveLines {
		return errors.New("null") // NullPointerException in Java
	}
	for _, l := range r.initLines {
		if err := r.properties.Load(l); err != nil {
			return err
		}
	}
	if err := r.game.InitReferee(r.playerCount, r.properties); err != nil {
		return err
	}
	r.properties = r.game.Configuration()
	return nil
}

func (r *Referee) handleGetGameInfo() error {
	r.lastPlayer = r.status
	next, err := r.nextPlayer()
	if err != nil {
		return err
	}
	r.status = next
	if r.round >= r.game.MaxRoundCount(r.playerCount) {
		return GameOver("maxRoundsCountReached")
	}
	r.dumpView()
	r.dumpInfos()
	if r.newRound {
		r.game.Prepare(r.round)
		if !r.game.IsTurnBasedGame() && !r.opts.InputsOnDemand {
			for _, p := range r.players {
				if !p.lost {
					p.nextInput = r.game.InputForPlayer(r.round, p.id)
				} else {
					p.nextInput = nil
				}
			}
		}
	}
	r.dumpNextPlayerInput()
	r.dumpNextPlayerInfos()
	return nil
}

func (r *Referee) handlePlayerOutput(lineCount int) error {
	r.frame++
	output := make([]string, lineCount)
	for i := range output {
		var err error
		if output[i], err = r.in.NextLine(); err != nil {
			return err
		}
	}

	err := r.game.HandlePlayerOutput(r.frame, r.round, r.nextPlayerIdx, output)
	if err == nil {
		return nil
	}
	code, values := playerError(err)
	switch err.(type) {
	case *LostError, *InvalidInputError:
		r.status.score = r.game.Score(r.nextPlayerIdx)
		r.status.lost = true
		r.status.info = r.reasonFor(code, values)
		r.status.reasonCode = code
		if _, ok := err.(*InvalidInputError); ok {
			r.AddToolTip(r.nextPlayerIdx, r.Translate("InvalidActionTooltip", r.nextPlayerIdx))
		} else {
			r.AddToolTip(r.nextPlayerIdx, r.Translate("LostTooltip", r.nextPlayerIdx, r.Translate(code, values...)))
		}
		r.alivePlayerCount--
		if r.alivePlayerCount < r.game.MinimumPlayerCount() && r.game.IsTurnBasedGame() {
			r.lastPlayer = r.status
			return GameOver("")
		}
		return nil
	case *WinError:
		r.status.score = r.game.Score(r.nextPlayerIdx)
		r.status.win = true
		r.status.info = r.reasonFor(code, values)
		r.status.reasonCode = code
		r.AddToolTip(r.nextPlayerIdx, r.Translate("WinTooltip", r.nextPlayerIdx))
		r.alivePlayerCount--
		if r.alivePlayerCount < r.game.MinimumPlayerCount() {
			r.lastPlayer = r.status
			return GameOver("")
		}
		return nil
	}
	return err
}

func (r *Referee) handlePlayerTimeout() error {
	r.frame++
	count := r.game.ExpectedOutputLineCount(r.nextPlayerIdx)
	r.game.SetPlayerTimeout(r.frame, r.round, r.nextPlayerIdx)
	r.status.lost = true
	if r.playerCount <= 1 {
		r.status.info = r.Translate("playerTimeoutSolo", count)
	} else {
		r.status.info = r.Translate("playerTimeoutMulti", count, r.nextPlayerIdx)
	}
	r.AddToolTip(r.nextPlayerIdx, r.Translate("TimeoutTooltip", r.nextPlayerIdx))
	r.alivePlayerCount--
	if r.alivePlayerCount < r.game.MinimumPlayerCount() && r.game.IsTurnBasedGame() {
		r.lastPlayer = r.status
		return GameOver("")
	}
	return nil
}

// reasonFor is GameException.getReason: null when there is no code.
func (r *Referee) reasonFor(code string, values []any) string {
	if code == "" {
		return ""
	}
	return r.Translate(code, values...)
}

func (r *Referee) nextPlayer() (*playerStatus, error) {
	r.currentPlayer = r.nextPlayerIdx
	r.newRound = false
	for {
		r.nextPlayerIdx++
		if r.nextPlayerIdx >= r.playerCount {
			if err := r.nextRound(); err != nil {
				return nil, err
			}
			r.nextPlayerIdx = 0
		}
		p := r.players[r.nextPlayerIdx]
		if !p.lost && !p.win {
			return p, nil
		}
	}
}

func (r *Referee) nextRound() error {
	r.newRound = true
	r.round++
	if r.round > 0 {
		if err := r.game.UpdateGame(r.round); err != nil {
			return err
		}
		if r.opts.UpdateScoresEachRound {
			r.updateScores()
		}
	}
	if r.gameOver() {
		return GameOver("")
	}
	return nil
}

func (r *Referee) gameOver() bool {
	if g, ok := r.game.(GameOverChecker); ok {
		return g.GameOver()
	}
	return r.alivePlayerCount < r.game.MinimumPlayerCount()
}

func (r *Referee) updateScores() {
	for i, p := range r.players {
		if !p.lost && r.game.IsPlayerDead(i) {
			r.alivePlayerCount--
			p.lost = true
			p.info = r.game.DeathReason(i)
			r.AddToolTip(i, p.info)
		}
		p.score = r.game.Score(i)
	}
}

func (r *Referee) dumpView() {
	data := newOutput("VIEW")
	reasonCode := r.reasonCode
	if reasonCode == "" && r.status != nil {
		reasonCode = r.status.reasonCode
	}
	if r.newRound {
		if reasonCode != "" {
			data.add(fmt.Sprintf("KEY_FRAME %d %s", r.frame, reasonCode))
		} else {
			data.add(fmt.Sprintf("KEY_FRAME %d", r.frame))
		}
		if r.frame == 0 {
			data.add(r.game.GameName())
			data.addAll(r.game.InitDataForView())
		}
	} else {
		if reasonCode != "" {
			data.add(fmt.Sprintf("INTERMEDIATE_FRAME %d %s", r.frame, reasonCode))
		} else {
			data.add(fmt.Sprintf("INTERMEDIATE_FRAME %d", r.frame))
		}
	}
	if r.newRound || r.game.IsTurnBasedGame() {
		data.addAll(r.game.FrameDataForView(r.round, r.frame, r.newRound))
	}
	r.println(data)
}

func (r *Referee) dumpInfos() {
	data := newOutput("INFOS")
	if r.reason != "" && r.game.IsTurnBasedGame() {
		data.add(r.ColoredReason(true, r.reason))
	} else if r.lastPlayer != nil {
		if head := r.lastPlayer.info; head != "" {
			data.add(r.ColoredReason(r.lastPlayer.lost, head))
		} else if r.frame > 0 {
			round := r.round
			if r.newRound {
				round--
			}
			data.addAll(r.game.PlayerActions(r.currentPlayer, round))
		}
	}
	r.println(data)

	if r.newRound && r.round >= -1 && r.playerCount > 1 {
		summary := newOutput("SUMMARY")
		if r.frame == 0 {
			if head := r.game.HeadlineAtGameStart(); head != "" {
				summary.add(head)
			}
		}
		if r.round >= 0 {
			summary.addAll(r.game.GameSummary(r.round))
		}
		if !r.game.IsTurnBasedGame() && r.reason != "" {
			summary.add(r.ColoredReason(true, r.reason))
		}
		r.println(summary)
	}

	if len(r.tooltips) > 0 && (r.newRound || r.game.IsTurnBasedGame()) {
		data := newOutput("TOOLTIP")
		for _, t := range r.tooltips {
			data.add(t.message)
			data.add(strconv.Itoa(t.player))
		}
		r.tooltips = r.tooltips[:0]
		r.println(data)
	}
}

func (r *Referee) dumpNextPlayerInfos() {
	data := newOutput("NEXT_PLAYER_INFO")
	data.add(strconv.Itoa(r.nextPlayerIdx))
	data.add(strconv.Itoa(r.game.ExpectedOutputLineCount(r.nextPlayerIdx)))
	if r.round == 0 {
		data.add(strconv.Itoa(r.game.MillisTimeForFirstRound()))
	} else {
		data.add(strconv.Itoa(r.game.MillisTimeForRound()))
	}
	r.println(data)
}

func (r *Referee) dumpNextPlayerInput() {
	data := newOutput("NEXT_PLAYER_INPUT")
	if r.round == 0 {
		data.addAll(r.game.InitInputForPlayer(r.nextPlayerIdx))
	}
	p := r.players[r.nextPlayerIdx]
	if r.game.IsTurnBasedGame() || r.opts.InputsOnDemand {
		p.nextInput = r.game.InputForPlayer(r.round, r.nextPlayerIdx)
	}
	data.addAll(p.nextInput)
	r.println(data)
}

// appendDataToEnd is MultiReferee.appendDataToEnd. Unlike the other blocks
// it is written line by line, without OutputData's trim. Keys are printed
// sorted, not in Java's Hashtable order.
func (r *Referee) appendDataToEnd() {
	props := r.properties
	if props == nil {
		props = jprops.New()
	}
	fmt.Fprintf(r.out, "[[UINPUT] %d]\n", props.Len())
	for _, k := range props.Keys() {
		v, _ := props.Get(k)
		fmt.Fprintf(r.out, "%s=%s\n", k, v)
	}
	r.out.Flush()
}
