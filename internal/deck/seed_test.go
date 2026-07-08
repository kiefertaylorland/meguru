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

// testDef is a synthetic Definition used to exercise seedDeck/seedFresh/
// updateInPlace/insertNote in isolation from the real embedded decks and
// from each other, so these unit tests don't depend on BuiltinDecks' order
// or the real decks' content.
func testDef() Definition {
	return Definition{Slug: "test-deck", Name: "Test Deck", Kind: "kana"}
}

// withEmbeddedContent temporarily swaps the embedded hiragana.json content
// for a test, restoring the real one afterward. hiraganaJSON is a plain
// package var (not const), so this is a safe, package-local way to exercise
// paths (malformed JSON) that the real, fixed, valid embedded file can
// never produce.
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

	require.ErrorContains(t, err, "parse embedded kana-hiragana deck")
}

func TestDefinitionContent_ErrorOnMissingLoader(t *testing.T) {
	_, err := Definition{Slug: "missing"}.Content()

	require.ErrorContains(t, err, "invalid Definition for missing: missing content loader")
}

func TestKatakana_ParsesEmbeddedJSON(t *testing.T) {
	content, err := Katakana()
	require.NoError(t, err)
	require.Equal(t, 1, content.ContentVersion)
	require.Len(t, content.Notes, 46)
	require.Equal(t, "ア", content.Notes[0].Expression)
	require.Equal(t, "a", content.Notes[0].Reading)
}

func TestJLPTN5Kanji_ParsesEmbeddedJSON(t *testing.T) {
	content, err := JLPTN5Kanji()
	require.NoError(t, err)
	require.Equal(t, 1, content.ContentVersion)
	require.GreaterOrEqual(t, len(content.Notes), 20)
	require.LessOrEqual(t, len(content.Notes), 40)
	for _, n := range content.Notes {
		require.NotEmpty(t, n.Expression)
		require.NotEmpty(t, n.Reading)
		require.NotEmpty(t, n.Meaning)
	}
}

func TestJLPTN5Vocab_ParsesEmbeddedJSON(t *testing.T) {
	content, err := JLPTN5Vocab()
	require.NoError(t, err)
	require.Equal(t, 1, content.ContentVersion)
	require.GreaterOrEqual(t, len(content.Notes), 20)
	require.LessOrEqual(t, len(content.Notes), 40)
	for _, n := range content.Notes {
		require.NotEmpty(t, n.Expression)
		require.NotEmpty(t, n.Reading)
		require.NotEmpty(t, n.Meaning)
	}
}

// BuiltinDecks must list every built-in deck exactly once, with slugs and
// kinds matching the decks.kind CHECK constraint, and content that actually
// parses — the registry Seed iterates over.
func TestBuiltinDecks_AllDistinctAndValid(t *testing.T) {
	decks := BuiltinDecks()
	require.Len(t, decks, 4)

	seenSlugs := map[string]bool{}
	validKinds := map[string]bool{"kana": true, "kanji": true, "vocab": true, "keigo": true, "sentence": true}
	for _, d := range decks {
		require.NotEmpty(t, d.Slug)
		require.NotEmpty(t, d.Kind)
		require.False(t, seenSlugs[d.Slug], "duplicate slug %s", d.Slug)
		seenSlugs[d.Slug] = true
		require.True(t, validKinds[d.Kind], "unexpected kind %s for %s", d.Kind, d.Slug)
		require.NotEmpty(t, d.Name)

		content, err := d.Content()
		require.NoError(t, err)
		require.NotEmpty(t, content.Notes, "deck %s has no notes", d.Slug)

		seenExpr := map[string]bool{}
		for _, n := range content.Notes {
			require.False(t, seenExpr[n.Expression], "duplicate expression %q within deck %s", n.Expression, d.Slug)
			seenExpr[n.Expression] = true
		}
	}
}

func TestBuiltinDecks_ReturnsCopy(t *testing.T) {
	decks := BuiltinDecks()
	decks[0] = Definition{}

	content, err := Hiragana()

	require.NoError(t, err)
	require.NotEmpty(t, content.Notes)
	require.Equal(t, HiraganaSlug, BuiltinDecks()[0].Slug)
}

func TestLookupBuiltin_UnknownSlugReturnsError(t *testing.T) {
	_, err := lookupBuiltin("missing")

	require.ErrorContains(t, err, "unknown builtin deck missing")
}

