package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"meguru/internal/deck"
)

// Seeding twice must not duplicate the deck, its notes, or its cards
// (FR-002, FR-003).
func TestSeed_DoesNotDuplicateOnSecondRun(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, deck.Seed(ctx, db, now))
	require.NoError(t, deck.Seed(ctx, db, now))

	var deckCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM decks`).Scan(&deckCount))
	require.Equal(t, 1, deckCount)

	content, err := deck.Hiragana()
	require.NoError(t, err)

	var noteCount, cardCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&noteCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&cardCount))
	require.Equal(t, len(content.Notes), noteCount)
	require.Equal(t, len(content.Notes), cardCount)
}
