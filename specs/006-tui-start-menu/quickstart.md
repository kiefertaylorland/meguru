# Quickstart: Validating the TUI Start Menu & Full-Window Layout

Prerequisites: Go toolchain matching `go.mod`. No new dependencies to fetch — this feature adds
no `go.mod` entries, only new usage of `bubbletea/v2`/`lipgloss/v2` APIs already vendored.

## 1. Unit + golden tests for the TUI package

```sh
go test ./internal/tui/... -v
```

**Expected**: `update_test.go` covers start-menu navigation (arrow keys and `j`/`k`, clamping at
both ends), screen transitions (`Enter` on each menu item, `Esc` from the stats screen),
`tea.WindowSizeMsg` handling, and the new `fakeStatsService`-backed stats-fetch success/error
paths (contracts/tui-screens.md). `view_test.go` covers per-screen rendering plus the
"terminal too small" message below 80x24. `model_test.go`'s golden test drives the real flow
end-to-end via `teatest`: start menu appears first, navigate to "Start Review", `Enter`, then the
existing review assertions apply unchanged.

## 2. Confirm the CLI wiring

```sh
go test ./internal/cli/... -v
```

**Expected**: `review_test.go` (or an added case) confirms `runReview` constructs a
`stats.Service` and passes it into `tui.New` alongside the existing `review.Service`, with no
change to `--plain`/non-TTY dispatch behavior.

## 3. Manual check at multiple terminal sizes

```sh
go build -o bin/meguru ./cmd/meguru
rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"   # clean profile so a due card exists

./bin/meguru review
```

At each of the following, confirm the start menu (then stats, then review screen) fills the
window rather than rendering a small box, per SC-002:

- Resize the terminal to roughly 80x24 before/while running — content fills it, or if you shrink
  below this, a "terminal too small" message appears instead (SC-005).
- Resize to roughly 120x40 — content reflows to the larger size live (SC-003).
- Resize to roughly 200x60 — same.

While a card is revealed mid-review, resize the terminal and confirm the same card/reveal state
is still shown after the layout recalculates (SC-003, contracts/tui-screens.md).

From the start menu, select "View Stats" (arrow keys/`j`/`k` + `Enter`) and confirm the same
due/streak/retention figures `meguru stats` would report are shown; press `Esc` and confirm you
land back on the start menu (SC-004).

Confirm `q`/`Ctrl+C` exits immediately (exit code 0) from the start menu, the stats screen, and
the review screen.

## 4. Regression: `--plain` and full suite unaffected

```sh
./bin/meguru review --plain <<< "good"
go test ./...
```

**Expected**: `--plain` output and behavior are byte-for-byte identical to before this feature
(FR-012) — no menu, no window-size concept, unchanged linear text flow. The full test suite
(`cli`, `tui`, `plain`, `storage`, `deck`, `scheduler`, `review`, `stats`, `textwidth`) stays
green, confirming this feature's scope stayed inside `internal/tui` and the one additive wiring
change in `internal/cli/review.go`, exactly as planned.
