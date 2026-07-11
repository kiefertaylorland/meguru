# Contract: `internal/scheduler`

Internal package contract only — `internal/scheduler` has no external (CLI/API) surface; this
document is the contract between `internal/scheduler` and its sole caller,
`internal/review/service.go`, replacing the M1-era `NextDue` contract this same package used to
expose.

## Public surface

```go
package scheduler

type Rating int
const (
    Again Rating = 1
    Hard  Rating = 2
    Good  Rating = 3
    Easy  Rating = 4
)

type State int
const (
    StateNew State = iota
    StateLearning
    StateReview
    StateRelearning
)

type CardState struct {
    State        State
    Stability    float64
    Difficulty   float64
    Reps         int
    Lapses       int
    LastReviewAt *time.Time // nil means "never reviewed"
}

type Outcome struct {
    NextState     State
    Stability     float64
    Difficulty    float64
    Reps          int
    Lapses        int
    DueAt         time.Time
    ElapsedDays   float64
    ScheduledDays float64
}

func Schedule(current CardState, rating Rating, now time.Time) Outcome
```

## Preconditions

- `rating` MUST be one of the four defined constants. Behavior for other values is undefined
  (caller error) — the naive scheduler's silent "default to Hard" fallback is not preserved,
  since `internal/review` only ever constructs `Rating` from its own validated constants (there is
  no external input path that could produce an invalid value; see `contracts/cli.md`-equivalent
  in `specs/001-walking-skeleton/`, which enumerates TUI/plain-mode key mapping as the only rating
  source).
- `current.LastReviewAt == nil` MUST be used exactly when the card has never been reviewed
  (`State == StateNew`, `Reps == 0`). Passing a non-nil `LastReviewAt` on a `StateNew` card is a
  caller error.

## Postconditions

- `Outcome.DueAt` MUST be strictly after `now` for every rating (FR-002, SC-002 corollary — even
  `Again` schedules a short-but-nonzero relearning step, never `now` itself or earlier).
- For two calls with identical `current` and `now` but `rating` ordered Again < Hard < Good < Easy,
  the resulting `Outcome.DueAt` values MUST be non-decreasing in that same order (SC-002).
- `Outcome.Stability` and `Outcome.Difficulty` MUST remain within the bounds `go-fsrs` itself
  documents/enforces (no NaN, no negative values).
- `Outcome.Lapses == current.Lapses + 1` if and only if this rating causes a transition out of
  `StateReview` via `Again`; otherwise `Outcome.Lapses == current.Lapses` (FR-007).
- `Outcome.ElapsedDays` corresponds to the same elapsed-time value `go-fsrs` used internally to
  decay stability for this call — callers MUST NOT recompute `elapsed_days` independently, they
  must persist exactly what `Schedule` returns (research.md's "no divergent accounting" decision).
- `Schedule` is a pure function: identical inputs always produce identical `Outcome` values, no
  package-level mutable state changes across calls (required for the property-based determinism
  test and for `internal/review/service.go` to remain the sole source of truth for persistence).

## Caller contract (`internal/review/service.go`)

`Rate` MUST, within the existing single transaction:

1. Read `state, stability, difficulty, reps, lapses, last_review_at` from `srs_state` for the
   card and construct `CardState`.
2. Call `Schedule(current, rating, now)` exactly once.
3. Insert one `review_log` row using `current.Stability`/`current.Difficulty` as
   `stability_before`/`difficulty_before`, and `Outcome.ElapsedDays`/`Outcome.ScheduledDays` —
   with `elapsed_days` written as SQL NULL when `current.LastReviewAt == nil` (FR-006).
4. Update `srs_state` with every field from `Outcome` (`state`, `stability`, `difficulty`,
   `due_at`, `reps`, `lapses`) plus `last_review_at = now` — no field is hand-derived outside
   `Outcome` any more (this is the behavior change from M1, where `state`/`lapses` were
   hardcoded/derived in `service.go` itself).

`Service`'s exported interface (`NextDueCard`, `Rate(ctx, cardID, rating, now)`) is unchanged —
this contract is entirely internal to `Rate`'s implementation.
