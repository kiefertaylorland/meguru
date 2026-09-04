package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"meguru/internal/deck"
	"meguru/internal/review"
)

func TestShouldUsePlain(t *testing.T) {
	cases := []struct {
		name        string
		plainFlag   bool
		stdoutIsTTY bool
		want        bool
	}{
		{"plain flag forces it even on a TTY", true, true, true},
		{"plain flag forces it on a non-TTY too", true, false, true},
		{"non-TTY forces it without the flag", false, false, true},
		{"TTY without the flag uses the interactive TUI", false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldUsePlain(tc.plainFlag, tc.stdoutIsTTY))
		})
	}
}

func TestProgramOptions_NoColorSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	opts := programOptions()

	require.Len(t, opts, 1)
}

func TestProgramOptions_NoColorUnset(t *testing.T) {
	require.NoError(t, os.Unsetenv("NO_COLOR"))

	opts := programOptions()

	require.Empty(t, opts)
}

func TestNewReviewCommand_RegistersPlainFlag(t *testing.T) {
	cmd := newReviewCommand()

	flag := cmd.Flags().Lookup("plain")
	require.NotNil(t, flag)
	require.Equal(t, "false", flag.DefValue)
}

func TestResolveDeckFlag(t *testing.T) {
	t.Run("empty scope", func(t *testing.T) {
		scope, err := resolveDeckFlag("")

		require.NoError(t, err)
		require.Equal(t, review.DeckScope{}, scope)
	})

	t.Run("known deck", func(t *testing.T) {
		scope, err := resolveDeckFlag("kana-hiragana")

		require.NoError(t, err)
		require.Equal(t, review.DeckScope{
			Slug: "kana-hiragana",
			Name: "Hiragana",
		}, scope)
	})

	t.Run("unknown deck", func(t *testing.T) {
		_, err := resolveDeckFlag("missing")

		require.EqualError(t, err, `unknown deck "missing" — valid decks: kana-hiragana (Hiragana), kana-katakana (Katakana), jlpt-n5-kanji (JLPT N5 Kanji), jlpt-n5-vocab (JLPT N5 Vocabulary)`)
	})
}

func TestDeckScopes(t *testing.T) {
	defs := []deck.Definition{
		{Slug: "first", Name: "First"},
		{Slug: "second", Name: "Second"},
	}

	require.Equal(t, []review.DeckScope{
		{Slug: "first", Name: "First"},
		{Slug: "second", Name: "Second"},
	}, deckScopes(defs))
}
