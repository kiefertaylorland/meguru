# Quickstart: Validating Katakana + JLPT N5 Kanji/Vocab Built-In Decks (M2, US-1 + US-5)

Prerequisites: Go toolchain matching `go.mod` (no new dependencies added by this feature).

## 1. Build and unit-test `internal/deck` in isolation

```sh
go build ./...
go test ./internal/deck/... -v
```

**Expected**: `TestBuiltinDecks_AllDistinctAndValid` confirms the registry lists exactly four
decks with unique slugs, valid kinds, and parseable, duplicate-expression-free content.
`TestKatakana_ParsesEmbeddedJSON`, `TestJLPTN5Kanji_ParsesEmbeddedJSON`, and
`TestJLPTN5Vocab_ParsesEmbeddedJSON` confirm each new deck's content shape and size (this proves
FR-001, FR-002). `TestSeed_SeedsAllBuiltinDecks` and `TestSeed_SecondRunDoesNotDuplicateAnyBuiltinDeck`
confirm FR-003/FR-004 hold across all four decks, not just hiragana. The generalized
`seedDeck`/`seedFresh`/`updateInPlace`/`insertNote` unit tests (run against a synthetic
`Definition`) confirm the shared seed/update-in-place mechanism's error handling and
content-version behavior independent of any specific deck's real content.

## 2. Confirm the generalized no-duplication guarantee at the integration level

```sh
go test ./tests/integration/... -run TestSeed_DoesNotDuplicateOnSecondRun -v
```

**Expected**: row counts in `decks`/`notes`/`cards` after two `Seed` calls match the sum derived
from `deck.BuiltinDecks()` exactly — proving SC-002 across every built-in deck combined.

## 3. Confirm decks surface as due cards through the unmodified `internal/review` query

```sh
go test ./tests/integration/... -run TestFirstRun_DueCardImmediatelyAfterSeed -v
```

**Expected**: passes unmodified — `internal/review/service.go`'s due-card query already joins
across all decks generically, so a card from any built-in deck (not necessarily hiragana) can
satisfy this test, proving FR-006 required zero changes to `internal/review`.

## 4. Manual, human-observable check: all four decks appear in a real review session

```sh
go build -o bin/meguru ./cmd/meguru
rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"   # clean profile
yes g | ./bin/meguru review --plain   # rate every due card "Good" until nothing is due
```

**Expected**: the session presents cards across all four built-in decks before printing "Nothing
due right now." — confirm by inspecting the local database directly:

```sh
sqlite3 "${XDG_DATA_HOME:-$HOME/.local/share}/meguru/meguru.db" \
  "SELECT d.slug, COUNT(*) FROM cards c JOIN notes n ON n.id = c.note_id JOIN decks d ON d.id = n.deck_id GROUP BY d.slug;"
# expect four rows: kana-hiragana, kana-katakana, jlpt-n5-kanji, jlpt-n5-vocab
sqlite3 "${XDG_DATA_HOME:-$HOME/.local/share}/meguru/meguru.db" \
  "SELECT d.kind, COUNT(*) FROM review_log rl JOIN cards c ON c.id = rl.card_id JOIN notes n ON n.id = c.note_id JOIN decks d ON d.id = n.deck_id GROUP BY d.kind;"
# expect three rows: kana, kanji, vocab, each with a nonzero review count
```

## 5. Regression: full suite unaffected outside `internal/deck`/`tests/integration`

```sh
go test ./...
```

**Expected**: all M1/M2 packages (`cli`, `tui`, `plain`, `storage`, `scheduler`, `review`,
`textwidth`, `deck`) remain green, confirming this feature's scope stayed inside `internal/deck`
and its direct test callers as designed (Project Structure in plan.md) — in particular,
`internal/review`, `internal/scheduler`, `internal/tui`, `internal/plain`, `internal/cli` needed no
source changes.
