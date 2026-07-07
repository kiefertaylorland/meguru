# Tasks: Real FSRS Scheduling

**Input**: Design documents from `/specs/002-fsrs-scheduler/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/scheduler.md, quickstart.md

**Tests**: Included. This project's existing testing posture (property-based scheduler invariants,
reference-vector parity, integration round-trips) is a binding part of `docs/product/TECH_STACK.md`
§7's testing strategy, not optional TDD scaffolding — and FR-005/FR-007/SC-001/SC-002/SC-004
require automated verification.

**Organization**: Single user story (spec.md: US1, P1) — this feature is one indivisible slice
(the scheduler swap), so all implementation tasks are grouped under it.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1)
- Paths are relative to the repo root, per plan.md's Project Structure (single Go module)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add the new dependency and pin/verify its API surface before any mapping code is
written (research.md's "verification step required before implementation").

- [X] T001 Run `go get github.com/open-spaced-repetition/go-fsrs@latest` and
      `go get -t pgregory.net/rapid@latest` in the repo root, then `go mod tidy`. Record the
      resolved `go-fsrs` version in `go.mod`.
- [X] T002 Run `go doc github.com/open-spaced-repetition/go-fsrs` (and sub-types `Card`,
      `SchedulingInfo`, `FSRS`, `Parameters`, `Rating`, `State`) against the version pinned in
      T001; write the confirmed field names and enum ordinals as a code comment at the top of
      `internal/scheduler/fsrs.go` (created in T004) before writing any mapping logic —
      per research.md, the reference-vector test (T007) is what actually proves the mapping,
      this step just prevents guessing at names.

**Checkpoint**: Dependency pinned; API shape confirmed and documented.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Remove the M1 placeholder so the package is ready for its replacement. Blocks all
US1 work since both old and new schedulers can't coexist in one package under the same contract.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Delete `internal/scheduler/naive.go` and `internal/scheduler/naive_test.go` (superseded
      wholesale per `naive.go`'s own header comment: "go-fsrs replaces it wholesale in M2").

**Checkpoint**: `internal/scheduler` package is empty and ready for the FSRS implementation.

---

## Phase 3: User Story 1 - Honest, adaptive review scheduling (Priority: P1) 🎯 MVP

**Goal**: `meguru review` computes each card's next due date from that card's own accumulated
memory state and the rating given, so intervals lengthen with consistent success and shorten on
lapses — instead of a fixed interval identical for every card.

**Independent Test**: Seed a card, review it repeatedly with consistent "Good" ratings, and
confirm the interval between reviews grows longer each time; separately, confirm an "Again"
rating always produces a sooner due date than an "Easy" rating on an identical fresh card.

### Implementation for User Story 1

- [X] T004 [US1] In `internal/scheduler/fsrs.go`, define `Rating` (unchanged: Again=1, Hard=2,
      Good=3, Easy=4), the new `State` enum (`StateNew`, `StateLearning`, `StateReview`,
      `StateRelearning` mapping to `srs_state.state`'s CHECK values), and the `CardState`/
      `Outcome` structs exactly per `contracts/scheduler.md`'s Public Surface section.
- [X] T005 [US1] In `internal/scheduler/fsrs.go`, implement `Schedule(current CardState, rating
      Rating, now time.Time) Outcome`: a package-level `var defaultFSRS =
      fsrs.NewFSRS(fsrs.DefaultParam())` (or the constructor name confirmed in T002), private
      `toFSRSCard(CardState) fsrs.Card` / `fromSchedulingInfo(fsrs.SchedulingInfo) Outcome`
      mapping helpers, and `Schedule` wiring them through `defaultFSRS.Repeat`. Leave a `//
      TODO(M2.x): expose Parameters once per-user optimization lands` comment at the
      `defaultFSRS` declaration (depends on T004).
- [X] T006 [US1] Update `internal/review/service.go`'s `Rate` method per
      `contracts/scheduler.md`'s Caller Contract: widen the `SELECT` to `state, stability,
      difficulty, reps, lapses, last_review_at`; build a `scheduler.CardState`; call
      `scheduler.Schedule(current, rating, now)` once; insert the `review_log` row using the
      pre-update `stability`/`difficulty` as `stability_before`/`difficulty_before` and
      `outcome.ElapsedDays` (written as SQL NULL when `lastReviewAt` was not valid) /
      `outcome.ScheduledDays`; update `srs_state` writing `state`, `stability`, `difficulty`,
      `due_at`, `reps`, `lapses` directly from `outcome` instead of hand-deriving `state='learning'`
      or incrementing `lapses` locally (depends on T005). Add the string↔`State` helper this
      needs in the same file or in `internal/scheduler` (caller's choice; keep it next to its
      only use).

### Tests for User Story 1

> **NOTE**: Write T007/T008 first against the T004 types, confirm they fail without T005, then
> implement T005.

- [X] T007 [P] [US1] Property-based tests in `internal/scheduler/fsrs_test.go` (using
      `pgregory.net/rapid`): generate random `(CardState, Rating, now)` triples and assert
      `Schedule`'s postconditions from `contracts/scheduler.md` — `Outcome.DueAt` always after
      `now`; Again/Hard/Good/Easy on identical `current` produce non-decreasing `DueAt`;
      `Stability`/`Difficulty` stay in FSRS-documented bounds (no NaN/negative); state transitions
      only follow the graph in `data-model.md`; `Schedule` is deterministic for identical inputs.
- [X] T008 [P] [US1] Reference-vector parity test in
      `internal/scheduler/fsrs_reference_test.go`: a table-driven test hardcoding a handful of
      published upstream FSRS test vectors (rating sequences against a fixed start date under
      `DEFAULT_PARAMETERS`) and asserting `Schedule`'s stability/difficulty/interval outputs match
      exactly — this is what proves the `CardState`↔`fsrs.Card` mapping (T005) introduces no
      drift.
- [X] T009 [US1] Update `internal/review/service_test.go` per research.md's lapse-semantics
      decision and plan.md: fix `TestRate_ComputesScheduledDaysFromNextDue` (line 107, currently
      asserts `≈7.0 days` for Easy — replace with a pinned reference value or an Easy > Good >
      Hard > Again ordering check); fix `TestRate_SecondRatingComputesElapsedDays` (line 135,
      widen the fixture's 2-hour gap to 1+ full days since FSRS's `ElapsedDays` is whole-day);
      split `TestRate_AgainIncrementsLapses` (line 152) into an on-new-card case (does not
      increment lapses) and an on-review-state-card case (increments lapses), per FR-007; re-run
      `TestRate_GoodDoesNotIncrementLapses` (line 204) and `TestRate_FirstRatingLeavesElapsedDaysNull`
      (line 122) as regression checks (depends on T006).
- [X] T010 [P] [US1] Update `tests/integration/review_roundtrip_test.go`'s
      `TestReview_RateAgainAndEasy_ReschedulesAndLogs` (line 17): replace the hardcoded
      `WithinDuration(now.Add(1*time.Minute))`/`WithinDuration(now.Add(7*24*time.Hour))`
      assertions with structural invariants — `due.After(now)` for both ratings, and
      `againDue.Before(easyDue)` on freshly-seeded identical cards (SC-002) (depends on T006).

**Checkpoint**: User Story 1 is fully functional and independently testable — review scheduling
is adaptive per-card, matching spec.md's Acceptance Scenarios 1–5.

---

## Phase 4: Polish & Cross-Cutting Concerns

**Purpose**: Confirm the swap didn't regress anything outside its scope and that offline/CI
guarantees still hold.

- [X] T011 [P] Run `go test ./internal/tui/... ./internal/plain/... ./internal/cli/...` as a
      sanity check — no source changes expected here (only the `scheduler.Rating` enum is
      referenced, and it's unchanged), but confirm nothing incidentally broke.
- [X] T012 Run `go test ./...` (full-suite regression) and confirm all M1 packages (`cli`, `tui`,
      `plain`, `storage`, `deck`, `textwidth`) plus this feature's updated/new tests are green.
- [X] T013 Confirm the M1 network-denied CI job (`.github/workflows/ci.yml`) still passes
      unmodified, proving `go-fsrs`/`pgregory.net/rapid` introduce zero egress (P-1/SEC-8).
- [X] T014 Run the `quickstart.md` validation guide (§§1–4) end-to-end and confirm every expected
      outcome holds, including the manual interval-lengthening check in §3.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Setup (T001/T002 inform what T003 is clearing the way
  for). BLOCKS User Story 1.
- **User Story 1 (Phase 3)**: Depends on Foundational (T003). T005 depends on T004; T006 depends
  on T005; T009/T010 depend on T006.
- **Polish (Phase 4)**: Depends on User Story 1 being complete.

### Parallel Opportunities

- T007, T008 (US1 tests) can be written in parallel once T004 lands (both only need the types,
  not the finished `Schedule` implementation — write them first, expect them to fail, per the
  NOTE above).
- T009, T010 can run in parallel once T006 lands (different files).
- T011 in Polish can run any time after Phase 3 completes, in parallel with T012's setup.

---

## Parallel Example: User Story 1

```bash
Task: "Property-based scheduler invariants in internal/scheduler/fsrs_test.go"
Task: "Reference-vector parity test in internal/scheduler/fsrs_reference_test.go"
```

---

## Implementation Strategy

### MVP First (and only) — User Story 1

1. Complete Phase 1: Setup (pin + verify go-fsrs API)
2. Complete Phase 2: Foundational (remove naive scheduler)
3. Complete Phase 3: User Story 1 (types → Schedule → service.go wiring → tests)
4. **STOP and VALIDATE**: run quickstart.md — confirm intervals lengthen and Again < Easy holds
5. Complete Phase 4: Polish (full regression, network-denied CI, quickstart sign-off)

This feature has exactly one user story; there is no incremental multi-story delivery plan here —
Phase 3 IS the MVP, and Phase 4 is the ship gate.

---

## Notes

- [P] tasks = different files, no dependencies.
- [Story] label maps task to the single user story (US1) for traceability.
- Commit after each task or logical group.
- Avoid: vague tasks, same-file conflicts, skipping T002's API verification before T005's mapping
  code (this is exactly the risk research.md flags).
