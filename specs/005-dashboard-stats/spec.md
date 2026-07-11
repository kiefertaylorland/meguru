# Feature Specification: Dashboard Stats (`meguru stats`)

**Feature Branch**: `005-dashboard-stats`

**Created**: 2026-07-06

**Status**: Draft

**Input**: User description: "Dashboard with due counts, streak, and retention (M2, US-7), plus
non-interactive `meguru stats --json` output (M2, US-11), so that progress stays visible and
motivating, and so a power user can script and track their own metrics. Reads the existing local
SQLite DB directly — no new schema, no network, no telemetry."

## User Scenarios & Testing _(mandatory)_

### User Story 1 - See progress at a glance (Priority: P1)

As a learner, I want to run one command and see how many cards are due, how many days in a row
I've studied, and roughly how well I'm retaining what I've learned, so that my progress stays
visible and motivating instead of being invisible between review sessions.

**Why this priority**: This is the entire value proposition of US-7 — a learner who never sees
their own progress has no motivating feedback loop, which directly undermines the "daily habit
that doesn't burn out" goal stated in the PRD problem statement. Without it, streaks and
retention are invisibly tracked in the database but never surfaced, which is functionally the
same as not tracking them at all.

**Independent Test**: Seed a database with a mix of reviewed and unreviewed cards across several
simulated days, run `meguru stats`, and confirm the printed due count, streak, and retention
figures match what can be independently computed from the same `review_log`/`srs_state` rows.
Deliverable value (a visible dashboard) is observable without any other M2 feature existing.

**Acceptance Scenarios**:

1. **Given** a database with some cards due now and some not yet due, **When** the learner runs
   `meguru stats`, **Then** the output shows the exact count of cards due right now and the total
   card count.
2. **Given** a learner who has logged at least one review on each of the last 3 calendar days
   (their local time) up to and including today, **When** they run `meguru stats`, **Then** the
   reported streak is 3.
2a. **Given** a learner who reviewed yesterday but has not yet reviewed today, **When** they run
    `meguru stats`, **Then** the streak still counts yesterday (a streak isn't broken until a full
    calendar day passes with no review) — the count run ends at whichever of "today" or
    "yesterday" is the most recent day with a review, and gaps before that end the count.
3. **Given** a learner whose last review was 3 or more days ago, **When** they run `meguru
   stats`, **Then** the reported streak is 0 (the streak is broken).
4. **Given** a learner with zero reviews ever recorded, **When** they run `meguru stats`, **Then**
   the streak is 0 and retention is reported as unavailable (not 0%, since there is no data to
   compute a percentage from) rather than a misleading number.
5. **Given** a learner with review history within the retention window, **When** they run
   `meguru stats`, **Then** retention is reported as the percentage of those reviews that were
   not rated "Again".
6. **Given** any state of the database, **When** the learner runs `meguru stats --json` instead of
   the default output, **Then** the same figures are emitted as a single machine-readable JSON
   object on stdout, suitable for scripting, with no interactive prompts and a `0` exit code on
   success.

### Edge Cases

- Zero reviews ever recorded: streak is 0, retention is reported as unavailable (not a divide-by-
  zero 0%), due/total counts are still reported accurately (Acceptance Scenario 4).
- A broken streak (a gap of one or more full calendar days with no review, followed by today or
  not): only the unbroken run ending at today or yesterday counts; anything before the gap is not
  added on top (Acceptance Scenario 3).
