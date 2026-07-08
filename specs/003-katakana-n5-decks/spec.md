# Feature Specification: Katakana + JLPT N5 Kanji/Vocab Built-In Decks

**Feature Branch**: `003-katakana-n5-decks`

**Created**: 2026-07-06

**Status**: Draft

**Input**: User description: "Katakana + N5 kanji/vocab decks (M2, US-1 + US-5): Generalize the
M1 single-deck (hiragana-only) embed+seed pipeline in `internal/deck` to support multiple embedded
decks without duplicating seed/update-in-place logic per deck, then add a built-in katakana deck
and curated starter N5 kanji and N5 vocab decks, so learners get guided kana coverage and built-in
kanji/vocab content organized by JLPT level without hunting for decks. This satisfies PRD US-1 and
US-5. No UI or CLI changes; existing decks/notes/cards schema is reused as-is."

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Complete kana foundation (Priority: P1)

As a new learner running `meguru review` for the first time, I want both hiragana and katakana
built in, so that I build the full kana foundation without hunting for or importing a second deck
myself.

**Why this priority**: PRD US-1 explicitly names "hiragana/katakana lessons" as the entry point
into the whole app. M1 shipped hiragana only; a learner who reaches the end of the hiragana deck
today has no built-in path to katakana, which is half of the kana foundation every other deck
(kanji readings, vocab, sentences) assumes is already known. Without this, the very first learning
milestone the product promises is half-missing.

**Independent Test**: On a completely fresh profile, run `meguru review` and confirm katakana
cards appear as due alongside hiragana cards, with correct expression/reading/meaning content, and
that reviewing them records progress exactly like hiragana cards do today.

**Acceptance Scenarios**:

1. **Given** a fresh install with no prior state, **When** the learner starts their first review
   session, **Then** both hiragana and katakana cards are present and due, each showing its own
   correct character, romaji reading, and meaning.
2. **Given** a learner has been reviewing hiragana cards for a while, **When** a katakana card
   comes due, **Then** it is presented, rated, and rescheduled the same way any other due card is
   — no special-cased flow.
3. **Given** the app is later updated with corrected katakana content, **When** the learner next
   runs the app, **Then** existing katakana cards' expression/reading/meaning update in place and
   the learner's own scheduling progress on those cards (due date, stability, review history) is
   preserved.

### User Story 2 - Built-in JLPT N5 kanji and vocab decks (Priority: P1)

As a learner progressing past kana, I want built-in kanji and vocabulary decks organized by JLPT
level, so that my content matches a recognized progression instead of me having to source or build
my own deck.

**Why this priority**: PRD US-5 names this directly, and it's the next rung after kana in the
product's stated curated progression (kana → kanji → vocab → keigo → sentences). Without it, a
learner who finishes kana has nowhere built-in to go next, which breaks the "guided" promise that
differentiates this product from a blank Anki deck.

**Independent Test**: On a fresh profile, confirm N5 kanji and N5 vocab cards are present, due,
and correctly labeled by deck kind (`kanji`, `vocab`), independent of whether kana decks exist or
have been reviewed.

**Acceptance Scenarios**:

1. **Given** a fresh install, **When** the learner reviews long enough to exhaust kana cards,
   **Then** N5 kanji and N5 vocab cards are already present and become due without any import or
   configuration step.
2. **Given** the N5 kanji and N5 vocab decks, **When** a learner inspects deck content, **Then**
   each entry shows a real, correct JLPT N5-level kanji or vocabulary item with its reading and
   meaning — not placeholder or incorrect data.
3. **Given** future content-correction updates to the N5 kanji/vocab decks, **When** the app
   ships an updated build, **Then** existing entries' content updates in place on next run without
   resetting the learner's scheduling progress on those cards (matching the hiragana precedent from
   M1).

### Edge Cases

- What happens if two built-in decks are seeded on the very same first run (fresh profile, no
  existing `decks` rows for any of them)? All of them must be created in one pass, none skipped,
  none partially created if any single deck's insert fails outright (rather than leaving some
  decks present and others missing).
- What happens on the Nth startup, once all built-in decks already exist at their current content
  version? Nothing changes — no deck, note, card, or scheduling row is touched, and no duplicate
  rows are created for any deck (this must hold per-deck, not just for whichever deck happens to be
  seeded first).
