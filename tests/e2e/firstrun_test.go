package e2e

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A user on a freshly wiped machine can go from first launch to seeing
// their first due card in under 5 seconds, with zero manual setup (SC-001,
// User Story 1).
func TestFirstRun_ShowsDueCardUnderFiveSeconds(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	cmd := exec.Command(bin, "review", "--plain")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+dataDir))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stdin = strings.NewReader("") // no rating submitted — just observe the shown card

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Less(t, elapsed, 5*time.Second, "first run must show a due card in under 5s")
	require.Contains(t, stdout.String(), "Expression:")
}
