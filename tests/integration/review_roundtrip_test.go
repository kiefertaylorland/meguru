package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"meguru/internal/deck"
	"meguru/internal/review"
	"meguru/internal/scheduler"
)

// Rating a card writes a permanent review record and reschedules it via
// real FSRS output: due dates always move into the future, and an Again
// rating always comes due sooner than an Easy rating on an identical fresh
// card (FR-005, FR-006, SC-002).
func TestReview_RateAgainAndEasy_ReschedulesAndLogs(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, deck.Seed(ctx, db, now))
	svc := review.NewService(db)

	again, err := svc.NextDueCard(ctx)
	require.NoError(t, err)
	require.NotNil(t, again)
	require.NoError(t, svc.Rate(ctx, again.ID, scheduler.Again, now))

	var dueAt string
	require.NoError(t, db.QueryRow(`SELECT due_at FROM srs_state WHERE card_id = ?`, again.ID).Scan(&dueAt))
	againDue, err := time.Parse(time.RFC3339, dueAt)
	require.NoError(t, err)
	require.True(t, againDue.After(now), "Again's next due date must be in the future")

	var logCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_log WHERE card_id = ?`, again.ID).Scan(&logCount))
	require.Equal(t, 1, logCount)

	// Next due card is a different one (the "Again" card isn't due again immediately).
	require.True(t, againDue.After(time.Now().UTC()), "Again card became due before the test could inspect the next card")
	easy, err := svc.NextDueCard(ctx)
	require.NoError(t, err)
	require.NotNil(t, easy)
	require.NotEqual(t, again.ID, easy.ID)
	require.NoError(t, svc.Rate(ctx, easy.ID, scheduler.Easy, now))

	require.NoError(t, db.QueryRow(`SELECT due_at FROM srs_state WHERE card_id = ?`, easy.ID).Scan(&dueAt))
	easyDue, err := time.Parse(time.RFC3339, dueAt)
	require.NoError(t, err)
	require.True(t, easyDue.After(now), "Easy's next due date must be in the future")
	require.True(t, againDue.Before(easyDue), "Again should come due before Easy on identical fresh cards")

	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_log WHERE card_id = ?`, easy.ID).Scan(&logCount))
	require.Equal(t, 1, logCount)

	// The Easy card must not be due again the same day.
	stillDue, err := svc.NextDueCard(ctx)
	require.NoError(t, err)
	if stillDue != nil {
		require.NotEqual(t, easy.ID, stillDue.ID)
	}
}
