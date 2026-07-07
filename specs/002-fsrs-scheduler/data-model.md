# Phase 1 Data Model: Real FSRS Scheduling

No schema migration this feature (see research.md). This document describes the in-memory types
`internal/scheduler` introduces and how they map onto the existing SQLite columns from
`specs/001-walking-skeleton/data-model.md` — it is a mapping document, not a new schema.

## In-memory entities (`internal/scheduler`)

### `Rating`

Unchanged from M1: `Again=1, Hard=2, Good=3, Easy=4`. Preserved exactly because `internal/tui`
and `internal/plain` already depend on these ordinal values for key-mapping and display.

### `State`

New enum mirroring `srs_state.state`'s existing CHECK constraint values:

| Go value | DB string |
| --- | --- |
| `StateNew` | `'new'` |
| `StateLearning` | `'learning'` |
| `StateReview` | `'review'` |
| `StateRelearning` | `'relearning'` |

Requires a small string↔`State` mapping helper in `internal/scheduler` (or `internal/review`,
whichever is called from `service.go`'s read/write path) — no new DB column, just a typed view
onto the existing `TEXT CHECK (...)` column.

### `CardState`

Everything `Schedule` needs as input, read out of `srs_state` at the top of `Rate` before this
feature's card update:

| Field | Source column | Notes |
| --- | --- | --- |
| `State` | `srs_state.state` | mapped via the table above |
| `Stability` | `srs_state.stability` | `REAL`, currently always 0 under the naive scheduler |
| `Difficulty` | `srs_state.difficulty` | `REAL`, currently always 0 |
| `Reps` | `srs_state.reps` | `INTEGER` |
| `Lapses` | `srs_state.lapses` | `INTEGER` |
| `LastReviewAt` | `srs_state.last_review_at` | `*time.Time`, nil when column is NULL (never reviewed) |

### `Outcome`

Everything `Schedule` produces, written back to `srs_state` and `review_log` in the same
transaction `Rate` already opens:

| Field | Destination column | Notes |
| --- | --- | --- |
| `NextState` | `srs_state.state` | mapped back to its string form |
| `Stability` | `srs_state.stability` | |
| `Difficulty` | `srs_state.difficulty` | |
| `Reps` | `srs_state.reps` | sourced from FSRS's own bookkeeping, not hand-incremented |
| `Lapses` | `srs_state.lapses` | sourced from FSRS's own bookkeeping — see research.md's lapse-semantics decision |
| `DueAt` | `srs_state.due_at` | |
| `ElapsedDays` | `review_log.elapsed_days` | only written when `CardState.LastReviewAt` was non-nil; NULL on first-ever review (FR-006, preserves M1's existing NULL-on-first-review contract) |
| `ScheduledDays` | `review_log.scheduled_days` | |

Additionally, `review_log.stability_before`/`difficulty_before` are populated directly from the
*input* `CardState.Stability`/`CardState.Difficulty` (the pre-update snapshot), not from
`Outcome` — these two columns capture "what the memory state was right before this rating"
(FR-005), which is the `CardState` passed into `Schedule`, not what it produced.

## Relationship to existing schema

No new tables, no new columns, no changed CHECK constraints. This feature is purely: (1) a new
Go-level mapping between `internal/scheduler`'s types and the columns already defined in
`internal/storage/migrations/0001_init.sql`, and (2) `internal/review/service.go`'s `Rate`
finally reading and writing the full row instead of the M1 subset (`state`, `last_review_at`
only).

## State transitions

FSRS's card-state machine (enforced inside `go-fsrs`, not reimplemented here):

```
new         --[any rating]-->      learning | review
learning    --[Again/Hard]-->      learning
learning    --[Good/Easy]-->       review
review      --[Again]-->           relearning   (this transition increments Lapses)
review      --[Hard/Good/Easy]-->  review
relearning  --[Again/Hard]-->      relearning
relearning  --[Good/Easy]-->       review
```

`internal/scheduler.Schedule` does not implement this graph directly — it is `go-fsrs`'s
`FSRS.Repeat` behavior, mapped through `CardState`/`Outcome`. The property-based tests in
`internal/scheduler/fsrs_test.go` assert observed transitions never leave this graph, as a
regression guard on the mapping code, not a reimplementation of FSRS's own logic.
