# Contract: Interactive TUI Screens & Keybindings (`meguru review`, interactive mode)

This extends `specs/001-walking-skeleton/contracts/cli.md`'s `meguru review` contract with the
three-screen structure this feature introduces. Applies only to the interactive TUI (not
`--plain`, not a non-TTY stdout — see that file's existing dispatch rule, unchanged here).

## Screen: Start Menu (initial screen)

**Shown**: Immediately when the interactive session opens, before any due-card loading.

**Content**: A list of exactly three actions: "Start Review", "View Stats", "Quit".

**Keybindings**:

| Key | Effect |
| --- | --- |
| `↓` / `j` | Move highlight to the next item (clamps at the last item). |
| `↑` / `k` | Move highlight to the previous item (clamps at the first item). |
| `Enter` | Activate the highlighted item. |
| `q` / `Ctrl+C` | Exit the program (exit code 0). |

**Activating "Start Review"** transitions to the Review screen and begins the existing due-card
load (unchanged from `specs/001-walking-skeleton/contracts/cli.md`).

**Activating "View Stats"** transitions to the Stats screen.

**Activating "Quit"** exits the program (exit code 0), identical to `q`/`Ctrl+C`.

## Screen: Stats

**Shown**: After selecting "View Stats" from the Start Menu.

**Content**: Due-card count, total card count, current streak (days), and retention percentage
(or an explicit "unavailable" indicator with no review history) — the same figures `meguru stats`
reports, computed via the same `internal/stats.Service`.

**Keybindings**:

| Key | Effect |
| --- | --- |
| `Esc` | Return to the Start Menu. |
| `q` / `Ctrl+C` | Exit the program (exit code 0). |

If the stats computation fails, an error message is shown in place of the figures; `Esc` still
returns to the Start Menu (a stats-fetch failure does not end the session).

## Screen: Review

**Shown**: After selecting "Start Review" from the Start Menu.

**Behavior**: Unchanged from `specs/001-walking-skeleton/contracts/cli.md`'s existing `meguru
review` contract — reveal, rate (1-4/a/h/g/e), loop until nothing is due, `q`/`Ctrl+C` to quit at
any point.

## Full-window layout (all screens)

- Every screen renders to fill the terminal's current width and height (tracked via
  `tea.WindowSizeMsg`), not a small fixed-size box.
- The alt-screen buffer is active for the whole interactive session.
- Resizing the terminal at any point recalculates the layout without crashing and without losing
  the active screen's state (e.g. a revealed card, the stats already fetched, the current menu
  selection).
- Below 80 columns x 24 rows, every screen instead shows a "terminal too small" message.

## Exit codes

Unchanged from `specs/001-walking-skeleton/contracts/cli.md`: `0` on any clean exit (including
quitting from the Start Menu or Stats screen), `1` on an unrecoverable error.
