# Tasks: Katakana + JLPT N5 Kanji/Vocab Built-In Decks

**Input**: Design documents from `/specs/003-katakana-n5-decks/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/deck-registry.md,
quickstart.md

**Tests**: Included. This project's existing testing posture (unit + integration round-trips) is a
binding part of `docs/product/TECH_STACK.md` §7's testing strategy, not optional TDD scaffolding —
and FR-003/FR-004/FR-005/SC-002/SC-004 require automated verification.

**Organization**: Two user stories (spec.md: US1 kana completion, US2 N5 kanji/vocab), both P1,
sharing one prerequisite generalization of the seed pipeline (Foundational phase) since neither
deck can land cleanly without it.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, or none for Setup/Foundational/
  Polish)
- Paths are relative to the repo root, per plan.md's Project Structure (single Go module)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: No new dependencies or tooling are needed (research.md) — this phase is a no-op
placeholder to match the standard template shape; work begins directly at Foundational.

- [X] T001 Confirm no new dependency is required: `internal/deck` already imports only stdlib
      `embed`/`encoding/json`/`database/sql`/`fmt`/`context`/`time` (plan.md Technical Context).
      No `go.mod` change.

**Checkpoint**: Confirmed — proceed directly to Foundational.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Generalize the M1 hiragana-only embed+seed pipeline into a `Definition` registry
before either new deck's content can be added, per FR-003/FR-004's "one shared implementation"
requirement.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T002 In `internal/deck/embed.go`, introduce `Definition{Slug, Name, Kind string; raw func()
      []byte}` with a `Content() (Content, error)` method, replacing the hardcoded
      `HiraganaSlug`/`Hiragana()`-only shape, per `contracts/deck-registry.md`'s Public Surface.
      Keep `Note`/`Content` types unchanged (data-model.md).
- [X] T003 In `internal/deck/embed.go`, add a `builtinDecks []Definition` package var containing
      exactly the existing hiragana entry (`{HiraganaSlug, "Hiragana", "kana", ...}`) and an
      exported `BuiltinDecks() []Definition` accessor, plus a `lookupBuiltin(slug string)
      (Content, error)` helper backing a kept `Hiragana()` accessor for existing callers/tests
      (depends on T002).
- [X] T004 In `internal/deck/seed.go`, change `Seed` to loop over `BuiltinDecks()`, and
      parameterize `seedDeck` (new, replacing `Seed`'s old inline lookup body)/`seedFresh` by
      `Definition` instead of the hiragana-specific constants they closed over; `updateInPlace`/
      `insertNote` are unchanged (already `deckID`-scoped, no per-deck specialization needed) per
      `data-model.md`'s "Seed flow" section (depends on T003).
- [X] T005 Rewrite `internal/deck/seed_test.go`'s low-level unit tests (`seedFresh`/
      `updateInPlace`/`insertNote` error-path and content-version-bump tests) to call the new
      `seedDeck` directly against a synthetic `testDef()` `Definition` and hand-built `Content`
      values, decoupling them from `BuiltinDecks()`'s registry order and from real embedded deck
      content (depends on T004).
- [X] T006 [P] Add `TestBuiltinDecks_AllDistinctAndValid` to `internal/deck/seed_test.go`/
      `embed_test.go`: assert `BuiltinDecks()` has no duplicate slugs, every `Kind` is one of the
      `decks.kind` CHECK values, every `Definition.Content()` parses without error, and no deck's
      notes contain a duplicate `Expression` (contracts/deck-registry.md Preconditions) (depends
      on T003).

**Checkpoint**: `internal/deck`'s seed/update-in-place logic is deck-agnostic and covered by
tests that don't hardcode hiragana-only assumptions. Ready to add new decks as pure data.

---

## Phase 3: User Story 1 - Complete kana foundation (Priority: P1) 🎯 MVP

**Goal**: A built-in katakana deck exists alongside hiragana, seeded and reviewable the same way,
satisfying PRD US-1's "hiragana/katakana lessons feeding into SRS."

**Independent Test**: On a fresh profile, confirm katakana cards appear as due alongside hiragana
cards with correct content, and that reviewing one records progress like any other due card.

### Implementation for User Story 1

- [X] T007 [US1] Create `internal/deck/katakana.json`: the standard 46-character katakana
      syllabary, `content_version: 1`, structured identically to `hiragana.json` (expression =
      katakana character, reading = romaji, meaning = romaji repeated, matching M1's convention
      for kana decks).
- [X] T008 [US1] In `internal/deck/embed.go`, add `//go:embed katakana.json` +
      `KatakanaSlug = "kana-katakana"` + a `Katakana()` accessor, and append `{KatakanaSlug,
      "Katakana", "kana", ...}` to `builtinDecks` (depends on T003, T007).
