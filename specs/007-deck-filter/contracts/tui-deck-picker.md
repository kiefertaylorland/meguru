# Contract: Deck Picker Screen (extends `specs/006-tui-start-menu/contracts/tui-screens.md`)

## Start Menu (updated)

The start menu's item list grows from three to four: **Start Review, Study a Deck, View Stats,
Quit** — in that order. "Start Review" and "View Stats"/"Quit" behavior is otherwise unchanged
from `specs/006-tui-start-menu/contracts/tui-screens.md`, except that "Start Review" now reviews
whatever the current deck scope is (unfiltered by default, or whatever `--deck`/a prior "Study a
Deck" visit this session set it to — research.md #5).

## Screen: Deck Picker

**Shown**: After selecting "Study a Deck" from the Start Menu.

**Content**: A list of the four built-in decks by display name (Hiragana, Katakana, JLPT N5
Kanji, JLPT N5 Vocabulary).

**Keybindings**:

| Key | Effect |
| --- | --- |
| `↓` / `j` | Move highlight to the next deck (clamps at the last). |
| `↑` / `k` | Move highlight to the previous deck (clamps at the first). |
| `Enter` | Set the active deck scope to the highlighted deck and begin the Review screen, scoped to only that deck. |
| `Esc` | Return to the Start Menu; the active deck scope is unchanged. |
| `q` / `Ctrl+C` | Exit the program (exit code 0). |

## Screen: Review (updated)

While a deck scope is active, the screen shows which deck is being studied (e.g. a "Studying:
Hiragana" line), and its "nothing due" state names the deck (FR-008/FR-009) instead of the
generic message. With no scope active, the screen is unchanged from
`specs/006-tui-start-menu/contracts/tui-screens.md`.

## Exit codes

Unchanged: `0` on any clean exit, `1` on an unrecoverable error.
