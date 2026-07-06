# Implementation Plan: Walking Skeleton (M1)

**Branch**: `001-walking-skeleton` | **Date**: 2026-07-02 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-walking-skeleton/spec.md`

## Summary

Stand up the end-to-end offline plumbing for Meguru: a Cobra CLI wrapping a Bubble Tea v2 TUI
(with a linear `--plain` fallback), a SQLite store (WAL, `modernc.org/sqlite`, pure Go) with a
first-run migration path, an embedded hiragana deck (`go:embed` JSON) that seeds/updates without
duplication, and a `meguru review` loop driven by a deliberately naive, hardcoded interval-bump
scheduler (NOT FSRS) that writes `review_log` and updates `srs_state`. Non-functional requirements
—`NO_COLOR` support, `0600`/`0700` file permissions enforced and self-healing at startup, and a
3-OS CI matrix with a network-denied test proving the core loop makes zero egress — are built in
from this milestone, not retrofitted.

## Technical Context

**Language/Version**: Go (current stable, pinned in `go.mod`; target 1.23+)

**Primary Dependencies**: `charm.land/bubbletea/v2` + Bubbles + Lip Gloss (TUI); Cobra (CLI);
`modernc.org/sqlite` (pure-Go SQLite driver, WAL mode); `adrg/xdg` (data/config dir resolution);
`go-runewidth`/`uniseg` wrapped in an internal `textwidth` package (CJK-safe width math);
`testify/require` + stdlib `testing` (unit); `teatest` (TUI golden-frame tests). **Explicitly
excluded from this milestone**: `go-fsrs` (FSRS lands in M2 per spec Out-of-Scope) and anything
under `internal/ai` (no AI provider layer yet, per CON-2/P-1 network boundary).

**Storage**: Single SQLite file, WAL mode, at `$XDG_DATA_HOME/meguru/meguru.db` (platform paths
via `adrg/xdg`); `decks`, `notes`, `cards`, `srs_state`, `review_log` tables from
`docs/product/TECH_STACK.md` §3 — only the subset needed for one embedded hiragana deck and the
naive scheduler is populated this milestone (`ai_cache` table not needed yet, may be deferred).

**Testing**: stdlib `testing` + `testify/require` for unit/service tests; `teatest` for TUI golden
frames (plain-mode output included); integration tests against a temp SQLite file exercising
migrations + seed + review + reschedule round-trips; one E2E/network-denied test per SC-005 and
FR-017; GitHub Actions 3-OS matrix (ubuntu/macos/windows).

**Target Platform**: Cross-platform terminal application — Linux, macOS, Windows (desktop OS
families per spec Assumptions).

**Project Type**: Single Go module, CLI/TUI application (Option 1 layout).

**Performance Goals**: First due card visible within 5 seconds of first launch on a clean machine
(SC-001); no other perf targets in scope for this milestone.

**Constraints**: Core loop (storage, seed, review, reschedule) MUST make zero network calls
(P-1/FR-009/SEC-8); DB file `0600`, containing dir `0700`, self-corrected with a warning if found
looser (FR-012/FR-013/SEC-12); `--plain` mode and `NO_COLOR` must each independently suppress
interactive redraws / color codes (FR-010/FR-011); schema migrations must be forward-only and
lossless (FR-014); an interrupted review (killed after card shown, before rating submitted) must
leave zero partial `review_log` rows (FR-015).

**Scale/Scope**: One embedded deck (hiragana), a fixed 4-tier rating scale (Again/Hard/Good/Easy),
a single naive scheduling rule. No import/export, no stats dashboards, no AI touchpoints — all
explicitly out of scope per spec.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

| Principle / Rule                                    | Applies? | Assessment                                                                                                                                                                                                                                                          |
| --------------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| P-1 Offline-First Is Inviolable                     | Yes      | Entire feature is the core loop; FR-009/FR-017 + SEC-8 network-denied CI test enforce this directly. No `internal/ai` package touched. **PASS**                                                                                                                     |
| P-2 Local-Only By Default                           | Yes      | All data (deck, srs_state, review_log) lives in the local SQLite file only; no sync/export in scope. **PASS**                                                                                                                                                       |
| P-3 User-Supplied AI Only                           | N/A      | No AI touchpoint in this milestone.                                                                                                                                                                                                                                 |
| P-4 Data Minimization At The AI Boundary            | N/A      | No AI payloads exist yet.                                                                                                                                                                                                                                           |
| P-5 No Telemetry                                    | Yes      | No telemetry/analytics added. **PASS**                                                                                                                                                                                                                              |
| SEC-6/SEC-7 (untrusted input, deck data-only)       | Yes      | Embedded deck is trusted, versioned, shipped-in-binary content — not an import path — but must still be treated as data, never executed. Card fields render as text only. **PASS**                                                                                  |
| SEC-8 (network isolation, CI gate)                  | Yes      | FR-016/FR-017/SC-005 require exactly this: 3-OS CI matrix + network-denied test. Must be implemented, not just claimed. **Gate carried into tasks.**                                                                                                                |
| SEC-10 (dependency hygiene)                         | Yes      | New deps (Bubble Tea v2, Cobra, modernc.org/sqlite, adrg/xdg, go-runewidth/uniseg, testify, teatest) are all named in TECH_STACK.md already — not novel additions requiring separate justification. `go-fsrs` explicitly NOT added this milestone (CON-2). **PASS** |
| SEC-12 (file perms, no telemetry)                   | Yes      | FR-012/FR-013 (0600/0700, self-heal + warn) are first-class functional requirements this milestone, not deferred. **Gate carried into tasks.**                                                                                                                      |
| CON-1 (load constitution + tech stack first)        | Yes      | Done — this plan is derived from both.                                                                                                                                                                                                                              |
| CON-2 (no stray network calls / deps / weakened CI) | Yes      | No dependency outside the TECH_STACK.md-approved set; no code outside a future `internal/ai` may dial out.                                                                                                                                                          |
| CON-3 (no secrets/real user data in fixtures)       | Yes      | Hiragana deck content and test fixtures are synthetic/public linguistic data, not user data.                                                                                                                                                                        |

No violations requiring Complexity Tracking justification. Gate: **PASS**.

## Project Structure

### Documentation (this feature)

```text
specs/001-walking-skeleton/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/
└── meguru/
    └── main.go                # entrypoint: build Cobra root, wire dependencies