- [X] T009 [P] [US1] Add `TestKatakana_ParsesEmbeddedJSON` to `internal/deck/seed_test.go`:
      assert `content_version == 1`, exactly 46 notes, first note's expression/reading match the
      expected katakana character/romaji (depends on T008).

**Checkpoint**: User Story 1 is independently functional — katakana is a fully seeded, reviewable
built-in deck alongside hiragana.

---

## Phase 4: User Story 2 - Built-in JLPT N5 kanji and vocab decks (Priority: P1)

**Goal**: Built-in N5 kanji and N5 vocab decks exist, seeded and reviewable, satisfying PRD US-5's
"built-in kanji and vocab decks organized by JLPT level."

**Independent Test**: On a fresh profile, confirm N5 kanji and N5 vocab cards are present, due,
and correctly labeled by deck kind (`kanji`, `vocab`), independent of kana deck state.

### Implementation for User Story 2

- [X] T010 [P] [US2] Create `internal/deck/jlpt_n5_kanji.json`: a curated 30-entry N5 kanji
      starter subset (numbers 1-10, common time/date/size kanji), `content_version: 1`, each entry
      a real, correct N5-level kanji with romaji reading and English meaning (spec.md Assumptions:
      curated starter subset, not the full ~100-entry list).
- [X] T011 [P] [US2] Create `internal/deck/jlpt_n5_vocab.json`: a curated 30-entry N5 vocabulary
      starter subset (common nouns, verbs, adjectives), `content_version: 1`, each entry a real,
      correct N5-level word with romaji reading and English meaning.
- [X] T012 [US2] In `internal/deck/embed.go`, add `//go:embed jlpt_n5_kanji.json` +
      `//go:embed jlpt_n5_vocab.json` + `JLPTN5KanjiSlug`/`JLPTN5VocabSlug` constants +
      `JLPTN5Kanji()`/`JLPTN5Vocab()` accessors, and append both `{..., "kanji", ...}`/
      `{..., "vocab", ...}` entries to `builtinDecks` (depends on T003, T010, T011).
- [X] T013 [P] [US2] Add `TestJLPTN5Kanji_ParsesEmbeddedJSON` and
      `TestJLPTN5Vocab_ParsesEmbeddedJSON` to `internal/deck/seed_test.go`: assert
      `content_version == 1`, entry count in [20, 40], and every entry has non-empty
      expression/reading/meaning (SC-003) (depends on T012).

**Checkpoint**: User Story 2 is independently functional — N5 kanji and N5 vocab are fully seeded,
reviewable built-in decks, correctly labeled by kind.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Confirm the generalized registry behaves correctly across all four decks together,
and that nothing outside `internal/deck`/`tests/integration` regressed.

- [X] T014 [US1][US2] Add `TestSeed_SeedsAllBuiltinDecks` and
      `TestSeed_SecondRunDoesNotDuplicateAnyBuiltinDeck` to `internal/deck/seed_test.go`: seed a
      fresh DB via the public `Seed`, assert deck/note/card/srs_state counts match
      `BuiltinDecks()` exactly per-deck, then reseed and assert no duplication (FR-004, SC-002)
      (depends on T006, T008, T012).
