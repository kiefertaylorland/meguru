# Contract: Scheduler Interface (M1)

This is the internal Go interface contract that isolates the naive M1 scheduler so it is a
drop-in replacement point for FSRS in M2 (see research.md §4 and plan.md's Structure Decision).

## Package: `internal/scheduler`

```go
type Rating int

const (
	Again Rating = 1
	Hard  Rating = 2
	Good  Rating = 3
	Easy  Rating = 4
)

// NextDue is the entire M1 scheduling contract: a pure function from a rating and
// the current time to the next due timestamp. It has no side effects, does not read
// or write srs_state, and is not aware of FSRS-style stability/difficulty — those
// remain zero-valued in srs_state this milestone.
func NextDue(rating Rating, now time.Time) time.Time
```

**Fixed rule (FR-008 — explicitly temporary, must not be made more sophisticated)**:

| Rating | Interval |
|---|---|
| Again | `now + 1 minute` |
| Hard | `now + 1 day` |
| Good | `now + 3 days` |
| Easy | `now + 7 days` |

**Invariants** (property-tested per TECH_STACK.md §7 testing conventions):
- `NextDue` never returns a time ≤ `now`.
- `NextDue` is deterministic: same `(rating, now)` always yields the same result.
- `NextDue` never panics for any of the four valid `Rating` values; an invalid `Rating` value is
  a caller programming error (the CLI/TUI layer only ever constructs one of the four constants).

## Package: `internal/review`

```go
// Service orchestrates one review action: look up the next due card, or record a
// rating against a shown card. It is the sole caller of scheduler.NextDue and the
// sole writer of review_log/srs_state, and does so inside one DB transaction per
// rating (see data-model.md's "Write ordering" note — this is what satisfies FR-015).
type Service interface {
	NextDueCard(ctx context.Context) (*Card, error) // nil, nil if nothing is due
	Rate(ctx context.Context, cardID int64, rating scheduler.Rating, now time.Time) error
}
```

This interface is what both the Bubble Tea `tui` package and the `plain` renderer call — neither
UI layer talks to storage or the scheduler directly, keeping the naive-to-FSRS swap and the
TUI-vs-plain swap independent of each other.
