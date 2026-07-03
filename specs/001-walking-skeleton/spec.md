# Feature Specification: Walking Skeleton (M1)

**Feature Branch**: `001-walking-skeleton`

**Created**: 2026-07-02

**Status**: Draft

**Input**: User description: "Walking skeleton for Meguru M1: a Bubble Tea v2 TUI + Cobra CLI + SQLite schema/migrations + an embedded hiragana deck that syncs on first run + a minimal `meguru review` loop using a naive interval-bump scheduler (explicitly NOT FSRS) that writes to `review_log` and updates `srs_state`. Requirements: a `--plain` fallback rendering mode and respect for `NO_COLOR`; DB files created with 0600 permissions and their directory with 0700; CI must run on 3 OSes (ubuntu/macos/windows) and include a network-denied test proving the core loop needs zero network access. Naive scheduler rule (placeholder until go-fsrs lands in M2): Again -> due now+1 minute, Hard -> due now+1 day, Good -> due now+3 days, Easy -> due now+7 days. Out of Scope: FSRS itself, katakana/kanji/vocab decks beyond the embedded hiragana deck, and the AI provider layer."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - First run seeds a deck and shows a due card (Priority: P1)

A user installs Meguru and runs the review command for the first time on a machine with no
existing data. The app creates its local storage, loads the built-in hiragana deck, and
immediately presents the first due card for review — with no setup step, no account, and no
network access required.

**Why this priority**: This is the entire point of the walking skeleton — it proves the
riskiest plumbing (storage, embedded content, offline operation) end-to-end. Without this,
there is no product to dogfood.

**Independent Test**: On a clean machine/profile, run the review command once. Verify local
storage now exists, contains the hiragana deck's cards, and a card is shown for study —
without any network call being made.

**Acceptance Scenarios**:

1. **Given** no prior local data exists, **When** the user starts a review session, **Then**
   the app creates its local database and directory, loads the hiragana deck into it, and shows
   a due card.
2. **Given** local data already exists from a previous run, **When** the user starts a review
   session again, **Then** the app does not re-seed or duplicate deck content, and shows the
   next due card (or a "nothing due" message if none are due).

---

### User Story 2 - Answer a card and have progress persist (Priority: P1)

A user studies a due hiragana card, submits a self-graded rating (e.g. Again/Hard/Good/Easy),
and the app records that this card was reviewed and schedules it to reappear later based on the
rating given. Progress is never lost between sessions.

**Why this priority**: Recording review outcomes and rescheduling cards is the minimum loop
that makes this a spaced-repetition tool rather than a static flashcard viewer. Without
persisted progress, nothing has been proven.

**Independent Test**: Answer one due card with a given rating, exit, and restart the app.
Verify a record of that review exists, the card's next-due time has moved into the future by an
amount consistent with the rating given, and the card is no longer shown as due until that time
passes.

**Acceptance Scenarios**:

1. **Given** a due card is displayed, **When** the user submits a rating, **Then** a review
   record is created for that attempt and the card's next-due time is updated according to the
   rating.
2. **Given** a card was just rated "Again", **When** the user checks shortly after, **Then**
   the card becomes due again almost immediately (short delay), not after days.
3. **Given** a card was just rated "Easy", **When** the user checks the same day, **Then** the
   card is not shown as due again (delay measured in days).
4. **Given** all due cards have been answered in a session, **When** the user continues,
   **Then** the app clearly communicates there is nothing left to review right now, without
   crashing or looping.

---

### User Story 3 - Usable without color or a fancy terminal (Priority: P2)

