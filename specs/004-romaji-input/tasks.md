# Tasks: Romaji Answer Input

**Input**: Design documents from `/specs/004-romaji-input/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/romaji.md,
contracts/answer-check.md, quickstart.md

**Tests**: Included. This project's testing posture (table-driven pure-function coverage,
package-isolated unit tests, e2e fixture coverage) is a binding part of
`docs/product/TECH_STACK.md` §7's testing strategy, and FR-001/FR-006/FR-007/SC-002/SC-003
require automated verification.

**Organization**: Single user story (spec.md: US1, P1) — this feature is one indivisible slice
(typed-answer auto-check before rating), so all implementation tasks are grouped under it.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1)
- Paths are relative to the repo root, per plan.md's Project Structure (single Go module)

---

## Phase 1: Foundational — `internal/romaji` (no dependencies)

**Purpose**: Build and prove the pure conversion function in complete isolation before anything
else depends on it (FR-001, FR-007).

- [X] T001 [US1] Create `internal/romaji/romaji.go`: package doc comment, the digraph/monograph
      lookup tables (vowels; k/s/t/n/h/m/y/r/w rows incl. alternate spellings si/ti/tu/di/zi;
      dakuten/handakuten g/z/d/b/p rows; wo; yōon digraphs for every row that has them), the
      `isSokuonPair` helper, and `ToHiragana(input string) string` implementing the greedy
      longest-match tokenizer per `contracts/romaji.md`.
- [X] T002 [P] [US1] `internal/romaji/romaji_test.go`: table-driven tests covering every base
      mora (a-row through wa/wo/n), every dakuten/handakuten mora, every yōon digraph, sokuon
      (`"kka"`→`"っか"`, `"matcha"`→`"まっちゃ"`), `n`-disambiguation (`"annai"`→`"あんない"`,
      `"kon'ya"`→`"こんや"`, documented ambiguity case `"kinyoubi"`→`"きにょうび"` vs.
      `"kin'youbi"`→`"きんようび"`), alternate spellings (si/shi, ti/chi, tu/tsu, di/zi/ji all
      producing their shared kana), case-insensitivity, empty-string passthrough, and
      unrecognized-rune passthrough (SC-002, FR-006).

**Checkpoint**: `internal/romaji` is complete, tested, and has zero dependents yet — verify with
`go test ./internal/romaji/...`.

---

## Phase 2: User Story 1 - Type an answer instead of just self-grading (Priority: P1) 🎯 MVP

**Goal**: A learner running `meguru review --plain` types a romaji answer for a due card, sees an
auto-check result against the card's expected reading, then still reaches the existing
Again/Hard/Good/Easy rating step (spec.md Acceptance Scenarios 1–4).

**Independent Test**: Present a due card, type a matching romaji answer, confirm a "correct"
report; type a non-matching answer on another card, confirm a "not quite" report plus the reveal;
confirm both paths reach the rating step and `Service.Rate` is called exactly as before.

### Implementation for User Story 1

- [X] T003 [US1] Create `internal/review/answercheck.go`: `AnswerResult` struct and
      `CheckAnswer(card *Card, typedRomaji string) AnswerResult` per
      `contracts/answer-check.md`'s dual-comparison semantics (converted kana OR raw
      trimmed/lowercased text matches `card.Reading`) (depends on T001).
- [X] T004 [P] [US1] `internal/review/answercheck_test.go`: unit tests for `CheckAnswer` —
      match via converted kana (future kana-reading-style fixture), match via raw romaji text
      (today's hiragana-deck convention, e.g. reading `"a"` typed `"a"`), no-match still returns
      the converted `Kana`, empty typed string doesn't panic and reports no match against a
      non-empty reading (depends on T003).
- [X] T005 [US1] Modify `internal/plain/renderer.go`'s `Run` loop per
      `contracts/answer-check.md`'s UI integration contract: print `Expression` alone as the
      prompt, read one answer line (returning cleanly on scanner EOF/failure exactly like the
      existing rating-read path), call `review.CheckAnswer` once and print a match/no-match
      feedback line, then reveal `Reading`/`Meaning` and proceed into the existing,
      byte-for-byte-unchanged rating prompt/parse/`Rate` call (depends on T003).

### Tests for User Story 1

- [X] T006 [US1] Update `internal/plain/renderer_test.go`: extend every existing test's stdin
      fixture with an answer line before the rating line where a card is shown; add new cases for
      a matching-answer "Correct" feedback line, a non-matching-answer "Not quite" feedback line
      that still reveals `Reading`/`Meaning`, and an EOF-during-answer-read case (returns cleanly,
      `Service.Rate` never called) mirroring the existing EOF-during-rating test (depends on T005).
- [X] T007 [P] [US1] Update `tests/e2e/plain_test.go`'s stdin fixture to include an answer line
      before the rating line (depends on T005).
- [X] T008 [P] [US1] Update `tests/e2e/networkdenied_test.go`'s stdin fixture(s) to include an
      answer line before each rating line (depends on T005).

**Checkpoint**: User Story 1 is fully functional and independently testable — typed-answer
auto-check runs before rating in `--plain` mode, matching spec.md's Acceptance Scenarios 1–4.

---

## Phase 3: Polish & Cross-Cutting Concerns

**Purpose**: Confirm the addition didn't regress anything outside its scope, and that
`internal/tui`/offline guarantees still hold untouched.

- [X] T009 [P] Run `go test ./internal/tui/... ./internal/scheduler/... ./internal/cli/...
      ./internal/deck/... ./internal/storage/...` as a sanity check — zero source changes
      expected in any of these packages; confirm nothing incidentally broke.
- [X] T010 Run `go vet ./...` and `go build ./...`, confirming a clean build with no new
      dependencies added to `go.mod`/`go.sum` (SEC-10, CON-2).
- [X] T011 Run `go test ./...` (full-suite regression) and confirm every package is green,
      including this feature's new/updated tests.
- [X] T012 Run the `quickstart.md` validation guide (§§1–5) end-to-end, including the manual
      `bin/meguru review --plain` check against a clean XDG profile, and confirm every expected
      outcome holds.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 1)**: No dependencies — start immediately. Blocks Phase 2 (T003 needs
  `internal/romaji` to exist).
- **User Story 1 (Phase 2)**: Depends on Phase 1 (T001). T004 depends on T003; T005 depends on
  T003; T006/T007/T008 depend on T005.
- **Polish (Phase 3)**: Depends on Phase 2 being complete.

### Parallel Opportunities

- T002 can be written alongside T001 (same package, but the test file and implementation file
  are conventionally written together/iteratively — listed [P] since it's a distinct file with
  no other same-file conflict).
- T004 can run in parallel with T005 once T003 lands (different files: `answercheck_test.go` vs.
  `renderer.go`).
- T007, T008 can run in parallel with each other and with T006 once T005 lands (three different
  files).
- T009 in Polish can run any time after Phase 2 completes, in parallel with T010's setup.

---

## Parallel Example: User Story 1

```bash
Task: "Unit tests for CheckAnswer in internal/review/answercheck_test.go"
Task: "Update tests/e2e/plain_test.go stdin fixture"
Task: "Update tests/e2e/networkdenied_test.go stdin fixture"
```

---

## Implementation Strategy

### MVP First (and only) — User Story 1

1. Complete Phase 1: Foundational (`internal/romaji`, fully tested in isolation)
2. Complete Phase 2: User Story 1 (`CheckAnswer` → `internal/plain` wiring → tests)
3. **STOP and VALIDATE**: run quickstart.md — confirm the manual `--plain` session shows the
   answer prompt, auto-check feedback, reveal, and rating step in order
4. Complete Phase 3: Polish (full regression, `internal/tui`/scheduler untouched, quickstart
   sign-off)

This feature has exactly one user story; there is no incremental multi-story delivery plan here —
Phase 2 IS the MVP, and Phase 3 is the ship gate.

---

## Notes

- [P] tasks = different files, no dependencies.
- [Story] label maps task to the single user story (US1) for traceability.
- Commit after each task or logical group.
- Avoid: vague tasks, same-file conflicts, wiring `internal/plain` (T005) before `CheckAnswer`
  (T003) exists to call.
