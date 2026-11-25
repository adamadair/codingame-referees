package cgref

import "github.com/adamadair/codingame-referees/internal/jprops"

// Game is the part of a referee that a port has to write. Each method
// matches an abstract (or overridable) method of the Java MultiReferee /
// AbstractReferee, under the same name without the "get" prefix.
//
// Embed Base to get the Java defaults for everything except the core
// methods: InitReferee, Configuration, ExpectedOutputLineCount,
// HandlePlayerOutput, InitInputForPlayer, InputForPlayer,
// MinimumPlayerCount, UpdateGame and Score.
//
// Strings returned as "" stand for Java's null where the framework checks
// for null (HeadlineAtGameStart, DeathReason).
type Game interface {
	// InitReferee receives the game parameters. Return an InvalidFormatError
	// (or any error) to report an init error.
	InitReferee(playerCount int, props *jprops.Properties) error
	// Configuration returns the parameters printed in UINPUT at the end, so
	// the game can be replayed. It usually includes the seed.
	Configuration() *jprops.Properties
	PopulateMessages(messages map[string]string)
	InitDataForView() []string
	FrameDataForView(round, frame int, keyFrame bool) []string
	ExpectedOutputLineCount(player int) int
	GameName() string
	// HandlePlayerOutput applies one player's output. It may return a
	// LostError, WinError or InvalidInputError. Any other error crashes the
	// referee, like an uncaught Java exception.
	HandlePlayerOutput(frame, round, player int, output []string) error
	InitInputForPlayer(player int) []string
	InputForPlayer(round, player int) []string
	HeadlineAtGameStart() string
	MinimumPlayerCount() int
	ShowTooltips() bool
	// UpdateGame advances the simulation one round. It may return a
	// GameOverError.
	UpdateGame(round int) error
	Prepare(round int)
	IsPlayerDead(player int) bool
	DeathReason(player int) string
	Score(player int) int
	GameSummary(round int) []string
	PlayerActions(player, round int) []string
	SetPlayerTimeout(frame, round, player int)

	IsTurnBasedGame() bool
	MillisTimeForFirstRound() int
	MillisTimeForRound() int
	MaxRoundCount(playerCount int) int
}

// GameOverChecker is implemented by games that override gameOver(). Without
// it, the game ends when fewer than MinimumPlayerCount players are alive.
type GameOverChecker interface {
	GameOver() bool
}

// refereeSetter is implemented by Base so the framework can give the game
// access to Translate, AddToolTip and the other protected helpers.
type refereeSetter interface {
	SetReferee(r *Referee)
}

// Base provides the Java framework defaults. Embed it in a game struct.
type Base struct {
	ref *Referee
}

// SetReferee is called by Run before InitReferee.
func (b *Base) SetReferee(r *Referee) { b.ref = r }

// Ref returns the running referee, for Translate, AddToolTip and PrintError.
func (b *Base) Ref() *Referee { return b.ref }

func (*Base) PopulateMessages(map[string]string)                   {}
func (*Base) InitDataForView() []string                            { return nil }
func (*Base) FrameDataForView(round, frame int, key bool) []string { return nil }
func (*Base) GameName() string                                     { return "" }
func (*Base) HeadlineAtGameStart() string                          { return "" }
func (*Base) ShowTooltips() bool                                   { return false }
func (*Base) Prepare(round int)                                    {}
func (*Base) IsPlayerDead(player int) bool                         { return false }
func (*Base) DeathReason(player int) string                        { return "" }
func (*Base) GameSummary(round int) []string                       { return nil }
func (*Base) PlayerActions(player, round int) []string             { return nil }
func (*Base) SetPlayerTimeout(frame, round, player int)            {}
func (*Base) IsTurnBasedGame() bool                                { return false }
func (*Base) MillisTimeForFirstRound() int                         { return 1000 }
func (*Base) MillisTimeForRound() int                              { return 150 }
func (*Base) MaxRoundCount(playerCount int) int                    { return 400 }
