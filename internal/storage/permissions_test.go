package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// Exact octal permission bits are only meaningful on POSIX (research.md §2's
// Windows caveat) — Windows is skipped rather than asserted against.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("exact permission bits are not asserted on Windows; see research.md §2")
	}
}

// A newly created data directory and DB file are owner-only: 0700 / 0600
// (SC-004, FR-012).
func TestEnsurePerm_CreatesWithOwnerOnlyPermissions(t *testing.T) {
	skipOnWindows(t)

	dir := filepath.Join(t.TempDir(), "meguru")
	require.NoError(t, ensurePerm(dir, dirPerm, func() error { return os.MkdirAll(dir, dirPerm) }))

	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(dirPerm), info.Mode().Perm())

	dbPath := filepath.Join(dir, "meguru.db")
	require.NoError(t, ensurePerm(dbPath, filePerm, func() error {
		f, ferr := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, filePerm)
		if ferr != nil {
			return ferr
		}
		return f.Close()
	}))

	info, err = os.Stat(dbPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(filePerm), info.Mode().Perm())
}
