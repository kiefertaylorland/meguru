package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/stretchr/testify/require"
)

// withXDGDataHome points the process-global xdg.DataHome at a temp dir for
// the duration of one test, restoring it afterward. adrg/xdg computes
// XDG_DATA_HOME once at package init, so DataDir()/Open() (which use it
// directly, unlike the injectable dataDirIn/openIn variants) can only be
// exercised in-process via this reload hook.
func withXDGDataHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", dir)
	xdg.Reload()
	t.Cleanup(xdg.Reload) // t.Setenv restores the env var; re-sync xdg's cache to match
}

// openIn/dataDirIn create the directory and DB file fresh, with a working
// WAL-mode, foreign-keys-on connection (FR-001).
func TestOpenIn_CreatesDirAndDBFresh(t *testing.T) {
	base := t.TempDir()

	db, err := openIn(base)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Ping())

	dir := filepath.Join(base, "meguru")
	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	_, err = os.Stat(filepath.Join(dir, "meguru.db"))
	require.NoError(t, err)
}

// A second Open against the same base dir reuses the existing dir/file
// without erroring (idempotent startup path).
func TestOpenIn_ReopensExistingDB(t *testing.T) {
	base := t.TempDir()

	db1, err := openIn(base)
	require.NoError(t, err)
	require.NoError(t, db1.Close())

	db2, err := openIn(base)
	require.NoError(t, err)
	defer db2.Close()
	require.NoError(t, db2.Ping())
}

// dataDirIn surfaces a stat error other than "not exist" (e.g. a permission
// error walking to the path) rather than silently swallowing it.
func TestDataDirIn_PropagatesStatError(t *testing.T) {
	skipOnWindows(t) // chmod-based permission denial isn't meaningful on Windows

	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	require.NoError(t, os.MkdirAll(blocked, 0o000))
	t.Cleanup(func() { os.Chmod(blocked, 0o700) }) // allow TempDir cleanup

	_, err := dataDirIn(blocked)
	require.Error(t, err)
}

// openIn propagates an error from dataDirIn (e.g. directory creation
// failure) instead of proceeding to open a DB file in a nonexistent dir.
func TestOpenIn_PropagatesDataDirError(t *testing.T) {
	skipOnWindows(t)

	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	require.NoError(t, os.MkdirAll(blocked, 0o000))
	t.Cleanup(func() { os.Chmod(blocked, 0o700) })

	_, err := openIn(blocked)
	require.Error(t, err)
}

// DataDir is dataDirIn(xdg.DataHome) — exercised directly (not just via its
// injectable half) against a temp XDG_DATA_HOME.
func TestDataDir_UsesRealXDGDataHome(t *testing.T) {
	base := t.TempDir()
	withXDGDataHome(t, base)

	dir, err := DataDir()

	require.NoError(t, err)
	require.Equal(t, filepath.Join(base, "meguru"), dir)
}

// Open is openIn(xdg.DataHome) — exercised directly against a temp
// XDG_DATA_HOME, same rationale as TestDataDir_UsesRealXDGDataHome.
func TestOpen_UsesRealXDGDataHome(t *testing.T) {
	base := t.TempDir()
	withXDGDataHome(t, base)

	db, err := Open()

	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())
}
