# Phase 0 Research: Per-Deck Review Filtering

## 1. Where the deck scope lives on the query

**Decision**: `NextDueCard`'s existing single query gains one more join
(`JOIN decks d ON d.id = n.deck_id`) and one more predicate
(`AND (? = '' OR d.slug = ?)`), with the scope's slug bound to both `?` placeholders. An empty
slug is the unfiltered case — the `? = ''` short-circuit makes the predicate a no-op, so the
query plan and result for an unscoped call are unchanged from today.

**Rationale**: `notes.deck_id` and `decks.slug` (`UNIQUE`) already exist
(`internal/storage/migrations/0001_init.sql`) — no migration needed. A single parameterized
predicate is the simplest correct implementation; it also keeps the "no deck scope = identical
behavior to today" requirement (FR-002) trivially true by construction rather than requiring two
separate code paths to stay in sync.

**Alternatives considered**: Two separate queries (scoped vs. unscoped) selected by an `if`:
rejected — doubles the SQL to maintain for one predicate's difference, with no benefit. A
`WHERE d.slug = COALESCE(NULLIF(?, ''), d.slug)`-style trick: rejected as needlessly clever next to
the plain `? = '' OR d.slug = ?` form.

## 2. Where `DeckScope` lives

**Decision**: A small `DeckScope{Slug, Name string}` value type in `internal/review`, alongside
`Card`/`Service`. Its zero value (`Slug == ""`) is the "no scope" sentinel used throughout.

**Rationale**: `internal/plain` and `internal/tui` already depend on `internal/review` for
`Card`/`Service`; giving them `review.DeckScope` too avoids introducing a dependency on
`internal/deck` (the registry/seeding package) into either renderer. `internal/cli` is the only
place that needs to know the registry exists at all (it already imports both `internal/deck` and
`internal/review` for seeding), so it's the natural place to resolve a `--deck` string against
`deck.BuiltinDecks()` into a `review.DeckScope`.

**Alternatives considered**: Putting `DeckScope` in `internal/deck` and having `review`/`plain`/`tui`
import it from there: rejected — that would pull the deck-registry package into the review and TUI
layers for what is, from their point of view, just an opaque slug+label filter; they don't need to
know decks are seeded, versioned, or embedded. Passing a bare `string` slug everywhere instead of a
small struct: rejected — the deck-named "nothing due"/"studying" messages (FR-008/FR-009) need a
human-readable name too, and threading two loose parameters (slug, name) through `plain.Run`,
`tui.New`, and `Model` is worse than one small value type.

## 3. CLI flag validation and error message

**Decision**: `--deck <slug>` is resolved in `internal/cli` against `deck.BuiltinDecks()` by exact
slug match. An unrecognized value produces one error listing every valid slug alongside its
display name (e.g. `unknown deck "bogus" — valid decks: kana-hiragana (Hiragana), kana-katakana
(Katakana), jlpt-n5-kanji (JLPT N5 Kanji), jlpt-n5-vocab (JLPT N5 Vocabulary)`), returned before
any database work happens, so it's a fast, side-effect-free failure (FR-004, SC-003).

**Rationale**: Exact-match against the existing stable slugs needs no new naming scheme (spec.md
Assumptions) and reuses `deck.BuiltinDecks()`, which already carries both `Slug` and `Name` for
exactly this purpose. Failing before `storage.Open`/`deck.Seed` keeps an invalid flag value cheap
to detect and matches the existing exit-code contract (`1` = unrecoverable/user error,
`specs/001-walking-skeleton/contracts/cli.md`).

**Alternatives considered**: Case-insensitive or fuzzy matching against display names ("hiragana",
"Hiragana", "HIRAGANA"): rejected as unnecessary complexity for a fixed, small, already-documented
set of four slugs — the clear error message itself is the discovery mechanism (spec.md
Assumptions: no dedicated `meguru decks` command in this slice).

## 4. TUI deck picker as a fourth screen vs. a mode of the start menu

**Decision**: A new `screenDeckPicker` screen (parallel to `screenStartMenu`/`screenStats`/
`screenReview` from 006-tui-start-menu), reached via a new "Study a Deck" start-menu item, listing
`Model.deckOptions []review.DeckScope`. Esc returns to the start menu without starting anything;
Enter sets `Model.activeDeck` to the chosen scope and transitions straight to `screenReview`.

**Rationale**: Directly extends 006-tui-start-menu's existing screen/routing pattern (a `screen`
enum switched on in both `Update` and `View`, per that feature's data-model.md) rather than
inventing a new mechanism — the deck picker is structurally identical to the start menu itself
(a list, a cursor, Enter to act, Esc/`q` to back out or quit), just with a different item source
and a different Enter action.

**Alternatives considered**: Folding deck selection into the start menu itself (e.g. cycling
through decks with left/right before pressing Enter on "Start Review"): rejected — conflates two
independent choices (which action, which deck) into one control, and would make "Start Review"
mean different things depending on hidden cursor state, which is worse for discoverability than a
dedicated, clearly-labeled second screen.

## 5. What `--deck` means for the interactive session's start menu

**Decision**: `--deck <slug>` sets `Model`'s initial `activeDeck` (i.e., what "Start Review" reviews
by default for that launch), but the start menu still always appears first (unchanged from
006-tui-start-menu FR-001), and "Study a Deck" remains available to override it mid-session.

**Rationale**: Keeps a single, consistent meaning for `activeDeck` state across the whole Model
regardless of how it was first set (CLI flag vs. picker selection) — `loadNextCard` only ever
needs to read `m.activeDeck`, never branch on where it came from. Avoids adding a second
"skip the menu" behavior that spec.md never asked for (YAGNI/Simplicity First) — the request was
for a menu option and a CLI flag, not a flag that changes what the menu does structurally.

**Alternatives considered**: `--deck` skipping the start menu and jumping straight to a scoped
review screen: rejected as scope creep — not requested, and it would need its own edge cases
(what happens on "nothing due" — return to menu? exit?) that 006-tui-start-menu's existing
contract doesn't define for any other flag today.
