# Implementation Plan: Katakana + JLPT N5 Kanji/Vocab Built-In Decks

**Branch**: `003-katakana-n5-decks` | **Date**: 2026-07-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-katakana-n5-decks/spec.md`

## Summary

Generalize `internal/deck`'s M1 single-deck (hiragana-only) embed+seed pipeline into a small
built-in-deck registry (`Definition` + `BuiltinDecks()`) so the existing content-version-aware,
update-in-place seed logic (`seedFresh`/`updateInPlace`/`insertNote`) is shared by every built-in
deck instead of reimplemented per deck, then add three new embedded JSON deck files — katakana
(`kana-katakana`), a curated N5 kanji starter set (`jlpt-n5-kanji`), and a curated N5 vocab starter
set (`jlpt-n5-vocab`) — as new registry entries. This is a pure data + `internal/deck` package
change: no schema migration, no new dependency, no CLI/TUI/`internal/review` changes, since
`internal/review/service.go`'s due-card query already joins across all decks generically
(confirmed by reading it before writing this plan).

## Technical Context

**Language/Version**: Go (current stable, pinned in `go.mod`; same as M1/M2, 1.25.1).

**Primary Dependencies**: None new. `encoding/json` (stdlib) and Go's `embed` (stdlib) are the only
mechanisms involved, exactly as M1 used for the hiragana deck.

**Storage**: Same single SQLite file (WAL, `modernc.org/sqlite`) from M1/002. No migration — the
`decks.kind` CHECK constraint in `internal/storage/migrations/0001_init.sql` already includes
`'kana'`, `'kanji'`, `'vocab'`, and `decks`/`notes`/`cards`/`srs_state` place no limit on the
number of deck rows.

**Testing**: stdlib `testing` + `testify/require` (existing pattern, unchanged). Unit tests in
`internal/deck` are refactored to exercise the shared seed/update-in-place logic (`seedDeck`,
`seedFresh`, `updateInPlace`, `insertNote`) against a synthetic `Definition`, decoupling them from
both the real embedded JSON content and from `BuiltinDecks()`'s registry order — plus new tests
asserting `BuiltinDecks()` lists exactly the four expected decks with valid, parseable,
duplicate-free content. Integration tests in `tests/integration/` are generalized from
hardcoded hiragana-only counts to counts derived from `deck.BuiltinDecks()`, so the
no-duplication guarantee is proven across every built-in deck, not just hiragana.

**Target Platform**: Same cross-platform CLI/TUI target as M1/M2 (Linux/macOS/Windows) — pure Go,
no CGo, no OS/network dependency; no change from existing `internal/deck` platform posture.

**Project Type**: Single Go module, CLI/TUI application (unchanged — Option 1 layout).

**Performance Goals**: No new performance targets. Seeding four small JSON-embedded decks
(currently ~46 + 46 + 30 + 30 = 152 notes total) on every startup is the same order of magnitude
of work M1's single 46-note hiragana seed already did per startup; no measurable regression
expected or required.

**Constraints**: Zero network calls (P-1/SEC-8) — `internal/deck` remains pure `embed`+`encoding/
json`+`database/sql`, no I/O beyond the local SQLite file, same as M1. Deck content is
curated/embedded at build time, not imported at runtime, so SEC-6 (schema-validate/size-cap
imported decks) does not apply to this feature — these are build-time trusted fixtures, not
runtime untrusted deck imports (which remain a separate, not-yet-built feature per PRD scope).

**Scale/Scope**: One package change (`internal/deck`: `embed.go` generalized to a `Definition`
registry, `seed.go`'s functions parameterized by `Definition` instead of hardcoded to hiragana),
three new embedded JSON files, test updates in `internal/deck/seed_test.go` and
`tests/integration/seed_test.go`. No new packages, no new CLI subcommands, no changes to
`internal/review`, `internal/scheduler`, `internal/tui`, `internal/plain`, `internal/cli`.

## Constitution Check

_GATE: Must pass before Phase 0 research. Re-check after Phase 1 design._

| Principle / Rule | Applies? | Assessment |
| --- | --- | --- |
| P-1 Offline-First Is Inviolable | Yes | No network I/O anywhere in `internal/deck`; content is embedded at build time via `//go:embed`. **PASS** |
| P-2 Local-Only By Default | Yes | All deck/note/card/srs_state rows are written to the existing local SQLite file only. **PASS** |
| P-3 User-Supplied AI Only | N/A | No AI touchpoint in this feature. |
| P-4 Data Minimization At The AI Boundary | N/A | No AI payloads. |
| P-5 No Telemetry | Yes | No telemetry added. **PASS** |
| SEC-6/SEC-7 (untrusted input) | N/A | These three decks are build-time embedded fixtures curated by this PR, not runtime-imported user decks; the untrusted-import path (schema validation, size caps) is a separate not-yet-built feature and out of this slice's scope. |
| SEC-8 (network isolation, CI gate) | Yes | No new I/O; the existing M1 network-denied CI job continues to pass untouched, confirming no accidental egress was introduced by this data-only feature. **PASS** |
| SEC-10 (dependency hygiene) | Yes | No new dependencies added at all — stdlib `embed`/`encoding/json` only. **PASS** |
| SEC-12 (file perms, no telemetry) | Yes | No changes to file-permission handling (`internal/storage` untouched); no telemetry. **PASS** |
| CON-1 (load constitution + tech stack first) | Yes | Done — this plan is derived from both, and from reading the existing M1 `internal/deck`/`internal/review` code before designing anything. |
| CON-2 (no stray network calls / deps / weakened CI) | Yes | No new dependencies, no network calls, no CI gate weakened — this feature adds test coverage (generalized no-duplication guarantee across four decks), it does not remove any. |
| CON-3 (no secrets/real user data in fixtures) | Yes | Katakana/N5-kanji/N5-vocab content is curated from well-established public JLPT reference material, not collected from or generated on behalf of any user. |

