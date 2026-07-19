# Feature Specification: TUI Start Menu & Full-Window Layout

**Feature Branch**: `006-tui-start-menu`

**Created**: 2026-07-18

**Status**: Draft

**Input**: User description: "Add a navigable start menu screen to the Meguru TUI and make the
overall TUI use the full available terminal window (full-screen responsive layout) instead of a
fixed small area. The start menu should appear when the app launches, let the user navigate with
the keyboard (arrow keys / vim motions) to choose an action (e.g. start review session, view
stats/dashboard if available, quit), and the whole TUI — start menu and review screen alike —
should reflow to use all available rows/columns of the terminal rather than rendering a small
fixed-size box, handling terminal resize gracefully."

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Choose an action from a start menu (Priority: P1)

As a learner running the interactive review session, I want to land on a menu of actions when
the session opens, so I can choose what to do next instead of being dropped straight into
studying with no other option visible.

**Why this priority**: This is the core of the request and the first thing every learner sees on
every interactive session — without it, there is no navigable entry point to build on.

**Independent Test**: Launch the interactive session with a seeded database. Confirm a menu
appears listing at least "Start Review", "View Stats", and "Quit" before any card is shown, and
that it is fully keyboard-navigable and delivers a working session on its own.

**Acceptance Scenarios**:

1. **Given** the interactive session has just opened, **When** it finishes loading, **Then** the
   learner sees a menu of selectable actions instead of a card or a loading message.
2. **Given** the start menu is showing, **When** the learner presses the Down arrow or `j`,
   **Then** the highlighted selection moves to the next option (and Up/`k` moves to the previous
   one).
3. **Given** an option is highlighted on the start menu, **When** the learner presses Enter,
   **Then** the corresponding action begins (review session starts, stats are shown, or the
   program exits).
4. **Given** the start menu is showing, **When** the learner presses `q` or Ctrl+C, **Then** the
   program exits cleanly, the same as quitting from any other screen.

---

### User Story 2 - View stats without leaving the session (Priority: P2)

As a learner, I want to check my due count, streak, and retention from the start menu, so I can
see my progress before deciding whether to study, without running a separate command.

**Why this priority**: Valuable and explicitly requested ("view stats/dashboard if available"),
but the session is still fully usable for its primary purpose (studying) without it — it depends
on User Story 1's menu existing first.

**Independent Test**: From the start menu, select "View Stats" and confirm the same figures
`meguru stats` would report (due count, total cards, streak, retention) are shown, then confirm
the learner can return to the start menu from that screen.

**Acceptance Scenarios**:

1. **Given** the start menu is showing, **When** the learner selects "View Stats", **Then** the
   due count, total card count, streak, and retention (or "unavailable" when there is no review
   history) are displayed.
2. **Given** the stats screen is showing, **When** the learner presses Esc, **Then** they return
   to the start menu.

---

### User Story 3 - Interface fills the terminal at any size (Priority: P1)

As a learner, I want the session to use the whole terminal window I've sized for it, so the
interface feels intentional and readable instead of a small box floating in a mostly-empty
terminal.

**Why this priority**: Affects every screen the session shows (menu, stats, review) and is called
out as equally important to the menu itself in the request — a menu that still renders as a tiny
fixed box would not satisfy the ask.

**Independent Test**: Launch the interactive session in terminals of at least three different
sizes (e.g. 80x24, 120x40, 200x60) and confirm the rendered content reflows to fill the available
width and height at each size, on every screen (menu, stats, review), and that resizing the
terminal live updates the layout without crashing.

**Acceptance Scenarios**:

1. **Given** the interactive session is open at any screen, **When** the terminal is resized,
   **Then** the layout recalculates to fill the new width and height without crashing or leaving
   stale content on screen.
2. **Given** a card is revealed mid-review, **When** the terminal is resized, **Then** the same
   card and reveal state remain visible after the layout recalculates.
3. **Given** the terminal is smaller than the supported minimum size, **When** the session is
   showing any screen, **Then** a clear "terminal too small" message is shown instead of a broken
   or truncated layout.

---

### Edge Cases

- Nothing due when "Start Review" is chosen: the existing "Nothing due right now" outcome still
  occurs, now sized to the full window instead of a small box.
- Resizing while the stats screen or start menu (not just the review card) is open: layout
  recalculates the same way on every screen, not just during review.
