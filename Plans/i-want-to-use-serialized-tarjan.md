# Meguru — Spec-Kit Scaffold + M1 Walking Skeleton

## Context

Four spec documents (BRD, PRD, CONSTITUTION, TECH_STACK) already exist for **Meguru**, an offline-first, terminal-native Japanese SRS written in Go. The goal now is to turn those docs into a working GitHub Spec Kit project and implement the first milestone end-to-end, following the project's own SDLC (spec → plan → tasks → implement → verify) rather than writing code ad hoc. The repo directory is currently named `meguro` and empty; per user decision it will be renamed to `meguru` to match the product name everywhere (binary, Go module, docs).

Decisions locked in with the user:
- **Name:** "Meguru" everywhere; directory renamed `meguro` → `meguru`.
- **Module path:** `github.com/kiefertaylorland/meguru` (gh already authed as `kiefertaylorland`). No remote GitHub repo is created/pushed this session — local git only.
- **Scope:** full spec-kit scaffold (constitution + first feature spec/plan/tasks) covering the whole product vision, but **code implementation limited to Milestone M1** ("walking skeleton") per BRD: TUI shell + DB migrations + embedded hiragana deck + minimal review loop + 3-OS CI including a network-denied test. FSRS engine, additional decks, AI layer are explicitly out of scope (M2+).
- Environment confirmed ready: Go 1.25.1, `specify` CLI v0.9.5 installed (`~/.local/bin/specify`), `gh` authed as `kiefertaylorland`, git configured.

## A. Repo scaffolding

