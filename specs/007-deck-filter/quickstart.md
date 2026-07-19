# Quickstart: Validating Per-Deck Review Filtering

Prerequisites: Go toolchain matching `go.mod`. No new dependencies.

## 1. Unit + integration tests

```sh
go test ./internal/review/... ./internal/plain/... ./internal/tui/... ./tests/integration/... -v
```

**Expected**: `internal/review/service_test.go` covers a scoped `NextDueCard` returning only that
deck's cards, a non-matching scope returning nothing (not another deck's card), and an empty
scope behaving exactly as before. `internal/plain/renderer_test.go` and `internal/tui/*_test.go`
cover the scope threading through `Run`/`Model` and the deck-named "nothing due" text.
`tests/integration/deck_filter_test.go` seeds cards across two decks and confirms a scoped session
never crosses into the other deck's cards.

## 2. CLI flag behavior

```sh
go build -o bin/meguru ./cmd/meguru
rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"

./bin/meguru review --deck kana-hiragana --plain <<< "good"
# Expected: only a Hiragana card is shown.

./bin/meguru review --deck bogus
# Expected: "unknown deck \"bogus\" — valid decks: kana-hiragana (Hiragana), ..." and exit 1;
# confirm no meguru.db was created/touched by this invocation if one didn't already exist.

./bin/meguru review --plain <<< "good"
# Expected: unchanged from before this feature — pulls from any due deck.
```

## 3. Interactive check: "Study a Deck"

```sh
./bin/meguru review
```

- Confirm the start menu now lists four items: Start Review, Study a Deck, View Stats, Quit.
- Select "Study a Deck" (arrow keys/`j`/`k` + Enter) — confirm the four decks are listed.
- Choose one deck and confirm the review screen shows a "Studying: <Deck Name>" indicator and
  only shows that deck's cards.
- From the deck picker, press `Esc` and confirm you land back on the start menu with nothing
  changed.
- Select "Start Review" (not "Study a Deck") and confirm it reviews across every deck as before
  this feature (SC-002).

## 4. Regression: full suite

```sh
go test ./... && go test -race ./... && golangci-lint run ./...
```

**Expected**: every package green, confirming the interface change (`review.Service.NextDueCard`)
was threaded correctly through every existing caller and fake, with no leftover unfiltered-only
assumption anywhere in the codebase.
