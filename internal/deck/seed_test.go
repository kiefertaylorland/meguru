package deck

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"meguru/internal/storage"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "meguru.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, storage.Migrate(db))
	return db
}

// withEmbeddedContent temporarily swaps the embedded hiragana.json content
// for a test, restoring the real one afterward. hiraganaJSON is a plain
// package var (not const), so this is a safe, package-local way to exercise
// paths (malformed JSON, content-version bumps) that the real, fixed,
// valid embedded file can never produce.
func withEmbeddedContent(t *testing.T, content Content) {
	t.Helper()
	raw, err := json.Marshal(content)
	require.NoError(t, err)
	withRawEmbeddedContent(t, raw)
}

func withRawEmbeddedContent(t *testing.T, raw []byte) {
	t.Helper()
	original := hiraganaJSON
	hiraganaJSON = raw
	t.Cleanup(func() { hiraganaJSON = original })
}

func TestHiragana_ParsesEmbeddedJSON(t *testing.T) {
	content, err := Hiragana()
	require.NoError(t, err)
	require.Equal(t, 1, content.ContentVersion)
	require.NotEmpty(t, content.Notes)
	require.Equal(t, "あ", content.Notes[0].Expression)
}

func TestHiragana_ErrorOnMalformedJSON(t *testing.T) {
	withRawEmbeddedContent(t, []byte("not json"))

	_, err := Hiragana()

	require.ErrorContains(t, err, "parse embedded hiragana deck")
}

func TestSeed_FreshSeedCreatesDeckNotesCardsAndSrsState(t *testing.T) {
	db := openTestDB(t)
	withEmbeddedContent(t, Content{ContentVersion: 1, Notes: []Note{
		{Expression: "x", Reading: "x", Meaning: "x"},
		{Expression: "y", Reading: "y", Meaning: "y"},
	}})
	now := time.Now()

	require.NoError(t, Seed(context.Background(), db, now))

	var deckCount, noteCount, cardCount, stateCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks WHERE slug = ?`, HiraganaSlug).Scan(&deckCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&noteCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&cardCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM srs_state`).Scan(&stateCount))
	require.Equal(t, 1, deckCount)
	require.Equal(t, 2, noteCount)
	require.Equal(t, 2, cardCount)
	require.Equal(t, 2, stateCount)
}

func TestSeed_NoOpWhenContentVersionUnchanged(t *testing.T) {
	db := openTestDB(t)
	withEmbeddedContent(t, Content{ContentVersion: 1, Notes: []Note{{Expression: "x", Reading: "x", Meaning: "x"}}})
	now := time.Now()

	require.NoError(t, Seed(context.Background(), db, now))
	require.NoError(t, Seed(context.Background(), db, now))

	var updatedAt1, updatedAt2 string
	require.NoError(t, db.QueryRow(`SELECT updated_at FROM notes LIMIT 1`).Scan(&updatedAt1))
	require.NoError(t, Seed(context.Background(), db, now.Add(time.Hour)))
	require.NoError(t, db.QueryRow(`SELECT updated_at FROM notes LIMIT 1`).Scan(&updatedAt2))
	require.Equal(t, updatedAt1, updatedAt2, "unchanged content_version must not touch notes")
}

func TestSeed_LookupErrorPropagates(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Close())

	err := Seed(context.Background(), db, time.Now())

	require.Error(t, err)
}

func TestSeedFresh_ErrorInsertingDeckPropagates(t *testing.T) {
	db := openTestDB(t)
	// Recreate decks with a CHECK that rejects seedFresh's own 'kana'/'builtin'
	// values, so the initial (empty-table) lookup still succeeds with
	// sql.ErrNoRows and only the subsequent INSERT fails.
	_, err := db.Exec(`DROP TABLE decks`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE decks (
		id INTEGER PRIMARY KEY,
		slug TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('nonsense')),
		source TEXT NOT NULL,
		content_version INTEGER NOT NULL DEFAULT 1)`)
	require.NoError(t, err)
	withEmbeddedContent(t, Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}})

	err = Seed(context.Background(), db, time.Now())

	require.ErrorContains(t, err, "insert deck")
}

func TestSeedFresh_ErrorInsertingNoteRollsBackDeckInsert(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(`DROP TABLE notes`)
	require.NoError(t, err)
	withEmbeddedContent(t, Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}})

	err = Seed(context.Background(), db, time.Now())
	require.ErrorContains(t, err, "insert note")

	var deckCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks`).Scan(&deckCount))
	require.Zero(t, deckCount, "a failed seed must roll back the deck row too")
}

