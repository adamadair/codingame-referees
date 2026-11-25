package cgref

// These errors replace the exceptions the Java framework uses for control
// flow. A game returns them from HandlePlayerOutput, UpdateGame and
// InitReferee.
//
// A reason code of "" stands for Java's null.

// LostError is LostException: the player is eliminated.
type LostError struct {
	Code   string
	Values []any
}

func (e *LostError) Error() string { return "lost: " + e.Code }

// Lost returns a LostError with a message code and its format arguments.
func Lost(code string, values ...any) error { return &LostError{Code: code, Values: values} }

// WinError is WinException: the player has won and leaves the turn order.
type WinError struct {
	Code   string
	Values []any
}

func (e *WinError) Error() string { return "win: " + e.Code }

// Win returns a WinError with a message code and its format arguments.
func Win(code string, values ...any) error { return &WinError{Code: code, Values: values} }

// InvalidInputError is InvalidInputException: the player's output could
// not be parsed. Its reason code is always "InvalidInput".
type InvalidInputError struct {
	Expected, Found string
}

func (e *InvalidInputError) Error() string {
	return "invalid input: expected " + e.Expected + " but found " + e.Found
}

// InvalidInput returns an InvalidInputError.
func InvalidInput(expected, found string) error {
	return &InvalidInputError{Expected: expected, Found: found}
}

// GameOverError is GameOverException: the game ends immediately.
type GameOverError struct {
	Code   string
	Values []any
}

func (e *GameOverError) Error() string { return "game over: " + e.Code }

// GameOver returns a GameOverError. Use "" for a game over with no reason.
func GameOver(code string, values ...any) error { return &GameOverError{Code: code, Values: values} }

// InvalidFormatError is InvalidFormatException: the game parameters are
// unusable. Returned from InitReferee.
type InvalidFormatError struct {
	Msg string
}

func (e *InvalidFormatError) Error() string { return e.Msg }

// InvalidFormat returns an InvalidFormatError.
func InvalidFormat(msg string) error { return &InvalidFormatError{Msg: msg} }

// errGameError is GameErrorException: the referee stops after reporting an
// init error, but still prints the scores.
type errGameError struct{}

func (errGameError) Error() string { return "game error" }

// playerError extracts the code and values of a Lost, Win or InvalidInput
// error.
func playerError(err error) (code string, values []any) {
	switch e := err.(type) {
	case *LostError:
		return e.Code, e.Values
	case *WinError:
		return e.Code, e.Values
	case *InvalidInputError:
		return "InvalidInput", []any{e.Expected, e.Found}
	}
	return "", nil
}
