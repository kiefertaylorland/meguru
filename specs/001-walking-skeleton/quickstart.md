# Quickstart: Validating the Walking Skeleton (M1)

Prerequisites: Go toolchain matching `go.mod`, a POSIX shell (macOS/Linux) or PowerShell
(Windows), and a clean `$XDG_DATA_HOME` (or equivalent) so the "first run" scenarios are genuine.

## 1. Build

```sh
go build -o bin/meguru ./cmd/meguru
```

## 2. First-run seed + due card (User Story 1, FR-001/002/003, SC-001)

```sh
# ensure a clean profile
rm -rf "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"   # macOS/Linux
# Windows (PowerShell): Remove-Item -Recurse -Force "$env:LOCALAPPDATA\meguru"

time ./bin/meguru review
```

**Expected**: completes in well under 5 seconds; a hiragana card is shown; no network activity
(verify with your OS's connection monitor, or run under the network-denied CI job — see §5).
Run it a second time immediately: the same due cards are shown (or a "nothing due" message if the
tester also rated them), never duplicated content.

## 3. Answer a card and confirm persistence (User Story 2, FR-006/007/008)

```sh
./bin/meguru review
# rate the shown card "Again"
./bin/meguru review
# expected: card is due again almost immediately (or shown again on this same invocation
# if enough wall-clock time has passed within the 1-minute window)
```

Then rate a (different, or the same after it comes due) card "Easy" and re-run `review`
immediately — the card must NOT reappear as due (7-day interval).

Inspect the DB directly to confirm the review record:

```sh
sqlite3 "${XDG_DATA_HOME:-$HOME/.local/share}/meguru/meguru.db" \
  "SELECT rating, reviewed_at FROM review_log ORDER BY id DESC LIMIT 5;"
```

## 4. Plain mode / NO_COLOR (User Story 3, FR-010/011, SC-003)

```sh
./bin/meguru review --plain | cat -v   # cat -v reveals any escape sequences
```

**Expected**: no `^[` (ESC) sequences in the output; a full session (see a card, submit a rating)
completes via plain sequential stdin/stdout.

```sh
NO_COLOR=1 ./bin/meguru review | cat -v
```

**Expected**: no color escape codes, but redraw/layout escape sequences (cursor movement) are
still permitted since this is not `--plain`.

## 5. File permissions (User Story 4, FR-012/013, SC-004)

```sh
./bin/meguru review >/dev/null
stat -f "%Sp" "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"          # macOS
stat -c "%a" "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"           # Linux
stat -f "%Sp" "${XDG_DATA_HOME:-$HOME/.local/share}/meguru/meguru.db"
```

**Expected**: directory `700`, file `600`. Then loosen permissions and re-run to confirm self-heal:

```sh
chmod 755 "${XDG_DATA_HOME:-$HOME/.local/share}/meguru"
chmod 644 "${XDG_DATA_HOME:-$HOME/.local/share}/meguru/meguru.db"
./bin/meguru review 2>&1 | grep -i permission   # expect a warning line
stat -c "%a" "${XDG_DATA_HOME:-$HOME/.local/share}/meguru/meguru.db" # expect 600 again
```

## 6. Interrupted review leaves no partial state (Edge Case, FR-015, SC-006)

```sh
./bin/meguru review &
PID=$!
sleep 1        # after the card is shown, before a rating is entered
kill -9 $PID
./bin/meguru review
```

**Expected**: the same card that was shown before the kill is shown again as due; no corrupt or
partial row appears in `review_log`.

## 7. Automated test suites (maps to tasks.md, not hand-run normally)

```sh
go test ./...                                  # unit + integration
go test -tags networkdenied ./tests/e2e/...    # only meaningful inside the sandboxed CI job —
                                                # see research.md §6 for how network is actually blocked
```

CI (GitHub Actions) runs this matrix on ubuntu/macos/windows per FR-016, with the network-denied
job as an additional ubuntu-only step per FR-017/SC-005 — running it locally without the OS-level
network block only proves the code makes no calls by inspection, not by enforcement.
