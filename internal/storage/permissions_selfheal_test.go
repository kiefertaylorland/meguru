package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// On startup, a data directory or DB file found with broader-than-expected
// permissions is corrected and a warning is printed (FR-013).
func TestEnsurePerm_SelfHealsLoosenedPermissions(t *testing.T) {
	skipOnWindows(t)

	dir := filepath.Join(t.TempDir(), "meguru")
	require.NoError(t, os.MkdirAll(dir, dirPerm))
	dbPath := filepath.Join(dir, "meguru.db")
	require.NoError(t, os.WriteFile(dbPath, nil, filePerm))

	require.NoError(t, os.Chmod(dir, 0o755))
	require.NoError(t, os.Chmod(dbPath, 0o644))

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	require.NoError(t, ensurePerm(dir, dirPerm, func() error { return os.MkdirAll(dir, dirPerm) }))
	require.NoError(t, ensurePerm(dbPath, filePerm, func() error {
		f, ferr := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, filePerm)
		if ferr != nil {
			return ferr
		}
		return f.Close()
	}))

	w.Close()
	os.Stderr = origStderr
	var captured bytes.Buffer
	_, _ = captured.ReadFrom(r)

	require.Contains(t, captured.String(), "warning")

	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(dirPerm), info.Mode().Perm())

	info, err = os.Stat(dbPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(filePerm), info.Mode().Perm())
}
