package e2e

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A full review session can be completed entirely in --plain mode with zero
// color or interactive escape sequences in the output (SC-003, FR-010).
func TestPlainMode_NoEscapeSequences(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	cmd := exec.Command(bin, "review", "--plain")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+dataDir))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stdin = strings.NewReader("a\nagain\n")

	require.NoError(t, cmd.Run())

	out := stdout.String()
	require.NotContains(t, out, "\x1b", "plain mode output must contain no ESC escape sequences")
	require.Contains(t, out, "Expression:")
	require.Contains(t, out, "Recorded: Again")
}
