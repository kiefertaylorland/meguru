package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"meguru/internal/deck"
	"meguru/internal/review"
)

// Immediately after a fresh seed, a due card must be available with no
// further action (SC-001; data-model.md "Initial state").
func TestFirstRun_DueCardImmediatelyAfterSeed(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, deck.Seed(ctx, db, now))

	svc := review.NewService(db)
	card, err := svc.NextDueCard(ctx)
	require.NoError(t, err)
	require.NotNil(t, card, "expected a due card immediately after seeding")
	require.NotEmpty(t, card.Expression)
}
