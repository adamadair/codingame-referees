# codingame-referees

Go ports of the official CodinGame Java referees. Given the same seed and
the same bot outputs, a ported referee plays the same game as the Java
original. The players get the same input, and the game ends with the same
outcome and scores. Lines that only feed the viewer don't need to match.

## Commands

Each ported referee is a command that speaks the CodinGame runner protocol
on stdin and stdout, so local runners such as cg-brutaltester can use it.

| Command | Game |
|---|---|
| `cg-game-of-drones` | Game of Drones |

```bash
go install github.com/adamadair/codingame-referees/cmd/cg-game-of-drones@latest
```

```bash
cg-game-of-drones -h
```

`-l <file>` appends errors to a log file as well as stderr. By default no
log file is written.

## Layout

| Package | Purpose |
|---|---|
| `cmd/cg-*` | One command per ported referee |
| `internal/games/*` | The ported game referees |
| `internal/cgref` | Port of the framework embedded in the Java referees (`AbstractReferee`, `MultiReferee`) and its stdin/stdout protocol |
| `internal/harness` | Plays the game runner's side of the protocol against a Java or Go referee, records the game and compares two games |
| `internal/jrandom` | `java.util.Random`, bit for bit |
| `internal/jprops` | `java.util.Properties.load` parsing rules |
| `internal/jscanner` | `java.util.Scanner`'s `nextLine` and `nextInt`, used to read runner commands |
| `internal/jstrings` | Java `String.split`, `Integer.parseInt` and `Long.parseLong` edge cases |

## Tests

```bash
go test ./...
```

The comparison tests need `java` and `javac` (JDK 21 is known to work) and
the Java referee sources. They look for the sources in `../cg-java-referees`
next to this repository, or in `$CG_JAVA_REFEREES`. Without them those tests
are skipped.

`internal/cgref` is checked with a small toy game written twice: in
`testdata/ToyGame.java`, appended at test time to a framework copy taken
from an official referee, and in Go. Every scenario must produce the same
game on both. No CodinGame code is copied into this repository.

## Porting a referee

1. Implement `cgref.Game` and embed `cgref.Base` for the Java defaults.
2. Pick `cgref.Options` to match the referee's copy of the framework. The
   copies differ:

   | Referee | `UpdateScoresEachRound` | `InputsOnDemand` |
   |---|---|---|
   | Coders of the Caribbean | | |
   | Code Busters, Fantastic Bits, Hyper Sonic, SmashTheCode | ✓ | |
   | Coders Strike Back, Back to the Code | ✓ | ✓ |
   | Platinum Rift 1/2, Poker Chip Race, The Great Escape | to check | ✓ |

   Implement `cgref.GameOverChecker` if the referee overrides `gameOver()`
   and its framework calls it (Code Busters, Hyper Sonic).
3. Compare the port with the Java referee using `harness.RunCommand` and
   `harness.RunFunc` with deterministic bots, then `harness.Compare`.

Not yet supported: `SoloReferee`, the oldest framework copies' method
signatures (Platinum Rift, Poker Chip Race, The Great Escape), and the
hand-written protocol code in onboarding.

Referees with their own protocol code, like Game of Drones, are ported
without `cgref`: see `internal/games/gameofdrones`. Its tests also show how
to redirect a hard-coded Java log path at test time.

### Java traps found so far

- **Field initializers never run during the game.** `AbstractReferee`'s
  constructor runs the entire game, before the `Referee` subclass's field
  initializers execute. A field declared `int x = -1;` is `0` for the
  whole game, and an object field declared with `= new ...` is `null`
  unless `initReferee` assigns it. Port the zero value, not the
  initializer.
- `Math.round` rounds halves up (`round(-2.5) == -2`); Go's `math.Round`
  doesn't.
- Java `int` is 32 bits, and shift counts are masked to 5 bits. Use `int32`
  where overflow or shifts can happen.
- `Collections.sort` is stable. Use `slices.SortStableFunc`.
- Iterating a `HashMap` or `HashSet` follows Java's bucket order. That
  order matters in Platinum Rift. It isn't reproducible at all for keys
  without `hashCode()` (Poker Chip Race, Code Busters, Hyper Sonic): use a
  fixed order and document it.
- `OutputData.toString()` trims its text, so trailing spaces on a block's
  last line are lost. `cgref` does the same.
- `Integer.parseInt` accepts any Unicode decimal digit in the Basic
  Multilingual Plane (`"١٢٣"` is 123), so use `jstrings.ParseInt`, not
  `strconv.Atoi`, wherever Java parses player output or parameters.
- `String.split` drops trailing empty strings. Use `jstrings.Split`.
- Go may fuse `a*b + c` into one FMA instruction on some CPUs, which Java
  never does. Wrap products in `float64(...)` where the result matters.

# List of Referees to Port
1. Ghost in the cell
2. Coders of the Caribbean
3. Code 4 life
4. Wondev Woman
5. Mean Max
6. Coders strike back
7. Back to the Code
8. Codebusters
9. Game of Drones (ported: `cg-game-of-drones`)
10. Hypersonic
11. Smash the Code
12. The Great Escape
13. Tron
