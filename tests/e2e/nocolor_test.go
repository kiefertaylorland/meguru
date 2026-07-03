package e2e

import (
	"context"
	"io"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
)

var sgrColorEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// NO_COLOR suppresses all color/style escape codes even when the
// interactive TUI (not --plain) is used — interactive redraws are still
// permitted, only color/style is suppressed (FR-011).
func TestNoColor_SuppressesColorInInteractiveMode(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "review")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+dataDir, "NO_COLOR=1"))

	ptmx, err := pty.Start(cmd)
	require.NoError(t, err)
	defer ptmx.Close()

	// Let the first frame render, then quit.
	time.Sleep(300 * time.Millisecond)
	_, err = ptmx.Write([]byte("q"))
	require.NoError(t, err)

	output, _ := io.ReadAll(ptmx) // reads until the pty slave closes on process exit
	_ = cmd.Wait()

	require.False(t, sgrColorEscape.Match(output), "NO_COLOR must suppress color/style escape codes, got: %q", output)
}
