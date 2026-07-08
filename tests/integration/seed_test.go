package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"meguru/internal/deck"
)

// Seeding twice must not duplicate any built-in deck, its notes, or its
// cards (FR-002, FR-003) — proven across every built-in deck (hiragana,
// katakana, N5 kanji, N5 vocab), not just the single M1 hiragana deck.
func TestSeed_DoesNotDuplicateOnSecondRun(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, deck.Seed(ctx, db, now))
	require.NoError(t, deck.Seed(ctx, db, now))

	builtins := deck.BuiltinDecks()
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
