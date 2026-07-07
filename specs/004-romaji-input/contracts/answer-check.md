# Contract: Answer auto-check (`internal/review.CheckAnswer`) and UI integration

Internal package contract. `CheckAnswer` is the seam between the pure `internal/romaji` package
and the review flow; `internal/plain` (this slice's UI integration point) is its caller.

## Public surface

```go
package review

// AnswerResult is the outcome of auto-checking a learner's typed romaji
// answer against a card's expected reading.
type AnswerResult struct {
    Kana    string // typedRomaji converted to hiragana
    Correct bool   // whether the typed answer matched card.Reading
}

// CheckAnswer converts typedRomaji to hiragana and compares it against
// card.Reading, matching on either the converted kana or the raw typed
// text (see research.md's rationale for the dual comparison).
func CheckAnswer(card *Card, typedRomaji string) AnswerResult
```

`CheckAnswer` is a package-level function, not a `Service` method: it needs no storage access
(unlike `NextDueCard`/`Rate`), so it is directly unit-testable and directly callable by any UI
layer without a `Service` in scope.

## Preconditions

- `card` MUST be non-nil (caller error otherwise — the only caller in this codebase,
  `internal/plain.Run`, only calls this after confirming `svc.NextDueCard` returned a non-nil
  card).

## Postconditions

- `Correct == true` if and only if `romaji.ToHiragana(typedRomaji) == strings.TrimSpace(card.
  Reading)` OR `strings.ToLower(strings.TrimSpace(typedRomaji)) ==
  strings.ToLower(strings.TrimSpace(card.Reading))`.
- `Kana` always holds `romaji.ToHiragana(typedRomaji)` regardless of match outcome, so the caller
  can show the learner what their input was interpreted as even on a miss.
- `CheckAnswer` never panics or errors, including on an empty `typedRomaji` (FR-006) — an empty
  string simply fails to match unless `card.Reading` is also empty.
- Pure function: identical `(card, typedRomaji)` always produces an identical `AnswerResult`, no
  side effects, no storage access.

## UI integration contract (`internal/plain.Run`)

Per FR-002–FR-005, `Run`'s per-card loop MUST, in order:

1. Print the card's `Expression` (the prompt).
2. Read one line from the input scanner as the typed answer. On scanner EOF/failure here, return
   immediately (`scanner.Err()`) exactly as the existing rating-read EOF path already does — no
   partial state, no rating submitted.
3. Call `CheckAnswer(card, typedLine)` exactly once and print a match/no-match feedback line.
4. Reveal `Reading` and `Meaning` (unconditionally — regardless of match/no-match, per FR-004).
5. Proceed to the existing, unmodified rating prompt/parse/`Service.Rate` step (FR-005: this step
   is untouched by this feature).

`internal/tui` is unmodified by this feature (see research.md's "TUI text input out of scope"
decision) — its `Service` usage, key handling, and render states are unaffected.