- [X] T015 Generalize `tests/integration/seed_test.go`'s `TestSeed_DoesNotDuplicateOnSecondRun` to
      derive expected note/card counts from `deck.BuiltinDecks()` instead of hardcoding hiragana's
      count, so the no-duplication guarantee is proven across every built-in deck (depends on
      T008, T012).
- [X] T016 Run `go test ./tests/integration/... -run TestFirstRun_DueCardImmediatelyAfterSeed -v`
      as a sanity check — confirm it still passes unmodified, proving `internal/review` needed no
      changes for the new decks to become reviewable (depends on T008, T012).
- [X] T017 Run `go build ./...`, `go vet ./...`, and `go test ./...` (full-suite regression) and
      confirm all packages are green, including `internal/tui`/`internal/plain`/`internal/cli`
      unmodified.
- [X] T018 Run the `quickstart.md` validation guide (§§1-5) end-to-end, including the manual
      `bin/meguru review --plain` + `sqlite3` check in §4 confirming all four built-in decks
      surface as due cards and get logged under their correct `kind` in `review_log`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — confirmed no-op, start immediately.
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS both user stories — the registry
  generalization (T002-T004) must land before either new deck can be added as a `Definition`.
- **User Story 1 (Phase 3)**: Depends on Foundational (T003). Independent of User Story 2 — can
  be implemented and tested in isolation.
- **User Story 2 (Phase 4)**: Depends on Foundational (T003). Independent of User Story 1 — can
  be implemented and tested in isolation.
- **Polish (Phase 5)**: Depends on both user stories being complete (T008, T012), since T014-T018
  assert behavior across all four built-in decks together.

### Parallel Opportunities

- T006 (registry validity test) can be written in parallel with T005 once T003 lands (different
  concerns, same file — coordinate on file edits).
- T009 (US1 test) and T010/T011/T013 (US2 content/tests) can all proceed in parallel once
  Foundational (T002-T004) is complete — User Story 1 and User Story 2 touch disjoint new files
  (`katakana.json` vs. `jlpt_n5_kanji.json`/`jlpt_n5_vocab.json`) and only converge back on
  `embed.go` for their respective registry-append edits (T008, T012), which are single-line-ish
  additions unlikely to conflict.
- T010 and T011 (the two new JSON content files) are fully parallel — different files, no shared
  state.

---

## Parallel Example: Foundational + Both User Stories

```bash
Task: "Generalize embed.go into a Definition registry (T002-T003)"
# then, once T003 lands:
Task: "Add katakana.json + Katakana() accessor + registry entry (US1: T007-T009)"
Task: "Add jlpt_n5_kanji.json + jlpt_n5_vocab.json + accessors + registry entries (US2: T010-T013)"
```

---

## Implementation Strategy

### MVP First — User Story 1, then User Story 2

1. Complete Phase 1: Setup (confirm no new dependency needed)
2. Complete Phase 2: Foundational (generalize the registry — this is the prerequisite both
   stories share)
3. Complete Phase 3: User Story 1 (katakana) — **STOP and VALIDATE**: katakana cards appear as
   due, review correctly
4. Complete Phase 4: User Story 2 (N5 kanji/vocab) — **STOP and VALIDATE**: N5 kanji/vocab cards
   appear as due, correctly labeled by kind
5. Complete Phase 5: Polish (full regression, quickstart sign-off across all four decks together)

Both user stories are P1 and both are needed for this slice to satisfy its stated scope (US-1 +
US-5 together), but they are independently testable and could ship as separate PRs if desired —
this plan delivers them together since they share the same Foundational prerequisite and the task
brief scoped them as one slice.

---

## Notes

- [P] tasks = different files, no dependencies (or additive same-file edits unlikely to conflict).
- [Story] label maps task to its user story (US1, US2) for traceability; Setup/Foundational/Polish
  tasks carry no story label.
- Commit after each task or logical group.
- Avoid: vague tasks, same-file conflicts on `embed.go`'s registry-append edits (T008 and T012
  should each be a small, independent addition to the `builtinDecks` slice, not a rewrite of the
  whole file, to keep them easy to land in either order).
