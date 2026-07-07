# Feature Specification: Real FSRS Scheduling

**Feature Branch**: `002-fsrs-scheduler`

**Created**: 2026-07-06

**Status**: Draft

**Input**: User description: "Real FSRS scheduler (M2, US-4): Replace the naive fixed-interval placeholder scheduler with a real FSRS (Free Spaced Repetition Scheduler) implementation, so that review scheduling reflects each learner's actual memory stability/difficulty (Again/Hard/Good/Easy ratings) instead of fixed intervals. This satisfies PRD US-4. No UI or CLI changes."

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Honest, adaptive review scheduling (Priority: P1)

As a learner using `meguru review`, when I rate a card Again, Hard, Good, or Easy, I want the
next due date to reflect how well I actually know that specific card — based on its own review
history — rather than a fixed interval that's the same for every card regardless of how many
times I've seen it or how consistently I've gotten it right.

**Why this priority**: This is the entire value proposition of an SRS tool. A fixed-interval
scheduler (the current placeholder) treats a card I've reviewed 20 times with perfect recall
exactly the same as a card I just saw for the first time — it cannot get more efficient as my
memory strengthens, nor catch up when I'm struggling with a specific item. Without honest
scheduling, every other planned feature (dashboards, retention stats, more content) is built on
a foundation that actively works against the stated goal of "review load stays efficient and
honest" (PRD US-4).

**Independent Test**: Seed a card, review it repeatedly over simulated time with consistent
"Good" ratings, and confirm the interval between reviews grows longer each time (rather than
staying fixed). Separately, review a card with "Again" and confirm it comes back sooner than one
rated "Good" or "Easy" under the same starting conditions. Deliverable value (adaptive
scheduling) is observable without any other feature existing.

**Acceptance Scenarios**:

1. **Given** a brand-new card with no review history, **When** the learner rates it, **Then**
   the system schedules its next due date based on that rating and records the card's updated
   memory state (so a later review can build on it).
2. **Given** a card that has been reviewed several times with consistently high ratings (Good or
   Easy), **When** the learner rates it again with a high rating, **Then** the interval until its
   next due date is longer than it was after the previous review (scheduling gets more efficient
   as memory strengthens).
3. **Given** a card the learner has been rating well, **When** the learner rates it Again (i.e.,
   they forgot it), **Then** the card's next due date is much sooner than it would have been on
   a successful rating, and the system records that this card was forgotten.
4. **Given** two identical fresh cards, **When** one is rated Again and the other is rated Easy,
   **Then** the Again card's next due date is sooner than the Easy card's next due date.
5. **Given** a completed review, **When** the learner inspects their review history later, **Then**
   each past review records enough information (what the memory state was before and after, how
   many days had elapsed, how many days until the next review) to reconstruct how the schedule
   decision was made.

### Edge Cases

- What happens on a card's very first-ever review (no prior review to compare against)? The
  system must still produce a valid next-due date and initial memory state without needing a
  "previous" state to diff against.
- What happens if a learner rates a card Again repeatedly, many times in a row? The system must
  keep producing a valid (non-crashing, forward-moving) next-due date every time, and must
  distinguish "still learning this new card" from "lost a card I used to know" when counting how
  many times a card has been forgotten.
- What happens to a learner's already-scheduled cards (seeded under the old, fixed-interval
  behavior) once this change ships? Existing due dates remain valid and are honored as-is; the
  new scheduling logic only takes effect starting with each card's next rating.

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: The system MUST compute each card's next due date from that specific card's own
  review history (accumulated memory strength) and the rating just given, not from a fixed
  interval that ignores review history.
- **FR-002**: The system MUST support all four existing ratings (Again, Hard, Good, Easy) with
  Again producing the soonest next-due date and Easy producing the furthest, for otherwise
  identical cards.
- **FR-003**: The system MUST make review intervals lengthen over successive successful
  (non-Again) reviews of the same card, reflecting strengthening memory.
- **FR-004**: The system MUST track, per card, enough memory-state information (at minimum: a
  notion of retention strength and item difficulty) to make FR-001–FR-003 possible, and this
  state MUST persist between review sessions.
- **FR-005**: The system MUST record, for every submitted rating, a permanent log entry capturing
  the card's memory state immediately before the rating, the rating given, how many days had
  elapsed since the prior review (or none, if first review), and how many days until the newly
  scheduled due date.
- **FR-006**: The system MUST correctly schedule a card that has never been reviewed before
  (first-ever rating), producing both a valid next-due date and an initial memory state.
- **FR-007**: The system MUST distinguish, when tallying how many times a card has been
  "forgotten," between an Again rating on a card still being newly learned and an Again rating on
  a card that had already reached a stable, well-known state — only the latter counts as a
  memory lapse.
- **FR-008**: The system MUST NOT require any change to how a learner interacts with `meguru
  review` (same four ratings, same command, same flow) — this is purely a change to how the next
  due date and memory state are computed internally.
- **FR-009**: The system MUST NOT require any change to previously stored cards or review
  history — cards already scheduled under prior behavior continue to have valid due dates, and
  the new scheduling logic applies starting from each card's next rating.
- **FR-010**: The system MUST use one consistent, deterministic scheduling policy for every
  learner in this release (no per-user tuning yet) — personalizing the policy from a learner's
  own history is out of scope for this feature.

### Key Entities

- **Card memory state**: Per-card record of how well-retained an item currently is, expressed
  through a retention-strength value and a difficulty value, plus counts of how many times the
  card has been reviewed and how many times it has been forgotten. Drives when the card next
  becomes due.
- **Review record**: One immutable entry per submitted rating, capturing the rating given, the
  card's memory state just before the rating, how much time had passed since the previous review,
  and how far out the new due date was scheduled. Forms the history a future feature could use to
  personalize scheduling per learner.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: For any card reviewed at least twice with consistently high ratings, the gap until
  its next due date is strictly greater after the second high rating than it was after the first.
- **SC-002**: Given two otherwise-identical cards, the one rated Again always comes due before the
  one rated Easy — in 100% of cases, not just typically.
- **SC-003**: A learner reviewing a brand-new card with no history receives a valid next-due date
  and never encounters an error or crash caused by the card lacking review history.
- **SC-004**: 100% of submitted ratings produce a permanent review-history record containing the
  before-state, elapsed time, and scheduled interval — sufficient for a future feature to
  recompute or audit the scheduling decision without re-deriving it from scratch.
- **SC-005**: Learners notice no change in how they interact with reviews (same ratings, same
  command) — only the resulting schedule feels smarter over time.

## Assumptions

- This feature introduces one fixed, shared scheduling policy for all learners; letting the
  policy adapt to an individual learner's own accuracy/speed history is explicitly deferred to a
  later feature (per `docs/product/TECH_STACK.md` §4, tuning is "post-MVP, not a schema
  migration").
- The underlying data already has a place to store per-card memory state and pre-rating snapshots
  in review history (established in the M1 walking-skeleton schema specifically to support this
  feature) — this feature populates and uses that existing storage rather than introducing new
  storage concepts.
- "Forgotten" (lapse) is defined as an Again rating on a card that had already reached a stable,
  well-known state — not any Again rating whatsoever — matching how spaced-repetition systems
  conventionally define a lapse.
- No dashboard, stats view, or other UI surfaces this feature's richer memory-state data yet;
  that consumption is future scope. This feature only ensures the data is computed and recorded
  correctly.
