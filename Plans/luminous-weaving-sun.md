# Close the CI Verification-Gate Gap (Constitution/TECH_STACK Mandate)

## Context

**Goal:** analyze current repo state vs. planned desired state, plan the closure, implement, and land with all tests passing.

**Current state (audited 2026-07-18, main @ 15b3195, clean tree):**

- All five feature specs are 100% complete — every task checked in `specs/00{1..5}-*/tasks.md`, and the code verifiably exists (spot-checked per spec). `go build ./...`, `go vet ./...`, and `go test ./... -count=1` are all green across 13 packages.
- Per-package coverage: scheduler/tui/textwidth 100%, romaji 98.0%, plain 96.2%, review 90.9%, deck 88.7%, storage 86.4%, stats 82.8%, **cli 51.4%** (untested `runReview`/`runStats` OS-wiring paths).

**The gap:** `docs/product/TECH_STACK.md:223` and the binding constitution (`.specify/memory/constitution.md` — SEC-1..3, SEC-8, SEC-10, CON-2, CON-4) mandate a CI gate set of `-race`, `golangci-lint`, `govulncheck`, `gitleaks`, **≥80% coverage on core packages**, and the network-denied run. The actual `.github/workflows/ci.yml` implements only `go build` + `go test` (3-OS matrix) and network-denied. The four security/quality gates and the coverage floor are absent, and `internal/cli` (51.4%) would fail the coverage floor today. CON-4 says merges require green CI *including SEC gates* — gates that currently don't exist.

**Deliberately out of scope (documented future work, not gaps in completed specs):**

- Import/export (PRD US-9), `config.toml`, GoReleaser signed releases (SEC-10 artifacts) — remaining M2 roadmap items with no spec dir yet; per the repo SDLC they enter via `speckit-specify` as specs 006+.
- TUI romaji answer input — explicitly scoped out of spec 004 as a "documented fast-follow" (`specs/004-romaji-input/spec.md:117-119`).
- Doc nits noticed, not fixed here (surgical-changes rule): all `spec.md` front-matters still say `Status: Draft` despite completion; README under-documents the actual feature surface; a few exported helpers are test-only (`deck.Hiragana()` et al., `storage.DataDir()`, `textwidth.Truncate`).

**Why this scope:** the CI gate set is the only place where binding project documents say MUST and the repo currently doesn't comply. It is bounded, verifiable ("all tests pass" extends to the new gates), and touches zero app behavior — CI config, lint config, small lint-driven fixes, and new tests only. This parallels prior chore PRs (e.g. #10), not feature work, so no `specs/006-*` dir is created.

## Implementation plan

Design assumptions verified against source: `runReview` uses `cmd.InOrStdin()`/`cmd.OutOrStdout()` on the plain path (`internal/cli/review.go:54`), `runStats` writes to `cmd.OutOrStdout()` (`internal/cli/stats.go:50`), `cli.NewRootCommand()` is the exported entry (`internal/cli/root.go:10`), and the XDG isolation pattern to mirror is `withXDGDataHome` (`internal/storage/db_test.go:17-22` — `t.Setenv` + `xdg.Reload()` + cleanup, required because adrg/xdg caches at init).

### Step 1 — Lift `internal/cli` to ≥80% coverage (new `internal/cli/run_test.go`)

Uncovered code is exactly `runReview` (~15 stmts), `runStats` (~12 stmts), and the two 1-line `RunE` closures. Drive everything in-process via `NewRootCommand()` + `SetArgs`/`SetIn`/`SetOut`, with a local `setTempXDGDataHome(t)` helper mirroring `withXDGDataHome`:

| Test | Path covered | Mechanics |
|---|---|---|
| `TestRunReview_Plain_FullSession` | `runReview` happy path via `plain.Run` | args `review --plain`, stdin `"a\nagain\n"` (session ends on EOF — proven in `tests/e2e/plain_test.go`), assert `Expression:` + `Recorded: Again` |
| `TestRunReview_StorageOpenFailure_ReturnsError` | error return after `storage.Open` | `chmod 0o000` blocked dir as `XDG_DATA_HOME`; skip on windows (same guard as e2e) |
| `TestRunReview_MigrateFailure_ReturnsError` | error return after `Migrate` | pre-create conflicting `decks` table (recipe: `tests/e2e/error_exit_test.go`) |
| `TestRunStats_PlainOutput_FreshDB` | `runStats` plain branch | args `stats` on fresh dir; assert `Due now:`, `no cards scheduled` |
| `TestRunStats_JSONOutput_FreshDB` | `--json` branch | unmarshal output; assert zero counts, null retention |
| `TestRunStats_AfterPlainReview_CountsSeededCards` | stats with real data | run `review --plain` then `stats` on same dir |

Only the real-TTY `tea.NewProgram` branch (`review.go:57-68`, ~5-7 stmts, covered by PTY e2e) stays uncovered → estimated ~87-90% package coverage. `t.Setenv` forbids `t.Parallel` — consistent with the suite (none used anywhere).

