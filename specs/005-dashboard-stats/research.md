# Phase 0 Research: Dashboard Stats (`meguru stats`)

No `NEEDS CLARIFICATION` markers remain in the Technical Context. This document records the
decisions and rationale for the record, per the research.md format used by
`specs/002-fsrs-scheduler/research.md`.

## Decision: Compute streak/retention on demand, no new persisted state

**Decision**: Both the streak and retention figures are computed fresh on every `meguru stats`
invocation by querying `review_log` (and `srs_state`/`cards` for the count fields), rather than
maintaining a running counter (e.g. an `app_state` key updated after every review).

**Rationale**: `review_log` is already the append-only source of truth for review history
(established in M1's schema specifically "to enable FSRS re-optimization & stats" per
`docs/product/TECH_STACK.md` §3's own comment on the table). A separately persisted streak counter
would be a second source of truth that could drift from the log (e.g. if a review is ever deleted,
imported, or replayed) with no way to detect or repair the drift. The task's own instructions and
Simplicity First both flag this directly: avoid premature caching/optimization when the log is
small enough (bounded by a learner's total review count, not an unbounded external input) that a
full scan/aggregate query is cheap.

**Alternatives considered**: A persisted `app_state["streak_days"]` counter updated inside
`review.Service.Rate`'s existing transaction — rejected: it would require `internal/review` (which
currently has no reason to know about streaks) to take on dashboard-presentation logic, coupling
two concerns the M1 architecture deliberately kept separate (`review` is "the sole writer of
review_log/srs_state" per its own package doc; stats is a read-only consumer). It would also need
its own backfill/migration story for databases that already have review history before this
feature ships, whereas computing on demand works correctly on existing data with zero migration.

## Decision: Retention window = 30 days, not all-time or configurable

**Decision**: Retention is computed over the trailing 30 days from `now`, hardcoded.

**Rationale**: See spec.md Assumptions — a rolling window keeps the number actionable and
reflective of recent performance, avoiding an all-time figure that never recovers from an early
rough patch. 30 days is a common, easily-explained default (documented plainly in both the CLI
help text and this spec) requiring no configuration surface. `docs/product/PRD.md`'s Feature Scope
table places "Review heatmap + retention analytics" (i.e., richer, presumably configurable,
retention views) in Post-MVP Wave 1, confirming a single fixed MVP window here is correctly scoped
below that later feature, not overlapping it.

**Alternatives considered**: All-time retention — rejected as the default per the rationale above,
though nothing here prevents a future config option for window length. A 7-day window — considered
too small a sample for a learner reviewing only a handful of cards a day, elevating noise (a
single bad day is a much stronger swing on a 7-day denominator than a 30-day one).

## Decision: Streak day-boundary uses local time, retention window uses UTC duration

**Decision**: `StreakDays` converts each `review_log.reviewed_at` timestamp (stored as UTC
RFC3339, per `internal/review/service.go`'s existing `now.UTC().Format(time.RFC3339)` writes) into
the process's local time zone before grouping into calendar days. The retention window, by
contrast, is a simple duration cutoff (`now.UTC().Add(-30*24*time.Hour)`) compared directly against
the stored UTC timestamps — no calendar-day conversion needed, since "was this review within the
last 30 days" doesn't depend on what "day" it happened on in any particular zone.

**Rationale**: A streak is fundamentally about "did I study on this calendar day," which is a
concept a learner evaluates against their own wall-clock day, not UTC — a review at 11:45 PM local
time must count for *that* local calendar day even if UTC has already rolled over to the next
date (spec.md's Edge Cases section calls this out explicitly). Retention has no such
day-boundary concept — it is purely "what fraction of reviews in a trailing time window were not
Again" — so a duration-based UTC comparison is simpler and exactly as correct, with no
timezone-conversion code needed at all (Simplicity First: don't add day-boundary logic where a
plain duration comparison already answers the question).

**Alternatives considered**: Using UTC calendar days for the streak too — rejected because it can
silently misdrop or double-count a late-evening review relative to the learner's own sense of
"today," which spec.md's Edge Cases explicitly requires handling correctly. Using SQLite's
`date(reviewed_at, 'localtime')` to do the day-bucketing in SQL — rejected in favor of pulling raw
timestamps and computing in Go: it keeps the day-boundary logic in one pure, directly unit-testable
Go function (`StreakDays`) instead of splitting it between SQL and Go, and it sidesteps SQLite's
`'localtime'` modifier depending on the *server process's* OS timezone database being correctly
configured, which is no more or less trusted than doing the same conversion via Go's stdlib
`time` package already used everywhere else in this codebase.

## Decision: No interactive TUI mode for `stats`

**Decision**: `meguru stats` always emits linear text (human-readable or `--json`) — there is no
Bubble Tea program for this command, regardless of whether stdout is a TTY.

**Rationale**: `docs/product/TECH_STACK.md` §2 states "TTY launches the TUI; non-TTY (or
`--json`/`--plain`) emits scriptable output" as the general CLI/TUI split policy, but this is
motivated by the *review loop*, which is inherently interactive (present a card, wait for a
keypress, repeat). A stats snapshot has no interaction to drive — it's a single read followed by a
single print, matching how `docs/product/PRD.md` US-11 frames it as strictly about
non-interactive/scriptable output. Building an interactive dashboard view (e.g. auto-refreshing,
navigable panels) is real, but speculative, surface not requested by either user story in scope
here (US-7 asks for a dashboard to exist and be visible, not for it to be a persistent interactive
screen); Simplicity First and the constitution's CON-2 dependency-discipline both favor not adding
Bubble Tea wiring, key handling, or golden-file snapshot tests for a screen nothing in this
feature's scope requires.

**Alternatives considered**: Reusing `internal/tui` to render a static, non-interactive Bubble Tea
frame for `stats` on a TTY — rejected: it would add TUI test surface (teatest golden files) for a
screen with no interaction to test, purely to look more "native," which is exactly the kind of
unrequested polish Simplicity First flags. A plain, always-linear command is trivially correct,
trivially scriptable, and matches how many mature CLIs (e.g. `git status --short`, `docker stats
--no-stream`) implement a one-shot snapshot subcommand.

## Verification step performed before implementation

Confirmed by reading `internal/storage/migrations/0001_init.sql` directly (not assumed from
`docs/product/TECH_STACK.md`'s schema listing, which could in principle drift from the applied
migration) that `cards`, `srs_state.due_at`, `review_log.reviewed_at`, and `review_log.rating`
exist exactly as documented, with `idx_srs_due` already covering the due-count query. No
migration is required; this is confirmed, not assumed.