// Seed must load every builtin deck on a fresh database.
func TestSeed_SeedsAllBuiltinDecks(t *testing.T) {
	db := openTestDB(t)
	now := time.Now()

	require.NoError(t, Seed(context.Background(), db, now))

	builtins := BuiltinDecks()
	var deckCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks`).Scan(&deckCount))
	require.Equal(t, len(builtins), deckCount)

	for _, d := range builtins {
		content, err := d.Content()
		require.NoError(t, err)

		var deckID int64
		require.NoError(t, db.QueryRow(`SELECT id FROM decks WHERE slug = ?`, d.Slug).Scan(&deckID))

		var noteCount, cardCount, stateCount int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notes WHERE deck_id = ?`, deckID).Scan(&noteCount))
		require.NoError(t, db.QueryRow(
			`SELECT COUNT(*) FROM cards c JOIN notes n ON n.id = c.note_id WHERE n.deck_id = ?`, deckID).
			Scan(&cardCount))
		require.NoError(t, db.QueryRow(
			`SELECT COUNT(*) FROM srs_state s JOIN cards c ON c.id = s.card_id JOIN notes n ON n.id = c.note_id WHERE n.deck_id = ?`,
			deckID).Scan(&stateCount))

		require.Equal(t, len(content.Notes), noteCount, "deck %s note count", d.Slug)
		require.Equal(t, len(content.Notes), cardCount, "deck %s card count", d.Slug)
		require.Equal(t, len(content.Notes), stateCount, "deck %s srs_state count", d.Slug)
	}
}

// Re-running Seed must not duplicate any builtin deck's row, notes, or
// cards.
func TestSeed_SecondRunDoesNotDuplicateAnyBuiltinDeck(t *testing.T) {
	db := openTestDB(t)
	now := time.Now()

	require.NoError(t, Seed(context.Background(), db, now))
	require.NoError(t, Seed(context.Background(), db, now))

	builtins := BuiltinDecks()
	var deckCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks`).Scan(&deckCount))
	require.Equal(t, len(builtins), deckCount)

	var wantNotes int
	for _, d := range builtins {
		content, err := d.Content()
		require.NoError(t, err)
		wantNotes += len(content.Notes)
	}

	var noteCount, cardCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&noteCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&cardCount))
	require.Equal(t, wantNotes, noteCount)
	require.Equal(t, wantNotes, cardCount)
}

func TestSeed_LookupErrorPropagates(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Close())

	err := Seed(context.Background(), db, time.Now())

	require.Error(t, err)
}

func TestSeed_ParsesAllBuiltinDecksBeforeWriting(t *testing.T) {
	original := builtinDecks
	t.Cleanup(func() { builtinDecks = original })
	validRaw, err := json.Marshal(Content{
		ContentVersion: 1,
		Notes:          []Note{{Expression: "x", Reading: "x", Meaning: "x"}},
	})
	require.NoError(t, err)
	builtinDecks = []Definition{
		{Slug: "valid", Name: "Valid", Kind: "kana", raw: func() []byte { return validRaw }},
		{Slug: "bad", Name: "Bad", Kind: "kana", raw: func() []byte { return []byte("not json") }},
	}
	db := openTestDB(t)

	err = Seed(context.Background(), db, time.Now())

	require.ErrorContains(t, err, "parse embedded bad deck")
	var deckCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks`).Scan(&deckCount))
	require.Zero(t, deckCount)
}

func TestSeed_RollsBackAllBuiltinWritesOnLaterSeedError(t *testing.T) {
	original := builtinDecks
	t.Cleanup(func() { builtinDecks = original })
	raw, err := json.Marshal(Content{
		ContentVersion: 1,
		Notes:          []Note{{Expression: "x", Reading: "x", Meaning: "x"}},
	})
	require.NoError(t, err)
	builtinDecks = []Definition{
		{Slug: "valid", Name: "Valid", Kind: "kana", raw: func() []byte { return raw }},
		{Slug: "bad", Name: "Bad", Kind: "invalid", raw: func() []byte { return raw }},
	}
	db := openTestDB(t)

	err = Seed(context.Background(), db, time.Now())

	require.ErrorContains(t, err, "insert deck")
	var deckCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks`).Scan(&deckCount))
	require.Zero(t, deckCount)
}

func TestSeedDeck_FreshSeedCreatesDeckNotesCardsAndSrsState(t *testing.T) {
	db := openTestDB(t)
	content := Content{ContentVersion: 1, Notes: []Note{
		{Expression: "x", Reading: "x", Meaning: "x"},
		{Expression: "y", Reading: "y", Meaning: "y"},
	}}
	now := time.Now()

	require.NoError(t, seedDeck(context.Background(), db, testDef(), content, now))

	var deckCount, noteCount, cardCount, stateCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks WHERE slug = ?`, testDef().Slug).Scan(&deckCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&noteCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&cardCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM srs_state`).Scan(&stateCount))
	require.Equal(t, 1, deckCount)
	require.Equal(t, 2, noteCount)
	require.Equal(t, 2, cardCount)
	require.Equal(t, 2, stateCount)
}

func TestSeedDeck_NoOpWhenContentVersionUnchanged(t *testing.T) {
	db := openTestDB(t)
	content := Content{ContentVersion: 1, Notes: []Note{{Expression: "x", Reading: "x", Meaning: "x"}}}
	now := time.Now()

	require.NoError(t, seedDeck(context.Background(), db, testDef(), content, now))
	require.NoError(t, seedDeck(context.Background(), db, testDef(), content, now))

	var updatedAt1, updatedAt2 string
	require.NoError(t, db.QueryRow(`SELECT updated_at FROM notes LIMIT 1`).Scan(&updatedAt1))
	require.NoError(t, seedDeck(context.Background(), db, testDef(), content, now.Add(time.Hour)))
	require.NoError(t, db.QueryRow(`SELECT updated_at FROM notes LIMIT 1`).Scan(&updatedAt2))
	require.Equal(t, updatedAt1, updatedAt2, "unchanged content_version must not touch notes")
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
	content := Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}}

	err = seedDeck(context.Background(), db, testDef(), content, time.Now())

	require.ErrorContains(t, err, "insert deck")
}

func TestSeedFresh_ErrorInsertingNoteRollsBackDeckInsert(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(`DROP TABLE notes`)
	require.NoError(t, err)
	content := Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}}

	err = seedDeck(context.Background(), db, testDef(), content, time.Now())
	require.ErrorContains(t, err, "insert note")

	var deckCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks`).Scan(&deckCount))
	require.Zero(t, deckCount, "a failed seed must roll back the deck row too")
}

