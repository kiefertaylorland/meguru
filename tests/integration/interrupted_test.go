package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"meguru/internal/deck"
	"meguru/internal/review"
)

// If the process is interrupted after a card is shown but before a rating is
// submitted, Rate() is simply never called — no transaction was opened, so
// no partial review_log row exists and the card remains due (FR-015, SC-006).
func TestReview_InterruptedBeforeRating_LeavesNoPartialState(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, deck.Seed(ctx, db, now))
	svc := review.NewService(db)

	shown, err := svc.NextDueCard(ctx)
	require.NoError(t, err)
	require.NotNil(t, shown)

	// Simulated interruption: the process dies here, before svc.Rate is called.

	var logCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&logCount))
	require.Zero(t, logCount, "no review_log row may exist before a rating is submitted")

	var reps int
	require.NoError(t, db.QueryRow(`SELECT reps FROM srs_state WHERE card_id = ?`, shown.ID).Scan(&reps))
	require.Zero(t, reps)

	// On the next run, the same card is still presented as due.
	stillShown, err := svc.NextDueCard(ctx)
	require.NoError(t, err)
	require.NotNil(t, stillShown)
	require.Equal(t, shown.ID, stillShown.ID)
}
