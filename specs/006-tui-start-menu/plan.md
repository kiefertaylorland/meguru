# Implementation Plan: TUI Start Menu & Full-Window Layout

**Branch**: `006-tui-start-menu` | **Date**: 2026-07-18 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/006-tui-start-menu/spec.md`

## Summary

Give the interactive Bubble Tea TUI (reached via `meguru review` without `--plain`, on a TTY) a
navigable start-menu screen that appears first, offering "Start Review", "View Stats", and
"Quit". Add a third screen, "Stats", that reuses the existing `internal/stats.Service` (already
powering `meguru stats`) to show due count/streak/retention without leaving the session. Make all
three screens (start menu, stats, review) fill the current terminal width/height by tracking
`tea.WindowSizeMsg` and rendering through lipgloss's box-placement primitives, enable the
alt-screen buffer, and show a "terminal too small" message below an 80x24 floor. No new
dependencies, no schema changes, no changes to `--plain` or root's no-args behavior.

## Technical Context

**Language/Version**: Go (current stable, pinned in `go.mod`; same as prior features).

**Primary Dependencies**: None new. Uses only what's already in `go.mod`:
`charm.land/bubbletea/v2` (v2.0.8) for `tea.WindowSizeMsg`/`tea.View.AltScreen`,
`charm.land/lipgloss/v2` (v2.0.5) for `Style.Width/Height/Align`/`lipgloss.Place` full-window
layout, `meguru/internal/review` (existing `Service`), `meguru/internal/stats` (existing
`Service`, introduced in 005-dashboard-stats — reused as-is, no changes to its interface).

**Storage**: No change. `internal/stats.Service.Compute` already reads the same local SQLite file
`internal/review.Service` uses; both are constructed from the same `*sql.DB` handle in
`internal/cli/review.go`.

**Testing**: stdlib `testing` + `testify/require`, matching `internal/tui`'s existing conventions
found in `model_test.go` (golden/E2E via `teatest.NewTestModel` +
`teatest.WithInitialTermSize(w, h)`, asserting on `tm.Output()`), `update_test.go` (white-box unit
tests calling `m.Update`/`m.handleKey` directly against a hand-written `fakeService` in the same
package — no mockgen anywhere in this repo), and `view_test.go` (constructs `Model` via `New`,
sets private fields directly, asserts `m.View().Content`). A new hand-written `fakeStatsService`
(same convention as the existing `fakeService`) is added alongside it to implement
`stats.Service.Compute` for tests, since no fake for `stats.Service` exists yet anywhere in the
repo.

**Target Platform**: Same cross-platform CLI target as prior features (Linux/macOS/Windows) — no
platform-specific concerns. `tea.WindowSizeMsg` delivery is native to Bubble Tea v2 on all three;
Windows lacks `SIGWINCH` but Bubble Tea v2 handles that internally (existing library behavior, not
something this feature needs to special-case).

**Project Type**: Single Go module, CLI application (unchanged — Option 1 layout).

**Performance Goals**: No new performance targets. Layout recalculation on resize is pure string
formatting (lipgloss) with no I/O; "View Stats" issues the same single `stats.Service.Compute`
call `meguru stats` already performs once per invocation, now triggered on menu selection instead
of process startup.

**Constraints**: Zero network calls (P-1/SEC-8) — unchanged, since both `review.Service` and
`stats.Service` are already local-only. `--plain` renderer and root's documented "no args → print
help, exit 0" behavior (`specs/001-walking-skeleton/contracts/cli.md` §`meguru` root) are
explicitly unchanged (FR-012, spec.md Assumptions).

**Scale/Scope**: Modifies three existing files (`internal/tui/model.go`, `update.go`, `view.go`)
and one call site (`internal/cli/review.go`, to construct and pass a `stats.Service` into
`tui.New`). No new package. No changes to `internal/review`, `internal/stats`, `internal/plain`,
`internal/scheduler`, `internal/storage`, or any CLI flag/command surface.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

| Principle / Rule | Applies? | Assessment |
| --- | --- | --- |
| P-1 Offline-First Is Inviolable | Yes | Both services this feature calls (`review.Service`, `stats.Service`) are already local-SQLite-only; no network call is added anywhere. **PASS** |
| P-2 Local-Only By Default | Yes | Nothing new leaves the device — the stats screen only renders figures already computed locally. **PASS** |
| P-3 User-Supplied AI Only | N/A | No AI touchpoint. |
| P-4 Data Minimization At The AI Boundary | N/A | No AI payloads. |
| P-5 No Telemetry | Yes | No telemetry added; this is a local, on-demand UI, not automatic reporting. **PASS** |
| SEC-6/SEC-7 (untrusted input) | N/A | No new untrusted input surface — menu selection is a bounded enum of three local actions, not user-supplied text. |
| SEC-8 (network isolation, CI gate) | Yes | No I/O beyond the existing local DB handle; the network-denied CI job continues to cover this unchanged. **PASS** |
| SEC-10 (dependency hygiene) | Yes | Zero new dependencies — only already-vendored `bubbletea/v2`/`lipgloss/v2` APIs not yet used. **PASS** |
| SEC-12 (file perms, no telemetry) | Yes | No changes to `internal/storage` file-permission handling; no telemetry. **PASS** |
| CON-1 (load constitution + tech stack first) | Yes | Done — this plan is derived from the constitution, `docs/product/TECH_STACK.md`, the existing `internal/tui`/`internal/stats`/`internal/cli` code, and `specs/001-walking-skeleton/contracts/cli.md`'s root-command contract. |
| CON-2 (no stray network calls / deps / weakened CI) | Yes | Zero new dependencies; no CI gate touched. |
| CON-3 (no secrets/real user data in fixtures) | Yes | All test fixtures (synthetic cards, fake services) match the existing convention — no real user data. |

No violations requiring Complexity Tracking justification. Gate: **PASS**.

## Project Structure

### Documentation (this feature)

```text
specs/006-tui-start-menu/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   └── tui-screens.md
├── checklists/
│   └── requirements.md  # Spec quality checklist (/speckit-specify command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
├── tui/
│   ├── model.go                 # MODIFIED: add screen enum, menu state, width/height,
│   │                               statsSvc dependency, stats result field
│   ├── model_test.go             # MODIFIED: golden test updated for start-menu-first flow
│   │                               (navigate → Enter → existing review assertions)
│   ├── update.go                 # MODIFIED: tea.WindowSizeMsg handling; per-screen key
│   │                               routing (start menu nav, stats screen Esc, existing
│   │                               review handling extracted/kept as-is)
│   ├── update_test.go             # MODIFIED: adds fakeStatsService; tests for menu
│   │                               navigation, screen transitions, resize handling
│   ├── view.go                    # MODIFIED: per-screen View() rendering, full-window
│   │                               placement via lipgloss, AltScreen on tea.View, min-size
│   │                               "terminal too small" gate
│   └── view_test.go               # MODIFIED: assertions for menu/stats screen rendering
│                                    and the too-small message
└── cli/
    └── review.go                 # MODIFIED: construct stats.NewService(db), pass to
                                     tui.New(ctx, reviewSvc, statsSvc)
```

**Structure Decision**: No new package. `internal/tui` gains a `screen` field (small int enum:
start menu, stats, review) that both `Update` and `View` switch on, keeping the existing
review-screen logic in `handleKey`/the review branch of `View` essentially unchanged and additive.
`stats.Service` is wired in as a second constructor dependency alongside `review.Service`, mirroring
how `internal/cli/stats.go` already constructs `stats.NewService(db)` independently — no changes
to either service's public interface. `internal/plain` and `internal/review`/`internal/stats`
themselves are untouched, matching the spec's Out of Scope section.

## Complexity Tracking

_No Constitution Check violations — table not needed._
