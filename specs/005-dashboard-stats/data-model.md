# Phase 1 Data Model: Dashboard Stats (`meguru stats`)

No schema migration this feature. This document describes the in-memory types `internal/stats`
introduces, how they are derived from the existing SQLite columns confirmed in research.md, and
the streak/retention derivation logic (per the task's requirement to document this even though no
new tables are added).

## In-memory entities (`internal/stats`)

### `Summary`

Everything `meguru stats` reports, in both human-readable and `--json` form:

| Field | Type | Notes |
| --- | --- | --- |
| `DueCards` | `int` | Count of `srs_state` rows with `due_at IS NOT NULL AND due_at <= now`. |
| `TotalCards` | `int` | `COUNT(*)` over `cards`. |
| `StreakDays` | `int` | See "Streak derivation" below. `0` when there is no current streak. |
| `RetentionPercent` | `*float64` | `nil` when there are zero reviews in the retention window (spec.md FR-003/SC-003); otherwise a value in `[0, 100]`. |
| `RetentionWindowDays` | `int` | Always `30` this feature (research.md); included in output so the figure is self-describing without reading docs. |
| `NextDueAt` | `*time.Time` | Earliest `srs_state.due_at` across all cards with a non-NULL `due_at`, or `nil` if no card has ever been scheduled. May be in the past (i.e. already due) when `DueCards > 0`; most meaningful when `DueCards == 0`. |

`Summary` is a plain value type with no behavior — it is what `stats.Service.Compute` returns and
what `internal/cli/stats.go` renders. It is never written back to the database.

### `Service` / `service`

Mirrors `internal/review.Service`/`service`'s shape exactly:

```go
type Service interface {
    // Compute returns a fresh Summary as of now. now is passed in (not
    // read via time.Now() internally) so it is deterministic in tests,
    // matching review.Service.Rate's existing (ctx, ..., now) convention.
    Compute(ctx context.Context, now time.Time) (Summary, error)
}
```

`service` is the only thing in `internal/stats` that touches `*sql.DB`; every other function in
the package (`StreakDays`, `Retention`) is a pure function taking already-fetched data.

## Streak derivation

**Input**: every `review_log.reviewed_at` timestamp (stored as UTC RFC3339 strings, written by
`internal/review/service.go`'s `Rate` method — confirmed by reading that file, not assumed), `now
time.Time`, and a `*time.Location` (the process's local zone, `time.Local`, injected as a
parameter so tests can pin a specific zone rather than depending on the test machine's TZ).

**Algorithm** (`internal/stats.StreakDays`):

1. Convert every `reviewed_at` timestamp into `loc` and truncate it to a calendar day (year, month,
   day at midnight in `loc`). Collect the distinct set of such days.
2. Compute `today` = `now` converted to `loc` and truncated to a calendar day the same way.
3. If `today` is in the set, start the walk at `today`. Otherwise, if `today - 1 day` (`yesterday`)
   is in the set, start the walk there instead (a streak isn't broken until a full day passes with
   no review — spec.md Acceptance Scenario 2a). If neither is in the set, the streak is `0`.
4. Starting from that day, walk backward one day at a time, incrementing a counter for each
   consecutive day present in the set, stopping at the first gap.
5. Return the counter.

This directly implements spec.md's Acceptance Scenarios 2, 2a, 3, and 4, and the Edge Cases
around zero reviews, broken streaks, today-only reviews, and local/UTC boundary timestamps (step 1
does all timezone conversion once, up front, so every later step operates purely on local calendar
days).

## Retention derivation

**Input**: every `review_log.rating` value (an `int` in `1..4`, matching `scheduler.Rating`'s
existing `Again=1, Hard=2, Good=3, Easy=4`) for rows with `reviewed_at >=` a 30-day-ago UTC cutoff
(a plain duration comparison against the stored UTC timestamps — see research.md for why no
timezone conversion is needed here, unlike the streak).

**Algorithm** (`internal/stats.Retention`):

1. If the input slice is empty, return `(0, false)` — `ok=false` signals "no data," which the
   caller renders as an explicit "unavailable" indicator, never a bare `0%` (spec.md SC-003).
2. Otherwise, count how many ratings are **not** `scheduler.Again` (`1`).
3. Return `(nonAgainCount / totalCount * 100, true)`.

## Relationship to existing schema

No new tables, no new columns, no changed CHECK constraints. This feature is purely: (1) three
read-only queries against `cards`, `srs_state`, and `review_log` (all confirmed to exist in
`internal/storage/migrations/0001_init.sql`), and (2) two pure Go functions
(`StreakDays`, `Retention`) that transform the query results into the `Summary` reported by
`meguru stats`.
