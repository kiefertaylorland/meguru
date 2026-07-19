package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"meguru/internal/deck"
	"meguru/internal/review"
	"meguru/internal/scheduler"
	"meguru/internal/stats"
)

// A learner who has never reviewed anything still gets a clean, non-error
// stats summary reflecting the freshly-seeded deck (spec.md Edge Cases,
// Acceptance Scenario 4).
func TestStats_FreshlySeededDatabase_NoReviewsYet(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, deck.Seed(ctx, db, now))

	svc := stats.NewService(db)
	summary, err := svc.Compute(ctx, now)

	require.NoError(t, err)
	require.Positive(t, summary.TotalCards, "the seeded hiragana deck must contribute cards")
	require.Equal(t, summary.TotalCards, summary.DueCards, "every freshly seeded card starts due")
	require.Equal(t, 0, summary.StreakDays)
	require.Nil(t, summary.RetentionPercent)
}

// Rating cards over several simulated days produces a streak and retention
// that match what an independent count of the same review_log rows would
// give (spec.md Acceptance Scenarios 1, 2, 5; SC-002).
func TestStats_AfterReviewsAcrossSeveralDays_StreakAndRetentionMatch(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, deck.Seed(ctx, db, now))
	reviewSvc := review.NewService(db)

	// Day -2: one Good review.
	card, err := reviewSvc.NextDueCard(ctx, review.DeckScope{})
	require.NoError(t, err)
	require.NotNil(t, card)
	require.NoError(t, reviewSvc.Rate(ctx, card.ID, scheduler.Good, now.AddDate(0, 0, -2)))

	// Day -1: one Again review on a different card.
	card, err = reviewSvc.NextDueCard(ctx, review.DeckScope{})
	require.NoError(t, err)
	require.NotNil(t, card)
	require.NoError(t, reviewSvc.Rate(ctx, card.ID, scheduler.Again, now.AddDate(0, 0, -1)))

	// Today: one Easy review.
	card, err = reviewSvc.NextDueCard(ctx, review.DeckScope{})
	require.NoError(t, err)
	require.NotNil(t, card)
	require.NoError(t, reviewSvc.Rate(ctx, card.ID, scheduler.Easy, now))

	statsSvc := stats.NewService(db)
	summary, err := statsSvc.Compute(ctx, now)

	require.NoError(t, err)
	require.Equal(t, 3, summary.StreakDays, "reviews on day -2, -1, and today form an unbroken 3-day streak")
	require.NotNil(t, summary.RetentionPercent)
	// 2 non-Again out of 3 total reviews in the 30-day window.
	require.InDelta(t, 66.67, *summary.RetentionPercent, 0.01)
}

// An entirely empty database (no decks ever seeded) is not an error.
func TestStats_EmptyDatabase_NoDecksSeeded(t *testing.T) {
	db := openTestDB(t)

	svc := stats.NewService(db)
	summary, err := svc.Compute(context.Background(), time.Now())

	require.NoError(t, err)
	require.Equal(t, 0, summary.DueCards)
	require.Equal(t, 0, summary.TotalCards)
	require.Nil(t, summary.NextDueAt)
}
