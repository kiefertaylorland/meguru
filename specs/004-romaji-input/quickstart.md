# Quickstart: Validating Romaji Answer Input (M2, US-3)

Prerequisites: Go toolchain matching `go.mod`. No new dependencies to fetch — this feature adds
zero third-party imports.

## 1. Unit-test the conversion package in isolation

```sh
go build ./...
go test ./internal/romaji/... -v
```

**Expected**: `romaji_test.go`'s table-driven tests pass across the full hiragana syllabary
(vowels, all consonant rows, dakuten/handakuten, yōon digraphs), sokuon (doubled consonants), and
`n`-disambiguation cases — proving FR-001 and SC-002 without needing the DB, review package, or
any UI layer at all.

## 2. Confirm the `internal/review.CheckAnswer` integration

```sh
go test ./internal/review/... -v
```

**Expected**: `answercheck_test.go`'s assertions pass — matches on converted kana, matches on raw
romaji (covering the M1 hiragana deck's romaji-reading convention), and non-matches correctly
report `Correct: false` while still returning the converted `Kana`.

## 3. Confirm the `internal/plain` flow: answer step precedes reveal and rating

```sh
go test ./internal/plain/... -v
```

**Expected**: `renderer_test.go`'s updated fixtures pass — each per-card interaction now reads an
answer line, prints a match/no-match line, reveals `Reading`/`Meaning`, then still reaches the
existing rating prompt and calls `Service.Rate` exactly as before (FR-002–FR-005, SC-003).

## 4. Manual, human-observable check

```sh
go build -o bin/meguru ./cmd/meguru
rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"   # clean profile
./bin/meguru review --plain
```

**Expected interaction** (first due card is the hiragana deck's "あ" card, reading "a"):

```text
Expression:あ
Type the reading (romaji):
> a
Correct! (あ)
Reading:   a
Meaning:   a
Rate: (a)gain / (h)ard / (g)ood / (e)asy
> good
Recorded: Good
```

Try a card with a multi-mora reading (e.g. "き" → "ki", or once further along, "きゃ"-style
content) and type a deliberately wrong answer to confirm the "not quite" path still reveals the
reading and still reaches the rating prompt (SC-003). Try typing a doubled-consonant/`n'`
example against a future non-hiragana card's kana reading, if available, to sanity check sokuon/
`n`-disambiguation end-to-end (this milestone's built-in deck is single-mora hiragana only, so
full digraph/sokuon coverage is exercised by the unit tests in §1, not this manual step).

## 5. Regression: full suite + offline guarantee unaffected

```sh
go test ./...
```

**Expected**: all M1/M2 packages (`cli`, `tui`, `scheduler`, `storage`, `deck`, `textwidth`,
`review`, `plain`) remain green, confirming this feature's scope stayed inside `internal/romaji`,
`internal/review` (additive only), and `internal/plain` as designed. `internal/tui` requires zero
source changes and its existing tests must pass unmodified, confirming the documented scope
decision (research.md) didn't leak into it. Re-run the network-denied CI job
(`.github/workflows/ci.yml`) to confirm zero egress (P-1/SEC-8) — trivially true since this
feature performs no I/O at all.
