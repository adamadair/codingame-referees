package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// JavaRefereesEnv names the environment variable that points at the folder
// of official Java referees (one subfolder per game).
const JavaRefereesEnv = "CG_JAVA_REFEREES"

// JavaRefereesDir returns the folder of Java referee sources: $CG_JAVA_REFEREES,
// or ../cg-java-referees next to this module. ok is false if neither exists.
func JavaRefereesDir() (dir string, ok bool) {
	if d := os.Getenv(JavaRefereesEnv); d != "" {
		return d, isDir(d)
	}
	root, err := moduleRoot()
	if err != nil {
		return "", false
	}
	d := filepath.Join(filepath.Dir(root), "cg-java-referees")
	return d, isDir(d)
}

// JavaAvailable reports whether java and javac are on the PATH.
func JavaAvailable() bool {
	_, err1 := exec.LookPath("java")
	_, err2 := exec.LookPath("javac")
	return err1 == nil && err2 == nil
}

// CompileJava compiles Java sources into outDir with javac.
func CompileJava(ctx context.Context, outDir string, sources ...string) error {
	args := append([]string{"-d", outDir, "-encoding", "UTF-8", "-nowarn", "-Xlint:none"}, sources...)
	out, err := exec.CommandContext(ctx, "javac", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("javac: %w\n%s", err, out)
	}
	return nil
}

// JavaCommand returns a command that runs the compiled Referee class in
// classDir. Pass it to RunCommand.
func JavaCommand(ctx context.Context, classDir string) *exec.Cmd {
	return exec.CommandContext(ctx, "java", "-cp", classDir, "Referee")
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// moduleRoot finds the directory holding this module's go.mod, starting
// from the working directory (tests run in their package directory).
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
