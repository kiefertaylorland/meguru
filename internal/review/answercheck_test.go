package review

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckAnswer_MatchesViaConvertedKana(t *testing.T) {
	// Simulates a future kanji/vocab-style card whose Reading is stored as
	// kana, per this feature's documented dual-comparison design.
	card := &Card{Expression: "食べる", Reading: "たべる", Meaning: "to eat"}

	result := CheckAnswer(card, "taberu")

	require.True(t, result.Correct)
	require.Equal(t, "たべる", result.Kana)
}

func TestCheckAnswer_MatchesViaRawRomajiText(t *testing.T) {
	// The M1 built-in hiragana deck stores Reading as plain romaji (a kana
	// card's own pronunciation), e.g. expression "か" / reading "ka".
	card := &Card{Expression: "か", Reading: "ka", Meaning: "ka"}

	result := CheckAnswer(card, "ka")

	require.True(t, result.Correct)
	require.Equal(t, "か", result.Kana)
}

func TestCheckAnswer_NoMatchStillReturnsConvertedKana(t *testing.T) {
	card := &Card{Expression: "か", Reading: "ka", Meaning: "ka"}

	result := CheckAnswer(card, "shi")

	require.False(t, result.Correct)
	require.Equal(t, "し", result.Kana)
}

func TestCheckAnswer_EmptyTypedAnswerDoesNotPanicAndDoesNotMatch(t *testing.T) {
	card := &Card{Expression: "か", Reading: "ka", Meaning: "ka"}

	result := CheckAnswer(card, "")

	require.False(t, result.Correct)
	require.Equal(t, "", result.Kana)
}

func TestCheckAnswer_CaseAndWhitespaceInsensitiveOnRawMatch(t *testing.T) {
	card := &Card{Expression: "か", Reading: "ka", Meaning: "ka"}

	result := CheckAnswer(card, "  KA  ")

	require.True(t, result.Correct)
}

func TestCheckAnswer_UnrecognizedRomajiFailsGracefully(t *testing.T) {
	card := &Card{Expression: "か", Reading: "ka", Meaning: "ka"}

	result := CheckAnswer(card, "1234")

	require.False(t, result.Correct)
	require.Equal(t, "1234", result.Kana)
}
