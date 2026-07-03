# Contract: CLI Surface (M1)

Meguru is a CLI tool (Cobra) whose primary user-facing contract is its command-line interface and
exit behavior — not a network API. This documents the M1 command surface.

## `meguru review`

**Purpose**: Run one review session against due cards (User Stories 1 & 2).

**Flags**:

| Flag | Type | Default | Behavior |
|---|---|---|---|
| `--plain` | bool | `false` | Force the linear, non-interactive renderer (FR-010). Also forced automatically when stdout is not a TTY. |

**Environment**:

| Var | Behavior |
|---|---|
| `NO_COLOR` | Any non-empty value suppresses color/style escape codes regardless of `--plain` (FR-011). Interactive redraws still occur if `--plain` was not also given and stdout is a TTY. |

**Startup sequence** (every invocation):

1. Resolve data dir via `adrg/xdg`; create dir (`0700`) + DB file (`0600`) if absent (FR-001).
2. If dir/file already exist with looser permissions, chmod them back and print a warning to
   stderr (FR-013).
3. Run any pending schema migrations (FR-014).
4. Seed or update-in-place the embedded hiragana deck (FR-002/003/004).
5. Query for the next due card.

**Behavior — due card exists**:
- Interactive mode: Bubble Tea program renders the card, accepts one keypress from a fixed set
  mapped to Again/Hard/Good/Easy, then loops to the next due card until none remain.
- Plain mode: prints the card as plain text, reads one line of stdin as the rating (accepts the
  rating word or its first letter), echoes what was recorded, then loops.

**Behavior — no due card**: prints/renders a clear "nothing due right now" message (FR-005) and
exits 0. This is not an error condition.

**Exit codes**:

| Code | Meaning |
|---|---|
| `0` | Session completed (including "nothing due") |
| `1` | Unrecoverable error (e.g. DB open failure, corrupt embedded deck) |

**Non-functional guarantees** (contract, not just implementation detail):
- Zero network calls at any point during this command's execution (FR-009/P-1).
- Interrupting the process (SIGKILL/crash) after a card is displayed but before a rating is
  submitted leaves no partial `review_log` row; the card remains due on next invocation (FR-015).

## `meguru` (root, no subcommand)

M1 scope: root command with no args prints help/usage and exits 0. No default-to-review behavior
is required this milestone (out of scope beyond the `review` subcommand itself).
