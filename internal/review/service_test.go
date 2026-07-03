package review

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"meguru/internal/scheduler"
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

func seedOneCard(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	_, err := db.Exec(`INSERT INTO decks (slug, name, kind, source, content_version) VALUES ('t','T','kana','builtin',1)`)
	require.NoError(t, err)
	res, err := db.Exec(`INSERT INTO notes (deck_id, fields, created_at, updated_at) VALUES (1, '{"expression":"a","reading":"a","meaning":"a"}', '2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	noteID, err := res.LastInsertId()
	require.NoError(t, err)
	res, err = db.Exec(`INSERT INTO cards (note_id, direction) VALUES (?, 'recognition')`, noteID)
	require.NoError(t, err)
	cardID, err := res.LastInsertId()
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO srs_state (card_id, state, due_at) VALUES (?, 'new', '2020-01-01T00:00:00Z')`, cardID)
	require.NoError(t, err)
	return cardID
}

func TestNextDueCard_ReturnsNilWhenNothingDue(t *testing.T) {
	db := openTestDB(t)
	svc := NewService(db)

	card, err := svc.NextDueCard(context.Background())

	require.NoError(t, err)
	require.Nil(t, card)
}

func TestNextDueCard_ReturnsErrorOnClosedDB(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Close())
	svc := NewService(db)

	_, err := svc.NextDueCard(context.Background())

	require.Error(t, err)
}

func TestNextDueCard_ReturnsErrorOnMalformedFieldsJSON(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(`INSERT INTO decks (slug, name, kind, source, content_version) VALUES ('t','T','kana','builtin',1)`)
	require.NoError(t, err)
	res, err := db.Exec(`INSERT INTO notes (deck_id, fields, created_at, updated_at) VALUES (1, 'not-json', '2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	noteID, _ := res.LastInsertId()
	res, err = db.Exec(`INSERT INTO cards (note_id, direction) VALUES (?, 'recognition')`, noteID)
	require.NoError(t, err)
	cardID, _ := res.LastInsertId()
	_, err = db.Exec(`INSERT INTO srs_state (card_id, state, due_at) VALUES (?, 'new', '2020-01-01T00:00:00Z')`, cardID)
	require.NoError(t, err)

	svc := NewService(db)
	_, err = svc.NextDueCard(context.Background())

	require.ErrorContains(t, err, "parse card fields")
}

func TestRate_ReturnsErrorForUnknownCard(t *testing.T) {
	db := openTestDB(t)
	svc := NewService(db)

	err := svc.Rate(context.Background(), 999, scheduler.Good, time.Now())

	require.Error(t, err)
}

func TestRate_ReturnsErrorOnClosedDB(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	require.NoError(t, db.Close())
	svc := NewService(db)

	err := svc.Rate(context.Background(), cardID, scheduler.Good, time.Now())

	require.Error(t, err)
}

// scheduled_days is the number of days between now and the newly computed
// due_at, per scheduler.NextDue's fixed intervals.
func TestRate_ComputesScheduledDaysFromNextDue(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	svc := NewService(db)
	now := time.Now()

	require.NoError(t, svc.Rate(context.Background(), cardID, scheduler.Easy, now))

	var scheduledDays float64
	require.NoError(t, db.QueryRow(`SELECT scheduled_days FROM review_log WHERE card_id = ?`, cardID).Scan(&scheduledDays))
	require.InDelta(t, 7.0, scheduledDays, 0.01, "Easy schedules 7 days out")
}

// elapsed_days is only populated once a card has a prior last_review_at; the
// first-ever rating leaves it NULL.
func TestRate_FirstRatingLeavesElapsedDaysNull(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	svc := NewService(db)

	require.NoError(t, svc.Rate(context.Background(), cardID, scheduler.Good, time.Now()))

	var elapsed sql.NullFloat64
	require.NoError(t, db.QueryRow(`SELECT elapsed_days FROM review_log WHERE card_id = ?`, cardID).Scan(&elapsed))
	require.False(t, elapsed.Valid)
}

// A second rating computes elapsed_days from the first rating's timestamp.
func TestRate_SecondRatingComputesElapsedDays(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	svc := NewService(db)

	first := time.Now()
	require.NoError(t, svc.Rate(context.Background(), cardID, scheduler.Again, first))
	second := first.Add(2 * time.Hour)
	require.NoError(t, svc.Rate(context.Background(), cardID, scheduler.Good, second))

	var elapsed sql.NullFloat64
	require.NoError(t, db.QueryRow(`SELECT elapsed_days FROM review_log WHERE card_id = ? ORDER BY id DESC LIMIT 1`, cardID).Scan(&elapsed))
	require.True(t, elapsed.Valid)
	require.InDelta(t, 2.0/24.0, elapsed.Float64, 0.001)
}

// A rating of Again increments lapses; any other rating does not.
func TestRate_AgainIncrementsLapses(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	svc := NewService(db)

	require.NoError(t, svc.Rate(context.Background(), cardID, scheduler.Again, time.Now()))

	var lapses, reps int
	require.NoError(t, db.QueryRow(`SELECT lapses, reps FROM srs_state WHERE card_id = ?`, cardID).Scan(&lapses, &reps))
	require.Equal(t, 1, lapses)
	require.Equal(t, 1, reps)
}

func TestRate_ErrorInsertingReviewLogPropagates(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	_, err := db.Exec(`DROP TABLE review_log`)
	require.NoError(t, err)
	svc := NewService(db)

	err = svc.Rate(context.Background(), cardID, scheduler.Good, time.Now())

	require.ErrorContains(t, err, "insert review_log")
}

func TestRate_ErrorUpdatingSrsStatePropagates(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	_, err := db.Exec(`DROP TABLE srs_state`)
	require.NoError(t, err)
	// Recreate with a CHECK that rejects Rate's own 'learning' state, so the
	// SELECT (reading the current 'nonsense' state) still succeeds and only
	// the subsequent UPDATE fails.
	_, err = db.Exec(`CREATE TABLE srs_state (
		card_id INTEGER PRIMARY KEY,
		state TEXT NOT NULL CHECK (state IN ('nonsense')),
		stability REAL NOT NULL DEFAULT 0,
		difficulty REAL NOT NULL DEFAULT 0,
		due_at TEXT,
		last_review_at TEXT,
		reps INTEGER NOT NULL DEFAULT 0,
		lapses INTEGER NOT NULL DEFAULT 0)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO srs_state (card_id, state, due_at) VALUES (?, 'nonsense', '2020-01-01T00:00:00Z')`, cardID)
	require.NoError(t, err)
	svc := NewService(db)

	err = svc.Rate(context.Background(), cardID, scheduler.Good, time.Now())

	require.ErrorContains(t, err, "update srs_state")
}

func TestRate_GoodDoesNotIncrementLapses(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	svc := NewService(db)

	require.NoError(t, svc.Rate(context.Background(), cardID, scheduler.Good, time.Now()))

	var lapses int
	require.NoError(t, db.QueryRow(`SELECT lapses FROM srs_state WHERE card_id = ?`, cardID).Scan(&lapses))
	require.Zero(t, lapses)
}