### Step 2 — `.golangci.yml` (v2 schema, new file)

```yaml
version: "2"
linters:
  default: standard        # errcheck, govet, ineffassign, staticcheck, unused
  enable:
    - gosec                # constitution is security-first (SEC-*)
    - misspell
  exclusions:
    presets:
      - common-false-positives
      - std-error-handling
    rules:
      - path: _test\.go
        linters: [gosec]
formatters:
  enable: [gofmt]
```

No style-nit linters (revive/wsl/gocritic) — they'd force non-surgical churn. golangci-lint's govet subsumes `go vet` in CI; `go vet` stays in the local sequence. **Finding policy:** real bug → fix; false positive → targeted `//nolint:<linter> // reason`; systematic test-only noise → config rule. Anticipated: errcheck on `defer tx.Rollback()` (`internal/review/service.go:85` → `_ =` wrap, behavior-identical); gosec G304 on the XDG-derived DB path and G302 on the constitution-mandated `0o700` chmod (`internal/storage/db.go` → `//nolint` with reasons). Verify empirically — run locally before committing.

### Step 3 — Coverage gate (`scripts/coverage.sh`, new, executable)

"Core packages" = all `./internal/...` (cmd/meguru is a 6-line wiring main, verified by e2e; tests/ are test-only packages). Script runs `go test -count=1 -cover ./internal/...`, guards that every `go list ./internal/...` package reported coverage (a new package without tests fails the gate), and fails if any package < 80.0%. Lives in `scripts/` so local runs and CI execute the identical gate; runs in the ubuntu `coverage` job.

### Step 4 — `.github/workflows/ci.yml`

`test` matrix and `network-denied` stay byte-identical **except** `go test ./...` → `go test -race ./...` (strict strengthening; pure-Go sqlite makes -race viable on all 3 OSes; fallback to ubuntu/macos-only -race is a documented last resort if Windows is intolerably slow). Add four parallel jobs, tag-pinned like the existing ones:

- `lint`: `golangci/golangci-lint-action` (verify current major, ~v8) with pinned golangci-lint v2.x
- `vulncheck`: `golang/govulncheck-action@v1` (network in CI is fine — SEC-8 restricts the app; SEC-10 mandates the scan). If it flags a stdlib vuln, bump the go.mod toolchain patch in the same PR
- `gitleaks`: `gitleaks/gitleaks-action@v2` with `fetch-depth: 0`; no license key needed (personal account); one-time historical sweep happens locally
- `coverage`: `./scripts/coverage.sh`

### Step 5 — Race pre-flight

`go test -race -count=1 ./...` locally **before** touching ci.yml. Expected clean (no `t.Parallel`, no first-party goroutines, `defaultFSRS` read-only after init). Fix anything surfaced in its own commit.

### Step 6 — Local verification sequence (macOS)

```bash
brew install golangci-lint gitleaks   # match CI pin
go install golang.org/x/vuln/cmd/govulncheck@latest
gofmt -l .                            # expect empty
go build ./... && go vet ./...
go test -race -count=1 ./...
go test -count=1 -cover ./internal/cli   # confirm ≥80%
./scripts/coverage.sh
golangci-lint run                     # triage per Step 2 policy
govulncheck ./...
gitleaks git --redact                 # full-history sweep; add .gitleaks.toml only if false positives (none expected)
```

### Step 7 — Branch / commits / PR

Branch `chore/ci-security-gates` (parallels `chore/formatting-cleanup`). Commits, each independently green:
1. `test(cli): cover runReview/runStats wiring in-process`
2. `chore(lint): add golangci-lint v2 config and resolve findings`
3. `chore(ci): add race, lint, govulncheck, gitleaks, and coverage gates`

Draft PR (`gh pr create --draft`) with gate-by-gate mapping to TECH_STACK §7 / SEC-3 / SEC-8 / SEC-10, a CON-2 statement (existing gates untouched, only strengthened), and the note that every `//nolint` carries a reason. Request Copilot review per the standard ritual.

## Verification

1. Full local sequence in Step 6 green — this is "all tests pass" including the new gates.
2. Push branch; **definition of done = all seven CI jobs green on the draft PR** (test×3 with -race, lint, vulncheck, gitleaks, coverage, network-denied).
3. Zero app-behavior change: no diffs outside `.github/`, `.golangci.yml`, `scripts/`, `internal/cli/run_test.go`, and contingent one-line lint fixes.
4. Human merges (CON-4) — the agent never merges.

## Files

- New: `internal/cli/run_test.go`, `.golangci.yml`, `scripts/coverage.sh`, (`.gitleaks.toml` only if needed)
- Modified: `.github/workflows/ci.yml`; contingent one-liners in `internal/review/service.go`, `internal/storage/db.go`

## Verify at implementation time

Action major versions (golangci-lint-action, govulncheck-action, gitleaks-action) and latest golangci-lint v2 minor; whether the `std-error-handling` preset absorbs the `Close` errcheck hits; exact gosec finding set; govulncheck vs Go 1.25.1 stdlib; Windows -race runtime.
