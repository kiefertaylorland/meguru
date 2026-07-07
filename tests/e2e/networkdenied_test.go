//go:build networkdenied

package e2e

import (
	"bytes"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"
)

// Proves the core loop (storage creation, deck seeding, review,
// rescheduling) completes fully with no network access. This test makes no
// network calls by construction — the meguru binary imports nothing under
// internal/ai — and is meant to be run under an OS-level network block (see
// the ubuntu-only CI job), which is the actual enforcement FR-017 requires;
// a code-level assertion here would only prove intent, not enforcement
// (research.md §6).
func TestNetworkDenied_CoreLoopCompletesFully(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	run := func(stdin string) string {
		cmd := exec.Command(bin, "review", "--plain")
		cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+dataDir))
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stdin = strings.NewReader(stdin)
		require.NoError(t, cmd.Run())
		return out.String()
	}

	first := run("a\ngood\n")
	require.Contains(t, first, "Expression:")
	require.Contains(t, first, "Recorded: Good")

	db, err := sql.Open("sqlite", "file:"+filepath.Join(dataDir, "meguru", "meguru.db")+"?mode=ro")
	require.NoError(t, err)
	defer db.Close()

	var reviewCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&reviewCount))
	require.Equal(t, 1, reviewCount)
}
