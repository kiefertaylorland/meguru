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

func TestRate_ReturnsErrorOnMalformedLastReviewAt(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	_, err := db.Exec(`UPDATE srs_state SET last_review_at = 'not-rfc3339' WHERE card_id = ?`, cardID)
	require.NoError(t, err)
	svc := NewService(db)

	err = svc.Rate(context.Background(), cardID, scheduler.Good, time.Now())

	require.ErrorContains(t, err, "parse last_review_at")
}

func TestRate_ReturnsErrorWhenNonNewStateMissingLastReviewAt(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	_, err := db.Exec(`UPDATE srs_state SET state = 'learning', last_review_at = NULL WHERE card_id = ?`, cardID)
	require.NoError(t, err)
	svc := NewService(db)

	err = svc.Rate(context.Background(), cardID, scheduler.Good, time.Now())

	require.ErrorContains(t, err, "missing last_review_at")
}

// scheduled_days/due_at reflect real FSRS output now, not a fixed interval;
// assert the invariant the scheduler guarantees (Again <= Hard <= Good <
// Easy's next due date on identical fresh cards) rather than a specific
// magic number. On a brand-new card, Again/Hard/Good all stay short-term
// (scheduled_days == 0, due within minutes) while Easy alone jumps to a
// real multi-day interval — so due_at, not scheduled_days, is the
// meaningful ordering signal here.
func TestRate_ComputesScheduledDaysFromNextDue(t *testing.T) {
	now := time.Now()

	dueAtFor := func(rating scheduler.Rating) time.Time {
		db := openTestDB(t)
		cardID := seedOneCard(t, db)
		svc := NewService(db)
		require.NoError(t, svc.Rate(context.Background(), cardID, rating, now))
		var dueAt string
		require.NoError(t, db.QueryRow(`SELECT due_at FROM srs_state WHERE card_id = ?`, cardID).Scan(&dueAt))
		due, err := time.Parse(time.RFC3339, dueAt)
		require.NoError(t, err)
		return due
	}

	again := dueAtFor(scheduler.Again)
	hard := dueAtFor(scheduler.Hard)
	good := dueAtFor(scheduler.Good)
	easy := dueAtFor(scheduler.Easy)

	require.False(t, hard.Before(again), "Hard should schedule no sooner than Again")
	require.False(t, good.Before(hard), "Good should schedule no sooner than Hard")
	require.True(t, easy.After(good), "Easy should schedule strictly later than Good")
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
// FSRS's ElapsedDays is a whole number of days (floor), so the gap here must
// be at least a full day for a meaningful non-zero assertion.
func TestRate_SecondRatingComputesElapsedDays(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	svc := NewService(db)

	first := time.Now()
	require.NoError(t, svc.Rate(context.Background(), cardID, scheduler.Again, first))
	second := first.Add(30 * time.Hour)
	require.NoError(t, svc.Rate(context.Background(), cardID, scheduler.Good, second))

	var elapsed sql.NullFloat64
	require.NoError(t, db.QueryRow(`SELECT elapsed_days FROM review_log WHERE card_id = ? ORDER BY id DESC LIMIT 1`, cardID).Scan(&elapsed))
	require.True(t, elapsed.Valid)
	require.InDelta(t, 1.0, elapsed.Float64, 0.001)
}

// Under real FSRS semantics, a lapse (srs_state.lapses increment) only
// happens when Again drops a card out of the 'review' state — an Again on a
// brand-new card is not a lapse, it's the card's first (still-learning)
// attempt.
func TestRate_AgainOnNewCardDoesNotIncrementLapses(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	svc := NewService(db)

	require.NoError(t, svc.Rate(context.Background(), cardID, scheduler.Again, time.Now()))

	var lapses, reps int
	require.NoError(t, db.QueryRow(`SELECT lapses, reps FROM srs_state WHERE card_id = ?`, cardID).Scan(&lapses, &reps))
	require.Equal(t, 0, lapses)
	require.Equal(t, 1, reps)
}

// An Again rating on a card that has already reached the 'review' state IS a
// lapse.
func TestRate_AgainOnReviewStateCardIncrementsLapses(t *testing.T) {
	db := openTestDB(t)
	cardID := seedOneCard(t, db)
	svc := NewService(db)
	ctx := context.Background()

	// Get the card into 'review' state first via a run of Good ratings.
	now := time.Now()
	for i := 0; i < 3; i++ {
		require.NoError(t, svc.Rate(ctx, cardID, scheduler.Good, now))
		var dueAt string
		require.NoError(t, db.QueryRow(`SELECT due_at FROM srs_state WHERE card_id = ?`, cardID).Scan(&dueAt))
		due, err := time.Parse(time.RFC3339, dueAt)
		require.NoError(t, err)
		now = due
	}
	var stateBeforeLapse string
	require.NoError(t, db.QueryRow(`SELECT state FROM srs_state WHERE card_id = ?`, cardID).Scan(&stateBeforeLapse))
	require.Equal(t, "review", stateBeforeLapse)

	require.NoError(t, svc.Rate(ctx, cardID, scheduler.Again, now))

	var lapses int
	require.NoError(t, db.QueryRow(`SELECT lapses FROM srs_state WHERE card_id = ?`, cardID).Scan(&lapses))
	require.Equal(t, 1, lapses)
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
	// Recreate without due_at, so the SELECT (a valid 'new' state row) still
	// succeeds and only the subsequent UPDATE (which writes due_at) fails.
	_, err = db.Exec(`CREATE TABLE srs_state (
		card_id INTEGER PRIMARY KEY,
		state TEXT NOT NULL CHECK (state IN ('new','learning','review','relearning')),
		stability REAL NOT NULL DEFAULT 0,
		difficulty REAL NOT NULL DEFAULT 0,
		last_review_at TEXT,
		reps INTEGER NOT NULL DEFAULT 0,
		lapses INTEGER NOT NULL DEFAULT 0)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO srs_state (card_id, state) VALUES (?, 'new')`, cardID)
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

func TestStateToString_PanicsOnUnknownState(t *testing.T) {
	require.Panics(t, func() {
		stateToString(scheduler.State(99))
	})
}