- What happens if only one built-in deck's content version is bumped in a future release while the
  others are unchanged? Only that one deck's notes update in place; the other built-in decks (and
  all cards/scheduling state across every deck) are left untouched.

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: The system MUST include a built-in katakana deck covering the standard katakana
  syllabary, structured the same way (expression/reading/meaning per character) as the existing M1
  hiragana deck.
- **FR-002**: The system MUST include a built-in JLPT N5 kanji deck and a built-in JLPT N5
  vocabulary deck, each containing real, correct N5-level content (character or word, reading, and
  English meaning per entry).
- **FR-003**: The system MUST seed every built-in deck (hiragana, katakana, N5 kanji, N5 vocab) on
  first run, using one shared seed/update-in-place implementation — adding or updating a built-in
  deck's content MUST NOT require writing new per-deck seeding code.
- **FR-004**: The system MUST NOT duplicate any built-in deck's row, notes, or cards on any
  subsequent run, regardless of how many other built-in decks exist or in what order they are
  processed (generalizing the M1 no-duplication guarantee from one deck to many).
- **FR-005**: The system MUST update an individual built-in deck's existing notes' content in
  place, keyed by each note's stable identity within its own deck, when that deck's embedded
  content version increases — without resetting or touching that deck's cards, scheduling state, or
  review history, and without affecting any other built-in deck.
- **FR-006**: The system MUST make katakana, N5 kanji, and N5 vocab cards eligible to appear as due
  cards in a normal review session through the same due-card selection the hiragana deck already
  uses — no deck-specific review flow.
- **FR-007**: The system MUST label each built-in deck with its correct kind (`kana` for katakana,
  `kanji` for the N5 kanji deck, `vocab` for the N5 vocab deck) so future features that filter or
  report by deck kind work correctly without modification.

### Key Entities

- **Built-in deck definition**: The stable identity (slug, display name, kind) and embedded
  content source for one built-in deck. Four exist as of this feature: hiragana, katakana, N5
  kanji, N5 vocab. Adding a fifth in the future means adding one more definition, not new seeding
  logic.
- **Deck note**: One fact (character or word) within a built-in deck — an expression, a reading,
  and a meaning — matching the existing M1 note shape, reused unchanged for every deck kind in this
  feature.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: On a completely fresh profile, a review session presents cards from all four
  built-in decks (hiragana, katakana, N5 kanji, N5 vocab) as due, with zero manual import or setup
  steps.
- **SC-002**: Running the seed step any number of times in a row never changes the total row count
  in `decks`, `notes`, or `cards` after the first run — for all four built-in decks combined, and
  for each individually.
- **SC-003**: 100% of N5 kanji and N5 vocab entries shipped in this feature have a non-empty,
  correct expression, reading, and meaning (verified by construction against well-established
  public N5 reference material, not sourced from user data).
- **SC-004**: A future content-version bump to exactly one built-in deck updates only that deck's
  notes; every other built-in deck's notes, and every deck's cards/scheduling state, remain
  byte-for-byte unchanged (measured via unit tests asserting untouched `updated_at`/`due_at`/
  `stability` on unaffected rows).

## Assumptions

- **Curated starter subset, not full lists**: The N5 kanji and N5 vocab decks in this feature are
  modest curated starter subsets (roughly 20-40 entries each), not the complete ~100-entry N5
  kanji list or the full N5 vocabulary list. This keeps the slice bounded per Simplicity First;
  expanding either deck to full N5 coverage is explicit future work, tracked as a follow-up
  content update (a content-version bump to the same deck, using the same update-in-place
  mechanism this feature generalizes — no new pipeline work required).
- **Curated public reference content, not user data**: N5 kanji/vocab entries are constructed from
  well-established public JLPT N5 reference knowledge (readings and meanings), not collected from
  or generated on behalf of any user — this is public linguistic reference data, consistent with
  CON-3 (no secrets or real user data in fixtures).
- **No schema changes**: The existing `decks`/`notes`/`cards` schema from
  `docs/product/TECH_STACK.md` §3 already supports `kind IN ('kana','kanji','vocab',...)` and an
  arbitrary number of deck rows; this feature adds data and generalizes existing Go seeding code,
  it does not migrate the schema.
- **`internal/review` needs no changes**: `internal/review/service.go`'s due-card query already
  joins across all decks generically (no hiragana-specific filtering), so newly seeded decks become
  reviewable purely by being seeded — confirmed by reading the existing implementation before
  writing this spec.