func TestInsertNote_ErrorInsertingCardPropagates(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(`DROP TABLE cards`)
	require.NoError(t, err)
	withEmbeddedContent(t, Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}})

	err = Seed(context.Background(), db, time.Now())

	require.ErrorContains(t, err, "insert card")
}

func TestInsertNote_ErrorInsertingSrsStatePropagates(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(`DROP TABLE srs_state`)
	require.NoError(t, err)
	withEmbeddedContent(t, Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}})

	err = Seed(context.Background(), db, time.Now())

	require.ErrorContains(t, err, "insert srs_state")
}

func TestUpdateInPlace_ErrorUpdatingNotePropagates(t *testing.T) {
	db := openTestDB(t)
	withEmbeddedContent(t, Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}})
	require.NoError(t, Seed(context.Background(), db, time.Now()))

	_, err := db.Exec(`DROP TABLE notes`)
	require.NoError(t, err)
	withEmbeddedContent(t, Content{ContentVersion: 2, Notes: []Note{{Expression: "x", Reading: "new"}}})

	err = Seed(context.Background(), db, time.Now())

	require.ErrorContains(t, err, "update note")
}

// A content-version bump updates existing notes' fields in place by their
// stable expression key, bumps the stored content_version, and never
// touches cards/srs_state/review_log for notes that already exist (FR-004).
func TestUpdateInPlace_UpdatesFieldsPreservesSchedulingState(t *testing.T) {
	db := openTestDB(t)
	withEmbeddedContent(t, Content{ContentVersion: 1, Notes: []Note{{Expression: "x", Reading: "old", Meaning: "old"}}})
	now := time.Now()
	require.NoError(t, Seed(context.Background(), db, now))

	var cardID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM cards LIMIT 1`).Scan(&cardID))
	_, err := db.Exec(`UPDATE srs_state SET reps = 5, due_at = '2030-01-01T00:00:00Z' WHERE card_id = ?`, cardID)
	require.NoError(t, err)
	var reviewLogCountBefore int
	_, err = db.Exec(`INSERT INTO review_log (card_id, rating, reviewed_at, state_before) VALUES (?, 3, ?, 'new')`,
		cardID, now.UTC().Format(time.RFC3339))
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&reviewLogCountBefore))

	withEmbeddedContent(t, Content{ContentVersion: 2, Notes: []Note{{Expression: "x", Reading: "new", Meaning: "new"}}})
	require.NoError(t, Seed(context.Background(), db, now.Add(time.Hour)))

	var reading string
	var storedVersion, reps, reviewLogCountAfter int
	var dueAt string
	require.NoError(t, db.QueryRow(`SELECT json_extract(fields, '$.reading') FROM notes LIMIT 1`).Scan(&reading))
	require.NoError(t, db.QueryRow(`SELECT content_version FROM decks WHERE slug = ?`, HiraganaSlug).Scan(&storedVersion))
	require.NoError(t, db.QueryRow(`SELECT reps, due_at FROM srs_state WHERE card_id = ?`, cardID).Scan(&reps, &dueAt))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&reviewLogCountAfter))

	require.Equal(t, "new", reading)
	require.Equal(t, 2, storedVersion)
	require.Equal(t, 5, reps, "content update must not reset scheduling progress")
	require.Equal(t, "2030-01-01T00:00:00Z", dueAt, "content update must not touch due_at")
	require.Equal(t, reviewLogCountBefore, reviewLogCountAfter, "content update must not touch review_log")
}
