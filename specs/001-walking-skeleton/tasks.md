# Tasks: Walking Skeleton (M1)

**Input**: Design documents from `/specs/001-walking-skeleton/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli.md, contracts/scheduler.md, quickstart.md

**Tests**: Included. FR-016/FR-017/SC-005/SC-001..SC-006 explicitly require an automated test
process (unit, integration, E2E, network-denied, 3-OS CI); these are binding functional/success
requirements for this milestone, not optional TDD scaffolding.

**Organization**: Tasks are grouped by user story (spec.md priorities P1/P1/P2/P2) to enable
independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US4)
- Paths are relative to the repo root, per plan.md's Project Structure (single Go module)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project/module initialization and directory scaffolding

- [X] T001 Initialize/verify `go.mod` (module `meguru`, Go 1.23+) and add pinned dependencies:
      `github.com/spf13/cobra`, `charm.land/bubbletea/v2` + Bubbles + Lip Gloss,
      `modernc.org/sqlite`, `github.com/adrg/xdg`, `github.com/mattn/go-runewidth`,
      `github.com/rivo/uniseg`, `github.com/stretchr/testify`, `github.com/charmbracelet/x/exp/teatest`
- [X] T002 [P] Create directory skeleton per plan.md Project Structure: `cmd/meguru/`,
      `internal/{cli,tui,plain,textwidth,storage,storage/migrations,deck,scheduler,review}/`,
      `tests/{integration,e2e,unit}/`
- [X] T003 [P] Scaffold `.github/workflows/ci.yml` with a 3-OS build+test matrix
      (ubuntu-latest/macos-latest/windows-latest) running `go build ./...` and `go test ./...` (FR-016)

**Checkpoint**: Module builds empty; CI skeleton exists.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Storage, migrations, embedded deck, scheduler, and the review service that every
user story is built on. No user story can be implemented until this phase is complete.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Implement `internal/storage/db.go`: `Open()` resolves the data path via `adrg/xdg`,
      `os.MkdirAll(dir, 0700)`, `os.OpenFile(dbPath, O_RDWR|O_CREATE, 0600)`, then opens
      `modernc.org/sqlite` with `_pragma=journal_mode(WAL)` and `_pragma=foreign_keys(1)`; on every
      call, `os.Stat` dir+file, `os.Chmod` back to 0700/0600 and print a one-line stderr warning if
      looser (FR-001, FR-012, FR-013; research.md §2)
- [X] T005 Implement `internal/storage/migrate.go` + `internal/storage/migrations/0001_init.sql`
      (embedded via `go:embed`): create `decks`, `notes`, `cards`, `srs_state`, `review_log`,
      `app_state` tables per data-model.md; forward-only runner gated on
      `app_state.schema_version` (FR-014)
- [X] T006 [P] Implement `internal/textwidth/textwidth.go` wrapping `go-runewidth`/`uniseg` for
      CJK-safe string width math (shared by `tui` and `plain`)
- [X] T007 [P] Implement `internal/scheduler/naive.go`: `Rating` consts (Again/Hard/Good/Easy) and
      pure `NextDue(rating Rating, now time.Time) time.Time` per contracts/scheduler.md's fixed
      intervals (FR-008)
- [X] T008 [P] Implement `internal/deck/embed.go` + `internal/deck/hiragana.json`
      (`go:embed`): embedded hiragana note array with a top-level `content_version` field
- [X] T009 Implement `internal/deck/seed.go`: on startup, look up `decks` by
      `slug='kana-hiragana'`; insert deck+notes+cards+`srs_state` (due_at = now) if absent; if
      present and embedded `content_version` is greater, update existing notes' `fields` in place
      keyed by `fields->>'expression'`, bump stored `content_version`, never touching
      `cards`/`srs_state`/`review_log` (FR-002, FR-003, FR-004; research.md §3) — depends on T005, T008
- [X] T010 Implement `internal/review/service.go`: `Service` interface with `NextDueCard(ctx)` and
      `Rate(ctx, cardID, rating, now)` per contracts/cli.md; `Rate` performs one DB transaction that
      INSERTs `review_log` and UPDATEs `srs_state` atomically (FR-007, FR-008, FR-015) — depends on
      T004, T005, T007
- [X] T011 [P] Implement `cmd/meguru/main.go`: build the Cobra root command and wire dependencies
- [X] T012 [P] Implement `internal/cli/root.go`: root command with no subcommand prints help and
      exits 0

**Checkpoint**: Storage, migrations, seeding, and the review service all compile and are unit-testable — user story implementation can now begin.

---

## Phase 3: User Story 1 - First run seeds a deck and shows a due card (Priority: P1) 🎯 MVP

**Goal**: On a clean machine, `meguru review` creates local storage, seeds the hiragana deck, and
shows a due card — no setup, no network. Re-running does not duplicate content.

**Independent Test**: On a clean profile, run the review command once; verify local storage
exists, contains the hiragana deck's cards, and a card is shown — with no network call.

### Implementation for User Story 1

- [X] T013 [US1] Implement `internal/cli/review.go`: `review` command running the startup sequence
      from contracts/cli.md (open/migrate/seed via T004/T005/T009, then `NextDueCard`) and
      dispatching to the TUI (FR-001; depends on Phase 2)
- [X] T014 [US1] Implement `internal/tui/model.go`, `internal/tui/update.go`, `internal/tui/view.go`:
      minimal Bubble Tea v2 program that renders one due card (expression/reading) using
      `internal/textwidth` for layout

### Tests for User Story 1

- [X] T015 [P] [US1] Integration test `tests/integration/seed_test.go`: against a temp SQLite file,
      run migrate+seed twice and assert exactly one `decks` row and no duplicate `notes`/`cards`
      (FR-002, FR-003)
- [X] T016 [P] [US1] Integration test `tests/integration/firstrun_due_test.go`: after a fresh
      seed, assert `Service.NextDueCard` returns a card immediately (`due_at` ≤ now) (SC-001, data-model.md "Initial state")
- [X] T017 [P] [US1] E2E test `tests/e2e/firstrun_test.go`: run the compiled binary (`--plain`,
      no PTY dependency added — plain mode needs none) against a clean `XDG_DATA_HOME`, assert it
      exits 0, shows a card, and completes in well under
      5 seconds (SC-001)

**Checkpoint**: User Story 1 is fully functional and independently testable — first run seeds and shows a due card.

---

## Phase 4: User Story 2 - Answer a card and have progress persist (Priority: P1)

**Goal**: Submitting a rating writes a permanent review record and reschedules the card per the
naive interval rule; an interrupted session leaves no partial state.

**Independent Test**: Answer one due card with a rating, exit, restart. Verify a review record
exists, the card's due time moved per the rating, and it is no longer due until then.

### Implementation for User Story 2

- [X] T018 [US2] Implement rating input in `internal/tui/update.go`: map a fixed keypress set to
      Again/Hard/Good/Easy, call `review.Service.Rate`, then advance to the next due card (FR-006,
      depends on T010, T014)
- [X] T019 [US2] Implement the "nothing due" state in `internal/tui/view.go`: clearly communicate
      when `NextDueCard` returns nil and exit cleanly (FR-005, Edge Case)

### Tests for User Story 2

- [X] T020 [P] [US2] Integration test `tests/integration/review_roundtrip_test.go`: rate a card
      Again and assert `due_at` ≈ now+1m; rate a card Easy and assert `due_at` ≈ now+7d; assert a
      matching `review_log` row exists for each (FR-007, FR-008, SC-002)
- [X] T021 [P] [US2] Integration test `tests/integration/interrupted_test.go`: assert that if
      `Rate`'s transaction is never committed (simulated interruption between card-shown and
      rating-submitted), `srs_state.due_at` is unchanged and no `review_log` row is written
      (FR-015, SC-006)
- [X] T022 [P] [US2] Unit test `internal/scheduler/naive_test.go`: property tests asserting
      `NextDue` never returns ≤ `now`, is deterministic, and handles all four `Rating` values
      without panicking (contracts/scheduler.md invariants)

**Checkpoint**: User Stories 1 and 2 both work independently — the core review loop is complete.

---

## Phase 5: User Story 3 - Usable without color or a fancy terminal (Priority: P2)

**Goal**: A full review session can be completed via an explicit `--plain` linear text mode, and
`NO_COLOR` independently suppresses color/style without disabling interactive redraws.

**Independent Test**: Launch in `--plain` mode and separately with `NO_COLOR` set; complete a full
review session in each using only plain/sequential output (plain) or color-free interactive output
(`NO_COLOR`).

### Implementation for User Story 3

- [X] T023 [US3] Implement `internal/plain/renderer.go`: sequential `fmt.Println`-based card
      display + blocking `bufio.Scanner` rating prompt (accepts the rating word or its first
      letter), looping via `review.Service` until nothing is due (FR-010; depends on T010, T006)
- [X] T024 [US3] Wire `--plain` flag and dispatch logic in `internal/cli/review.go`: route to
      `internal/plain` when `--plain` is set OR stdout is not a TTY (research.md §5, FR-010)
- [X] T025 [US3] Implement `NO_COLOR`-aware output in `internal/cli/review.go`: when `NO_COLOR` is
      set and interactive mode is still used, pass `tea.WithColorProfile(colorprofile.Ascii)` to
      the Bubble Tea program so redraws continue but no color/style codes are emitted — lipgloss/v2
      has no `NewRenderer`-style API (superseded research.md §5's plan), so the profile is forced
      at the Program level instead of via a Lip Gloss renderer (FR-011)

### Tests for User Story 3

- [X] T026 [P] [US3] E2E test `tests/e2e/plain_test.go`: run `--plain`, capture raw output bytes,
      assert no ESC (`0x1b`) byte appears, and a full session (see card, submit rating) completes
      (SC-003)
- [X] T027 [P] [US3] E2E test `tests/e2e/nocolor_test.go`: run with `NO_COLOR=1` (no `--plain`),
      assert no color/style escape sequences appear while interactive layout is still used (FR-011)

**Checkpoint**: All P1/P2 stories through US3 work independently.

---

## Phase 6: User Story 4 - Local data stays private by construction (Priority: P2)

**Goal**: The DB file and its directory are created with owner-only permissions from the first
run, and looser permissions are detected and self-corrected with a warning.

**Independent Test**: After first run, inspect the DB file and directory permissions and confirm
owner-only access; loosen them and confirm the next run warns and corrects them.

> Implementation for this story (`Open()`'s create + self-heal logic) was already built in T004
> as a foundational dependency of every other story; this phase adds the verification the spec
> requires.

### Tests for User Story 4

- [X] T028 [P] [US4] Unit test `internal/storage/permissions_test.go` (POSIX-only, per research.md
      §2's Windows caveat): via `ensurePerm` (the helper `Open()`/`DataDir()` share — `Open()`
      itself can't be redirected off the real XDG path from within the test binary since
      `adrg/xdg` resolves `XDG_DATA_HOME` once at process init), assert a freshly created
      directory is `0700` and DB file is `0600` (SC-004, FR-012)
- [X] T029 [P] [US4] Test `internal/storage/permissions_selfheal_test.go` (same package as T028,
      not `tests/integration`, for access to the unexported `ensurePerm` helper): `chmod` the dir
      to `0755` and file to `0644`, call `ensurePerm` again, assert both are corrected back and a
      warning is printed to stderr (FR-013)

**Checkpoint**: All four user stories are independently functional and tested.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Requirements that span all user stories rather than belonging to one (network
isolation proof, 3-OS CI, TUI golden frames, quickstart validation).

- [X] T030 [P] Implement `tests/e2e/networkdenied_test.go` (build tag `networkdenied`): compiled
      binary runs the full core loop (temp dir → migrate → seed → review → rate → reschedule) with
      no code path importing `internal/ai`, so the OS-level network block proves zero egress
      (FR-017; research.md §6)
- [X] T031 Add an ubuntu-only CI job to `.github/workflows/ci.yml` that runs
      `go test -tags networkdenied ./tests/e2e/...` under `unshare --net` (or equivalent
      sandboxed step) (FR-017, SC-005)
- [X] T032 [P] Add a `teatest` golden-frame test for the interactive review screen in
      `internal/tui/model_test.go`, covering card render + rating keypress (plan.md Testing
      strategy)
- [X] T033 Run the `quickstart.md` validation checklist (§§1–6) manually on macOS (the machine
      this milestone was implemented on) — all steps passed (first-run seed, no re-seed
      duplication, Again/Easy rescheduling + `review_log` persistence, `--plain` zero-ESC output,
      permission creation + self-heal, interrupted-review safety). Linux and Windows legs are not
      runnable in this environment; the 3-OS CI matrix (T003) and the ubuntu-only network-denied
      job (T031) are what actually gate those platforms before M1 is considered done — this task's
      manual pass does not substitute for green CI on all three OSes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Setup. BLOCKS all user stories (T013 needs T004/T005/T009; T010 needs T004/T005/T007).
- **User Story 1 (Phase 3)**: Depends on Foundational only.
- **User Story 2 (Phase 4)**: Depends on Foundational + US1's TUI skeleton (T014) for the interaction loop it extends.
- **User Story 3 (Phase 5)**: Depends on Foundational (`review.Service`, `textwidth`) — independent of US2's rating logic, though it reuses the same `Service`.
- **User Story 4 (Phase 6)**: Implementation lives in Foundational (T004); this phase is verification-only and has no story dependency.
- **Polish (Phase 7)**: Depends on all user stories being implemented (exercises the full loop end-to-end).

### User Story Dependencies

- **US1 (P1)**: No dependencies on other stories.
- **US2 (P1)**: Builds on US1's TUI skeleton (T014) but is independently testable via `review.Service` directly.
- **US3 (P2)**: Independent of US2; both consume `review.Service` from Foundational.
- **US4 (P2)**: Independent — pure verification of Foundational behavior.

### Parallel Opportunities

- T002, T003 in Setup.
- T006, T007, T008 in Foundational (distinct files, no interdependency).
- T011, T012 in Foundational (distinct files).
- T015, T016, T017 (US1 tests) once T013/T014 land.
- T020, T021, T022 (US2 tests) once T018/T019 land.
- T026, T027 (US3 tests) once T023/T024/T025 land.
- T028, T029 (US4 tests) any time after T004.
- T030, T032 in Polish.

---

## Parallel Example: User Story 1

```bash
Task: "Integration test seed round-trip in tests/integration/seed_test.go"
Task: "Integration test immediate due-card in tests/integration/firstrun_due_test.go"
Task: "E2E first-run smoke test in tests/e2e/firstrun_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (storage, migrations, seed, scheduler, review service)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: run quickstart.md §2 — clean-profile first run shows a due card
5. Demo/dogfood if ready

### Incremental Delivery

1. Setup + Foundational → foundation ready
2. Add US1 → validate via quickstart.md §2 (MVP)
3. Add US2 → validate via quickstart.md §3
4. Add US3 → validate via quickstart.md §4
5. Add US4 → validate via quickstart.md §5
6. Polish (network-denied CI, 3-OS matrix, golden frames) → validate via quickstart.md §6–7

---

## Notes

- [P] tasks = different files, no dependencies.
- [Story] label maps task to specific user story for traceability.
- Every user story is independently testable per its Independent Test description from spec.md.
- Commit after each task or logical group.
- Avoid: vague tasks, same-file conflicts, cross-story dependencies that break independence.
