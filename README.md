# codingame-referees

Go ports of the official CodinGame Java referees. Given the same seed and
the same bot outputs, a ported referee plays the same game as the Java
original. The players get the same input, and the game ends with the same
outcome and scores. Lines that only feed the viewer don't need to match.

## Layout

| Package | Purpose |
|---|---|
| `internal/cgref` | Port of the framework embedded in the Java referees (`AbstractReferee`, `MultiReferee`) and its stdin/stdout protocol |
| `internal/harness` | Plays the game runner's side of the protocol against a Java or Go referee, records the game and compares two games |
| `internal/jrandom` | `java.util.Random`, bit for bit |
| `internal/jprops` | `java.util.Properties.load` parsing rules |

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
hand-written protocol code in onboarding and Game of Drones.

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