- Resizing below the minimum usable size and then back above it: the normal layout resumes once
  the terminal is large enough again.
- Rapid repeated resize events: the interface settles on the final size without flickering or
  crashing.
- Selecting "Quit" from the start menu behaves identically to pressing `q`/Ctrl+C from the start
  menu — both are immediate, no confirmation prompt (consistent with existing quit behavior on the
  review screen).

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: The interactive TUI MUST show a start menu immediately when the session opens,
  before any due-card content or loading message is displayed.
- **FR-002**: The start menu MUST offer at least three selectable actions: "Start Review", "View
  Stats", and "Quit".
- **FR-003**: Users MUST be able to move the highlighted selection with both arrow keys (Up/Down)
  and vim motions (`j`/`k`).
- **FR-004**: Users MUST be able to activate the highlighted action with Enter.
- **FR-005**: Selecting "Start Review" MUST hand off to the existing due-card loading and review
  flow unchanged (same rating keys, same "nothing due" handling).
- **FR-006**: Selecting "View Stats" MUST display the due count, total card count, streak, and
  retention (or an explicit "unavailable" indicator when there is no review history), matching the
  figures the existing stats computation reports, without any network call.
- **FR-007**: Users MUST be able to return from the stats screen to the start menu (Esc).
- **FR-008**: Selecting "Quit", or pressing `q`/Ctrl+C from any screen, MUST exit the program
  cleanly, consistent with current quit behavior.
- **FR-009**: Every screen in the interactive TUI (start menu, stats, review) MUST render its
  layout to fill the terminal's current width and height rather than a small fixed-size box.
- **FR-010**: The interactive TUI MUST recalculate its layout when the terminal is resized,
  without crashing and without losing the in-progress screen's state (e.g. a revealed card).
- **FR-011**: When the terminal is smaller than the supported minimum size, the interactive TUI
  MUST show a clear message indicating the terminal is too small, instead of a broken or
  truncated layout.
- **FR-012**: The `--plain` linear renderer's behavior and output MUST remain unchanged; the start
  menu and full-window layout apply to the interactive TUI only.

### Key Entities

- **Start Menu**: The ordered list of selectable actions ("Start Review", "View Stats", "Quit")
  and which one is currently highlighted.
- **Screen**: Which of start menu, stats, or review is currently active — governs what the
  learner's keypresses do and what is rendered.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: A learner can go from session launch to actively studying a card in exactly two
  actions: select "Start Review", press Enter.
- **SC-002**: At three or more distinct terminal sizes, every screen's rendered content fills the
  available width and height rather than occupying a small fixed area, verified by manual check
  at each size.
- **SC-003**: Resizing the terminal at any point during a session never crashes the program and
  never loses the current screen's in-progress state (verified across repeated resize events
  during both the start menu and mid-review with a card revealed).
- **SC-004**: A learner can view their current stats and return to the start menu within the same
  session, with no separate command invocation required.
- **SC-005**: Terminals below the supported minimum size always show the "terminal too small"
  message rather than garbled or truncated output, at every screen.

## Assumptions

- **Scope is the interactive TUI only**: this feature changes the interactive session reached via
  `meguru review` (without `--plain` and with a TTY stdout). The `--plain` renderer and the root
  command's existing no-args behavior (print help, exit 0 — a documented M1 decision) are
  unchanged; no new default-to-TUI entry point is introduced.
- **Menu selection does not wrap**: moving past the first or last option stops there rather than
  wrapping around, matching the simplest list-navigation convention and avoiding an extra decision
  point with no clear default.
- **Minimum usable terminal size is 80x24**, the standard baseline already used for terminal
  compatibility; below that, the "terminal too small" message is shown instead of a layout.
- **"View Stats" reuses the existing stats computation** introduced for `meguru stats`
  (specs/005-dashboard-stats) — same figures, same "unavailable" handling for missing data. No new
  metric, persisted state, or computation is introduced by this feature.
- **Full-window layout applies uniformly** to all three interactive screens (start menu, stats,
  review) — no screen is exempted or kept at a fixed small size.
- **No new persisted state**: the learner's menu selection or last-used screen is not remembered
  between sessions.

## Out of Scope (this slice)

- Any change to the `--plain` linear renderer.
- Any change to the root command's no-args behavior.
- New stats metrics beyond what the existing stats computation already reports.
- Persisting the user's last-selected menu option or any new settings/preferences.
- Mouse support.
