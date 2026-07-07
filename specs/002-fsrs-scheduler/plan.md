# Implementation Plan: Real FSRS Scheduling

**Branch**: `002-fsrs-scheduler` | **Date**: 2026-07-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/002-fsrs-scheduler/spec.md`

## Summary

Replace the M1 naive, fixed-interval scheduler with a real FSRS (Free Spaced Repetition
Scheduler) implementation via `github.com/open-spaced-repetition/go-fsrs`, so `meguru review`
computes each card's next due date from that card's own accumulated memory state (stability,
difficulty) and the rating given, instead of a hardcoded interval identical for every card. This
is a pure algorithm swap behind the seam `internal/review/service.go` already calls through — no
schema migration, no CLI/TUI changes, no new user-facing surface. The M1 schema
(`srs_state.stability`/`difficulty`, `review_log.stability_before`/`difficulty_before`) was built
specifically to receive this change without a migration.

## Technical Context

**Language/Version**: Go (current stable, pinned in `go.mod`; same as M1, 1.23+)

**Primary Dependencies**: `github.com/open-spaced-repetition/go-fsrs` (new, MIT, pre-approved per
`docs/product/TECH_STACK.md` §4) for the scheduling algorithm; `pgregory.net/rapid` (new,
test-only, pre-approved per `docs/product/TECH_STACK.md` §7) for property-based scheduler
invariant tests. No other new dependencies. `internal/tui`, `internal/plain`, `internal/cli`,
`internal/storage`, `internal/deck` are untouched.

**Storage**: Same single SQLite file (WAL, `modernc.org/sqlite`) from M1. No migration — every
field FSRS reads/writes (`state`, `stability`, `difficulty`, `due_at`, `last_review_at`, `reps`,
`lapses` on `srs_state`; `stability_before`/`difficulty_before`/`elapsed_days`/`scheduled_days` on
`review_log`) already exists in `internal/storage/migrations/0001_init.sql`, currently populated
with placeholder/zero values by the naive scheduler.

**Testing**: stdlib `testing` + `testify/require` (existing pattern); new property-based tests via
`pgregory.net/rapid` for scheduler invariants (due dates never precede `now`, stability/difficulty
stay within FSRS-documented bounds, state transitions follow FSRS's valid graph, determinism);
new reference-vector parity tests pinning a handful of published upstream FSRS test vectors
against `DEFAULT_PARAMETERS`; existing `internal/review` unit tests and
`tests/integration/review_roundtrip_test.go` updated where they hardcode naive-scheduler interval
assumptions.

**Target Platform**: Same cross-platform CLI/TUI target as M1 (Linux/macOS/Windows) — no
platform-specific concerns; `go-fsrs` is pure Go, no CGo, no OS/network dependency.

**Project Type**: Single Go module, CLI/TUI application (unchanged from M1 — Option 1 layout).

**Performance Goals**: No new performance targets. Scheduling is an in-memory pure computation per
review action; must not regress M1's due-card query or keypress-to-feedback budgets
(`docs/product/PRD.md` NFRs) — `go-fsrs`'s `Repeat` call is O(1) per rating, well within budget.

**Constraints**: Zero network calls (P-1/SEC-8) — `go-fsrs` is a pure computation library with no
I/O, satisfying this trivially. Existing file-permission and offline guarantees from M1 are
unaffected since no storage-layer code changes. Must preserve the `Service` interface's exact
signature (`Rate(ctx, cardID, rating, now)`) so `internal/tui`/`internal/plain` require zero
changes (surgical-change discipline).

**Scale/Scope**: One package rewrite (`internal/scheduler`), one call-site update
(`internal/review/service.go`'s `Rate` method), test updates in `internal/review/service_test.go`
and `tests/integration/review_roundtrip_test.go`. No new packages, no new CLI subcommands, no new
schema.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

| Principle / Rule | Applies? | Assessment |
| --- | --- | --- |
| P-1 Offline-First Is Inviolable | Yes | `go-fsrs` is a pure computation library with zero I/O — no network dependency added anywhere, let alone outside `internal/ai`. **PASS** |
| P-2 Local-Only By Default | Yes | All FSRS state (`stability`, `difficulty`, etc.) is written to the existing local SQLite file only; nothing new leaves the machine. **PASS** |
| P-3 User-Supplied AI Only | N/A | No AI touchpoint in this feature. |
| P-4 Data Minimization At The AI Boundary | N/A | No AI payloads. |
| P-5 No Telemetry | Yes | No telemetry added. **PASS** |
| SEC-6/SEC-7 (untrusted input) | N/A | This feature touches scheduling math only, not deck import or AI responses. |
| SEC-8 (network isolation, CI gate) | Yes | `go-fsrs` performs no I/O; the existing M1 network-denied CI test must continue to pass untouched as a sanity check that this dependency introduces no accidental egress. **Gate carried into tasks.** |
| SEC-10 (dependency hygiene) | Yes | `go-fsrs` and `pgregory.net/rapid` are both explicitly named/pre-approved in `docs/product/TECH_STACK.md` §4 and §7 — not novel additions requiring separate justification. **PASS** |
| SEC-12 (file perms, no telemetry) | Yes | No changes to file-permission handling (untouched `internal/storage`); no telemetry. **PASS** |
| CON-1 (load constitution + tech stack first) | Yes | Done — this plan is derived from both, and from the already-reviewed `Plans/review-the-repo-and-kind-pebble.md`. |
| CON-2 (no stray network calls / deps / weakened CI) | Yes | Both new dependencies are pre-approved in TECH_STACK.md; no CI gate is weakened — this feature adds test coverage (property + reference-vector), it does not remove any. |
| CON-3 (no secrets/real user data in fixtures) | Yes | Scheduler test fixtures are synthetic rating sequences and published FSRS test vectors, not user data. |

No violations requiring Complexity Tracking justification. Gate: **PASS**.

## Project Structure

### Documentation (this feature)

```text
specs/002-fsrs-scheduler/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md         # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
├── checklists/
│   └── requirements.md  # Spec quality checklist (/speckit-specify command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
├── scheduler/                  # REWRITTEN this feature: real FSRS, replaces naive placeholder
│   ├── fsrs.go                  # NEW: CardState, Outcome, State enum, Schedule(current, rating, now) -> Outcome
│   ├── fsrs_test.go              # NEW: property-based invariants (pgregory.net/rapid)
│   ├── fsrs_reference_test.go    # NEW: pinned upstream FSRS test-vector parity
│   ├── naive.go                  # DELETED
│   └── naive_test.go             # DELETED
└── review/                     # UNCHANGED structure, MODIFIED Rate() body only
    ├── service.go                 # MODIFIED: Rate() reads/writes full FSRS state via scheduler.Schedule
    └── service_test.go            # MODIFIED: assertions updated for FSRS semantics (see research.md)

tests/
└── integration/
    └── review_roundtrip_test.go  # MODIFIED: hardcoded-interval assertions replaced with structural invariants
```

**Structure Decision**: No new packages or directories. This feature exercises exactly the swap
point M1 deliberately built: `internal/scheduler` is rewritten behind its existing pure-function
contract, `internal/review/service.go` is the sole caller and sole writer of
`review_log`/`srs_state`, and `internal/tui`/`internal/plain`/`internal/cli` need zero changes
because `Service`'s exported interface is unchanged. This matches
`specs/001-walking-skeleton/plan.md`'s explicit Structure Decision that `internal/scheduler` was
isolated "so it is a drop-in replacement point for `go-fsrs` in M2 without touching
`internal/review` or storage code" (only `Rate`'s internal body changes, not `internal/review`'s
public contract or `internal/storage`).

## Complexity Tracking

_No Constitution Check violations — table not needed._
