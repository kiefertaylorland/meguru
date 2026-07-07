# Implementation Plan: Romaji Answer Input

**Branch**: `004-romaji-input` | **Date**: 2026-07-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-romaji-input/spec.md`

## Summary

Add a new, dependency-free `internal/romaji` package converting Hepburn-style romaji to
hiragana, and a pure `review.CheckAnswer` function comparing a typed romaji answer against a
card's expected reading. Wire this into `internal/plain`'s review loop as a new step between
presenting a card and the existing reveal/rating step, satisfying PRD US-3 and the "User types
answer" / "Auto-check match?" nodes of the Review Session Flow. `internal/tui` and
`internal/scheduler` are untouched (see research.md for the documented TUI-scope decision).

## Technical Context

**Language/Version**: Go (current stable, pinned in `go.mod`; same as M1/M2, 1.25+)

**Primary Dependencies**: None new. `internal/romaji` uses only the Go standard library
(`strings`). `internal/review`'s new `CheckAnswer` imports only `internal/romaji` and `strings`.
`internal/plain` gains no new imports beyond what it already has.

**Storage**: Unchanged. No migration — see `data-model.md` ("N/A — no schema migration").

**Testing**: stdlib `testing` + `testify/require` (existing pattern). Table-driven unit tests in
`internal/romaji/romaji_test.go` covering the full hiragana syllabary, dakuten/handakuten,
digraphs, sokuon, and `n`-disambiguation edge cases (SC-002). Pure-function unit tests in
`internal/review/answercheck_test.go` for `CheckAnswer`'s dual-comparison semantics. Updated
tests in `internal/plain/renderer_test.go` for the new per-card flow (answer prompt before
reveal/rating), plus updated `tests/e2e/plain_test.go` and `tests/e2e/networkdenied_test.go`
stdin fixtures (both now need an answer line before the rating line).

**Target Platform**: Same cross-platform CLI/TUI target as M1/M2 — no platform-specific concerns;
pure string processing, no CGo, no OS/network dependency.

**Project Type**: Single Go module, CLI/TUI application (unchanged — Option 1 layout).

**Performance Goals**: No new performance targets beyond existing NFRs. `ToHiragana` is a single
linear pass over the input string, called once per submitted answer line (not per keystroke in
this slice's `--plain`-only integration) — negligible against the keypress-to-feedback budget.

**Constraints**: Zero network calls (P-1/SEC-8) — trivially satisfied, no I/O anywhere in this
feature. `internal/romaji` MUST have no dependency on storage/TUI/CLI packages (FR-007). Must
preserve `review.Service`'s exact interface (`NextDueCard`, `Rate`) — `CheckAnswer` is a new
package-level function, not a `Service` method, so no interface change ripples to `internal/tui`
(surgical-change discipline; `internal/tui` needs zero source changes).

**Scale/Scope**: One new package (`internal/romaji`), one new file in an existing package
(`internal/review/answercheck.go` + test), one modified file (`internal/plain/renderer.go` +
test), two modified e2e fixture files. No new CLI subcommands, no new schema, no changes to
`internal/scheduler`, `internal/tui`, `internal/deck`, or `internal/storage`.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

| Principle / Rule | Applies? | Assessment |
| --- | --- | --- |
| P-1 Offline-First Is Inviolable | Yes | `internal/romaji` and `CheckAnswer` are pure in-memory string functions — zero I/O, let alone network. **PASS** |
| P-2 Local-Only By Default | Yes | Typed answers are never persisted or transmitted anywhere; they exist only as local variables for one review interaction. **PASS** |
| P-3 User-Supplied AI Only | N/A | No AI touchpoint in this feature. |
| P-4 Data Minimization At The AI Boundary | N/A | No AI payloads. |
| P-5 No Telemetry | Yes | No telemetry added. **PASS** |
| SEC-6/SEC-7 (untrusted input) | Partial | The typed answer is learner-entered local input, not imported deck content or an AI response — `ToHiragana`'s passthrough-on-unrecognized-rune behavior (research.md) ensures it never errors on adversarial/malformed input, satisfying the spirit of "never crash on untrusted input" even though this isn't deck-import or AI-response data. |
| SEC-8 (network isolation, CI gate) | Yes | No I/O anywhere in this feature's code; the existing M1/M2 network-denied CI job continues to pass untouched as a sanity check. **Gate carried into tasks.** |
| SEC-10 (dependency hygiene) | Yes | Zero new dependencies added — `internal/romaji` uses only `strings` from the stdlib. **PASS, trivially.** |
| SEC-12 (file perms, no telemetry) | Yes | No changes to file-permission handling or logging; no telemetry. **PASS** |
| CON-1 (load constitution + tech stack first) | Yes | Done — this plan is derived from both, plus the M1 walking-skeleton and M2 FSRS-scheduler specs for structural precedent. |
| CON-2 (no stray network calls / deps / weakened CI) | Yes | No network calls, no new dependencies, no CI gate touched or weakened — this feature adds test coverage, it does not remove any. |
| CON-3 (no secrets/real user data in fixtures) | Yes | All test fixtures are synthetic romaji strings and the existing synthetic hiragana test deck; no real user data anywhere. |

No violations requiring Complexity Tracking justification. Gate: **PASS**.

## Project Structure

### Documentation (this feature)

```text
specs/004-romaji-input/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md         # Phase 1 output (/speckit-plan command) — N/A, no new schema
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   ├── romaji.md
│   └── answer-check.md
├── checklists/
│   └── requirements.md  # Spec quality checklist (/speckit-specify command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
├── romaji/                       # NEW package this feature
│   ├── romaji.go                  # NEW: ToHiragana(input string) string
│   └── romaji_test.go             # NEW: table-driven syllabary + edge-case coverage
├── review/                     # MODIFIED: one new file, existing files untouched
│   ├── service.go                 # UNCHANGED
│   ├── service_test.go            # UNCHANGED
│   ├── answercheck.go             # NEW: AnswerResult, CheckAnswer(card, typedRomaji)
│   └── answercheck_test.go        # NEW: CheckAnswer unit tests
└── plain/                       # MODIFIED: new pre-rating step in the existing loop
    ├── renderer.go                 # MODIFIED: Run() gains an answer-prompt/auto-check step
    └── renderer_test.go            # MODIFIED: fixtures updated for the new per-card flow

tests/
└── e2e/
    ├── plain_test.go               # MODIFIED: stdin fixture gains an answer line
    └── networkdenied_test.go       # MODIFIED: stdin fixture gains an answer line

internal/tui/                    # UNCHANGED — see research.md's documented scope decision
internal/scheduler/              # UNCHANGED — no scheduling logic touched
```

**Structure Decision**: One new leaf package (`internal/romaji`), additive-only changes to
`internal/review` (a new file, no edits to `service.go`), and a scoped rewrite of
`internal/plain/renderer.go`'s loop body to insert one new step. This mirrors the M2 FSRS
scheduler feature's pattern of keeping a pure-function package (`internal/scheduler`) separate
from the orchestration layer that calls it (`internal/review`) — here `internal/romaji` is the
pure-function layer, `review.CheckAnswer` is the thin orchestration/comparison layer, and
`internal/plain` is the sole UI caller for this slice.

## Complexity Tracking

_No Constitution Check violations — table not needed._