internal/
├── cli/                       # Cobra commands (root, review) — TTY detection dispatches to TUI vs plain
│   ├── root.go
│   └── review.go
├── tui/                       # Bubble Tea v2 program: model/update/view for the review screen
│   ├── model.go
│   ├── update.go
│   └── view.go
├── plain/                     # linear, non-interactive renderer used by --plain and non-TTY stdout
│   └── renderer.go
├── textwidth/                 # sole CJK-safe width-math package (wraps go-runewidth/uniseg)
│   └── textwidth.go
├── storage/                   # SQLite access: connection, migrations, permission enforcement
│   ├── db.go                  # open (WAL), 0600/0700 enforcement + self-heal (FR-012/013)
│   ├── migrate.go             # forward-only schema migrations (FR-014)
│   └── migrations/            # embedded .sql migration files
├── deck/                      # embedded hiragana deck + seed/update-in-place logic
│   ├── embed.go                # go:embed hiragana.json
│   ├── hiragana.json
│   └── seed.go                 # seed on first run; content_version-aware update, no duplication
├── scheduler/                  # naive M1 placeholder scheduler (explicitly NOT FSRS)
│   └── naive.go                # pure fn: (rating, now) -> due_at, per FR-008 fixed intervals
└── review/                     # orchestration: due-card lookup, rate a card, persist log + state
    └── service.go

tests/
├── integration/                # temp-SQLite: migrate + seed + review + reschedule round-trips
├── e2e/                        # compiled-binary smoke test under a PTY, incl. network-denied run
└── unit/                       # (co-located _test.go files are preferred; this dir only if needed)
```

**Structure Decision**: Single Go module (Option 1). Business logic lives under `internal/`
(not importable outside this module, matching a solo-dev CLI tool with no public library surface
yet); `cmd/meguru` stays a thin entrypoint. `internal/scheduler` is isolated behind a pure
`(rating, now) -> due_at` function specifically so it is a drop-in replacement point for `go-fsrs`
in M2 without touching `internal/review` or storage code.

## Complexity Tracking

_No Constitution Check violations — table not needed._
