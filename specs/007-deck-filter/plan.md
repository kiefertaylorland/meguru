# Implementation Plan: Per-Deck Review Filtering

**Branch**: `007-deck-filter` | **Date**: 2026-07-19 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/007-deck-filter/spec.md`

## Summary

Add an optional deck scope to `review.Service.NextDueCard`, exposed as a `--deck <slug>` flag on
`meguru review` (both plain and interactive renderers) and as a new "Study a Deck" option on the
interactive start menu that opens a deck-picker screen. No scope (the existing default) behaves
exactly as today — due cards pulled from every deck together. A scope narrows `NextDueCard` to one
deck via a straightforward SQL join against the already-existing `decks`/`notes` tables — no
schema change. The four built-in decks and their stable slugs (`kana-hiragana`, `kana-katakana`,
`jlpt-n5-kanji`, `jlpt-n5-vocab`) are unchanged (`internal/deck`).

## Technical Context

**Language/Version**: Go (current stable, pinned in `go.mod`; same as prior features).

**Primary Dependencies**: None new. `internal/deck.BuiltinDecks()` (existing, from
003-katakana-n5-decks) is read at the `internal/cli` layer only, to resolve `--deck`'s value and
to build the picker's deck list — `internal/review`, `internal/plain`, and `internal/tui` gain no
new dependency on `internal/deck` (see Structure Decision).

**Storage**: No migration. The scope query joins `decks` (already has `slug`) through the
existing `notes.deck_id` foreign key — both columns already exist in
`internal/storage/migrations/0001_init.sql`.

**Testing**: stdlib `testing` + `testify/require`, matching every existing package's conventions.
This is an interface change (`review.Service.NextDueCard` gains a scope parameter), so every
existing caller and hand-written fake needs updating — full blast radius (confirmed via repo-wide
grep): `internal/review/service.go` + `service_test.go`, `internal/plain/renderer.go` +
`renderer_test.go` (+ its own local `fakeService`), `internal/tui/update.go` + `update_test.go`
(+ its own local `fakeService`), and four integration tests
(`tests/integration/{firstrun_due_test.go,review_roundtrip_test.go,stats_test.go,interrupted_test.go}`).
`tests/e2e` drives the compiled binary and is unaffected by the Go-level signature change, only by
the new `--deck` flag's behavior (new e2e case added).

**Target Platform**: Same cross-platform CLI target as prior features — no platform-specific
concerns; this is a SQL predicate and a CLI flag, nothing OS-dependent.

**Project Type**: Single Go module, CLI application (unchanged — Option 1 layout).

**Performance Goals**: No new performance targets. The scoped query adds one indexed-column join
(`decks.slug` is already `UNIQUE`) to an existing single-row `LIMIT 1` lookup — negligible.

**Constraints**: Zero network calls (P-1/SEC-8) — unchanged, this is a local SQL predicate change.

**Scale/Scope**: Modifies `internal/review/service.go` (interface + query), `internal/plain/renderer.go`
(threads a scope through), `internal/tui/{model,update,view}.go` (deck-picker screen + scope
state), and `internal/cli/review.go` (the `--deck` flag, slug resolution, and building the
picker's deck list from `internal/deck.BuiltinDecks()`). No new package, no changes to
`internal/deck`, `internal/scheduler`, `internal/storage`'s schema, or `internal/stats`.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

| Principle / Rule | Applies? | Assessment |
| --- | --- | --- |
| P-1 Offline-First Is Inviolable | Yes | Purely a local SQL predicate and CLI flag; no network call added anywhere. **PASS** |
| P-2 Local-Only By Default | Yes | Nothing new leaves the device. **PASS** |
| P-3/P-4 (AI boundary) | N/A | No AI touchpoint. |
| P-5 No Telemetry | Yes | No telemetry added. **PASS** |
| SEC-6/SEC-7 (untrusted input) | Yes | The `--deck` flag value is validated against the fixed built-in slug list before use; an unrecognized value errors out before any query runs (FR-004) — it is never interpolated into SQL (parameterized query, `?` placeholder) or treated as anything but an equality-compared string. **PASS** |
| SEC-8 (network isolation, CI gate) | Yes | No I/O beyond the existing local DB handle. **PASS** |
| SEC-10 (dependency hygiene) | Yes | Zero new dependencies. **PASS** |
| CON-1/CON-2/CON-3 | Yes | Constitution + TECH_STACK loaded; no stray network calls, no new deps, no secrets/real data in fixtures — all new test fixtures are the same synthetic seeded decks already used throughout the suite. |

No violations requiring Complexity Tracking justification. Gate: **PASS**.

## Project Structure

### Documentation (this feature)

```text
specs/007-deck-filter/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── review-cli.md
│   └── tui-deck-picker.md
├── checklists/
│   └── requirements.md  # Spec quality checklist (/speckit-specify command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
├── review/
│   ├── service.go                # MODIFIED: DeckScope type; NextDueCard(ctx, scope) — scope
│   │                                zero value ("" slug) means unfiltered, unchanged query plan
│   └── service_test.go           # MODIFIED: scoped-query cases (matching deck, non-matching
│                                    deck, empty scope unchanged)
├── plain/
│   ├── renderer.go                # MODIFIED: Run(ctx, svc, in, out, scope) — passes scope
│   │                                 through, deck-named "nothing due" message when scoped
│   └── renderer_test.go           # MODIFIED: fakeService signature; scoped-session case
├── tui/
│   ├── model.go                   # MODIFIED: screenDeckPicker; deckOptions []review.DeckScope;
│   │                                 deckSelected int; activeDeck review.DeckScope; New() gains
│   │                                 decks []review.DeckScope + initialScope review.DeckScope
│   ├── update.go                  # MODIFIED: actionStudyDeck; handleDeckPickerKey; loadNextCard
│   │                                 passes m.activeDeck
│   ├── view.go                    # MODIFIED: renderDeckPicker; deck-scope line + deck-named
│   │                                 "nothing due" in renderReview
│   └── {model,update,view}_test.go  # MODIFIED: fakeService signature; new picker/scope tests
└── cli/
    └── review.go                  # MODIFIED: --deck flag; resolveDeckFlag(slug) (deck.BuiltinDecks()
                                       lookup + clear error listing valid slugs); builds the
                                       []review.DeckScope list passed to tui.New

