package caribbean

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adamadair/codingame-referees/internal/harness"
)

// javaClasses holds the compiled official referee plus testdata's
// MoveToDriver, or "" when Java or the sources are unavailable.
var javaClasses string

func TestMain(m *testing.M) {
	code, cleanup := setupJava()
	if code == 0 {
		code = m.Run()
	}
	cleanup()
	os.Exit(code)
}

// setupJava compiles the official Referee.java unchanged, together with
// our MoveToDriver.java, once for all tests.
func setupJava() (int, func()) {
	src, ok := harness.JavaRefereesDir()
	if !ok || !harness.JavaAvailable() {
		return 0, func() {}
	}
	dir, err := os.MkdirTemp("", "caribbean-java")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, func() {}
	}
	cleanup := func() { os.RemoveAll(dir) }
	err = harness.CompileJava(context.Background(), dir,
		filepath.Join(src, "Coders of the Caribbean", "Referee.java"),
		filepath.Join("testdata", "MoveToDriver.java"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, cleanup
	}
	javaClasses = dir
	return 0, cleanup
}

func requireJava(t *testing.T) {
	t.Helper()
	if javaClasses == "" {
		t.Skipf("java/javac or the Java referees are unavailable (set %s)", harness.JavaRefereesEnv)
	}
}
