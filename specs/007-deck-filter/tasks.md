---
description: "Task list for Per-Deck Review Filtering"
---

# Tasks: Per-Deck Review Filtering

**Input**: Design documents from `/specs/007-deck-filter/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
contracts/{review-cli.md,tui-deck-picker.md}, quickstart.md (all present)

**Tests**: Included, matching this repo's existing convention (every prior feature ships paired
`_test.go` updates).

**Organization**: Foundational carries the `review.Service` interface change through every
existing caller so the repo compiles again with parity (no new behavior yet); US1 (CLI `--deck`
flag, P1) and US2 (TUI "Study a Deck", P1) then each add their own entry point on top.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Maps the task to US1/US2 from spec.md

---

## Phase 1: Setup

- [X] T001 Confirm no new dependency is needed (`internal/deck.BuiltinDecks()` and
      `go.mod` are both already in place) — verification checkpoint, no code change.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: `review.Service.NextDueCard` gains a `DeckScope` parameter — every existing caller
and hand-written fake across the repo must update in lockstep for the build to pass. This phase
restores full compile/test parity with an unfiltered scope everywhere; no new user-facing
behavior yet.

**⚠️ CRITICAL**: No user story task can begin until this phase is complete.

- [X] T002 In `internal/review/service.go`: add `DeckScope{Slug, Name string}` (zero value =
      unfiltered, data-model.md); change `Service.NextDueCard` to
      `NextDueCard(ctx context.Context, scope DeckScope) (*Card, error)`; update the query to
      join `decks` via `notes.deck_id` and add the `AND (? = '' OR d.slug = ?)` predicate
      (research.md #1).
- [X] T003 In `internal/review/service_test.go`: update every `NextDueCard` call site to pass
      `DeckScope{}` (preserves existing unfiltered assertions unchanged).
- [X] T004 In `internal/plain/renderer.go`: change `Run` to
      `Run(ctx context.Context, svc review.Service, in io.Reader, out io.Writer, scope review.DeckScope) error`,
      passing `scope` into every `NextDueCard` call; no behavior change yet (message text stays
      generic — US1 adds the deck-named variant).
- [X] T005 In `internal/plain/renderer_test.go`: update the local `fakeService`'s `NextDueCard`
      signature and every `Run(...)` call site to pass `review.DeckScope{}`.
- [X] T006 In `internal/tui/model.go`: add an `activeDeck review.DeckScope` field to `Model`
      (zero value initially — no picker/menu wiring yet, just the field and its use in T007).
- [X] T007 In `internal/tui/update.go`: change `loadNextCard` to call
      `m.svc.NextDueCard(m.ctx, m.activeDeck)`.
- [X] T008 In `internal/tui/update_test.go`: update the local `fakeService`'s `NextDueCard`
      signature to match (no new test cases here — existing assertions are unaffected since
      `m.activeDeck` defaults to the zero value).
- [X] T009 [P] Update the four `tests/integration/{firstrun_due_test.go,review_roundtrip_test.go,stats_test.go,interrupted_test.go}`
      call sites to pass `review.DeckScope{}` to every `NextDueCard` call.
- [X] T010 In `internal/cli/review.go`: update the existing `plain.Run(...)` call site to pass
      `review.DeckScope{}` so the package builds (US1 replaces this with the resolved flag value).

**Checkpoint**: `go build ./...`, `go vet ./...`, and `go test ./...` all pass again, with
identical behavior to before this feature (every call site is still effectively unfiltered).

---

## Phase 3: User Story 1 - Study one deck from the command line (Priority: P1) 🎯 MVP

**Goal**: `meguru review --deck <slug>` scopes the whole session (plain and interactive alike) to
one deck; an unrecognized slug fails clearly before anything starts; no flag behaves exactly as
before.

**Independent Test**: Seed cards across multiple decks; run `meguru review --deck jlpt-n5-kanji`
(and `--plain`) and confirm only kanji cards appear; run with an unrecognized value and confirm a
clear error, no session, exit 1; run with no flag and confirm unchanged pooled behavior.

### Implementation for User Story 1

- [X] T011 [US1] In `internal/cli/review.go`: add `resolveDeckFlag(slug string) (review.DeckScope, error)`
      that looks up `slug` against `deck.BuiltinDecks()` by exact `Slug` match, returning
      `review.DeckScope{}` for an empty input and a clear error listing every valid slug + display
      name for an unrecognized non-empty one (research.md #3, contracts/review-cli.md).
- [X] T012 [US1] In `internal/cli/review.go`: register the `--deck` string flag on
      `newReviewCommand`, call `resolveDeckFlag` before any DB work in `runReview` (returning its
      error immediately — FR-004), and pass the resolved scope into the `plain.Run` call site
      from T010 (replacing the placeholder `review.DeckScope{}`).
- [X] T013 [US1] In `internal/review/service_test.go`: add scoped-query tests — a scope matching
      the seeded deck returns only its cards; a scope naming a deck with nothing due returns nil
      even when another deck has due cards; an empty scope is unaffected (contracts baseline).
- [X] T014 [US1] In `internal/plain/renderer.go`: when `scope.Name != ""` and nothing is due,
      print "Nothing due in {scope.Name} right now." instead of the generic message (FR-008).
- [X] T015 [P] [US1] In `internal/plain/renderer_test.go`: add a scoped-session test (only the
      scoped deck's cards appear) and a deck-named "nothing due" test.
- [X] T016 [P] [US1] Add `tests/integration/deck_filter_test.go`: seed cards across two decks,
      confirm a scoped `NextDueCard` never returns the other deck's cards across repeated calls.
- [X] T017 [P] [US1] Add `tests/e2e/deck_flag_test.go`: run the compiled binary with
      `--deck <a known slug> --plain` (asserts scoped output) and `--deck bogus` (asserts the
      error text and exit code 1, and that no `meguru.db` is created for a fresh profile).

**Checkpoint**: User Story 1 is fully functional and independently testable — `--deck` works in
both renderers, invalid values fail clearly, and the no-flag case is unchanged (SC-001–SC-003).

---

## Phase 4: User Story 2 - Study one deck from the start menu (Priority: P1)

**Goal**: A new "Study a Deck" start-menu option opens a deck-picker screen; choosing a deck
scopes the ensuing review session to it; "Start Review" continues to review every deck together.

**Independent Test**: Launch the interactive session, select "Study a Deck", pick one deck, and
confirm only that deck's cards appear with a visible "Studying: X" indicator; confirm Esc from the
picker returns to the start menu unchanged; confirm "Start Review" still pools every deck.

### Implementation for User Story 2

- [X] T018 [US2] In `internal/tui/model.go`: add `screenDeckPicker` to the `screen` enum; add
      `deckOptions []review.DeckScope` and `deckSelected int` fields; add `actionStudyDeck` and
      insert `MenuItem{Label: "Study a Deck", Action: actionStudyDeck}` as the second menu item
      (data-model.md); change `New` to accept `decks []review.DeckScope` and
      `initialScope review.DeckScope`, storing them into `deckOptions`/`activeDeck`.
- [X] T019 [US2] In `internal/tui/update.go`: add `handleDeckPickerKey` (up/`k`, down/`j` with
      clamping, `Enter` sets `m.activeDeck` to the highlighted option and transitions to
      `screenReview` via `loadNextCard`, `Esc` returns to `screenStartMenu`, `q`/Ctrl+C quits);
      add the `actionStudyDeck` branch in `handleStartMenuKey` (transitions to
      `screenDeckPicker`, resets `deckSelected` to 0); route `screenDeckPicker` in `Update`'s
      `tea.KeyPressMsg` switch.
- [X] T020 [US2] In `internal/tui/view.go`: add `renderDeckPicker` (lists `deckOptions` by name,
      marks the highlighted one, same hint-line convention as the start menu); route
      `screenDeckPicker` in `render()`'s screen switch; in `renderReview`, prepend a
      "Studying: {activeDeck.Name}" line when `activeDeck.Slug != ""`, and use the deck-named
      "nothing due" text from US1's wording when scoped (FR-009).
- [X] T021 [US2] In `internal/cli/review.go`: build `[]review.DeckScope` from
      `deck.BuiltinDecks()` (slug + display name for all four) and pass it, plus the `--deck`-resolved
      scope from US1, into the `tui.New(...)` call site.
- [X] T022 [P] [US2] In `internal/tui/update_test.go`: test picker navigation/clamping,
      `actionStudyDeck` transitioning to `screenDeckPicker`, `Enter` setting `activeDeck` and
      starting review, `Esc` returning to the start menu unchanged, and `q`/Ctrl+C quitting from
      the picker.
- [X] T023 [P] [US2] In `internal/tui/view_test.go`: test the picker screen lists all decks and
      marks the current selection, the "Studying: X" line appears only when scoped, and the
      deck-named "nothing due" text renders correctly.
- [X] T024 [US2] In `internal/tui/model_test.go`: extend or add a `teatest` golden case navigating
      start menu → "Study a Deck" → pick a deck → confirm the review screen shows the scoped
      deck's card, alongside the existing "Start Review" golden path (unchanged).

**Checkpoint**: Both user stories are independently functional — CLI `--deck` and the TUI's
"Study a Deck" both scope a session to one deck; unscoped behavior is unchanged either way.

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T025 [P] Run `go test ./... -v`; fix any remaining assertions.
- [X] T026 Run `go build ./... && go vet ./... && gofmt -l .` and `golangci-lint run ./...`.
- [X] T027 Run `go test -race ./...` (matches CI).
- [X] T028 Manually verify quickstart.md §2–3 (CLI `--deck` valid/invalid, interactive "Study a
      Deck" flow, "Start Review" unaffected) — a pty-driven check of the real binary, not just
      the golden test, per this repo's UI-verification convention.
- [X] T029 Run quickstart.md §4 full regression (`go test ./...`, `-race`, lint) and confirm
      `--plain` with no `--deck` is byte-for-byte unchanged from before this feature.

---

## Dependencies & Execution Order

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS both user stories (shared interface
  signature).
- **User Story 1 (Phase 3)**: Depends on Phase 2 only — independently testable/demoable on its
  own (MVP).
- **User Story 2 (Phase 4)**: Depends on Phase 2, and on US1's `resolveDeckFlag`/error-message
  convention and deck-named "nothing due" wording (T011, T014) being in place to reuse — sequenced
  after US1 for that reason, even though both are P1.
- **Polish (Phase 5)**: Depends on Phases 3–4 complete.

### Parallel Opportunities

- T009 (Phase 2) is independent of T002–T008 landing in the same files and is marked [P].
- Within US1: T015, T016, T017 (different files) can run in parallel once T011–T014 land.
- Within US2: T022, T023 (different test files) can run in parallel once T018–T021 land.
- T025 in Polish is marked [P]; T026–T029 are sequential regression gates.

## Implementation Strategy

1. Setup + Foundational → parity checkpoint (identical behavior, new interface shape).
2. US1 (CLI `--deck`) → validate independently → MVP.
3. US2 (TUI "Study a Deck") → validate independently → full feature.
4. Polish → full regression + manual verification.
