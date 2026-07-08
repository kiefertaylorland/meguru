# Quickstart: Validating Dashboard Stats (M2, US-7/US-11)

Prerequisites: Go toolchain matching `go.mod`. No new dependencies to fetch — this feature adds no
`go.mod` entries.

## 1. Build and unit-test the stats package in isolation

```sh
go build ./...
go test ./internal/stats/... -v
```

**Expected**: `streak_test.go` and `retention_test.go`'s pure-function tests pass across the edge
cases from spec.md (zero reviews, broken streak, reviews only today, reviews spanning a
local/UTC day boundary, empty retention window). `stats_test.go`'s `Service.Compute` tests
against a real temp SQLite DB confirm the SQL layer matches the pure-function contracts. This
proves FR-001–FR-003 and the data-model.md derivation logic without needing the CLI at all.

## 2. Confirm the CLI wiring

```sh
go test ./internal/cli/... -v
```

**Expected**: `stats_test.go` confirms `--json` is registered with the right default, and that
JSON vs. plain output dispatch and rendering match `contracts/stats-cli.md`.

## 3. End-to-end check against a real binary

```sh
go build -o bin/meguru ./cmd/meguru
rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"   # clean profile

# First run: no reviews yet.
./bin/meguru stats
# Expected: due/total counts are 0 (stats does not seed; `review` hasn't run yet to seed the
# deck), streak is 0, retention reads "n/a (no reviews yet)".

./bin/meguru stats --json
# Expected: one JSON object, "retention_percent": null, "streak_days": 0.

# Seed one review via the plain review flow.
./bin/meguru review --plain <<< "good"

./bin/meguru stats
# Expected: streak is now 1, retention reflects the single "Good" rating (100%), due/total counts
# still accurate.

./bin/meguru stats --json | jq .
# Expected: valid JSON, "retention_percent": 100, "streak_days": 1.
```

**Note**: `meguru stats` never seeds decks itself (contracts/stats-cli.md) — running it against a
truly empty XDG profile before ever running `review` will show `total_cards: 0` and
`due_cards: 0`, which is correct, not a bug; run `review` at least once first (as above) to see
non-zero counts sourced from the embedded hiragana deck.

## 4. Regression: full suite + offline guarantee unaffected

```sh
go test ./...
```

**Expected**: all existing packages (`cli`, `tui`, `plain`, `storage`, `deck`, `scheduler`,
`review`, `textwidth`) remain green untouched, confirming this feature's scope stayed inside the
new `internal/stats` package and the additive `internal/cli/stats.go` file, exactly as planned.
Re-run the existing network-denied CI job (`.github/workflows/ci.yml`) to confirm `stats`
introduces zero egress (P-1/SEC-8) — no new dependency should even make this a meaningful risk,
but the gate still runs unmodified as a sanity check.