No violations requiring Complexity Tracking justification. Gate: **PASS**.

## Project Structure

### Documentation (this feature)

```text
specs/003-katakana-n5-decks/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md         # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   └── deck-registry.md
├── checklists/
│   └── requirements.md  # Spec quality checklist (/speckit-specify command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/
└── deck/                        # GENERALIZED this feature: registry of N built-in decks
    ├── embed.go                   # MODIFIED: hiragana-only embed -> Definition{} registry + BuiltinDecks()
    ├── seed.go                    # MODIFIED: Seed() loops BuiltinDecks(); seedFresh/seedDeck take a Definition
    ├── seed_test.go                # MODIFIED: unit tests decoupled onto a synthetic Definition + registry tests
    ├── hiragana.json               # UNCHANGED: existing M1 content
    ├── katakana.json               # NEW: standard katakana syllabary, content_version 1
    ├── jlpt_n5_kanji.json          # NEW: curated N5 kanji starter subset, content_version 1
    └── jlpt_n5_vocab.json          # NEW: curated N5 vocabulary starter subset, content_version 1

tests/
└── integration/
    └── seed_test.go              # MODIFIED: no-duplication assertion generalized to deck.BuiltinDecks()
```

**Structure Decision**: No new packages or directories. This feature stays entirely inside
`internal/deck` (plus its test callers), exactly the seam M1 built for this purpose:
`specs/001-walking-skeleton/plan.md` describes `internal/deck` as the seed/embed boundary, and
`internal/review/service.go`'s due-card query was already written generically (no hiragana-
specific filtering) so it needs zero changes for new decks to become reviewable. This matches
Simplicity First: the only new abstraction introduced is `Definition`, a plain struct with no
behavior beyond parsing its own embedded content — not a plugin system, not a config file format,
not a runtime deck-loading mechanism (all of which would be speculative scope beyond what four
fixed, build-time-known decks need).

## Complexity Tracking

_No Constitution Check violations — table not needed._
