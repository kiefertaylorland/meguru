# Phase 1 Data Model: Per-Deck Review Filtering

No database schema changes — this feature reads `decks.slug` through the existing
`notes.deck_id` foreign key (`internal/storage/migrations/0001_init.sql`), both already present.

## DeckScope (`internal/review`)

A value type carried through `Service.NextDueCard`, `plain.Run`, and `tui.Model` to say which
single deck (if any) a review session is scoped to.

| Field | Type | Notes |
| --- | --- | --- |
| `Slug` | string | The deck's existing stable identifier (`kana-hiragana`, `kana-katakana`, `jlpt-n5-kanji`, `jlpt-n5-vocab`). Empty string is the "no scope" sentinel — matches today's unfiltered behavior exactly (research.md #1). |
| `Name` | string | Display name (e.g. "Hiragana"), used only for user-facing text (the "Studying: X" line, the deck-named "nothing due" message) — never used in the query itself, which filters on `Slug`. |

Zero value `DeckScope{}` means "review across every deck," identical to today's behavior.

## Service interface change (`internal/review/service.go`)

```text
NextDueCard(ctx context.Context, scope DeckScope) (*Card, error)
```

`scope.Slug == ""` → the existing unfiltered query (research.md #1's `? = ''` short-circuit).
`scope.Slug != ""` → only cards whose note belongs to the deck with that slug are eligible.

`Rate` is unchanged — rating a card doesn't need to know which deck it came from.

## `plain.Run` signature change (`internal/plain/renderer.go`)

```text
Run(ctx context.Context, svc review.Service, in io.Reader, out io.Writer, scope review.DeckScope) error
```

Every `NextDueCard` call inside the loop passes `scope` through unchanged for the life of the
session (spec.md Assumptions: "scope is whole-session"). The "nothing due" message uses
`scope.Name` when `scope.Slug != ""` (FR-008).

## `tui.Model` field additions (`internal/tui/model.go`)

| Field | Type | Purpose |
| --- | --- | --- |
| `deckOptions` | `[]review.DeckScope` | The picker's fixed list, built once at construction from `internal/cli`'s call to `deck.BuiltinDecks()` — `tui` itself never imports `internal/deck` (research.md #2). |
| `deckSelected` | `int` | Cursor into `deckOptions` on the picker screen; clamps at both ends, same convention as the start menu's `menuSelected` (006-tui-start-menu). |
| `activeDeck` | `review.DeckScope` | The scope the next `loadNextCard` call uses. Zero value initially unless `New`'s `initialScope` parameter (sourced from `--deck`) sets it; overwritten when a deck is chosen on the picker screen (research.md #5). |

`New`'s signature grows to
`New(ctx, svc review.Service, statsSvc stats.Service, decks []review.DeckScope, initialScope review.DeckScope) Model`.

## Screen additions (extends 006-tui-start-menu's `screen` enum)

| Value | Meaning |
| --- | --- |
| `screenDeckPicker` | The list of decks from `deckOptions` is shown; Enter on one sets `activeDeck` and starts review; Esc returns to the start menu. |

**Transitions** (extends 006-tui-start-menu's data-model.md):

```text
screenStartMenu --(select "Study a Deck", Enter)--> screenDeckPicker
screenDeckPicker --(select a deck, Enter)-----------> screenReview   (sets activeDeck)
screenDeckPicker --(Esc)-----------------------------> screenStartMenu
screenDeckPicker --(q | ctrl+c)-----------------------> program exits
screenStartMenu  --(select "Start Review", Enter)----> screenReview   (activeDeck unchanged —
                                                                        whatever it already was:
                                                                        zero value, or set by
                                                                        --deck, or by a prior
                                                                        picker visit this session)
```

## New MenuItem (extends 006-tui-start-menu's start menu)

| Label | Action | Position |
| --- | --- | --- |
| "Study a Deck" | `actionStudyDeck` | Second item, between "Start Review" and "View Stats" |

The full menu order becomes: Start Review, Study a Deck, View Stats, Quit.
