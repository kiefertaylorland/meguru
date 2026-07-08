# Contract: `meguru stats` CLI Surface (M2)

Companion to `specs/001-walking-skeleton/contracts/cli.md`, which documents `meguru review` and
the root command. This documents the new `stats` subcommand added in this feature.

## `meguru stats`

**Purpose**: Report due-card count, total-card count, current streak, and 30-day retention from
the learner's local database (User Stories US-7, US-11).

**Flags**:

| Flag     | Type | Default | Behavior                                                                 |
| -------- | ---- | ------- | ------------------------------------------------------------------------ |
| `--json` | bool | `false` | Emit a single JSON object on stdout instead of human-readable text.      |

**Environment**:

| Var        | Behavior                                                                                          |
| ---------- | --------------------------------------------------------------------------------------------------- |
| `NO_COLOR` | No effect on this command's output — `stats` never emits ANSI color/style codes in either mode, in either `--json` or the default text mode, so there is nothing for `NO_COLOR` to suppress. This is stated explicitly as a contract guarantee, not left implicit. |

**Startup sequence** (every invocation):

1. Resolve data dir via `adrg/xdg`; create dir (`0700`) + DB file (`0600`) if absent — identical to
   `review`'s startup (contracts/cli.md steps 1–2).
2. Run any pending schema migrations (identical to `review`'s step 3).
3. **Unlike `review`**, `stats` does not seed or update any embedded deck — it has no reason to
   create data, only to read whatever already exists (including nothing, on a freshly created,
   never-reviewed database).
4. Compute the summary (due count, total count, streak, retention) as of the current time.

**Behavior — default (human-readable) mode**:

Prints one line per figure, left-aligned labels (reusing the `internal/textwidth`-based alignment
convention `internal/plain` already established for `review`'s field printer), for example:

```
Due now:            3
Total cards:        46
Streak:             4 day(s)
Retention (30d):    92%
```

When retention has no data to compute from (zero reviews in the last 30 days — which includes the
zero-reviews-ever case), the retention line reads:

```
Retention (30d):    n/a (no reviews yet)
```

When there are zero cards due right now, an additional line reports the next due time, if any
card has ever been scheduled:

```
Next due:           2026-07-08 09:15
```

or, if no card has ever been scheduled at all (e.g. a completely empty database):

```
Next due:           no cards scheduled
```

**Behavior — `--json` mode**:

Emits exactly one JSON object on stdout and nothing else (no banners, no trailing prose), so it is
safe to pipe directly into `jq` or store from a script:

```json
{
  "due_cards": 3,
  "total_cards": 46,
  "streak_days": 4,
  "retention_percent": 92.5,
  "retention_window_days": 30,
  "next_due_at": "2026-07-08T09:15:00Z"
}
```

Field notes:

- `retention_percent` is JSON `null` (not `0`) when there is no review history in the retention
  window — callers MUST check for `null` before treating the figure as a percentage (spec.md
  SC-003).
- `next_due_at`, when non-null, is an RFC 3339 UTC timestamp string (e.g.
  `"2026-07-08T09:15:00Z"`), matching every other timestamp format already used by this codebase
  (`internal/review/service.go`'s `now.UTC().Format(time.RFC3339)` convention). It is `null` only
  when no card has ever been scheduled (an empty database).
- All integer fields (`due_cards`, `total_cards`, `streak_days`, `retention_window_days`) are
  always present and never `null`.

**Exit codes**:

| Code | Meaning                                                                    |
| ---- | --------------------------------------------------------------------------- |
| `0`  | Stats computed and printed successfully — including on an empty database, zero due cards, and zero review history. None of these are error conditions. |
| `1`  | Unrecoverable error (e.g. DB open failure, corrupt database file, migration failure). |

**Non-functional guarantees** (contract, not just implementation detail):

- Zero network calls at any point during this command's execution (FR-006/P-1).
- This command never writes to the database — it opens the same connection `review` does (so
  migrations still run against a never-before-opened DB file) but issues only `SELECT` statements
  against `cards`, `srs_state`, and `review_log`.
- Running `stats` repeatedly against an unchanged database returns identical output (pure function
  of the DB contents and the current time — `streak_days`/`next_due_at` can only change once
  real time passes a relevant boundary, e.g. midnight local time or a card's `due_at`).

## `meguru` (root, no subcommand) — updated

`stats` is now listed alongside `review` when the root command's help/usage is printed. No other
change to root-command behavior from `specs/001-walking-skeleton/contracts/cli.md`.
