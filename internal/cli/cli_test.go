package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testUsage = "test-referee: a referee for tests\n"

func ok(in io.Reader, out io.Writer) error {
	_, err := io.WriteString(out, "[[SCORES] 1]\n0 1\n")
	return err
}

func fail(in io.Reader, out io.Writer) error {
	return errors.New("input ended before the game was over")
}

func run(play Play, args ...string) (int, string, string) {
	var out, errOut strings.Builder
	code := Run("test-referee", testUsage, play, args, strings.NewReader(""), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestHelp(t *testing.T) {
	for _, arg := range []string{"-h", "-help", "--help"} {
		code, _, help := run(ok, arg)
		if code != 0 {
			t.Errorf("%s: exit %d, want 0", arg, code)
		}
		for _, want := range []string{testUsage, "Exit status", "-l logfile"} {
			if !strings.Contains(help, want) {
				t.Errorf("%s: help text lacks %q", arg, want)
			}
		}
	}
}

func TestBadArguments(t *testing.T) {
	if code, _, _ := run(ok, "-x"); code != 2 {
		t.Errorf("unknown flag: exit %d, want 2", code)
	}
	if code, _, _ := run(ok, "extra"); code != 2 {
		t.Errorf("extra argument: exit %d, want 2", code)
	}
}

func TestNormalGame(t *testing.T) {
	log := filepath.Join(t.TempDir(), "referee.log")
	code, out, _ := run(ok, "-l", log)
	if code != 0 || out != "[[SCORES] 1]\n0 1\n" {
		t.Errorf("exit %d, output %q", code, out)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Error("log file was written on a normal game")
	}
}

// TestLogFileOnError checks that -l receives each error and keeps earlier
// entries.
func TestLogFileOnError(t *testing.T) {
	log := filepath.Join(t.TempDir(), "referee.log")
	for range 2 {
		code, _, errOut := run(fail, "-l", log)
		if code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		if !strings.Contains(errOut, "test-referee: input ended before the game was over") {
			t.Errorf("stderr = %q", errOut)
		}
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "test-referee: input ended before the game was over"); n != 2 {
		t.Errorf("log has %d entries, want 2:\n%s", n, data)
	}
}

// TestNoLogFileByDefault checks that an error without -l only reaches
// stderr.
func TestNoLogFileByDefault(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if code, _, _ := run(fail); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("files written without -l: %v", entries)
	}
}
