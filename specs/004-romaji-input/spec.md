# Feature Specification: Romaji Answer Input

**Feature Branch**: `004-romaji-input`

**Created**: 2026-07-06

**Status**: Draft

**Input**: User description: "Romaji-to-kana answer input (M2, US-3): let a learner type an
answer in romaji that auto-converts to kana, auto-checked against a card's expected reading,
before the existing Again/Hard/Good/Easy self-grade step — so a learner never needs to configure
a system IME. This satisfies PRD US-3 and the 'User types answer' / 'Auto-check match?' steps of
the Review Session Flow. The existing self-grade step is not replaced."

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Type an answer instead of just self-grading (Priority: P1)

As a learner running `meguru review`, when a card is presented I want to type my answer in plain
ASCII romaji (no system IME configuration) and have the app tell me whether it matches the
card's expected reading, so that I get an objective correctness signal before I self-grade —
instead of jumping straight to "Again/Hard/Good/Easy" on my own unverified recollection.

**Why this priority**: This is the entire value proposition of US-3. Self-grading alone (the M1
behavior) asks a learner to judge their own correctness from memory, which is generous to
wishful thinking. A typed, auto-checked answer is what makes the review loop "efficient and
honest" (PRD US-4's framing, but the honesty starts here, at the answer step) and is what lets
Meguru work with zero system-level Japanese input setup — the explicit pain point in US-3's "so
that" clause.

**Independent Test**: Present a due card, type a romaji string that correctly spells the card's
reading, and confirm the app reports a match. Separately, type a romaji string that does not
match, and confirm the app reports no match and still reveals the correct reading. Both cases
must still reach the existing Again/Hard/Good/Easy rating step afterward. Deliverable value
(objective auto-check) is observable without any other M2 feature existing.

**Acceptance Scenarios**:

1. **Given** a due card is presented, **When** the learner types a romaji string that converts
   to (or otherwise matches) the card's expected reading and submits it, **Then** the system
   reports the answer as correct before the rating step.
2. **Given** a due card is presented, **When** the learner types a romaji string that does not
   match the card's expected reading, **Then** the system reports the answer as incorrect,
   reveals the expected reading, and still proceeds to the rating step.
3. **Given** any typed-answer outcome (match or no match), **When** the auto-check completes,
   **Then** the learner still rates the card Again/Hard/Good/Easy exactly as in M1 — this feature
   adds a step before rating, it does not remove or auto-select a rating.
4. **Given** a learner types romaji containing a double consonant (e.g. "kk") or an "n" before a
   consonant, **When** the app converts the input to kana for comparison, **Then** the conversion
   follows standard Hepburn-style rules (small tsu for the doubled consonant; "ん" for "n" before
   a consonant) rather than silently dropping or mis-rendering the character.

### Edge Cases

- What happens if the learner submits an empty line as their answer? The system must treat it as
  a (non-matching) answer and still proceed through reveal and rating — it must not crash, hang,
  or skip the rating step.
- What happens if the learner's romaji is ambiguous (e.g. "nyo" could be the digraph にょ or "n"
  + "yo" depending on intent)? The system resolves ambiguity using the standard convention
  (digraph wins unless an apostrophe explicitly separates "n" from a following "y", e.g.
  "kin'youbi") — documented behavior, not an error.
- What happens on a card whose stored reading is itself plain romaji rather than kana (true today
  of the M1 built-in hiragana deck, where a kana card's "reading" is its own romanized
  pronunciation, e.g. expression "か" / reading "ka")? The auto-check must still work correctly
  for this case (see Assumptions).

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: The system MUST provide a pure, storage-independent romaji-to-hiragana conversion
  covering the standard hiragana syllabary (all five vowels, the k/s/t/n/h/m/y/r/w rows, their
  dakuten/handakuten forms g/z/d/b/p, and the yōon digraphs such as kya/sha/cha/nya/hya/mya/
  rya/gya/ja/bya/pya), common alternate spellings (si/shi, ti/chi, tu/tsu, di/zi/ji), sokuon
  (doubled consonants producing a small tsu), and "n" disambiguation (including the `n'`
  separator).