1. Rename directory: `mv /Users/kiefer.land/Developer/meguro /Users/kiefer.land/Developer/meguru`. All following steps run from the new path.
2. `git init -b main`.
3. `specify init --here --ai claude` — **no `--preset`** flag, so command files stay unprefixed (`/constitution`, `/specify`, `/clarify`, `/plan`, `/tasks`, `/analyze`, `/checklist`, `/implement`, `/taskstoissues` under `.claude/commands/`).
4. Verify what landed: `.specify/memory/`, `.specify/templates/`, `.specify/scripts/bash/`, `.claude/commands/*.md`. If the installed version's command set differs from the above (e.g. a `speckit.*` prefix), adjust later steps to match reality rather than assuming.
5. Install the Karpathy coding rules (from `https://github.com/multica-ai/andrej-karpathy-skills`, `CLAUDE.md`) as the project's root `CLAUDE.md`: four principles — think-before-coding, simplicity-first, surgical-changes, goal-driven-execution. Fetch the file, then compose root `CLAUDE.md` as: Karpathy rules content, followed by a short Meguru-specific section pointing to `.specify/memory/constitution.md` as the binding constitution and `docs/product/` for BRD/PRD/TECH_STACK. Do not duplicate constitution text into CLAUDE.md — reference it.
6. Copy the four source docs into `docs/product/{BRD,PRD,TECH_STACK,CONSTITUTION}.md`, plus a short `docs/product/README.md` explaining these are input docs, and `.specify/memory/constitution.md` / `specs/NNN-*/spec.md` are spec-kit's derived artifacts, not duplicates.
7. Scaffold `LICENSES/` (Apache-2.0.txt for code, CC-BY-SA-4.0.txt pre-staged for future dictionary data, README explaining M1's hiragana content is authored in-repo under Apache-2.0), plus root `LICENSE` = Apache-2.0.
8. Commit this scaffold as one baseline commit before running `/constitution`.

## B. Spec-kit workflow (run inside Claude Code, in order)

1. **`/constitution`** — feed it `docs/product/CONSTITUTION.md` in full; instruct it to populate `.specify/memory/constitution.md` mapping P-1..P-5, §2 AI inventory, §3 STRIDE, SEC-1..SEC-12, CON-1..CON-5 into the template. Verify afterward that rule IDs survived the reformatting (CI and reviews will reference them by ID, e.g. SEC-8).
2. **`/specify`** for the M1 feature — description: walking skeleton = Bubble Tea v2 TUI + Cobra CLI + SQLite schema/migrations + embedded hiragana deck synced on first run + minimal `meguru review` loop with a naive interval-bump scheduler (explicitly not FSRS) that writes `review_log` and updates `srs_state`; `--plain` fallback; `NO_COLOR` respected; 0600/0700 perms; 3-OS CI incl. network-denied test. This creates branch + `specs/001-walking-skeleton/spec.md`. Explicitly write "Out of Scope: FSRS, katakana/kanji/vocab decks, AI provider" into the spec so later steps don't scope-creep.
3. **`/clarify`** — resolve ambiguity markers, especially: exact naive-scheduling rule for M1 (e.g. Again→now+1min, Hard→+1d, Good→+3d, Easy→+7d, documented as disposable placeholder for go-fsrs).
4. **`/plan`** — produces `specs/001-walking-skeleton/plan.md`; steer it to the package layout in §C below (don't let it invent new architecture) and confirm its Constitution Check references SEC-8, SEC-12, P-1.
5. **`/tasks`** — produces `specs/001-walking-skeleton/tasks.md`, broken into: module/Cobra skeleton, SQLite schema/migration runner, embedded deck + sync, naive scheduler + review session, Bubble Tea TUI (model/update/view/plain), file-permission enforcement, CI workflow, test suites, LICENSES wiring.
6. Optionally **`/analyze`** to cross-check spec/plan/tasks consistency, then **`/implement`**.

## C. Go package layout (M1)

Module `github.com/kiefertaylorland/meguru`, Go 1.25.

```
cmd/meguru/main.go                  → internal/cli.Execute()
internal/cli/{root,review,config}.go
internal/config/{config,paths,perms}.go     -- XDG paths via adrg/xdg; 0600/0700 enforcement (SEC-12)
internal/store/
  store.go, migrate.go                       -- PRAGMA user_version-based migration runner (no 3rd-party migration lib)
  migrations/0001_init.sql                   -- decks/notes/cards/srs_state/review_log/ai_cache/app_state, verbatim from TECH_STACK §3
  decks_repo.go, notes_repo.go, cards_repo.go, srs_repo.go, review_log_repo.go
internal/deck/
  deck.go, embed.go (go:embed hiragana.json), hiragana.json, sync.go   -- content_version-based upsert sync
internal/review/
  scheduler.go   -- pure func (rating, now) -> due_at; same interface shape go-fsrs will fill in M2
  session.go     -- pulls due cards, applies rating, writes review_log, updates srs_state
internal/tui/
  model.go, update.go (pure, no I/O), view.go, keys.go, plain.go
internal/textwidth/textwidth.go     -- sole wrapper of go-runewidth+uniseg; no direct len() on user-visible strings
```

Key invariants to carry into implementation:
- `migrate.go` uses SQLite's own `PRAGMA user_version` counter — simplest approach, no extra dependency, consistent with pure-Go/no-CGo constraint.
- `scheduler.go`'s signature mirrors TECH_STACK §4's `(card state, rating, now) → (new state, due date)` so swapping in `go-fsrs` later is a drop-in replacement, not a rewrite.
- `tui/update.go` has zero side effects (Elm architecture); DB writes happen via `tea.Cmd` closures calling `internal/review`, results delivered back as messages — this is what makes it unit-testable per TECH_STACK's stated portfolio goal.

## D. CI (`.github/workflows/ci.yml`)

Jobs: `test` (3-OS matrix: ubuntu/macos/windows-latest, `go build`, `go vet`, `go test -race ./...`), `lint` (golangci-lint-action), `govulncheck`, `gitleaks` (gitleaks-action), and `network-denied` (ubuntu-latest only — use `iptables` to drop all egress except loopback before running the core test suite). Firewall-level blocking is chosen over a custom `net.Dialer` panic-on-dial because it catches any transitive dependency dialing out, not just code the test harness controls; scoped to Linux only since macOS/Windows runners don't offer an equivalent easy root-level firewall in Actions — the 3-OS job already proves cross-platform build/run, offline behavior is OS-independent.

Test layers for M1 (property-based FSRS tests are M2, out of scope here): unit tests for `tui/update`, `review/scheduler`, `store/*`; a `teatest` golden-file snapshot test including a CJK-width (hiragana) case; integration tests against a temp-file SQLite DB for migrations and deck sync. A PTY-based E2E smoke test is a stretch goal, flaggable as optional in `tasks.md`.

## E. Verification (end-to-end proof M1 works)

1. `go build ./...` succeeds.
2. Fresh run of `go run ./cmd/meguru review`: creates `meguru.db` (verify `0600`) and its dir (`0700`), applies migration to `user_version = 1`, seeds `decks`/`cards` from the hiragana deck, presents a due card.
3. Answer + self-grade a card; verify via `sqlite3`: new `review_log` row with correct rating/state_before, `srs_state.due_at` advanced per the naive scheduler, `reps` incremented.
4. `--plain` and `NO_COLOR=1` runs produce linear, escape-free output.
5. Second run after all cards answered shows "no due cards" messaging, doesn't crash or reseed.
6. Bump `hiragana.json`'s `content_version`, rerun, confirm idempotent upsert (no duplicate notes/cards).
7. `go test -race ./...`, `golangci-lint run ./...`, `govulncheck ./...`, `gitleaks detect --source .` all clean locally.
8. Push branch `001-walking-skeleton`, open a **draft** PR, confirm all CI jobs green — `network-denied` passing is the concrete proof of SEC-8/P-1.
9. Human review before merge to `main` (per CONSTITUTION CON-4); confirm no M2-scope content (no FSRS import, no extra decks, no AI code) crept in.
10. Re-check BRD's M1 definition clause-by-clause against the above before declaring M1 done.