A user running Meguru in a constrained terminal (no color support, accessibility tooling, CI
log capture, or a terminal that doesn't render the interactive UI well) can still complete a
full review session using a plain, linear text mode.

**Why this priority**: Terminal environments vary widely (CI runners, screen readers, dumb
terminals, `NO_COLOR`-conforming setups). A plain fallback ensures the tool remains usable and
testable everywhere, and is a stated non-functional requirement, not a nice-to-have.

**Independent Test**: Launch the app in the explicit plain mode (and separately with the
`NO_COLOR` convention active) and complete a full review of a due card using only plain,
sequential text output — no interactive redraws, no color/style escape codes.

**Acceptance Scenarios**:

1. **Given** the app is launched in plain mode, **When** a review session runs, **Then** all
   output is linear text with no escape sequences, and the user can still see a card and submit
   a rating.
2. **Given** the `NO_COLOR` convention is active, **When** the app runs in its normal
   (non-plain) mode, **Then** no color codes are emitted, though interactive layout may still be
   used.

---

### User Story 4 - Local data stays private by construction (Priority: P2)

A security- or privacy-conscious user (or an auditor) can verify that Meguru's local database
and its containing directory are created with restrictive file permissions from the very first
run, with no configuration required to get this protection.

**Why this priority**: This directly enforces binding constitution rules (local-only data,
restrictive file permissions) starting from the very first artifact the app ever writes. It is
cheap to get right at M1 and expensive to retrofit later.

**Independent Test**: After the first run on a machine, inspect the created database file and
its containing directory's permissions and confirm they are locked down to the owning user
only, with no other user or group access.

**Acceptance Scenarios**:

1. **Given** the app runs for the first time, **When** its local storage is created, **Then**
   the database file is created readable/writable only by the owning user, and its containing
   directory is accessible only by the owning user.
2. **Given** an existing database or directory has looser permissions than expected (e.g. due
   to being copied from elsewhere), **When** the app starts, **Then** it warns the user and
   corrects the permissions rather than silently proceeding.

---

### Edge Cases

- What happens when the review command is run and no cards are currently due? The app MUST say
  so clearly and exit/return cleanly, without crashing, looping, or fabricating a card.
- What happens when the app is upgraded and the embedded deck's content changes (e.g. a typo
  fix)? Existing user progress on those cards MUST be preserved; only the deck's own content
  fields are updated, without creating duplicate cards or resetting scheduling state.
- What happens if the process is interrupted (killed, crashes) mid-review, after a card is shown
  but before a rating is submitted? On the next run, that card MUST still be presented as due —
  no partial/corrupt review record may be left behind.
- What happens on a terminal that does not support the interactive UI at all (very old/dumb
  terminal)? The user MUST be able to fall back to the plain text mode to complete a session.
- What happens when local storage already exists but was created by a version of the app with
  an older schema? The app MUST bring it up to the current schema automatically on startup
  without data loss.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST, on first run, create its local storage (database and containing
  directory) automatically without requiring any manual setup step from the user.
- **FR-002**: System MUST seed its local storage with a built-in Japanese hiragana study deck on
  first run, with no other deck content included at this stage.
- **FR-003**: System MUST NOT duplicate or re-seed deck content on subsequent runs once it has
  already been loaded once.
- **FR-004**: System MUST, when the built-in hiragana deck's content is updated in a later
  release, update existing cards' content in place on next run rather than creating duplicates,
  while preserving each card's existing study/scheduling progress.
- **FR-005**: System MUST present the user with a due card when one exists, and clearly
  communicate when none are currently due (rather than showing nothing or erroring).
- **FR-006**: Users MUST be able to submit a self-graded rating for a displayed card from a
  fixed, small set of outcomes representing "did not know it" through "knew it easily."
- **FR-007**: System MUST record every submitted rating as a permanent review entry, including
  which card, what rating was given, and when.
- **FR-008**: System MUST reschedule a card's next-due time after each rating, using a
  documented, deterministic rule where a worse rating results in a much sooner next-due time
  than a better rating (specifically: worst rating ⇒ due again within minutes; best rating ⇒ due
  again after roughly a week; two intermediate ratings ⇒ due again after roughly one day and
  roughly three days, respectively). This rule is an explicitly temporary placeholder scheduling
  approach for this milestone and is expected to be replaced wholesale in a later milestone —
  it MUST NOT be treated as the product's real scheduling algorithm, and no attempt should be
  made to make it more sophisticated than this simple interval rule.
- **FR-009**: System MUST function fully — creating storage, seeding the deck, presenting
  cards, accepting ratings, recording history, rescheduling — with no network access at any
  point during this core loop.
- **FR-010**: System MUST offer an explicit plain/linear output mode producing no interactive
  redraws and no color or style escape sequences, sufficient to complete a full review session.
