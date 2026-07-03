package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func openRawTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "meguru.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

// Running Migrate twice against the same DB is a no-op the second time —
// every migration's version is already <= the stored schema_version.
func TestMigrate_IdempotentOnSecondRun(t *testing.T) {
	db := openRawTestDB(t)
	require.NoError(t, Migrate(db))
	require.NoError(t, Migrate(db))

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks`).Scan(&count))
	require.Zero(t, count)
}

// A corrupt (non-numeric) schema_version value surfaces as an error instead
// of silently defaulting to 0 and re-running migrations.
func TestSchemaVersion_ErrorsOnCorruptValue(t *testing.T) {
	db := openRawTestDB(t)
	_, err := db.Exec(`CREATE TABLE app_state (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO app_state (key, value) VALUES ('schema_version', 'not-a-number')`)
	require.NoError(t, err)

	_, err = schemaVersion(db)
	require.Error(t, err)
}

// schemaVersion returns 0 with no error when app_state has no row yet
// (fresh database, before any migration has run).
func TestSchemaVersion_ZeroWhenAbsent(t *testing.T) {
	db := openRawTestDB(t)
	_, err := db.Exec(`CREATE TABLE app_state (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	require.NoError(t, err)

	v, err := schemaVersion(db)
	require.NoError(t, err)
	require.Zero(t, v)
}

func TestMigrationVersion_InvalidFilename(t *testing.T) {
	_, err := migrationVersion("nounderscorehere.sql")
	require.Error(t, err)
}

func TestMigrationVersion_NonNumericPrefix(t *testing.T) {
	_, err := migrationVersion("abc_init.sql")
	require.Error(t, err)
}

func TestMigrationVersion_ParsesLeadingInteger(t *testing.T) {
	v, err := migrationVersion("0001_init.sql")
	require.NoError(t, err)
	require.Equal(t, 1, v)
}

func TestMigrateFrom_AppliesMigrationsInNumericOrderNotLexicographic(t *testing.T) {
	db := openRawTestDB(t)
	fsys := fstest.MapFS{
		"migrations/10_second.sql": {Data: []byte(`CREATE TABLE second (id INTEGER PRIMARY KEY);`)},
		"migrations/2_first.sql":   {Data: []byte(`CREATE TABLE first (id INTEGER PRIMARY KEY);`)},
	}

	require.NoError(t, migrateFrom(db, fsys, "migrations"))

	var version int
	require.NoError(t, db.QueryRow(`SELECT value FROM app_state WHERE key='schema_version'`).Scan(&version))
	require.Equal(t, 10, version)

	for _, table := range []string{"first", "second"} {
		var name string
		require.NoError(t, db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name))
	}
}

func TestMigrateFrom_ErrorReadingDirPropagates(t *testing.T) {
	db := openRawTestDB(t)
	fsys := fstest.MapFS{} // no "migrations" directory at all

	err := migrateFrom(db, fsys, "migrations")

	require.ErrorContains(t, err, "read migrations")
}

func TestMigrateFrom_InvalidMigrationFilenamePropagates(t *testing.T) {
	db := openRawTestDB(t)
	fsys := fstest.MapFS{
		"migrations/nounderscore.sql": {Data: []byte(`SELECT 1;`)},
	}

	err := migrateFrom(db, fsys, "migrations")

	require.Error(t, err)
}

func TestMigrateFrom_ErrorApplyingBrokenSQLRollsBackAndPropagates(t *testing.T) {
	db := openRawTestDB(t)
	fsys := fstest.MapFS{
		"migrations/0001_broken.sql": {Data: []byte(`THIS IS NOT VALID SQL;`)},
	}

	err := migrateFrom(db, fsys, "migrations")

	require.ErrorContains(t, err, "apply migration")

	var version int
	err2 := db.QueryRow(`SELECT value FROM app_state WHERE key='schema_version'`).Scan(&version)
	require.ErrorIs(t, err2, sql.ErrNoRows, "a failed migration must not record a schema_version")
}

func TestMigrateFrom_ErrorSettingSchemaVersionRollsBackAndPropagates(t *testing.T) {
	db := openRawTestDB(t)
	// No PRIMARY KEY/UNIQUE on key: setSchemaVersion's "ON CONFLICT(key)"
	// clause has nothing to match, so its INSERT fails.
	_, err := db.Exec(`CREATE TABLE app_state (key TEXT, value TEXT NOT NULL)`)
	require.NoError(t, err)
	fsys := fstest.MapFS{
		"migrations/0001_x.sql": {Data: []byte(`CREATE TABLE x (id INTEGER PRIMARY KEY);`)},
	}

	err = migrateFrom(db, fsys, "migrations")

	require.Error(t, err)

	var name string
	err2 := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='x'`).Scan(&name)
	require.ErrorIs(t, err2, sql.ErrNoRows, "a failed migration must roll back its own CREATE TABLE too")
}

func TestMigrateFrom_SkipsMigrationsAtOrBelowCurrentVersion(t *testing.T) {
	db := openRawTestDB(t)
	fsys := fstest.MapFS{
		"migrations/0001_first.sql": {Data: []byte(`CREATE TABLE first (id INTEGER PRIMARY KEY);`)},
	}
	require.NoError(t, migrateFrom(db, fsys, "migrations"))

	// Re-running with the same (already-applied) migration must not attempt
	// to re-create the table.
	require.NoError(t, migrateFrom(db, fsys, "migrations"))
}
