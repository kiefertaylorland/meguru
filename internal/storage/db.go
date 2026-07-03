// Package storage manages the on-disk SQLite database: connection, first-run
// creation, permission enforcement, and schema migrations.
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/adrg/xdg"
	_ "modernc.org/sqlite"
)

const (
	dirPerm  = 0o700
	filePerm = 0o600
)

// DataDir returns the directory Meguru stores its local data in, creating it
// (owner-only) if it does not exist, and self-healing looser permissions on
// an existing directory (FR-001, FR-012, FR-013).
func DataDir() (string, error) {
	return dataDirIn(xdg.DataHome)
}

// dataDirIn is DataDir with an injectable base directory, so it's directly
// unit-testable against a temp dir without depending on adrg/xdg's
// process-global XDG_DATA_HOME resolution (computed once at package init).
func dataDirIn(base string) (string, error) {
	dir := filepath.Join(base, "meguru")
	if err := ensurePerm(dir, dirPerm, func() error { return os.MkdirAll(dir, dirPerm) }); err != nil {
		return "", err
	}
	return dir, nil
}

// Open resolves the on-disk database path, creates the directory/file with
// owner-only permissions (self-healing looser existing permissions with a
// warning), and returns a WAL-mode SQLite connection with foreign keys
// enabled.
func Open() (*sql.DB, error) {
	return openIn(xdg.DataHome)
}

// openIn is Open with an injectable base directory; see dataDirIn.
func openIn(base string) (*sql.DB, error) {
	dir, err := dataDirIn(base)
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dir, "meguru.db")
	if err := ensurePerm(dbPath, filePerm, func() error {
		f, ferr := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, filePerm)
		if ferr != nil {
			return ferr
		}
		return f.Close()
	}); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// ensurePerm creates path via create() if absent. On an existing path, it
// self-heals permissions looser than wantPerm with a stderr warning
// (FR-013). Exact octal permission bits are only meaningful on POSIX, so the
// byte-for-byte comparison (and its warning) only runs there; on Windows,
// research.md §2 still requires the os.Chmod call to run unconditionally —
// Go's os package maps it to the nearest ACL-based equivalent (owner-only) —
// just without a comparison that would spuriously fire on every startup
// since Windows doesn't report POSIX-equivalent mode bits back reliably.
func ensurePerm(path string, wantPerm os.FileMode, create func() error) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return create()
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if runtime.GOOS == "windows" {
		return os.Chmod(path, wantPerm)
	}
	if info.Mode().Perm() != wantPerm {
		fmt.Fprintf(os.Stderr, "warning: %s had permissions %#o, correcting to %#o\n", path, info.Mode().Perm(), wantPerm)
		return os.Chmod(path, wantPerm)
	}
	return nil
}
