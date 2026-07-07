# Implementation Plan: Dashboard Stats (`meguru stats`)

**Branch**: `005-dashboard-stats` | **Date**: 2026-07-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-dashboard-stats/spec.md`

## Summary

Add a new `meguru stats` Cobra subcommand that reads the existing local SQLite database (no
schema changes) and reports due-card count, total-card count, current streak (consecutive local
calendar days with at least one review), and 30-day retention (percentage of reviews not rated
Again). Supports `--json` for scriptable output (US-11) and a plain human-readable default (US-7).
Streak and retention are computed on demand from `review_log`/`srs_state`/`cards` by a new,
storage-agnostic `internal/stats` package, mirroring how `internal/scheduler` is a pure,
independently testable computation package that `internal/review` calls into. No changes to
`internal/scheduler`, `internal/tui`, or `internal/plain` (see spec.md Assumptions for why the
"nothing due" dashboard enhancement is deferred).

## Technical Context

**Language/Version**: Go (current stable, pinned in `go.mod`; same as M1/002-fsrs-scheduler).

**Primary Dependencies**: None new. Uses only the existing `database/sql`, `modernc.org/sqlite`
(via `internal/storage`), `github.com/spf13/cobra`, `meguru/internal/scheduler` (for the `Rating`
enum, to avoid a second definition of "Again = 1"), and `meguru/internal/textwidth` (existing
CJK-safe label alignment, reused from `internal/plain`'s convention). `encoding/json` (stdlib) for
`--json`.

**Storage**: Same single SQLite file (WAL) from M1/002. No migration — every column `stats` reads
(`cards.id`, `srs_state.due_at`, `review_log.reviewed_at`, `review_log.rating`) already exists in
`internal/storage/migrations/0001_init.sql`. This feature is read-only against the DB: it never
writes to any table.

**Testing**: stdlib `testing` + `testify/require` (existing pattern). Unit tests for the pure
streak/retention functions in `internal/stats` (edge cases: zero reviews, broken streak, reviews
only today, reviews spanning a local/UTC day-boundary — per spec.md Edge Cases). Unit tests for
the Cobra command wiring in `internal/cli` (flag registration, JSON vs. plain dispatch), mirroring
`internal/cli/review_test.go`'s style. An integration test in `tests/integration` seeding a real
temp SQLite DB with synthetic review history and asserting `stats.Service.Compute` output,
mirroring `tests/integration/review_roundtrip_test.go`. An e2e test in `tests/e2e` running the
compiled binary with `stats --json` and `stats`, mirroring `tests/e2e/plain_test.go`.

**Target Platform**: Same cross-platform CLI target as M1/002 (Linux/macOS/Windows) — no
platform-specific concerns. Streak's local-time computation uses Go's stdlib `time.Local`, which
is already relied upon implicitly wherever `time.Now()` is used elsewhere in the codebase.

**Project Type**: Single Go module, CLI application (unchanged — Option 1 layout).

**Performance Goals**: No new performance targets beyond the existing NFRs. `stats` runs three
simple, indexed-or-small-table queries (`COUNT(*)` on `cards`, a `due_at` range count on
`srs_state` — already indexed via `idx_srs_due` — and a bounded `review_log` scan over the last 30
days) once per invocation; well within the existing due-card-query budget
(`docs/product/PRD.md` NFRs).

**Constraints**: Zero network calls (P-1/SEC-8) — `internal/stats` only opens the local SQLite
file via the existing `internal/storage.Open`/`Migrate` functions, identical to `review`'s
startup sequence, minus the deck-seeding step (stats has no reason to seed decks; an empty/fresh
database simply reports zero counts, per spec.md Edge Cases). No secrets, no user-identifying
data leaves the machine — the command's entire output is local counts and percentages printed to
the invoking terminal or piped to a local script.

**Scale/Scope**: One new package (`internal/stats`), one new CLI subcommand
(`internal/cli/stats.go`), one line added to `internal/cli/root.go` to register it. No changes to
`internal/scheduler`, `internal/review`, `internal/tui`, `internal/plain`, or
`internal/storage`'s schema.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

| Principle / Rule | Applies? | Assessment |
| --- | --- | --- |
| P-1 Offline-First Is Inviolable | Yes | `internal/stats` performs zero I/O beyond the existing local SQLite connection; no network dependency added anywhere. **PASS** |
| P-2 Local-Only By Default | Yes | All figures are derived from and stay on the local DB; nothing is transmitted. **PASS** |
| P-3 User-Supplied AI Only | N/A | No AI touchpoint in this feature. |
| P-4 Data Minimization At The AI Boundary | N/A | No AI payloads. |
| P-5 No Telemetry | Yes | This is explicitly a *local* dashboard the learner runs on demand to see their own data — the opposite of telemetry (which is automatic, provider-bound, and about the vendor observing the user). Nothing is sent anywhere, ever, by this command; `docs/product/PRD.md` US-7/US-11 and `docs/product/CONSTITUTION.md` SEC-12 are read as fully compatible with a local-only stats command. **PASS** |
| SEC-6/SEC-7 (untrusted input) | N/A | This feature only reads counts/ratings/timestamps it already trusts (written by `internal/review`'s own validated `Rate` path); no new untrusted input surface. |
| SEC-8 (network isolation, CI gate) | Yes | `internal/stats` does no I/O beyond the existing local DB handle; the M1 network-denied CI job continues to cover this command with zero code changes needed. **PASS** |
| SEC-10 (dependency hygiene) | Yes | No new dependencies added at all — stdlib + already-vendored packages only. **PASS** |
| SEC-12 (file perms, no telemetry) | Yes | No changes to file-permission handling (`internal/storage` untouched); no telemetry, per P-5 assessment above. **PASS** |
| CON-1 (load constitution + tech stack first) | Yes | Done — this plan is derived from both, plus `docs/product/PRD.md` US-7/US-11 and the existing M1/002 code read in full. |
| CON-2 (no stray network calls / deps / weakened CI) | Yes | Zero new dependencies; no CI gate touched or weakened — this feature adds test coverage, it removes none. |
| CON-3 (no secrets/real user data in fixtures) | Yes | All test fixtures (seeded cards, review timestamps, ratings) are synthetic, matching the existing test suite's conventions (e.g. `tests/integration/review_roundtrip_test.go`'s seeded hiragana deck). |

No violations requiring Complexity Tracking justification. Gate: **PASS**.

## Project Structure

### Documentation (this feature)

```text
specs/005-dashboard-stats/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md         # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   └── stats-cli.md
├── checklists/
│   └── requirements.md  # Spec quality checklist (/speckit-specify command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
├── stats/                       # NEW package: pure streak/retention math + DB read layer
│   ├── stats.go                  # NEW: Summary struct, Service interface, service.Compute
│   ├── stats_test.go             # NEW: Service.Compute tests against a real temp DB
│   ├── streak.go                 # NEW: StreakDays(reviewedAt []time.Time, now, loc) int — pure
│   ├── streak_test.go            # NEW: edge cases from spec.md (zero reviews, broken streak,
│   │                               reviews only today, local/UTC boundary)
│   ├── retention.go              # NEW: Retention(ratings []int) (percent float64, ok bool) — pure
│   └── retention_test.go         # NEW: edge cases (empty window, all-Again, no-Again)
└── cli/
    ├── stats.go                  # NEW: newStatsCommand, runStats, JSON/plain output rendering
    ├── stats_test.go             # NEW: flag registration, JSON vs. plain dispatch, rendering
    └── root.go                   # MODIFIED: root.AddCommand(newStatsCommand())

tests/
├── integration/
│   └── stats_test.go            # NEW: seeds a temp DB with synthetic review history across
│                                   several simulated days, asserts stats.Service.Compute output
└── e2e/
    └── stats_test.go            # NEW: runs the compiled binary's `stats` and `stats --json`
                                    against a fresh XDG profile, asserts exit 0 and shape of output
```

**Structure Decision**: `internal/stats` follows the same shape as `internal/scheduler`: pure,
unit-testable computation functions (`StreakDays`, `Retention`) with no `*sql.DB` access, plus a
thin `Service`/`service` wrapper (mirroring `internal/review.Service`/`service`) that is the only
thing touching SQL. `internal/cli/stats.go` follows `internal/cli/review.go`'s exact startup
pattern (`storage.Open` → `storage.Migrate` → build service → dispatch on a flag) minus the
deck-seeding step, which `stats` has no reason to perform. No existing package's public interface
changes; `internal/tui`, `internal/plain`, `internal/review`, `internal/scheduler`, and
`internal/deck` are all untouched.

## Complexity Tracking

_No Constitution Check violations — table not needed._
