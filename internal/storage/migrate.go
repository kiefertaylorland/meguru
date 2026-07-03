package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies any pending forward-only schema migrations, tracking the
// applied version in app_state.schema_version (FR-014). Safe to call on
// every startup, including against a fresh, empty database.
func Migrate(db *sql.DB) error {
	return migrateFrom(db, migrationFS, "migrations")
}

// migrateFrom is Migrate with an injectable fs.FS, so failure modes that the
// real, fixed, valid embedded migrations can never produce (unreadable
// directory, broken SQL, duplicate versions) are directly unit-testable
// against an fstest.MapFS.
func migrateFrom(db *sql.DB, migrationsFS fs.FS, dir string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS app_state (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("ensure app_state table: %w", err)
	}

	current, err := schemaVersion(db)
	if err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, dir)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	migrations := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return err
		}
		migrations = append(migrations, migrationFile{version: version, name: entry.Name()})
	}
	// Sort by the parsed numeric version, not the filename string — a
	// lexicographic sort would order "10_x.sql" before "2_y.sql".
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })

	for _, m := range migrations {
		if m.version <= current {
			continue
		}

		sqlBytes, err := fs.ReadFile(migrationsFS, path.Join(dir, m.name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", m.name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		if err := setSchemaVersion(tx, m.version); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		current = m.version
	}
	return nil
}

type migrationFile struct {
	version int
	name    string
}

func schemaVersion(db *sql.DB) (int, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM app_state WHERE key = 'schema_version'`).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read schema_version: %w", err)
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse schema_version %q: %w", value, err)
	}
	return v, nil
}

func setSchemaVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec(`INSERT INTO app_state (key, value) VALUES ('schema_version', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, strconv.Itoa(version))
	return err
}

// migrationVersion extracts the leading integer from a "NNNN_description.sql" filename.
func migrationVersion(filename string) (int, error) {
	prefix, _, ok := strings.Cut(filename, "_")
	if !ok {
		return 0, fmt.Errorf("invalid migration filename %q", filename)
	}
	return strconv.Atoi(prefix)
}