tests/
└── integration/
    ├── firstrun_due_test.go        # MODIFIED: NextDueCard call site (unfiltered scope)
    ├── review_roundtrip_test.go    # MODIFIED: same
    ├── stats_test.go               # MODIFIED: same
    ├── interrupted_test.go         # MODIFIED: same
    └── deck_filter_test.go         # NEW: seeds cards across two decks, asserts a scoped
                                       NextDueCard only ever returns the scoped deck's cards

tests/e2e/
└── deck_flag_test.go              # NEW: runs the compiled binary with `--deck <slug>` and
                                      `--deck bogus`, asserts scoped output and the clear error
```

**Structure Decision**: `review.DeckScope{Slug, Name string}` lives in `internal/review` (the
package that already owns `Card`/`Service`) rather than `internal/deck`, so `internal/plain` and
`internal/tui` depend on `review.DeckScope` — a value type with no deck-registry knowledge —
instead of importing `internal/deck` directly. Only `internal/cli/review.go` (which already
imports both `internal/deck` and `internal/review`) resolves a `--deck` string against
`deck.BuiltinDecks()` and builds `review.DeckScope` values; this keeps `internal/review` decoupled
from the deck registry's existence (mirroring how `review.Service` today has no idea decks are
seeded by `internal/deck.Seed` — that wiring already lives one layer up, in `internal/cli`) and
keeps `internal/tui`'s dependency list unchanged (still just `review` + `stats`).

## Complexity Tracking

_No Constitution Check violations — table not needed._