func TestInsertNote_ErrorInsertingCardPropagates(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(`DROP TABLE cards`)
	require.NoError(t, err)
	content := Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}}

	err = seedDeck(context.Background(), db, testDef(), content, time.Now())

	require.ErrorContains(t, err, "insert card")
}

func TestInsertNote_ErrorInsertingSrsStatePropagates(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(`DROP TABLE srs_state`)
	require.NoError(t, err)
	content := Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}}

	err = seedDeck(context.Background(), db, testDef(), content, time.Now())

	require.ErrorContains(t, err, "insert srs_state")
}

func TestUpdateInPlace_ErrorUpdatingNotePropagates(t *testing.T) {
	db := openTestDB(t)
	content := Content{ContentVersion: 1, Notes: []Note{{Expression: "x"}}}
	require.NoError(t, seedDeck(context.Background(), db, testDef(), content, time.Now()))

	_, err := db.Exec(`DROP TABLE notes`)
	require.NoError(t, err)
	content2 := Content{ContentVersion: 2, Notes: []Note{{Expression: "x", Reading: "new"}}}

	err = seedDeck(context.Background(), db, testDef(), content2, time.Now())

	require.ErrorContains(t, err, "update note")
}

// A content-version bump updates existing notes' fields in place by their
// stable expression key, bumps the stored content_version, and never
// touches cards/srs_state/review_log for notes that already exist (FR-004).
func TestUpdateInPlace_UpdatesFieldsPreservesSchedulingState(t *testing.T) {
	db := openTestDB(t)
	content := Content{ContentVersion: 1, Notes: []Note{{Expression: "x", Reading: "old", Meaning: "old"}}}
	now := time.Now()
	require.NoError(t, seedDeck(context.Background(), db, testDef(), content, now))

	var cardID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM cards LIMIT 1`).Scan(&cardID))
	_, err := db.Exec(`UPDATE srs_state SET reps = 5, due_at = '2030-01-01T00:00:00Z' WHERE card_id = ?`, cardID)
	require.NoError(t, err)
	var reviewLogCountBefore int
	_, err = db.Exec(`INSERT INTO review_log (card_id, rating, reviewed_at, state_before) VALUES (?, 3, ?, 'new')`,
		cardID, now.UTC().Format(time.RFC3339))
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&reviewLogCountBefore))

	content2 := Content{ContentVersion: 2, Notes: []Note{{Expression: "x", Reading: "new", Meaning: "new"}}}
	require.NoError(t, seedDeck(context.Background(), db, testDef(), content2, now.Add(time.Hour)))

	var reading string
	var storedVersion, reps, reviewLogCountAfter int
	var dueAt string
	require.NoError(t, db.QueryRow(`SELECT json_extract(fields, '$.reading') FROM notes LIMIT 1`).Scan(&reading))
	require.NoError(t, db.QueryRow(`SELECT content_version FROM decks WHERE slug = ?`, testDef().Slug).Scan(&storedVersion))
	require.NoError(t, db.QueryRow(`SELECT reps, due_at FROM srs_state WHERE card_id = ?`, cardID).Scan(&reps, &dueAt))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&reviewLogCountAfter))

	require.Equal(t, "new", reading)
	require.Equal(t, 2, storedVersion)
	require.Equal(t, 5, reps, "content update must not reset scheduling progress")
	require.Equal(t, "2030-01-01T00:00:00Z", dueAt, "content update must not touch due_at")
	require.Equal(t, reviewLogCountBefore, reviewLogCountAfter, "content update must not touch review_log")
}
