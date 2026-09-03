# Feature Specification: Per-Deck Review Filtering

**Feature Branch**: `007-deck-filter`

**Created**: 2026-07-19

**Status**: Draft

**Input**: User description: "Add per-deck filtering to the review flow, and a menu option to
study just one deck in isolation. Concretely: (1) a way to run a review session scoped to a
single deck (e.g. just Hiragana, or just JLPT N5 Kanji) instead of always pooling due cards
across all four built-in decks together, available both as a CLI-level option on `meguru review`
and (2) from the interactive TUI's start menu, an option/flow to pick which deck to study before
starting the review session, so a learner can say 'just kanji today' or 'just katakana' without
studying everything at once. Should work with the existing deck registry (internal/deck's four
built-in decks: Hiragana, Katakana, JLPT N5 Kanji, JLPT N5 Vocabulary) with no changes to deck
content itself, and should not break the existing default behavior of reviewing all due cards
across every deck when no filter is specified."

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Study one deck from the command line (Priority: P1)

As a learner running `meguru review` from a script or terminal directly, I want to scope the
session to a single deck by name, so I can say "just kanji today" without touching a menu.

**Why this priority**: This is the CLI half of the request and the simplest, most directly
testable slice — it doesn't require the interactive TUI to exist at all, and it's usable by both
the plain and interactive renderers.

**Independent Test**: With a seeded database containing due cards in multiple decks, run `meguru
review --deck jlpt-n5-kanji` (or `--plain`) and confirm only kanji-deck cards are shown across
the whole session, in both plain and interactive mode, while a plain `meguru review` with no flag
still pulls from every deck exactly as before.

**Acceptance Scenarios**:

1. **Given** due cards exist in more than one deck, **When** the learner runs `meguru review
   --deck <slug>` for a known deck slug, **Then** every card shown during that session belongs to
   that deck only, until nothing more is due in it.
2. **Given** the chosen deck has no cards due right now (but other decks do), **When** the
   learner runs the filtered command, **Then** the session reports nothing due — scoped to that
   deck's message, not a blanket "nothing due" that could be misread as no cards existing at all.
3. **Given** the learner passes an unrecognized deck identifier, **When** they run the command,
   **Then** the program prints a clear error naming the valid deck choices and exits without
   starting a session.
4. **Given** the learner runs `meguru review` with no `--deck` flag, **When** the session runs,
   **Then** behavior is unchanged from today — due cards are pulled from every deck together.

---

### User Story 2 - Study one deck from the start menu (Priority: P1)

As a learner using the interactive session, I want a menu option to pick a single deck to study,
so I can choose "just katakana" from the menu without knowing or typing a CLI flag.

**Why this priority**: This is the TUI half of the request and the primary way most learners will
actually use this feature day-to-day, since the interactive session is the main way the app is
used.

**Independent Test**: Launch the interactive session, select the new "Study a Deck" menu option,
choose one deck from the list shown, and confirm the ensuing review session only shows cards from
that deck; confirm the original "Start Review" option still reviews every deck together exactly
as before.

**Acceptance Scenarios**:

1. **Given** the start menu is showing, **When** the learner selects "Study a Deck", **Then** a
   list of the available decks is shown, navigable the same way as the start menu (arrow
   keys/`j`/`k`, Enter to choose).
2. **Given** the deck list is showing, **When** the learner selects a deck and presses Enter,
   **Then** the review session begins, scoped to only that deck's due cards.
3. **Given** the deck list is showing, **When** the learner presses Esc, **Then** they return to
   the start menu without starting a review.
4. **Given** the learner instead selects the original "Start Review" option, **When** the session
   begins, **Then** it reviews due cards from every deck together, exactly as before this feature.
5. **Given** a deck-scoped review session reaches the end of its due cards, **When** the "nothing
   due" screen is shown, **Then** it names the deck that was just studied, so it's clear the
   scoping is deck-specific rather than the whole collection being empty.

---

### Edge Cases

- A deck-scoped session where the chosen deck has zero cards due, but other decks do have cards
  due: the session reports nothing due for that deck specifically (User Story 1 Scenario 2, User
  Story 2 Scenario 5) — it must not silently fall back to showing cards from other decks.
- An unrecognized `--deck` value: a clear error naming the valid deck identifiers, no session
  starts, non-zero exit (User Story 1 Scenario 3).
- Combining `--deck` with `--plain`: the deck scope applies identically in both renderers (User
  Story 1 Scenario 1).