- **FR-002**: After a due card is presented and before the Again/Hard/Good/Easy rating step, the
  system MUST prompt the learner for a typed romaji answer.
- **FR-003**: The system MUST auto-check the typed answer against the presented card's expected
  reading and report whether it matched, before the rating step.
- **FR-004**: The system MUST reveal the card's expected reading and meaning regardless of
  whether the typed answer matched, so the learner can self-grade with full information (matching
  M1's existing reveal behavior, now sequenced after the auto-check).
- **FR-005**: The system MUST NOT remove, replace, or auto-select the existing Again/Hard/Good/
  Easy rating step — auto-check is strictly an assist that runs before it (per the PRD's Review
  Session Flow: "match" and "no match" both lead into "Rate: Again/Hard/Good/Easy").
- **FR-006**: The system MUST NOT crash, hang, or skip remaining steps if the learner submits an
  empty or unrecognized-character answer string.
- **FR-007**: The romaji-to-kana conversion MUST be implemented with no dependency on storage,
  TUI, or CLI packages, so it is testable in complete isolation from the rest of the app.

### Key Entities

- **Typed answer**: The raw romaji string a learner enters for one card, plus its converted-kana
  form and whether it matched that card's expected reading. Not persisted — it exists only for
  the duration of one review interaction (the persisted record remains the existing rating in
  `review_log`, unchanged by this feature).

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: A learner can complete a full review interaction (see card, type an answer, get an
  auto-check result, see the reveal, submit a rating) using only plain ASCII input — no system
  IME configuration required, satisfying US-3's stated goal directly.
- **SC-002**: 100% of the standard hiragana syllabary (all five vowels and the nine consonant
  rows, dakuten/handakuten forms, and yōon digraphs) converts to its correct hiragana character(s)
  via automated test coverage.
- **SC-003**: Every review interaction that reaches the auto-check step also reaches the rating
  step afterward, regardless of whether the typed answer matched — verified by automated test,
  not just manual inspection.
- **SC-004**: Learners see no change to the rating step itself (same four ratings, same meaning) —
  only a new step is added before it.

## Assumptions

- **Primary interactive surface for this slice is `internal/plain`** (the linear, `--plain`/
  non-TTY renderer). Full interactive text-input support in the Bubble Tea v2 TUI
  (`internal/tui`) is scoped OUT of this slice as a documented fast-follow: the TUI's key-handling
  today only dispatches single logical keys (reveal, rate); accepting free-form typed text would
  require hand-rolling character-by-character buffering, cursor/backspace handling, and new
  render states without an existing text-input component dependency in `go.mod` — a materially
  larger and riskier change than this slice's scope justifies. `internal/plain` already reads
  full answer lines via `bufio.Scanner` and needed no new capability to support this, making it
  the correct primary integration point for one bounded slice.
- **Reading-field convention**: the M1 built-in hiragana deck stores a card's `reading` field as
  plain romaji (its own pronunciation, e.g. "ka" for expression "か") rather than kana, since a
  kana glyph's "reading" is its romanization. Future kanji/vocab decks are expected to store
  `reading` as kana (e.g. "たべる"). This feature's auto-check is defined to work correctly under
  both conventions (see `contracts/answer-check.md`) rather than assuming one and requiring a
  content migration.
- Long-vowel contraction (e.g. treating "ou"/"oo" as a single lengthened vowel rather than two
  separate kana) is out of scope — romaji is converted mora-by-mora, which already produces
  correct hiragana spelling for the vast majority of words (hiragana orthography itself usually
  spells long vowels as two characters, e.g. おう), so this is not a functional gap for the
  hiragana/vocab content this milestone targets.
- No new persisted schema, table, or column is needed (see `data-model.md`) — the typed answer
  and its check result are ephemeral, scoped to one review interaction.
