# CLAUDE.md

Go ports of the official CodinGame Java referees. Read README.md first: it
covers the layout, the porting steps, the `cgref.Options` table and the Java
traps found so far. This file adds the working rules.

## Goal: same game per seed
- The Java referee is the spec, bugs and quirks included. Given the same
  parameters (or seed) and the same bot outputs, a port must give players
  the same input, the same outcome and the same scores.
- Viewer-only output (VIEW, INFOS, SUMMARY, TOOLTIP) should be close but
  doesn't have to match.
- Never "fix" Java behaviour in a port. If a quirk looks like a bug, keep
  it, add a comment, and mention it to the user.

## Checking work
- Run `go vet ./... && go test ./...`.
- The Java comparison tests need `java`/`javac` (JDK 21) and the Java
  sources in `../cg-java-referees` or `$CG_JAVA_REFEREES`. **They skip
  without them.** A skipped comparison means the port is not verified, so
  say so instead of reporting success.
- Every port needs a Java comparison test built on `internal/harness`:
  deterministic bots, a seed sweep, bad parameters, invalid output,
  timeouts and a replay from UINPUT. `internal/games/gameofdrones` is the
  example to follow.
- Check that new tests really ran (`-v`, look for SKIP) and exercise what
  their names claim.

## Rules
- Never copy CodinGame code into this repository. Tests read, extract or
  patch the Java sources at test time (see `setupJava` in the Game of
  Drones tests and `buildToyJava` in cgref).
- Test data derived from Java comes from a `testdata/Gen.java` generator
  (see `internal/jprops`, `internal/jstrings`). Regenerate it with Java;
  never edit `expected.txt` by hand.
- Where code goes:
  - `internal/j*` reproduces Java library behaviour;
  - `internal/cgref` is for referees built on the shared framework;
  - each game lives in `internal/games/<name>` with a command at
    `cmd/cg-<name>`, which has a `-h` help text and a `-l` log option.
- Port line by line and keep Java's names and structure, so a port can be
  checked against the Java source side by side.
- Use `jrandom`, `jstrings.ParseInt`/`ParseLong`/`Split` and `jscanner`
  wherever the Java code uses their Java counterparts. Don't use `strconv`
  or `math/rand` for these. Wrap floating-point products in
  `float64(...)` where results matter, so Go doesn't fuse them into FMA
  instructions.
- When you find a new Java trap, add it to the README's list.
