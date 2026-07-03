package e2e

import (
	"bytes"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// An unrecoverable startup error (here: storage.Open() failing because the
// data directory can't be created) must exit 1 with exactly one error line
// on stderr — no Cobra usage dump, no duplicated error text (finding #7:
// missing SilenceUsage/SilenceErrors; contracts/cli.md's exit-code table).
func TestReview_StorageOpenFailure_ExitsOneWithSingleErrorLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-denial via chmod isn't meaningful on Windows")
	}

	bin := buildBinary(t)
	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	require.NoError(t, os.MkdirAll(blocked, 0o000))
	t.Cleanup(func() { os.Chmod(blocked, 0o700) })

	cmd := exec.Command(bin, "review", "--plain")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+blocked))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	require.Equal(t, 1, exitErr.ExitCode())

	trimmed := strings.TrimSpace(stderr.String())
	require.NotEmpty(t, trimmed)
	require.NotContains(t, stderr.String(), "Usage:")
	require.Equal(t, 1, strings.Count(trimmed, "\n")+1,
		"expected exactly one error line, got: %q", stderr.String())
}

// A corrupt pre-existing DB file (e.g. from a crashed write or a truncated
// copy) makes SQLite itself fail to open/query it — this must surface as a
// clean exit 1, not a panic or hang, covering the Ping/Migrate failure path
// that a healthy fresh DB never exercises.
func TestReview_CorruptDBFile_ExitsOne(t *testing.T) {
	bin := buildBinary(t)
	base := t.TempDir()
	dataDir := filepath.Join(base, "meguru")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "meguru.db"), []byte("not a valid sqlite file"), 0o600))

	cmd := exec.Command(bin, "review", "--plain")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+base))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	require.Equal(t, 1, exitErr.ExitCode())
	require.NotEmpty(t, strings.TrimSpace(stderr.String()))
}

// A DB whose schema already conflicts with the embedded migration (here:
// "decks" already exists with an incompatible shape, while app_state
// reports no migration has run yet) makes storage.Migrate fail — covering
// the migration-failure branch of runReview's startup sequence.
func TestReview_MigrateFailure_ExitsOne(t *testing.T) {
	bin := buildBinary(t)
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

	cmd := exec.Command(bin, "review", "--plain")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+base))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	require.Equal(t, 1, exitErr.ExitCode())
}

// A DB whose schema_version already claims migration 0001 applied, but is
// actually missing the "notes" table (simulating a prior partial/corrupted
// migration), makes deck.Seed's update-in-place path fail — covering the
// seed-failure branch of runReview's startup sequence.
func TestReview_SeedFailure_ExitsOne(t *testing.T) {
	bin := buildBinary(t)
	base := t.TempDir()
	dataDir := filepath.Join(base, "meguru")
	require.NoError(t, os.MkdirAll(dataDir, 0o700))
	dbPath := filepath.Join(dataDir, "meguru.db")

	db, err := sql.Open("sqlite", "file:"+dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE app_state (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO app_state (key, value) VALUES ('schema_version', '1')`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE decks (
		id INTEGER PRIMARY KEY, slug TEXT UNIQUE NOT NULL, name TEXT NOT NULL,
		kind TEXT NOT NULL, source TEXT NOT NULL, content_version INTEGER NOT NULL DEFAULT 1)`)
	require.NoError(t, err)
	// content_version 0 is older than the embedded deck's, so Seed takes the
	// update-in-place path — which needs a "notes" table that, deliberately,
	// does not exist here.
	_, err = db.Exec(`INSERT INTO decks (slug, name, kind, source, content_version)
		VALUES ('kana-hiragana', 'Hiragana', 'kana', 'builtin', 0)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	require.NoError(t, os.Chmod(dbPath, 0o600))

	cmd := exec.Command(bin, "review", "--plain")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+base))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	require.Equal(t, 1, exitErr.ExitCode())
}