- **FR-011**: System MUST suppress all color/style escape sequences whenever the environment
  signals a no-color preference, independent of whether plain mode is explicitly requested.
- **FR-012**: System MUST create its local database file accessible only to the owning user (no
  group/other access), and its containing directory accessible (read/write/execute) only to the
  owning user.
- **FR-013**: System MUST detect, on startup, if its existing database file or directory has
  broader-than-expected permissions, and MUST warn the user and correct the permissions.
- **FR-014**: System MUST bring an existing local storage schema up to the current version
  automatically on startup, without requiring manual intervention and without data loss.
- **FR-015**: System MUST leave no partial or corrupted review record if interrupted between
  showing a card and receiving its rating; an interrupted card MUST simply remain due.
- **FR-016**: System's build and automated test process MUST verify correct behavior on three
  major desktop operating system families (Linux, macOS, Windows).
- **FR-017**: System's automated test process MUST include at least one test that runs the core
  review loop (storage creation, deck seeding, review, rescheduling) in an environment where all
  network access is actively blocked, and MUST fail that test if any network egress is
  attempted — serving as ongoing proof that the core loop has no network dependency.

**Out of Scope for this feature**:

- The real scheduling algorithm (spaced-repetition engine). FR-008's rule is a disposable
  placeholder only; algorithmic sophistication, parameter tuning, or scientific scheduling
  accuracy are explicitly deferred to a later milestone.
- Any deck content beyond the single built-in hiragana deck — katakana, kanji, vocabulary,
  sentence, or any other deck type are not part of this feature.
- Any AI-provider-backed feature (example generation, error explanation, conversation practice,
  mnemonics, batch augmentation, or any network call to an AI provider of any kind).
- Import/export of decks, statistics dashboards, and any account/sync/cloud feature — none of
  these exist at this stage.

### Key Entities *(include if feature involves data)*

- **Deck**: A named collection of study content; for this feature, exactly one built-in deck
  (hiragana) exists, tagged with a content version so future content updates can be detected and
  applied without duplication.
- **Card**: A single study item drawn from a deck (e.g. one hiragana character and its reading),
  the unit the user reviews and rates.
- **Study/Scheduling State**: Per-card state tracking when it next becomes due and how many
  times it has been reviewed; updated after every rating.
- **Review Record**: An permanent, append-only log entry of one review attempt — which card, the
  rating given, and when it happened — that is never edited or deleted by normal operation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user on a freshly wiped machine can go from first launch to seeing their first
  due study card in under 5 seconds, with zero manual setup steps.
- **SC-002**: 100% of submitted ratings result in a permanent, retrievable review record and a
  correctly updated next-due time, verified across at least one full session per supported
  operating system.
- **SC-003**: A full review session (from launch to completing all due cards) can be completed
  entirely in the plain/linear output mode, with zero color or interactive escape sequences
  observed in the output.
- **SC-004**: Immediately after first run on every supported operating system, the local
  database file and its directory are confirmed restricted to the owning user only, with no
  exceptions.
- **SC-005**: The automated core-loop test suite passes with all outbound network access
  blocked, with zero network egress attempts observed, on at least one supported operating
  system.
- **SC-006**: Restarting the app after an interrupted (killed mid-review) session never loses or
  corrupts prior review history, and the interrupted card is presented as due again.

## Assumptions

- "Three operating systems" means the three major desktop OS families commonly used by
  terminal-application users: Linux, macOS, and Windows.
- The built-in hiragana deck's exact character set and card count are a content decision made
  during implementation planning, not this specification; the requirement here is only that a
  complete, self-contained hiragana deck ships embedded in the application.
- The four-tier self-grading scale (Again/Hard/Good/Easy) is the standard spaced-repetition
  rating convention; this feature only requires that ratings exist and drive different
  reschedule outcomes, not that this exact terminology is user-facing.
- "Plain mode" and "no-color" are related but distinct: plain mode also disables interactive
  redraws, while no-color only suppresses color/style, and both may be requested independently
  or together.
- Users of this milestone are the developer(s) dogfooding the app and early technical testers,
  not yet the general public; this does not relax any of the above requirements, but it does
  mean no onboarding/tutorial UX is in scope.
