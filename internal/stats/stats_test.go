package stats

import (
	"context"
	"database/sql"
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

// seedCard inserts one deck/note/card/srs_state row and returns the new
// card's id, so tests can control due_at directly.
func seedCard(t *testing.T, db *sql.DB, dueAt string) int64 {
	t.Helper()
	var deckID int64
	err := db.QueryRow(`SELECT id FROM decks WHERE slug = 't'`).Scan(&deckID)
	if err == sql.ErrNoRows {
		res, ierr := db.Exec(`INSERT INTO decks (slug, name, kind, source, content_version) VALUES ('t','T','kana','builtin',1)`)
		require.NoError(t, ierr)
		deckID, err = res.LastInsertId()
	}
	require.NoError(t, err)

	res, err := db.Exec(`INSERT INTO notes (deck_id, fields, created_at, updated_at) VALUES (?, '{"expression":"a","reading":"a","meaning":"a"}', '2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`, deckID)
	require.NoError(t, err)
	noteID, err := res.LastInsertId()
	require.NoError(t, err)

	res, err = db.Exec(`INSERT INTO cards (note_id, direction) VALUES (?, 'recognition')`, noteID)
	require.NoError(t, err)
	cardID, err := res.LastInsertId()
	require.NoError(t, err)

	if dueAt == "" {
		_, err = db.Exec(`INSERT INTO srs_state (card_id, state) VALUES (?, 'new')`, cardID)
	} else {
		_, err = db.Exec(`INSERT INTO srs_state (card_id, state, due_at) VALUES (?, 'new', ?)`, cardID, dueAt)
	}
	require.NoError(t, err)
	return cardID
}

func insertReviewLog(t *testing.T, db *sql.DB, cardID int64, rating int, reviewedAt string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO review_log (card_id, rating, reviewed_at, state_before) VALUES (?, ?, ?, 'new')`,
		cardID, rating, reviewedAt)
	require.NoError(t, err)
}

func TestCompute_EmptyDatabase(t *testing.T) {
	db := openTestDB(t)
	svc := NewService(db)
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	summary, err := svc.Compute(context.Background(), now)

	require.NoError(t, err)
	require.Equal(t, 0, summary.DueCards)
	require.Equal(t, 0, summary.TotalCards)
	require.Equal(t, 0, summary.StreakDays)
	require.Nil(t, summary.RetentionPercent)
	require.Equal(t, 30, summary.RetentionWindowDays)
	require.Nil(t, summary.NextDueAt)
}

func TestCompute_DueAndTotalCounts(t *testing.T) {
	db := openTestDB(t)
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	seedCard(t, db, "2020-01-01T00:00:00Z") // due (in the past)
	seedCard(t, db, "2030-01-01T00:00:00Z") // not yet due

	svc := NewService(db)
	summary, err := svc.Compute(context.Background(), now)

	require.NoError(t, err)
	require.Equal(t, 1, summary.DueCards)
	require.Equal(t, 2, summary.TotalCards)
}

func TestCompute_NextDueAtIsEarliestScheduled(t *testing.T) {
	db := openTestDB(t)
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	seedCard(t, db, "2030-06-01T00:00:00Z")
	seedCard(t, db, "2030-01-01T00:00:00Z") // earliest
	seedCard(t, db, "")                     // never scheduled (due_at NULL) — excluded

	svc := NewService(db)
	summary, err := svc.Compute(context.Background(), now)

	require.NoError(t, err)
	require.NotNil(t, summary.NextDueAt)
	require.Equal(t, "2030-01-01T00:00:00Z", summary.NextDueAt.UTC().Format(time.RFC3339))
}

func TestCompute_StreakAndRetentionFromReviewLog(t *testing.T) {
	db := openTestDB(t)
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	cardID := seedCard(t, db, "2020-01-01T00:00:00Z")

	// Reviews today and yesterday (streak of 2), one Again and one Good
	// (retention 50%).
	insertReviewLog(t, db, cardID, 1 /* Again */, now.Format(time.RFC3339))
	insertReviewLog(t, db, cardID, 3 /* Good */, now.AddDate(0, 0, -1).Format(time.RFC3339))

	svc := NewService(db)
	summary, err := svc.Compute(context.Background(), now)

	require.NoError(t, err)
	require.Equal(t, 2, summary.StreakDays)
	require.NotNil(t, summary.RetentionPercent)
	require.Equal(t, 50.0, *summary.RetentionPercent)
}

func TestCompute_RetentionExcludesReviewsOutsideWindow(t *testing.T) {
	db := openTestDB(t)
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	cardID := seedCard(t, db, "2020-01-01T00:00:00Z")

	// Outside the 30-day window entirely.
	insertReviewLog(t, db, cardID, 1 /* Again */, now.AddDate(0, 0, -60).Format(time.RFC3339))
	// Inside the window.
	insertReviewLog(t, db, cardID, 4 /* Easy */, now.Format(time.RFC3339))

	svc := NewService(db)
	summary, err := svc.Compute(context.Background(), now)

	require.NoError(t, err)
	require.NotNil(t, summary.RetentionPercent)
	require.Equal(t, 100.0, *summary.RetentionPercent, "the 60-day-old Again rating must not count against retention")
}