- Reviews logged only earlier today (learner's local calendar day): streak counts as at least 1
  for that day.
- Reviews that fall near a UTC-day/local-day boundary (e.g. logged at 23:45 local time, which may
  be a different UTC calendar date): the streak's day-boundary determination MUST use the
  learner's local calendar day consistently, not UTC, so a late-evening review is not
  miscounted as belonging to the wrong day or silently dropped.
- No cards ever seeded (empty database): due count and total count are both 0; running `stats`
  does not error.

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: The system MUST provide a `meguru stats` command that reports, from the learner's
  existing local database: the number of cards due right now, the total number of cards, the
  current streak in consecutive calendar days, and a retention percentage (or an explicit
  "unavailable" indicator when there is no review history to compute one from).
- **FR-002**: The system MUST compute the streak as the number of consecutive calendar days,
  ending at today or yesterday (learner's local time), with at least one recorded review — derived
  on demand from the existing append-only `review_log`, not from a separately persisted counter
  that could drift out of sync with the log.
- **FR-003**: The system MUST compute retention as the percentage of reviews within a defined
  recent window that were rated something other than "Again"; the window's length MUST be
  documented (see Assumptions).
- **FR-004**: The system MUST support a `--json` flag that emits the same figures as a single
  JSON object on stdout instead of the human-readable format, with no interactive prompts, so it
  is safe to run in scripts and CI.
- **FR-005**: The system MUST exit 0 on success in both output modes, including when there are no
  cards, no due cards, or no review history at all — none of these are error conditions.
- **FR-006**: The system MUST NOT make any network call to compute or report any of these figures
  (P-1/SEC-8) — this is a local dashboard over the learner's own on-device data, not telemetry
  (nothing is sent anywhere; the command only reads the local SQLite file and prints to stdout).
- **FR-007**: The human-readable output MUST remain readable with `NO_COLOR` set, consistent with
  the rest of the CLI's accessibility conventions.

### Key Entities

- **Stats summary**: A derived, read-only snapshot — due-card count, total-card count, streak
  (days), retention (percentage or unavailable) — computed fresh on every invocation from
  `cards`, `srs_state`, and `review_log`. Not itself persisted; nothing new is written to the
  database by this feature.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: Running `meguru stats` against any valid local database (including an empty one)
  completes and exits 0, with due count and total card count always matching a direct count of
  the underlying rows.
- **SC-002**: For a review history with a known, hand-constructed sequence of review days, the
  reported streak exactly matches the number of consecutive days (ending today or yesterday) that
  a human would count by inspecting the same dates.
- **SC-003**: A learner with zero recorded reviews never sees a misleading retention percentage
  (e.g. "0%" implying poor performance) — they see an explicit "no data yet" indicator instead.
- **SC-004**: `meguru stats --json` output is valid, parseable JSON on every run, with no
  additional non-JSON text on stdout, so it can be piped directly into `jq` or stored by a script.

## Assumptions

- **Retention window**: 30 days. Chosen as a simple, fixed MVP window that reflects "recent"
  performance without requiring configuration (a configurable window is a reasonable post-MVP
  enhancement, out of scope here per Simplicity First — no speculative configurability before it's
  asked for). All-time retention was considered but rejected for the default: it would make early
  performance issues (e.g. a rough first week) persist in the displayed number indefinitely even
  after the learner has improved, which is less motivating and less actionable than a rolling
  window.
- **Streak boundary**: "consecutive calendar days" is evaluated in the learner's local system
  time zone, matching how a learner intuitively thinks about "did I study today/yesterday" —
  not UTC, which could show a passing grade or a broken streak that contradicts the learner's own
  clock.
- **No new persisted state**: streak and retention are both computed on demand from the existing
  `review_log` (which is already the FSRS feature's source of truth for review history). No new
  table, column, or cache is introduced — this avoids a second source of truth that could drift
  from the log (Simplicity First, CON-2 dependency/schema discipline).
- **Dashboard scope is the standalone `stats` command**: PRD's Review Session Flow diagram shows
  "Dashboard + time of next due card" as the state shown when `meguru review` finds nothing due.
  `internal/tui/view.go` and `internal/plain/renderer.go` currently show only a bare "Nothing due
  right now." message. Wiring due-count/next-due-time into both the interactive TUI's `Update`/
  `View` state machine and the plain renderer's `Run` loop would require changing the shared
  `review.Service` interface (and every fake/mock implementing it in existing tests) to also
  expose dashboard data it has no other reason to know about — a larger, cross-cutting change than
  this slice's scope. Per this feature's own instructions, that richer "nothing due" screen is
  explicitly deferred to a documented fast-follow; this slice ships the standalone `meguru stats`
  command, which delivers US-7 and US-11 completely on its own and is independently useful
  (scriptable, works before/after/without ever touching the review screens).
- **No interactive TUI for `stats` itself**: `meguru stats` always produces linear text output
  (human-readable or `--json`), with no Bubble Tea interactive mode. A dashboard's value here is
  reading a snapshot, not a persistent interactive screen; the CLI/TUI split
  (`docs/product/TECH_STACK.md` §2) already reserves the interactive mode for the active review
  loop. This keeps the command trivially scriptable and avoids speculative UI surface.

## Out of Scope (this slice)

- Improving the interactive TUI's or plain renderer's "nothing due" screen (see Assumptions).
- A review heatmap or historical trend view (explicitly Post-MVP Wave 1 per `docs/product/PRD.md`
  Feature Scope table: "Review heatmap + retention analytics").
- Configurable retention windows or per-deck stats breakdowns.
- Any change to `internal/scheduler` or FSRS scheduling behavior.
