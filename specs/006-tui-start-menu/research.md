# Phase 0 Research: TUI Start Menu & Full-Window Layout

All items below were resolved during planning (no items were carried into spec.md as
`[NEEDS CLARIFICATION]`); this records the decision, rationale, and alternatives considered for
each.

## 1. How to make the TUI use the full terminal window

**Decision**: Track `tea.WindowSizeMsg{Width, Height}` on the `Model` (`m.width`, `m.height`,
updated on every such message — Bubble Tea v2 sends one on startup and again on every resize),
and render each screen's content into a `width x height` box via lipgloss v2's
`lipgloss.NewStyle().Width(w).Height(h).Align(lipgloss.Center, lipgloss.Center)` (equivalently
`lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)`). Enable the alt-screen buffer by
setting `AltScreen: true` on the `tea.View` returned from `View()` — confirmed via `go doc` that in
Bubble Tea v2, alt-screen is a **per-`tea.View` field**, not a `tea.ProgramOption` (there is no
`tea.WithAltScreen` in this version; the option list is `WithColorProfile`, `WithContext`,
`WithEnvironment`, `WithFPS`, `WithFilter`, `WithInput`, `WithOutput`, `WithWindowSize`,
`WithoutCatchPanics`, `WithoutRenderer`, `WithoutSignalHandler`, `WithoutSignals`).

**Rationale**: This is the idiomatic v2 pattern and requires no new dependency — `lipgloss/v2` and
`bubbletea/v2` are already in `go.mod` (v2.0.5 / v2.0.8) but neither `Width`/`Height`/`Align`
styling nor `AltScreen` is used anywhere in the current `internal/tui` code, so today's small
fixed `cardStyle` box (padding 1,2 + rounded border, no width/height) is what leaves most of the
terminal empty.

**Alternatives considered**:

- Leaving alt-screen off and just widening the box: rejected — without the alt-screen buffer the
  rendered box still sits inline in scrollback rather than occupying the terminal the way a
  full-window TUI (as requested) is expected to; enabling it is a one-field change with no
  downside for this app (it's already exited cleanly via `tea.Quit` on every path).
- A third-party layout/flexbox library: rejected — lipgloss v2's built-in `Place`/`Style` sizing
  already covers every layout need in this feature (single centered content block per screen); no
  justification to add a dependency (CON-2).

## 2. Minimum terminal size / "too small" gate

**Decision**: 80x24, matching the standard terminal-compatibility baseline already used elsewhere
in this project's TUI conventions (tui-design skill's compatibility checklist).

**Rationale**: 80x24 is the universal terminal floor (the original VT100 dimensions) and the
existing golden test (`model_test.go`) already exercises a *smaller* test size (60x12) purely for
compact test output — this feature's real runtime floor is independent of that and should match
documented convention rather than invent a new number.

**Alternatives considered**: A smaller floor (e.g. 40x10) was considered but rejected — it would
let genuinely unreadable layouts through instead of showing a clear message.

## 3. Menu navigation and wrap behavior

**Decision**: Up/Down arrows and `j`/`k` move the highlighted selection; movement clamps at the
first/last item (no wraparound).

**Rationale**: Matches the existing codebase's preference for the simplest correct behavior
(Simplicity First, CLAUDE.md §2) — clamping needs no extra state or edge-case handling for "did we
wrap," and is a common, unsurprising convention for a 3-item menu.

**Alternatives considered**: Cyclic wraparound (common in some TUI menus) was considered but
rejected as an unnecessary decision with no clear default the spec called for.

## 4. Reusing stats computation for the in-TUI "View Stats" screen

**Decision**: Add `stats.Service` (unchanged interface from `internal/stats/stats.go`, already
used standalone by `meguru stats`) as a second constructor dependency to `tui.New`, alongside the
existing `review.Service`. `internal/cli/review.go`'s `runReview` constructs
`stats.NewService(db)` from the same `*sql.DB` handle it already opens, and passes both services
into `tui.New(ctx, reviewSvc, statsSvc)`.

**Rationale**: `stats.Service` and `review.Service` are already independent, DB-backed,
context-based interfaces with the identical "interface + `NewService(db)`" shape (confirmed by
reading both `internal/review/service.go` and `internal/stats/stats.go`) — wiring a second one
into the TUI's constructor is architecturally consistent and requires no interface changes to
either service. This also matches 005-dashboard-stats's own explicit deferral: that spec avoided
changing `review.Service` to expose dashboard data it had no other reason to know; adding
`stats.Service` as its own dependency (rather than bolting stats fields onto `review.Service`)
keeps that separation intact.

**Alternatives considered**: Extending `review.Service` with a `Stats` method: rejected — this is
exactly the cross-cutting interface change 005-dashboard-stats's Assumptions section explicitly
deferred, and every existing fake implementing `review.Service` (in `internal/tui` and
`internal/plain` test files) would need updating for a capability unrelated to reviewing cards.

## 5. Test-double convention for the new `stats.Service` dependency

**Decision**: Add a hand-written `fakeStatsService` in `internal/tui`'s test files (same
package-local, no-mockgen convention already used for `fakeService` in
`internal/tui/update_test.go` and separately in `internal/plain/renderer_test.go`).

**Rationale**: Confirmed via research that this repo has zero mockgen/testify-mock usage anywhere
— every consuming package hand-writes its own minimal fake for exactly the interface it needs.
`internal/cli/stats_test.go` doesn't even use a fake (it tests rendering functions directly against
hand-built `stats.Summary` values), so there is no existing `stats.Service` fake to reuse; this
feature is the first consumer that needs one.

**Alternatives considered**: Introducing a shared/exported fake in `internal/stats` itself for
other packages to import: rejected as speculative — no second consumer exists yet, and the
project's convention is deliberately per-package, minimal fakes (Simplicity First).

## 6. Screen/state modeling

**Decision**: A small unexported `screen` enum (`screenStartMenu`, `screenStats`, `screenReview`)
on `Model`, defaulting to `screenStartMenu`. `Update` and `View` both switch on it; the existing
review-flow logic (`loadNextCard`, `handleKey`'s rating/reveal handling) is kept essentially as-is
and only reached once `screen == screenReview`.

**Rationale**: Minimal, additive change — avoids introducing a second `tea.Model`/sub-program or a
routing framework for what is a 3-screen, single-process TUI. Keeps `Model` a single struct,
consistent with the existing design (`internal/tui/model.go` today is already one flat struct with
boolean state flags like `revealed`/`submitting`/`noneDue`).

**Alternatives considered**: Composing three separate `tea.Model` implementations (one per screen)
with a router: rejected as more machinery than three screens and one shared piece of state
(width/height) justify — YAGNI given the current scope (Simplicity First).
