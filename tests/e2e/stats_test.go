package e2e

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// `meguru stats` against a freshly created, never-reviewed profile exits 0
// and shows zero due/total counts (stats does not seed) with no color escape
// codes (contracts/stats-cli.md).
func TestStats_PlainMode_FreshProfile(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	cmd := exec.Command(bin, "stats")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+dataDir))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	require.NoError(t, cmd.Run())

	out := stdout.String()
	require.NotContains(t, out, "\x1b", "stats output must contain no ESC escape sequences")
	require.Contains(t, out, "Due now")
	require.Contains(t, out, "Total cards")
	require.Contains(t, out, "Streak")
	require.Contains(t, out, "n/a (no reviews yet)")
}

// `meguru stats --json` emits exactly one valid JSON object on stdout, safe
// to pipe into a script (SC-004, US-11).
func TestStats_JSONMode_FreshProfile(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	cmd := exec.Command(bin, "stats", "--json")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+dataDir))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	require.NoError(t, cmd.Run())

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &decoded), "stdout must be valid JSON: %q", stdout.String())
	require.Nil(t, decoded["retention_percent"], "no reviews yet must report retention as null, not 0")
	require.Equal(t, float64(0), decoded["streak_days"])
}

// After completing one review, stats reflects a 1-day streak and 100%
// retention, confirming the command reads live data rather than a stale
// snapshot.
func TestStats_AfterOneReview_ReflectsUpdatedStreakAndRetention(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()
	env := withCoverEnv(append(exec.Command(bin).Environ(), "XDG_DATA_HOME="+dataDir))

	reviewCmd := exec.Command(bin, "review", "--plain")
	reviewCmd.Env = env
	reviewCmd.Stdin = strings.NewReader("good\n")
	require.NoError(t, reviewCmd.Run())

	statsCmd := exec.Command(bin, "stats", "--json")
	statsCmd.Env = env
	var stdout bytes.Buffer
	statsCmd.Stdout = &stdout
	require.NoError(t, statsCmd.Run())

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &decoded))
	require.Equal(t, float64(1), decoded["streak_days"])
	require.Equal(t, float64(100), decoded["retention_percent"])
}
