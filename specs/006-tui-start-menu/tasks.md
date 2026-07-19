---
description: "Task list for TUI Start Menu & Full-Window Layout"
---

# Tasks: TUI Start Menu & Full-Window Layout

**Input**: Design documents from `/specs/006-tui-start-menu/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/tui-screens.md,
quickstart.md (all present)

**Tests**: Included — this repo's existing convention (every prior `internal/tui` change ships
paired `_test.go` updates; CLAUDE.md §4 Goal-Driven Execution) is to write/extend tests alongside
each implementation task, not as a separate top-level test-only phase.

**Organization**: Tasks are grouped by user story (US1 = Start Menu, P1; US2 = View Stats, P2; US3
= Full-Window Layout, P1) in the build order that lets each be demoed independently, per
research.md #6 and data-model.md.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Maps the task to US1/US2/US3 from spec.md
- File paths are exact and relative to the repo root

---

## Phase 1: Setup

**Purpose**: No new project scaffolding is needed — `internal/tui` and `internal/cli` already
exist, `internal/stats` already exists unchanged (005-dashboard-stats), no new dependency to add
(research.md #1).

- [X] T001 Confirm `go.mod` already pins `charm.land/bubbletea/v2 v2.0.8` and
      `charm.land/lipgloss/v2 v2.0.5` (no edit expected — this task is a verification checkpoint,
      not a code change, per plan.md Technical Context).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Struct/constructor shape shared by every user story below — the `screen` enum, menu
state, and the new `stats.Service` constructor dependency. Both US1 and US2 need `tui.New`'s
final signature; deciding it once here avoids reworking call sites twice.

**⚠️ CRITICAL**: No user story task can begin until this phase is complete.

- [X] T002 In `internal/tui/model.go`: add the `screen` enum (`screenStartMenu`, `screenStats`,
      `screenReview`, default/zero value `screenStartMenu` per data-model.md), the `MenuItem`
      struct and `action` enum (`actionStartReview`, `actionViewStats`, `actionQuit`), and add
      `screen`, `menuItems []MenuItem`, `menuSelected int` fields to `Model`.
- [X] T003 In `internal/tui/model.go`: add `statsSvc stats.Service`, `statsSummary *stats.Summary`,
      `statsErr error`, `width int`, `height int` fields to `Model`; add the `statsMsg` and
      `statsErrMsg` message types (data-model.md "New message types").
- [X] T004 In `internal/tui/model.go`: change `New(ctx, svc review.Service)` to
      `New(ctx context.Context, svc review.Service, statsSvc stats.Service) Model`, initializing
      `menuItems` to the fixed 3-item slice ("Start Review"/`actionStartReview`, "View
      Stats"/`actionViewStats`, "Quit"/`actionQuit`) and `screen: screenStartMenu`.
- [X] T005 In `internal/cli/review.go`: construct `stats.NewService(db)` alongside the existing
      `review.NewService(db)` in `runReview`, and update the `tea.NewProgram(tui.New(...), ...)`
      call to pass both services (research.md #4).
- [X] T006 [P] In `internal/tui/update_test.go`: add a hand-written `fakeStatsService`
      implementing `stats.Service.Compute(ctx, now) (stats.Summary, error)` (configurable summary
      / error / call-tracking fields), matching the existing `fakeService` convention
      (research.md #5) — no behavior wired to it yet, just the test double.
- [X] T007 Update every existing `tui.New(ctx, svc)` call site across `internal/tui/*_test.go` and
      `internal/cli/review_test.go` (if any construct a `Model` directly) to pass a
      `fakeStatsService`/`stats.NewService(db)` as the new third argument, so the package still
      builds — no behavioral assertions added yet, this task only restores compilation.

**Checkpoint**: `go build ./...` and `go vet ./...` succeed; existing tests may still assert the
*old* single-screen behavior and are expected to fail until Phase 3 lands (they get updated
there, not here).

---

## Phase 3: User Story 1 - Choose an action from a start menu (Priority: P1) 🎯 MVP

**Goal**: The interactive session opens on a keyboard-navigable menu ("Start Review", "View
Stats", "Quit") instead of loading a card immediately; selecting "Start Review" hands off to the
existing review flow unchanged.

**Independent Test**: Launch the interactive session against a seeded DB; confirm the menu
appears before any card/loading content, arrow keys/`j`/`k` move the highlight, Enter on "Start
Review" reaches the existing card flow, and `q`/Ctrl+C quits from the menu.

### Implementation for User Story 1

- [X] T008 [US1] In `internal/tui/update.go`: change `Init()` to return `nil` (no longer
      auto-loads a card — loading now happens only after "Start Review" is selected, per
      data-model.md's Screen transitions).
- [X] T009 [US1] In `internal/tui/update.go`: add `handleStartMenuKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd)`
      handling `up`/`k` (decrement `menuSelected`, clamp at 0), `down`/`j` (increment, clamp at
      `len(menuItems)-1`), `enter` (dispatch on the highlighted item's `Action`:
      `actionStartReview` → set `screen = screenReview`, return `m.loadNextCard`;
      `actionViewStats` → set `screen = screenStats`, return the stats-fetch `tea.Cmd` stubbed in
      Phase 4; `actionQuit` → `tea.Quit`), and `q`/`ctrl+c` (`tea.Quit`) — per
      contracts/tui-screens.md's Start Menu keybinding table.
- [X] T010 [US1] In `internal/tui/update.go`: route `Update`'s `tea.KeyPressMsg` case on
      `m.screen` (`screenStartMenu` → `handleStartMenuKey`, `screenReview` → existing `handleKey`,
      `screenStats` → stub returning `m, nil` until Phase 4).
- [X] T011 [US1] In `internal/tui/view.go`: add rendering for `screenStartMenu` — the three menu
      items, with the highlighted one visually distinguished (e.g. a leading marker plus a
      bold/reverse style consistent with existing `cardStyle`/`hintStyle` conventions) — still
      using the current fixed-box style; full-window placement is Phase 5's concern, not this
      one.
- [X] T012 [US1] In `internal/tui/view.go`: change `View()`'s top-level dispatch to switch on
      `m.screen` (`screenStartMenu` → new menu rendering, `screenReview` → existing card
      rendering, `screenStats` → stub until Phase 4).
- [X] T013 [P] [US1] In `internal/tui/update_test.go`: add tests for `handleStartMenuKey` —
      up/down/j/k movement and clamping at both ends, Enter on "Start Review" transitions to
      `screenReview` and returns a command that produces `cardMsg`/`errMsg`, Enter on "Quit"
      returns a `tea.Quit` command (via the existing `isQuitCmd` helper), `q`/Ctrl+C quit from the
      menu.
- [X] T014 [P] [US1] In `internal/tui/view_test.go`: add tests asserting the start-menu screen's
      `View().Content` lists all three actions and visually marks whichever `menuSelected` points
      to.
- [X] T015 [US1] In `internal/tui/model_test.go`: update the golden `teatest` flow — expect the
      start menu first, send the keypresses to navigate to and activate "Start Review", then keep
      the existing post-review assertions unchanged.

**Checkpoint**: User Story 1 is fully functional and testable independently — a learner can open
the session, navigate the menu, and reach the existing review flow or quit.

---

## Phase 4: User Story 2 - View stats without leaving the session (Priority: P2)

**Goal**: Selecting "View Stats" from the start menu shows due count/total cards/streak/retention
(reusing `internal/stats.Service`, unchanged), with `Esc` returning to the start menu.

**Independent Test**: From the start menu, select "View Stats" and confirm the same figures
`meguru stats` reports are shown (including the "unavailable" retention case with no review
history); press `Esc` and confirm return to the start menu.

### Implementation for User Story 2

- [X] T016 [US2] In `internal/tui/update.go`: implement the stats-fetch `tea.Cmd` (calls
      `m.statsSvc.Compute(m.ctx, time.Now())`, returning `statsMsg{summary}` on success or
      `statsErrMsg{err}` on failure) and wire it into `handleStartMenuKey`'s `actionViewStats`
      branch from T009.
- [X] T017 [US2] In `internal/tui/update.go`: handle `statsMsg` (store into `m.statsSummary`,
      clear `m.statsErr`) and `statsErrMsg` (store into `m.statsErr`) in `Update`.
- [X] T018 [US2] In `internal/tui/update.go`: implement the stats-screen key handler — `esc`
      returns to `screenStartMenu` (clearing `m.statsSummary`/`m.statsErr` is not required, next
      fetch overwrites them); `q`/`ctrl+c` quits — replacing the Phase 3 stub in `Update`'s
      `screenStats` routing.
- [X] T019 [US2] In `internal/tui/view.go`: render the stats screen — due count, total cards,
      streak (days), retention percentage or an explicit "unavailable" label when
      `statsSummary.RetentionPercent == nil`, or the error text when `m.statsErr != nil` — per
      contracts/tui-screens.md's Stats screen content, replacing the Phase 3 stub.
- [X] T020 [P] [US2] In `internal/tui/update_test.go`: using the `fakeStatsService` from T006,
      test the stats-fetch command produces `statsMsg`/`statsErrMsg` correctly, `Update` stores
      each into the right field, and `Esc` from `screenStats` returns to `screenStartMenu`.
- [X] T021 [P] [US2] In `internal/tui/view_test.go`: test the stats screen renders the expected
      figures for a populated `statsSummary`, the "unavailable" retention case
      (`RetentionPercent == nil`), and the error case (`statsErr != nil`).

**Checkpoint**: User Stories 1 AND 2 both work independently — full menu → stats → back-to-menu
→ review navigation is in place (still at the old fixed-box render size).

---

## Phase 5: User Story 3 - Interface fills the terminal at any size (Priority: P1)

**Goal**: Every screen (start menu, stats, review) fills the terminal's current width/height,
recalculates live on resize without losing state, and shows a clear message below the 80x24
floor.

**Independent Test**: Launch the session at 80x24, 120x40, and 200x60; confirm content fills the
window at each screen and size, that resizing mid-review (card revealed) preserves that state,
and that shrinking below 80x24 shows the "terminal too small" message instead of a broken layout.

### Implementation for User Story 3

- [X] T022 [US3] In `internal/tui/update.go`: add a `case tea.WindowSizeMsg:` to `Update` that
      stores `msg.Width`/`msg.Height` into `m.width`/`m.height` and returns `m, nil` — handled
      identically regardless of `m.screen` (research.md #1).
- [X] T023 [US3] In `internal/tui/view.go`: add a package-level minimum-size check (80x24,
      research.md #2); when `m.width`/`m.height` are below it, every screen (start menu, stats,
      review, and the existing error/quitting/noneDue branches) renders the "terminal too small"
      message instead of its normal content.
- [X] T024 [US3] In `internal/tui/view.go`: replace each screen's fixed-box rendering (menu from
      T011, stats from T019, review's existing `cardStyle.Render(...)`) with
      `lipgloss.NewStyle().Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center).Render(content)`
      (or equivalent `lipgloss.Place`, per research.md #1) so each screen's content is centered to
      fill the current terminal size.
- [X] T025 [US3] In `internal/tui/view.go`: set `AltScreen: true` on the `tea.View` returned by
      `View()` for every branch (research.md #1), so the whole session runs in the alternate
      screen buffer.
- [X] T026 [P] [US3] In `internal/tui/update_test.go`: test that `tea.WindowSizeMsg` updates
      `m.width`/`m.height` from every screen, and that a resize mid-review (after a card is
      revealed) leaves `m.card`/`m.revealed` unchanged.
- [X] T027 [P] [US3] In `internal/tui/view_test.go`: test that content is present/absent
      appropriately above/below the 80x24 floor on each screen, and that `View().Content` reflows
      (e.g. changes) between two different `m.width`/`m.height` values above the floor.
- [X] T028 [US3] In `internal/tui/model_test.go`: extend or add a `teatest` case using
      `teatest.WithInitialTermSize` at a size above 80x24 (e.g. 100x30, distinct from the existing
      60x12 compact case) to confirm the golden flow still completes with full-window rendering
      enabled; note in a test comment that 60x12 remains intentionally below the 80x24 runtime
      floor purely to keep the golden test's own output compact, matching existing convention.

**Checkpoint**: All three user stories are independently functional and now render full-window;
this is the complete feature per spec.md.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final regression and quickstart validation across the whole feature.

- [X] T029 [P] Run `go test ./internal/tui/... -v` and `go test ./internal/cli/... -v`; fix any
      remaining assertions left over from the pre-Phase-2 single-screen behavior.
- [X] T030 Run `go build ./... && go vet ./...` and quickstart.md §1-2.
- [X] T031 Manually run quickstart.md §3 (multi-size check at 80x24/120x40/200x60, mid-review
      resize, view-stats round trip, quit from every screen).
- [X] T032 Run quickstart.md §4: confirm `meguru review --plain` output is byte-for-byte unchanged
      (FR-012) and `go test ./...` is green across the full repo.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — verification only.
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories (shared `Model`
  shape/constructor signature).
- **User Story 1 (Phase 3)**: Depends on Phase 2 only.
- **User Story 2 (Phase 4)**: Depends on Phase 2 (constructor/fields) and Phase 3 (the start menu
  it's selected from, and the `screenStats` routing stub T010/T012 replace).
- **User Story 3 (Phase 5)**: Depends on Phase 2, and touches rendering/routing already built in
  Phases 3-4 (it replaces each screen's fixed-box render with a full-window one) — this is why P1
  US3 is sequenced last despite its priority: its Independent Test explicitly exercises "every
  screen (menu, stats, review)," which don't all exist until Phases 3-4 land.
- **Polish (Phase 6)**: Depends on Phases 3-5 all being complete.

### Parallel Opportunities

- T006 (Phase 2) has no dependency on T002-T005 landing first in the same file and is marked [P].
- Within Phase 3: T013 and T014 (different test files) can run in parallel once T008-T012 land.
- Within Phase 4: T020 and T021 (different test files) can run in parallel once T016-T019 land.
- Within Phase 5: T026 and T027 (different test files) can run in parallel once T022-T025 land.
- T029 in Phase 6 is marked [P] (independent of T030-T032, which are sequential regression gates).

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 (verification) + Phase 2 (Foundational).
2. Complete Phase 3 (US1: start menu, still fixed-box rendering).
3. **STOP and VALIDATE**: run the Phase 3 Checkpoint's independent test.
4. This alone satisfies the "navigable start menu" half of the original request; the "use all
   available screen real estate" half needs Phase 5.

### Incremental Delivery

1. Setup + Foundational → Phase 3 (US1) → validate → Phase 4 (US2) → validate → Phase 5 (US3) →
   validate → Phase 6 (Polish, full regression).
2. Each phase's Checkpoint is independently demoable, per spec.md's per-story Independent Test.

## Notes

- No `[Story]` label on Setup/Foundational/Polish tasks, per the task-format rules above.
- Every task names its exact file path; none introduce a new package or dependency
  (plan.md/Constitution Check: PASS, no Complexity Tracking needed).
- Commit after each Checkpoint (Phase 2, 3, 4, 5), consistent with this repo's per-slice PR
  history in `specs/00{1..5}-*`.
