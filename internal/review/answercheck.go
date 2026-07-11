package review

import (
	"strings"

	"meguru/internal/romaji"
)

// AnswerResult is the outcome of auto-checking a learner's typed romaji
// answer against a card's expected reading (PRD US-3's "Auto-check match?"
// step, contracts/answer-check.md).
type AnswerResult struct {
	// Kana is typedRomaji converted to hiragana via romaji.ToHiragana.
	Kana string
	// Correct reports whether the typed answer matched card.Reading.
	Correct bool
}

// CheckAnswer converts typedRomaji to hiragana and compares it against
// card.Reading. It matches on either the converted kana or the raw typed
// text: the M1 built-in hiragana deck stores Reading as plain romaji (a
// kana card's own pronunciation), while future kanji/vocab decks are
// expected to store Reading as kana — this dual comparison works correctly
// under both conventions (see research.md). It is a pure function with no
// storage access, callable directly by any UI layer.
func CheckAnswer(card *Card, typedRomaji string) AnswerResult {
	kana := romaji.ToHiragana(typedRomaji)
	expected := strings.TrimSpace(card.Reading)
	typedNormalized := strings.ToLower(strings.TrimSpace(typedRomaji))

	correct := kana == expected || typedNormalized == strings.ToLower(expected)
	return AnswerResult{Kana: kana, Correct: correct}
}
