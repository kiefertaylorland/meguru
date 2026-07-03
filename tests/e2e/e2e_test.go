package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	buildErr  error
	binDir    string
	binPath   string

	// coverDir, when set via MEGURU_E2E_COVERDIR, makes buildBinary produce a
	// coverage-instrumented binary and every spawned meguru process write
	// its coverage counters there (merged afterward with `go tool covdata`).
	// Unset (the default for a normal `go test` run), this is a no-op.
	coverDir = os.Getenv("MEGURU_E2E_COVERDIR")
)

// buildBinary compiles cmd/meguru once per test binary run and returns its
// path. The build directory is removed by TestMain after all tests finish.
func buildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		binDir, buildErr = os.MkdirTemp("", "meguru-e2e-bin")
		if buildErr != nil {
			return
		}
		binPath = filepath.Join(binDir, "meguru-e2e")
		args := []string{"build"}
		if coverDir != "" {
			args = append(args, "-cover")
		}
		args = append(args, "-o", binPath, "../../cmd/meguru")
		cmd := exec.Command("go", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = err
			t.Logf("build output: %s", out)
		}
	})
	if buildErr != nil {
		t.Fatalf("build meguru binary: %v", buildErr)
	}
	return binPath
}

// withCoverEnv appends GOCOVERDIR to env when coverage collection is active.
func withCoverEnv(env []string) []string {
	if coverDir == "" {
		return env
	}
	return append(env, "GOCOVERDIR="+coverDir)
}

func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		os.RemoveAll(binDir)
	}
	os.Exit(code)
}
