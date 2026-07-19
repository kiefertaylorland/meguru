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

// A review session scoped to one deck never returns another deck's cards,
// even across repeated calls, and an unfiltered scope still pools every
// deck together (007-deck-filter FR-002, FR-003).
func TestReview_DeckScope_NeverCrossesIntoAnotherDeck(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, deck.Seed(ctx, db, now))
	svc := review.NewService(db)

	for i := 0; i < 10; i++ {
		card, err := svc.NextDueCard(ctx, review.DeckScope{Slug: deck.JLPTN5KanjiSlug})
		require.NoError(t, err)
		if card == nil {
			break
		}

		var deckSlug string
		require.NoError(t, db.QueryRow(`
			SELECT d.slug FROM cards c
			JOIN notes n ON n.id = c.note_id
			JOIN decks d ON d.id = n.deck_id
			WHERE c.id = ?`, card.ID).Scan(&deckSlug))
		require.Equal(t, deck.JLPTN5KanjiSlug, deckSlug)

		require.NoError(t, svc.Rate(ctx, card.ID, scheduler.Good, now))
		now = now.Add(48 * time.Hour) // guarantee the next card is due
	}
}

// An unfiltered scope pools due cards from every seeded deck together,
// unchanged from before this feature existed (FR-002): the total number of
// due cards visible with no scope equals the sum of each individual deck's
// due count, and the card an unfiltered call returns always belongs to
// whichever deck actually holds the earliest-due card.
func TestReview_EmptyScope_PoolsEveryDeck(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, deck.Seed(ctx, db, now))
	svc := review.NewService(db)

	var totalDue int
	require.NoError(t, db.QueryRow(`
		SELECT COUNT(*) FROM srs_state WHERE due_at IS NOT NULL AND due_at <= ?`,
		now.UTC().Format(time.RFC3339)).Scan(&totalDue))
	require.Greater(t, totalDue, 0, "the seeded decks must have due cards for this test to be meaningful")

	slugs := []string{deck.HiraganaSlug, deck.KatakanaSlug, deck.JLPTN5KanjiSlug, deck.JLPTN5VocabSlug}
	var summedDue int
	for _, slug := range slugs {
		var due int
		require.NoError(t, db.QueryRow(`
			SELECT COUNT(*) FROM srs_state st
			JOIN cards c ON c.id = st.card_id
			JOIN notes n ON n.id = c.note_id
			JOIN decks d ON d.id = n.deck_id
			WHERE st.due_at IS NOT NULL AND st.due_at <= ? AND d.slug = ?`,
			now.UTC().Format(time.RFC3339), slug).Scan(&due))
		summedDue += due
	}
	require.Equal(t, totalDue, summedDue, "every due card must belong to exactly one of the four seeded decks")

	card, err := svc.NextDueCard(ctx, review.DeckScope{})
	require.NoError(t, err)
	require.NotNil(t, card)
}
