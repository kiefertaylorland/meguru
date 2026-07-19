package textwidth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWidth_ASCII(t *testing.T) {
	require.Equal(t, 5, Width("hello"))
}

func TestWidth_Empty(t *testing.T) {
	require.Equal(t, 0, Width(""))
}

// Hiragana characters are East-Asian wide — each occupies two terminal
// columns, unlike len() which would count runes or bytes.
func TestWidth_WideCJKCharacters(t *testing.T) {
	require.Equal(t, 2, Width("あ"))
	require.Equal(t, 6, Width("あいう"))
}

func TestWidth_MixedASCIIAndCJK(t *testing.T) {
	require.Equal(t, 4, Width("aあb")) // 1 (a) + 2 (あ) + 1 (b)
}

func TestTruncate_ShorterThanMaxReturnsUnchanged(t *testing.T) {
	require.Equal(t, "hi", Truncate("hi", 10))
}

func TestTruncate_ExactWidthReturnsUnchanged(t *testing.T) {
	require.Equal(t, "hello", Truncate("hello", 5))
}

func TestTruncate_CutsAtWidthBoundary(t *testing.T) {
	require.Equal(t, "hel", Truncate("hello world", 3))
}

// Truncate must not split a wide CJK character's cluster in half.
func TestTruncate_DoesNotSplitWideCharacters(t *testing.T) {
	// "あい" is 4 columns wide; asking for 3 must drop the whole second
	// character rather than emit a corrupt half-character.
	require.Equal(t, "あ", Truncate("あい", 3))
}

func TestTruncate_ZeroWidth(t *testing.T) {
	require.Equal(t, "", Truncate("hello", 0))
}

// A zero-width grapheme (e.g. a zero-width space) still counts as one
// column so it isn't invisible to width-based layout math.
func TestWidth_ZeroWidthGraphemeCountsAsOneColumn(t *testing.T) {
	require.Equal(t, 1, Width("\u200b"))
	require.Equal(t, 2, Width("a\u200b")) // 1 (a) + 1 (zero-width space, floored up)
}
