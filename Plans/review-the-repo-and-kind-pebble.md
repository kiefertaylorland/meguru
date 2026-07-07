# Next Slice: Real FSRS Scheduler (M2, US-4)

## Context

M1 (walking skeleton) is fully complete: all 33 tasks in `specs/001-walking-skeleton/tasks.md`
are checked off, the corresponding Go code exists and matches, `go build ./...` and
`go test ./...` are green, and CI (3-OS matrix + network-denied gate) is wired up. The repo sits
at a clean checkpoint on `main` with only cosmetic formatting drift uncommitted (whitespace in
`tasks.md`/`hiragana.json`/`docs/preview/index.html` — not part of this slice).

No `specs/002-*` directory exists yet. Per `docs/product/BRD.md`, M1 was scoped explicitly to
prove the riskiest plumbing first using a **deliberately naive, non-FSRS scheduler** — the
`internal/scheduler` package and its call site in `internal/review/service.go` were built with an
intentional swap point for M2's real FSRS engine (confirmed by comments in `naive.go` itself:
*"go-fsrs replaces it wholesale in M2"*, and by `specs/001-walking-skeleton/plan.md`'s Structure
Decision: *"isolated behind a pure `(rating, now) -> due_at` function specifically so it is a
drop-in replacement point for `go-fsrs` in M2"*).

M2's full scope (`docs/product/PRD.md` Feature Scope table) is real FSRS + katakana/N5 decks +
romaji input + dashboard + import/export + GoReleaser — too broad for one slice. **US-4 (FSRS
scheduling)** is the correct first cut: it's the foundational, highest-risk piece every other M2
story either depends on (dashboard stats need real retention data; deck breadth is meaningless
without honest scheduling) or is independent of (romaji input, import/export). The schema was
already built to accept it without migration (`srs_state.stability`/`difficulty`,
`review_log.stability_before`/`difficulty_before` exist today as unused placeholders — see
`specs/001-walking-skeleton/data-model.md`), so this is a contained, well-bounded slice that
follows the project's own "walking skeleton" philosophy: prove the core algorithm swap before
widening content.

## Recommended Approach

Replace `internal/scheduler`'s naive fixed-interval placeholder with a real FSRS scheduler backed
by `github.com/open-spaced-repetition/go-fsrs` (pre-approved, MIT, per
`docs/product/TECH_STACK.md` §4), and wire it into `internal/review.Service.Rate`. This is a pure
algorithm swap behind the existing seam — **no schema migration, no UI changes, no new CLI
surface.**

This should proceed through the project's normal SDLC (`speckit-specify` → `speckit-plan` →
`speckit-tasks` → `speckit-implement`), landing in a new `specs/002-fsrs-scheduler/` directory,
mirroring the structure of `specs/001-walking-skeleton/`. The design below is what that plan
should encode.

### 1. New `internal/scheduler` interface

Delete `internal/scheduler/naive.go`; add `internal/scheduler/fsrs.go`. Keep `Rating` (Again=1,
Hard=2, Good=3, Easy=4 — unchanged, since `internal/tui` and `internal/plain` already depend on
these exact values). Add a `State` enum mirroring `srs_state.state`'s CHECK values
(`new`/`learning`/`review`/`relearning`), plus two new types:

- `CardState` — the FSRS-relevant state read out of `srs_state` before scheduling (`State`,
  `Stability`, `Difficulty`, `Reps`, `Lapses`, `LastReviewAt *time.Time`).
- `Outcome` — everything `review.Service` needs to persist afterward (`NextState`, `Stability`,
  `Difficulty`, `Reps`, `Lapses`, `DueAt`, `ElapsedDays`, `ScheduledDays`).

Replace `NextDue(rating, now) time.Time` with `Schedule(current CardState, rating Rating, now
time.Time) Outcome` — a pure function wrapping `go-fsrs`'s `FSRS.Repeat` internally via a
package-level `fsrs.NewFSRS(fsrs.DefaultParam())` built from a constant (keeps `Schedule`
referentially transparent for property-based testing). Do **not** expose a constructor for custom
parameters yet — per-user FSRS parameter optimization is explicitly post-MVP
(`docs/product/TECH_STACK.md` §4); leave a `// TODO(M2.x): expose Parameters once per-user
optimization lands` comment at the `defaultFSRS` declaration instead of building speculative
surface now.

**Step 0 of implementation**: pin the exact `go-fsrs` version and run `go doc` against it to
confirm `Card`/`SchedulingInfo`/`FSRS`/`Parameters`/`Rating`/`State` field names and enum
ordinals before writing the mapping code — this plan's shapes are based on the well-known go-fsrs
API but weren't verified against a live module cache. The reference-vector test (below) is what
actually proves the mapping is correct.

### 2. Changes to `internal/review/service.go`

`Rate` currently: reads only `state, last_review_at`; hand-computes `elapsedDays` and
`scheduledDays`; calls `scheduler.NextDue(rating, now)`; hardcodes `state = 'learning'` on every
update; increments `lapses` itself whenever `rating == Again`. All of this moves into the
scheduler:

- Widen the `SELECT` to `state, stability, difficulty, reps, lapses, last_review_at`.
- Map the row into a `scheduler.CardState` (needs a small string↔`State` helper matching the
  `srs_state.state` CHECK values).
- Call `outcome := scheduler.Schedule(current, rating, now)` in place of the old `NextDue` call.
- Source `elapsed_days`/`scheduled_days` for the `review_log` insert from `outcome`, not
  hand-computed — but preserve the existing "NULL when there's no prior `last_review_at`"
  contract by gating in Go (only write `outcome.ElapsedDays` when `lastReviewAt.Valid`).
