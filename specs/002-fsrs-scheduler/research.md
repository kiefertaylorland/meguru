# Phase 0 Research: Real FSRS Scheduling

No `NEEDS CLARIFICATION` markers remain in the Technical Context — the design was already
validated against the actual M1 codebase in `Plans/review-the-repo-and-kind-pebble.md` before this
spec was written. This document records the decisions and rationale for the record, per the
research.md format, plus the open verification step implementation must perform first.

## Decision: Scheduling library

**Decision**: Use `github.com/open-spaced-repetition/go-fsrs` as the sole scheduling engine,
called from a new `internal/scheduler.Schedule` pure function.

**Rationale**: Pre-approved in `docs/product/TECH_STACK.md` §4 specifically for this purpose
("the scheduler sits behind one minimal interface... swappable in principle"). Maintained
reference Go implementation with published test vectors, removing the historical
implementation-simplicity argument for a simpler algorithm (SM-2). FSRS has been Anki's default
scheduler since 23.10 and produces materially fewer reviews at equal retention — directly serving
PRD US-4's "review load stays efficient and honest."

**Alternatives considered**: Hand-rolling FSRS math in-repo — rejected, no reason to duplicate a
maintained, test-vector-verified reference implementation for a well-specified public algorithm.
Keeping SM-2 as a stepping stone before FSRS — rejected, `TECH_STACK.md` already made this
decision at the architecture level; re-litigating it here would contradict CON-1 (load
constitution/tech-stack first, don't relitigate settled architecture decisions).

## Decision: Scheduler interface shape

**Decision**: Replace `NextDue(rating Rating, now time.Time) time.Time` with `Schedule(current
CardState, rating Rating, now time.Time) Outcome`, keeping the "pure function, no `*sql.DB`
access" contract from M1 intact.

**Rationale**: FSRS needs the card's full prior memory state (not just "now") to compute a new
one — a `(rating, now) -> due_at` signature is structurally insufficient once the naive
placeholder is gone. Keeping it a pure function (vs. a stateful `Scheduler` object bound to a DB
connection) preserves testability: `internal/review/service.go` remains the only place that talks
to storage, and `internal/scheduler` remains a self-contained, `rapid`-testable, reference-vector
verifiable package with no side effects.

**Alternatives considered**: Passing `*review.Card`/`*sql.Row` directly into the scheduler —
rejected, would collapse the storage/algorithm boundary the M1 plan explicitly built and make the
scheduler untestable without a database. Exposing a `NewScheduler(params fsrs.Parameters)`
constructor now for future per-user tuning — rejected as speculative scope; per-user parameter
optimization is explicitly post-MVP per `TECH_STACK.md` §4 ("post-MVP feature, not a schema
migration"). A `// TODO(M2.x): expose Parameters once per-user optimization lands` comment
documents the extension point instead of building unused surface now (Simplicity First).

## Decision: No schema migration

**Decision**: Ship this feature against the existing `0001_init.sql` schema unchanged.

**Rationale**: Every field `go-fsrs` needs to read or write — `state`, `stability`, `difficulty`,
`due_at`, `last_review_at`, `reps`, `lapses` on `srs_state`; `stability_before`,
`difficulty_before`, `elapsed_days`, `scheduled_days` on `review_log` — already exists as a
placeholder column populated by the naive scheduler with zero/null values. `data-model.md` in
`specs/001-walking-skeleton/` states this in plain terms: those columns are "present only so the
column exists for M2's FSRS swap without a migration." `go-fsrs`'s `Parameters` (17 weights) is
global scheduler configuration, not per-card row data, and stays hardcoded to
`fsrs.DefaultParam()` — a future feature can add config storage for personalized parameters
without touching this schema again.

**Alternatives considered**: Adding a `parameters` table or `config.toml` scheduler block now —
rejected as out of scope per this feature's spec (Assumptions: "one fixed, shared scheduling
policy for all learners... deferred to a later feature").

## Decision: Lapse semantics change (behavior note, not a design gap)

**Decision**: A "lapse" (increment to `srs_state.lapses`) is defined per `go-fsrs`'s own state
machine — an `Again` rating that drops a card out of the `Review` state — not "any `Again`
rating whatsoever" as the naive scheduler treated it.

**Rationale**: This is what FSRS's `Repeat` function itself computes; overriding it in
`internal/review/service.go` (as the naive scheduler's ad hoc `if rating == Again { lapses++ }`
did) would silently diverge from FSRS's own accounting and corrupt the `review_log` data a future
parameter-optimization feature will depend on (FR-007 in spec.md formalizes this).

**Alternatives considered**: Preserving the naive "every Again is a lapse" behavior for backward
test-compatibility — rejected; it would misrepresent the memory model to the learner and the
future optimizer, and the spec's Edge Cases section explicitly calls out that new-card-Again vs.
known-card-Again must be distinguished.

## Verification step required before implementation (not a design unknown)

The exact `go-fsrs` module version's public API shape (`Card`/`SchedulingInfo`/`FSRS`/
`Parameters` field names, `Rating`/`State` enum ordinals, `DefaultParam()` vs.
`DEFAULT_PARAMETERS` naming) was not verified against a live module cache while writing this plan
(no network access in the planning sandbox). This is **implementation step 0**, not a
`NEEDS CLARIFICATION` — it doesn't affect scope, requirements, or design decisions above, only the
mechanical mapping code. Resolve by running `go get github.com/open-spaced-repetition/go-fsrs` and
`go doc` against the pinned version before writing `internal/scheduler/fsrs.go`; the
reference-vector test in `data-model.md`/`quickstart.md` is what actually proves the mapping is
correct, not an assumption baked into this document.
