# Phase 0 Research: Romaji Answer Input

No `NEEDS CLARIFICATION` markers remain — the shape of this feature (a pure conversion package
plus a pre-rating step in the review flow) was fully specified by the task brief and validated
against the actual M1 codebase before this spec was written. This document records the decisions
and rationale for the record.

## Decision: New standalone `internal/romaji` package, no dependency on anything else

**Decision**: Implement romaji→hiragana conversion as `romaji.ToHiragana(input string) string` in
a brand-new `internal/romaji` package with zero imports outside the Go standard library.

**Rationale**: FR-007 requires the conversion be testable in complete isolation. It is a pure
string transformation with no reason to know about cards, storage, or any UI layer — mirroring
the existing project convention of `internal/scheduler.Schedule` and `internal/textwidth.Width`
as small, dependency-free pure-function packages consumed by higher layers. Keeping it dependency
-free also means it costs nothing against CON-2 (no new dependency to justify) and trivially
satisfies P-1/SEC-8 (no I/O of any kind, let alone network).

**Alternatives considered**: Putting the conversion logic directly inside `internal/review` or
`internal/plain` — rejected, it would make the conversion untestable without also standing up a
`review.Card`/DB fixture or a renderer harness, and would preclude future reuse (e.g. a future
TUI text-input integration would need the exact same function).

## Decision: Greedy longest-match tokenizer over the full input string

**Decision**: `ToHiragana` walks the lowercased input rune-by-rune, at each position trying (in
order): the `n'` apostrophe separator, sokuon detection (doubled consonant, or the `tch` special
case), a 3-rune digraph/monograph match, a 2-rune digraph/monograph match, a 1-rune monograph
match (vowels and bare `n`), and finally an unrecognized-rune passthrough.

**Rationale**: Hepburn romaji is naturally tokenized by matching the longest known mora spelling
first (`shi` before `s`+`h`+`i`, `kya` before `k`+`ya`), which is the standard approach used by
essentially every romaji IME. A single left-to-right pass with longest-match-first requires no
backtracking and stays linear in input length — well within the sub-50ms keypress budget in
`docs/product/PRD.md`'s NFRs (this is a single full-string call per submitted line, not per
keystroke, in this slice's `internal/plain` integration).

**Alternatives considered**: A regex-based replace table — rejected, doubled-consonant (sokuon)
and `n`-disambiguation both depend on lookahead/lookbehind that would require multiple
non-composable regex passes and would be harder to reason about correctness for than a single
tokenizer loop. A full finite-state-machine transliteration library — rejected, unnecessary
weight and a new dependency for a well-bounded, well-tested ~150-line function.

## Decision: Passthrough (not error) for unrecognized characters

**Decision**: Any rune that doesn't participate in a recognized mora (spaces, punctuation,
digits, or already-kana text) is copied to the output unchanged rather than dropped or causing an
error.

**Rationale**: FR-006 requires the system never crash or hang on an empty/unrecognized answer.
Passthrough is also strictly safer for the auto-check use case: a wrong-but-well-formed answer
should read back recognizably to the learner in the "you typed: ..." feedback line, not be
silently mangled or cause the whole review interaction to error out.

**Alternatives considered**: Returning `(string, error)` and having the caller decide — rejected
as unnecessary complexity for this slice; a conversion that "fails" here just means "this wasn't
a match," which `review.CheckAnswer` already reports via its `Correct` field. No caller needs to
distinguish "malformed input" from "wrong answer."

## Decision: `n` handling — digraph wins unless `n'` disambiguates

**Decision**: `ni`/`na`/`nu`/`ne`/`no` and `nya`/`nyu`/`nyo` are matched as their own mora
(longest-match-first) before falling back to bare `n` → ん. An apostrophe immediately after `n`
(`n'`) forces ん explicitly, consuming the apostrophe, e.g. `kon'ya` → こんや (not こにゃ).

**Rationale**: This is the same disambiguation convention essentially all romaji input systems
use (Google IME, standard Hepburn romanization guides). Doubled `nn` (e.g. `annai` → あんない)
resolves correctly for free under this rule with no special-casing: the first `n` fails to extend
into a digraph (its neighbor is another consonant, `n`, not a vowel/`y`) and falls back to ん;
the second `n` then matches normally against what follows.

**Alternatives considered**: Requiring the apostrophe always (rejecting bare `nyo` as invalid) —
rejected, this would make ordinary words typable only with fussy apostrophe discipline
(`sannin` would need to be `san'nin`, etc. depending on scheme), which is worse UX than accepting
the same convention every mainstream IME uses and documenting the known ambiguity (spec.md Edge
Cases) rather than "solving" it in a way that contradicts user expectations built from every
other romaji input method they've used.

## Decision: `CheckAnswer` matches on raw typed text OR converted kana

**Decision**: `review.CheckAnswer(card, typedRomaji)` reports a match if either (a) the converted
kana equals `card.Reading`, or (b) the trimmed, lowercased raw typed text equals `card.Reading`
(also trimmed/lowercased).

**Rationale**: As documented in spec.md's Assumptions, the M1 hiragana deck stores `Reading` as
plain romaji (a kana card's own pronunciation), while future decks are expected to store kana
readings. Branch (b) makes today's only real deck work correctly without modifying its content
file; branch (a) is what makes future kana-reading decks (kanji, vocab) work once they exist.
Both branches are one string comparison each — negligible complexity for meaningfully wider
correctness than picking only one.

**Alternatives considered**: Converting `card.Reading` itself through `ToHiragana` before
comparing (assuming `Reading` is always romaji) — rejected, this would break for any future deck
that stores a kana reading directly (a kanji card's reading is naturally authored as kana, not
romaji, in most SRS content pipelines) and there's no reliable way to detect "is this field
already kana" that's simpler than just checking both forms directly. Migrating the hiragana
deck's `reading` field to store kana instead of romaji — rejected as out of scope: it's existing
M1 content data, not something this feature's PRD story asks to change, and content-file changes
aren't warranted just to simplify one comparison when a two-way check is this cheap.

## Decision: `internal/tui` interactive text input is out of scope this slice

**Decision**: This slice integrates only with `internal/plain`. `internal/tui`'s Bubble Tea v2
model is left unchanged.

**Rationale**: Documented in full in spec.md's Assumptions. In short: `internal/tui/update.go`'s
`handleKey` today only ever dispatches single logical keys (space/enter to reveal, a digit/letter
to rate) — there is no text-accumulation state, no cursor, and no text-input component in
`go.mod` to lean on. Building this correctly (buffering runes from `tea.KeyPressMsg`, handling
backspace/enter, a new render state, updated golden-frame tests) is a meaningfully larger, more
failure-prone change than this bounded slice justifies, per the task brief's own explicit
permission to scope this out with documented rationale. `internal/plain` needed no such new
capability — it already reads full lines via `bufio.Scanner` — making it the correct, low-risk
primary integration point that still fully satisfies US-3 for the `--plain` and non-TTY path
every CI/E2E test already exercises.

**Alternatives considered**: Attempting a minimal TUI integration (buffer typed runes in a new
`typing` model field, submit on enter) — considered, but rejected for this slice after weighing
it against the added surface area (new `Model` field, new `View` render branch, new key-handling
branch interacting with the existing reveal/submitting state machine, plus updated golden-frame
`teatest` coverage) versus the bounded scope requested. Left as a fast-follow, not a gap in this
feature's own acceptance criteria (none of which require TUI support).
