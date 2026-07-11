# Tasks: Dashboard Stats (`meguru stats`)

**Input**: Design documents from `/specs/005-dashboard-stats/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/stats-cli.md,
quickstart.md

**Tests**: Included. Per `docs/product/TECH_STACK.md` §7's testing strategy, unit tests for pure
computation logic plus integration/e2e coverage are a binding part of this project's testing
posture, not optional TDD scaffolding — and FR-001–FR-005/SC-001–SC-004 require automated
verification.

**Organization**: Single user story (spec.md: US1, P1) — this feature is one indivisible slice
(the `stats` command), so all implementation tasks are grouped under it.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1)
- Paths are relative to the repo root, per plan.md's Project Structure (single Go module)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Nothing to add — this feature introduces no new dependencies (plan.md Technical
Context). Setup is limited to creating the new package skeleton.

- [X] T001 Create the `internal/stats` package directory (no files yet beyond what Phase 3
      creates) — confirms `go.mod`'s existing dependency set (no new `go get` needed) covers
      everything this feature requires.

**Checkpoint**: Package directory exists; no new dependencies pulled.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: None — unlike 002-fsrs-scheduler, there is no existing placeholder implementation to
remove first. `internal/stats` is purely additive.

**Checkpoint**: N/A — proceed directly to Phase 3.

---

## Phase 3: User Story 1 - See progress at a glance (Priority: P1) 🎯 MVP

**Goal**: `meguru stats` (and `meguru stats --json`) reports due-card count, total-card count,
streak, and retention, read live from the local database, per `contracts/stats-cli.md`.

**Independent Test**: Seed a database with a mix of reviewed/unreviewed cards across several
simulated days, run `meguru stats` and `meguru stats --json`, and confirm every figure matches
what can be independently computed from the same rows.

### Implementation for User Story 1

- [X] T002 [P] [US1] In `internal/stats/streak.go`, implement `StreakDays(reviewedAt []time.Time,
      now time.Time, loc *time.Location) int` per `data-model.md`'s Streak derivation algorithm —
      pure function, no `*sql.DB` access.
- [X] T003 [P] [US1] In `internal/stats/retention.go`, implement `Retention(ratings []int)
      (percent float64, ok bool)` per `data-model.md`'s Retention derivation algorithm, using
      `meguru/internal/scheduler.Again` for the "not Again" comparison rather than a bare magic
      number — pure function, no `*sql.DB` access.
- [X] T004 [US1] In `internal/stats/stats.go`, define the `Summary` struct exactly per
      `data-model.md`'s field table, the `Service` interface, and `NewService(db *sql.DB)
      Service`; implement `service.Compute(ctx, now)` running the three read-only queries from
      `contracts/stats-cli.md`/`data-model.md` (due count, total count, next-due-at, and
      review_log rows for streak + retention) and wiring their results through `StreakDays`/
      `Retention` (depends on T002, T003).
- [X] T005 [US1] In `internal/cli/stats.go`, add `newStatsCommand()` (registers `--json`, default
      `false`) and `runStats(cmd, jsonFlag)` following `internal/cli/review.go`'s exact startup
      pattern (`storage.Open` → `storage.Migrate` → build `stats.NewService(db)` → compute with
      `time.Now()`) but skipping `deck.Seed` (stats never seeds — contracts/stats-cli.md); add
      JSON rendering (`statsJSON` struct with the exact field names/nullability from
      `contracts/stats-cli.md`) and plain-text rendering (reusing the `internal/textwidth`-based
      label-alignment convention `internal/plain/renderer.go` established) per the two example
      outputs in `contracts/stats-cli.md` (depends on T004).
- [X] T006 [US1] In `internal/cli/root.go`, add `root.AddCommand(newStatsCommand())` alongside the
      existing `newReviewCommand()` registration (depends on T005).

### Tests for User Story 1

- [X] T007 [P] [US1] Unit tests in `internal/stats/streak_test.go` covering spec.md's Edge Cases
      and Acceptance Scenarios 2/2a/3/4: zero reviews ever (streak 0), a broken streak (gap of 1+
      full days before today, streak 0), reviews only today (streak 1), a streak of several
      consecutive days ending today, a streak ending yesterday with none logged yet today (still
      counts per Acceptance Scenario 2a), and a review timestamp near a local/UTC day boundary
      (e.g. 23:45 in a non-UTC `loc`) landing on the correct local calendar day.
- [X] T008 [P] [US1] Unit tests in `internal/stats/retention_test.go` covering: empty input
      (`ok=false`), all-Again ratings (0%, `ok=true`), all-non-Again ratings (100%, `ok=true`), a
      mixed set producing the expected percentage.
- [X] T009 [US1] Integration test in `tests/integration/stats_test.go` (mirroring
      `tests/integration/review_roundtrip_test.go`'s `openTestDB` helper): seed a temp DB with
      synthetic cards/srs_state/review_log rows spanning several simulated days and ratings, call
      `stats.NewService(db).Compute(ctx, now)`, and assert `Summary`'s fields match a
      hand-computed expectation — including the zero-reviews-ever case (`RetentionPercent ==
      nil`) and the zero-cards-ever case (`DueCards == 0`, `TotalCards == 0`, `NextDueAt == nil`)
      (depends on T004).
- [X] T010 [P] [US1] Unit tests in `internal/cli/stats_test.go` (mirroring
      `internal/cli/review_test.go`'s style): `newStatsCommand()` registers `--json` defaulting to
      `false`; JSON output is valid, parseable JSON with the exact field names from
      `contracts/stats-cli.md` and `null` (not `0`) for `retention_percent` when there is no
      review data; plain output contains the expected labels and the "n/a (no reviews yet)" /
      "no cards scheduled" fallback text (depends on T005).
- [X] T011 [P] [US1] E2E test in `tests/e2e/stats_test.go` (mirroring `tests/e2e/plain_test.go`'s
      `buildBinary`/`withCoverEnv` helpers): run the compiled binary's `stats` and `stats --json`
      subcommands against a fresh, empty `XDG_DATA_HOME` temp dir, assert exit code 0 for both,
      valid JSON for `--json` (e.g. via `encoding/json.Unmarshal`), and no ANSI escape sequences
      in the plain-mode output (depends on T005, T006).

**Checkpoint**: User Story 1 is fully functional and independently testable — `meguru stats` and
`meguru stats --json` report accurate figures per spec.md's Acceptance Scenarios 1–6.

---

## Phase 4: Polish & Cross-Cutting Concerns

**Purpose**: Confirm the new command didn't regress anything outside its scope and that
offline/CI guarantees still hold.

- [X] T012 [P] Run `go vet ./...` and `go build ./...` — confirm no vet warnings and a clean
      build across the whole module, not just the new package.
- [X] T013 Run `go test ./...` (full-suite regression) and confirm all existing packages (`cli`,
      `tui`, `plain`, `storage`, `deck`, `scheduler`, `review`, `textwidth`) plus this feature's
      new/updated tests are green.
- [X] T014 Confirm the network-denied CI job (`.github/workflows/ci.yml`) still passes unmodified,
      proving `internal/stats` introduces zero egress (P-1/SEC-8).
- [X] T015 Run the `quickstart.md` validation guide (§§1–4) end-to-end and confirm every expected
      outcome holds, including the manual before/after-one-review check in §3.
- [X] T016 Mark all tasks in this file `[X]` once done, per this feature's own process
      instructions.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: N/A — nothing to remove or block on this feature.
- **User Story 1 (Phase 3)**: Depends on Setup (T001). T004 depends on T002/T003; T005 depends on
  T004; T006 depends on T005; T009/T010 depend on T004/T005 respectively; T011 depends on
  T005/T006.
- **Polish (Phase 4)**: Depends on User Story 1 being complete.

### Parallel Opportunities

- T002, T003 (pure functions, different files) can be written in parallel.
- T007, T008 (unit tests for T002/T003) can be written in parallel once their respective
  implementations land, or written first (expected to fail) per standard TDD ordering.
- T010, T011 can run in parallel once T005/T006 land (different files, different test layers).
- T012 in Polish can run any time after Phase 3 completes, in parallel with T013's setup.

---

## Parallel Example: User Story 1

```bash
Task: "Pure streak derivation in internal/stats/streak.go + tests"
Task: "Pure retention derivation in internal/stats/retention.go + tests"
```

---

## Implementation Strategy

### MVP First (and only) — User Story 1

1. Complete Phase 1: Setup (package skeleton, confirm no new deps needed)
2. Skip Phase 2: Foundational (nothing to remove — purely additive feature)
3. Complete Phase 3: User Story 1 (pure functions → service → CLI command → root wiring → tests)
4. **STOP and VALIDATE**: run quickstart.md — confirm due/total/streak/retention are all correct
   before and after seeding one review, in both plain and `--json` modes
5. Complete Phase 4: Polish (full regression, network-denied CI, quickstart sign-off)

This feature has exactly one user story; there is no incremental multi-story delivery plan here —
Phase 3 IS the MVP, and Phase 4 is the ship gate.

---

## Notes

- [P] tasks = different files, no dependencies.
- [Story] label maps task to the single user story (US1) for traceability.
- Commit after each task or logical group.
- Avoid: vague tasks, same-file conflicts, computing streak/retention anywhere other than the
  pure functions in `internal/stats` (keep `internal/cli/stats.go` a thin rendering/wiring layer,
  matching `internal/review/service.go`'s existing separation of concerns).