- Choosing "Study a Deck" then Esc without picking anything: no filter is applied and the learner
  is back at the start menu, unchanged (User Story 2 Scenario 3).
- A deck that currently has zero total cards (hypothetically, if the registry ever changes):
  scoping to it behaves the same as "nothing due" — not an error, since an empty deck isn't a
  program failure.

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: `meguru review` MUST accept a way to scope the session to exactly one of the
  existing built-in decks (Hiragana, Katakana, JLPT N5 Kanji, JLPT N5 Vocabulary), identified by
  each deck's existing stable identifier.
- **FR-002**: When no deck scope is given, `meguru review` MUST behave exactly as it does today —
  due cards pulled from every deck together, in both plain and interactive mode.
- **FR-003**: When a deck scope is given, every due card presented during that session — in both
  plain and interactive mode — MUST belong to that deck only.
- **FR-004**: An unrecognized deck identifier passed on the command line MUST produce a clear
  error naming the valid choices, without starting a review session, and MUST exit non-zero.
- **FR-005**: The interactive start menu MUST offer a "Study a Deck" option, distinct from "Start
  Review", that lists the available decks and lets the learner choose one with the same
  navigation conventions as the rest of the menu (arrow keys/`j`/`k`, Enter, Esc to back out).
- **FR-006**: Choosing a deck from that list MUST begin a review session scoped to only that
  deck, equivalent to running `meguru review --deck <that deck>` from the command line.
- **FR-007**: The existing "Start Review" menu option MUST continue to review every deck
  together, unaffected by the new "Study a Deck" option's existence.
- **FR-008**: When a deck-scoped session (from either the CLI flag or the menu) has nothing due,
  the "nothing due" message MUST name the deck that was scoped, distinguishing it from the
  unscoped "nothing due at all" message.
- **FR-009**: The review screen, while a deck scope is active, MUST make the active deck visible
  to the learner (so it's never ambiguous whether the whole collection or just one deck is being
  studied).

### Key Entities

- **Deck scope**: An optional selector, present for the duration of one review session, that
  narrows which deck's cards are eligible — no scope means every deck, exactly as today.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: A learner can start a single-deck session, from either the command line or the
  start menu, and every card they see during that session belongs to the chosen deck — zero cards
  from any other deck appear.
- **SC-002**: A learner who runs `meguru review` with no deck scope sees identical due-card
  selection behavior to before this feature existed, in both plain and interactive mode.
- **SC-003**: A learner who mistypes or guesses at a deck identifier on the command line sees a
  clear list of the valid choices instead of a confusing failure, within the same command
  invocation.
- **SC-004**: A learner studying a single deck can always tell, just by looking at the screen,
  that they're in a scoped session and which deck it is.

## Assumptions

- **Deck identifiers**: the CLI flag and the "valid choices" error message use each deck's
  existing stable slug (already defined in the deck registry: `kana-hiragana`, `kana-katakana`,
  `jlpt-n5-kanji`, `jlpt-n5-vocab`) — no new naming scheme is introduced, and the four decks
  already ship with these identifiers today.
- **No new deck-discovery command**: a dedicated command to list decks (e.g. `meguru decks`) is
  out of scope for this slice — the CLI's own error message for an unrecognized deck identifier,
  and the interactive "Study a Deck" list, are the two ways a learner discovers valid choices.
  Adding a standalone list command is a reasonable, separately-scoped follow-up if it turns out to
  be needed.
- **"Study a Deck" always offers every deck**: the picker always lists all four built-in decks
  regardless of whether each one currently has anything due — due counts change moment to moment
  and hiding a deck because it happens to be empty right now would be surprising and would also
  require extra live computation this feature doesn't otherwise need.
- **No persisted "last deck studied" preference**: each session starts unscoped by default (all
  decks) unless a scope is explicitly chosen that time, whether via the CLI flag or the menu — no
  new setting or state is remembered between runs.
- **Scope is whole-session**: once a review session begins (scoped or not), the scope does not
  change mid-session; switching decks means returning to the start menu (or re-running the
  command) and choosing again.

## Out of Scope (this slice)

- A dedicated deck-listing CLI command.
- Scoping to more than one deck at a time (e.g. "kanji and vocab, but not kana").
- Remembering the last-chosen deck across sessions.
- Any change to deck content, the deck registry's shape, or how decks are seeded.
- Switching the active deck scope without returning to the start menu / re-running the command.