- Populate `review_log.stability_before`/`difficulty_before` from the pre-update
  `stability`/`difficulty` read at the top of `Rate` (these columns exist today, always NULL —
  this slice is what finally fills them in).
- Write `srs_state.state`/`stability`/`difficulty`/`due_at`/`reps`/`lapses` from `outcome`
  directly, rather than hardcoding `state='learning'` and hand-incrementing `lapses`.

**Known behavior change to call out in the PR**: the naive scheduler incremented `lapses` on
*every* `Again` rating; real FSRS only counts a lapse when a card falls out of the `Review` state
on `Again`. `TestRate_AgainIncrementsLapses` (`internal/review/service_test.go:152`) currently
asserts the naive (over-broad) behavior and must be corrected once true go-fsrs semantics are
confirmed empirically — likely splitting into an on-new-card case and an on-review-state-card
case.

`Service`'s exported interface (`Rate(ctx, cardID, rating, now)` signature) is unchanged, so
`internal/tui` and `internal/plain` need no changes.

### 3. Migration: none needed

Every field `go-fsrs` reads or writes (`State`, `Stability`, `Difficulty`, `Due`, `LastReview`,
`Reps`, `Lapses`, plus per-review `ElapsedDays`/`ScheduledDays`) already has a column in
`internal/storage/migrations/0001_init.sql` (all currently unused placeholders, per
`data-model.md`'s explicit note that these exist "for M2's FSRS swap without a migration").
`go-fsrs`'s `Parameters` (17 weights) is global scheduler config, not per-card data — hardcode
`fsrs.DefaultParam()` for this slice; a future slice can add config storage without touching
`srs_state`/`review_log` again.

### 4. Tests to update/add

- **Delete** `internal/scheduler/naive_test.go` (all 4 tests hardcode fixed intervals that are
  meaningless under FSRS); **add** `internal/scheduler/fsrs_test.go` (property-based invariants
  via `pgregory.net/rapid`: due dates never precede `now`, stability/difficulty stay in bounds,
  state transitions follow FSRS's valid graph, `Schedule` is deterministic) and
  `internal/scheduler/fsrs_reference_test.go` (a handful of pinned upstream FSRS test vectors,
  proving the `CardState`↔`fsrs.Card` mapping introduces no drift).
- **Update** in `internal/review/service_test.go`:
  - `TestRate_ComputesScheduledDaysFromNextDue` (line 107) — replace the hardcoded `≈7.0 days`
    assertion with a pinned reference value or an ordering check (Easy > Good > Hard > Again).
  - `TestRate_SecondRatingComputesElapsedDays` (line 135) — the current 2-hour gap fixture rounds
    to 0 whole days under FSRS; widen the fixture gap to 1+ full days.
  - `TestRate_AgainIncrementsLapses` (line 152) — correct per the behavior change in §2 above.
  - `TestRate_GoodDoesNotIncrementLapses` (line 204) — re-verify holds under real FSRS semantics.
  - `TestRate_FirstRatingLeavesElapsedDaysNull` (line 122) — should be unaffected; keep as a
    regression check.
- **Update** `tests/integration/review_roundtrip_test.go`'s
  `TestReview_RateAgainAndEasy_ReschedulesAndLogs` (line 17) — replace the hardcoded
  `WithinDuration(now.Add(1*time.Minute)/7*24h)` assertions with structural invariants
  (`due.After(now)`, `againDue.Before(easyDue)`).
- **No changes expected** in `internal/tui`/`internal/plain` tests — they only exercise the
  `scheduler.Rating` enum, which is unchanged. Run them as a sanity check, not because a change is
  anticipated.

### 5. Dependencies

Add `github.com/open-spaced-repetition/go-fsrs` (non-indirect, `internal/scheduler` only) and
`pgregory.net/rapid` (test-only, already the project's designated property-testing library for
this exact purpose per `docs/product/TECH_STACK.md` §7). Run `go mod tidy` after. Both are
pre-approved in `TECH_STACK.md` — no separate SEC-10 justification needed. Neither introduces
network or OS dependencies, consistent with the offline-first constitution (P-1); the existing
network-denied CI test should still pass untouched, which itself is a useful sanity check that no
accidental import chain reaches out.

## Verification

1. `go doc` the pinned `go-fsrs` version to confirm API shapes before writing mapping code.
2. `go test ./internal/scheduler/...` green (property tests + reference-vector parity test).
3. `go test ./internal/review/...` green (updated + new assertions).
4. `go test ./tests/integration/...` green (rewritten round-trip invariants).
5. `go test ./...` full-suite regression pass — confirms `internal/tui`, `internal/plain`,
   `internal/cli`, and e2e stay green untouched.
6. `go mod tidy`; confirm `go.sum` only adds the two expected modules.
7. Confirm the M1 network-denied CI job (`.github/workflows/ci.yml`) still passes, proving the new
   dependency makes zero egress.
8. Update the spec folder's `contracts/scheduler.md`-equivalent to describe `Schedule`/
   `CardState`/`Outcome` in place of `NextDue`, mirroring `specs/001-walking-skeleton`'s contract
   style (produced by `/speckit-plan` when the `002-fsrs-scheduler` spec is scaffolded).

## Scope estimate

Small-to-medium: ~80–120 lines in `fsrs.go`, ~150–250 lines of new tests, a ~25–35 line diff to
`service.go`'s `Rate`, 4 test files touched, 2 new `go.mod` entries, zero migrations, zero
TUI/CLI changes. Roughly one focused implementation day.
