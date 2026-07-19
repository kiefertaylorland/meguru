package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// setXDGDataHome mirrors internal/storage/db_test.go's withXDGDataHome:
// adrg/xdg computes XDG_DATA_HOME once at package init, so runReview/runStats
// (which call storage.Open) can only be isolated in-process via an explicit
// xdg.Reload().
func setXDGDataHome(t *testing.T, dir string) {
	t.Helper()
	// Run after t.Setenv restores the prior env var, so xdg's cached DataHome stays in sync.
	t.Cleanup(xdg.Reload)
	t.Setenv("XDG_DATA_HOME", dir)
	xdg.Reload()
}

// runCommand executes the full root command tree in-process with scripted
// stdin, capturing combined stdout output.
func runCommand(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	root := NewRootCommand()
	root.SetArgs(args)
	root.SetIn(strings.NewReader(stdin))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	err := root.Execute()
	return out.String(), err
}

// A full --plain review session runs the whole runReview startup sequence
// (open, migrate, seed, dispatch to the plain renderer) in-process. The
// scripted stdin answers one card and rates it Again; the session then ends
// cleanly on stdin EOF (same contract as tests/e2e/plain_test.go).
func TestRunReview_Plain_FullSession(t *testing.T) {
	setXDGDataHome(t, t.TempDir())

	out, err := runCommand(t, "a\nagain\n", "review", "--plain")

	require.NoError(t, err)
	require.Contains(t, out, "Expression:")
	require.Contains(t, out, "Recorded: Again")
}

// storage.Open failing (data dir can't be created) must surface as an error
// return from the review command, not a panic (contracts/cli.md).
func TestRunReview_StorageOpenFailure_ReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-denial via chmod isn't meaningful on Windows")
	}

	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	require.NoError(t, os.MkdirAll(blocked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })
	setXDGDataHome(t, blocked)

	_, err := runCommand(t, "", "review", "--plain")

	require.Error(t, err)
}

// A DB whose schema conflicts with the embedded migration makes
// storage.Migrate fail — the migration-failure branch of runReview
// (recipe from tests/e2e/error_exit_test.go).
func TestRunReview_MigrateFailure_ReturnsError(t *testing.T) {
	base := t.TempDir()
	dataDir := filepath.Join(base, "meguru")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	dbPath := filepath.Join(dataDir, "meguru.db")

	db, err := sql.Open("sqlite", "file:"+dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE decks (id INTEGER PRIMARY KEY)`) // conflicts with migration 0001's CREATE TABLE decks
	require.NoError(t, err)
	require.NoError(t, db.Close())
	require.NoError(t, os.Chmod(dbPath, 0o600))
	setXDGDataHome(t, base)

	_, err = runCommand(t, "", "review", "--plain")

	require.Error(t, err)
}

// runStats against a fresh (never-reviewed, never-seeded) DB prints the
// plain-text summary with the no-cards fallback.
func TestRunStats_PlainOutput_FreshDB(t *testing.T) {
	setXDGDataHome(t, t.TempDir())

	out, err := runCommand(t, "", "stats")

	require.NoError(t, err)
	require.Contains(t, out, "Due now:")
	require.Contains(t, out, "Total cards:")
	require.Contains(t, out, "no cards scheduled")
}

// The --json branch emits the contract shape from contracts/stats-cli.md,
// with null retention/next-due on a fresh DB.
func TestRunStats_JSONOutput_FreshDB(t *testing.T) {
	setXDGDataHome(t, t.TempDir())

	out, err := runCommand(t, "", "stats", "--json")

	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	require.Equal(t, float64(0), decoded["due_cards"])
	require.Equal(t, float64(0), decoded["total_cards"])
	require.Nil(t, decoded["retention_percent"])
	require.Nil(t, decoded["next_due_at"])
}

// storage.Open failing must surface as an error return from the stats
// command too (its open/migrate sequence mirrors runReview's).
func TestRunStats_StorageOpenFailure_ReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-denial via chmod isn't meaningful on Windows")
	}

	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	require.NoError(t, os.MkdirAll(blocked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })
	setXDGDataHome(t, blocked)

	_, err := runCommand(t, "", "stats")

	require.Error(t, err)
}

// After a review session has seeded the decks and logged one review, stats
// sees the data through the same XDG data dir.
func TestRunStats_AfterPlainReview_CountsSeededCards(t *testing.T) {
	setXDGDataHome(t, t.TempDir())

	_, err := runCommand(t, "a\nagain\n", "review", "--plain")
	require.NoError(t, err)

	out, err := runCommand(t, "", "stats", "--json")

	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	total, ok := decoded["total_cards"].(float64)
	require.True(t, ok)
	require.Greater(t, total, float64(0))
}
