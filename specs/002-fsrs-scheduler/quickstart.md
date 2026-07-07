# Quickstart: Validating Real FSRS Scheduling (M2, US-4)

Prerequisites: Go toolchain matching `go.mod`, this feature's `go-fsrs`/`pgregory.net/rapid`
dependencies added via `go get` + `go mod tidy` (see plan.md Technical Context).

## 1. Build and unit-test the scheduler in isolation

```sh
go build ./...
go test ./internal/scheduler/... -v
```

**Expected**: `internal/scheduler/fsrs_test.go`'s property-based tests pass across many
generated `(CardState, Rating, now)` inputs, and `fsrs_reference_test.go`'s pinned upstream FSRS
vectors match exactly. This is what proves FR-001–FR-004, FR-006, FR-007, and the
`contracts/scheduler.md` postconditions hold, without needing the DB or CLI at all.

## 2. Confirm the `internal/review` integration

```sh
go test ./internal/review/... -v
```

**Expected**: `service_test.go`'s updated assertions pass — in particular
`TestRate_AgainIncrementsLapses`'s corrected split (new-card Again vs. review-state Again, per
FR-007) and the widened `SELECT`/full-`Outcome` write path in `Rate`.

## 3. End-to-end interval-lengthening check (User Story 1, SC-001/SC-002)

```sh
go test ./tests/integration/... -run TestReview_RateAgainAndEasy_ReschedulesAndLogs -v
```

**Expected**: the rewritten structural assertions hold — an `Again`-rated card's due date is
before an `Easy`-rated card's due date on identical fresh cards (SC-002), and both are strictly
after `now` (contract postcondition).

For a manual, human-observable check of SC-001 (intervals lengthen over repeated good ratings):

```sh
go build -o bin/meguru ./cmd/meguru
rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"   # clean profile
./bin/meguru review   # rate the shown card "Good"
sqlite3 "${XDG_DATA_HOME:-$HOME/.local/share}/meguru/meguru.db" \
  "SELECT due_at, stability, difficulty, reps FROM srs_state LIMIT 1;"
# note the due_at gap from "now" and the stability value
./bin/meguru review   # if due again, rate "Good" a second time
sqlite3 "${XDG_DATA_HOME:-$HOME/.local/share}/meguru/meguru.db" \
  "SELECT due_at, stability, difficulty, reps FROM srs_state LIMIT 1;"
```

**Expected**: the second `due_at` is a strictly longer gap from its review time than the first
gap was, and `stability` has increased — confirming the schedule adapts per-card rather than
repeating a fixed interval (SC-001, SC-005: the CLI interaction itself is unchanged, only the
resulting schedule differs).

## 4. Regression: full suite + offline guarantee unaffected

```sh
go test ./...
```

**Expected**: all M1 packages (`cli`, `tui`, `plain`, `storage`, `deck`, `textwidth`) remain green
untouched, confirming this feature's scope stayed inside `internal/scheduler` and
`internal/review` as designed (Project Structure in plan.md). Re-run the existing network-denied
CI job (`.github/workflows/ci.yml`) to confirm the two new dependencies introduce zero egress
(P-1/SEC-8).
