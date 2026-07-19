# Phase 1 Data Model: TUI Start Menu & Full-Window Layout

No database schema changes. This feature adds only in-memory UI state to `internal/tui.Model`;
nothing here is persisted.

## Screen

Which of the interactive TUI's three screens is currently active. Governs both what `Update`
does with a keypress and what `View` renders.

| Value | Meaning |
| --- | --- |
| `screenStartMenu` | The menu of actions is shown; this is the initial screen on launch. |
| `screenStats` | The stats summary (due count, total cards, streak, retention) is shown, read-only. |
| `screenReview` | The existing card-review flow (front/back reveal, rating) is active. |

**Transitions**:

```text
screenStartMenu --(select "Start Review", Enter)--> screenReview
screenStartMenu --(select "View Stats", Enter)-----> screenStats
screenStartMenu --(select "Quit", Enter | q | ctrl+c)--> program exits
screenStats     --(Esc)-------------------------------> screenStartMenu
screenStats     --(q | ctrl+c)-------------------------> program exits
screenReview    --(existing behavior: nothing due, or q/ctrl+c)--> program exits
```

There is no transition back from `screenReview` to `screenStartMenu` mid-session — this matches
existing behavior, where the review loop runs until nothing is due or the user quits (spec.md is
scoped to adding the menu as an entry point and stats as a peer screen, not to changing the
review loop's own exit behavior).

## MenuItem

An entry in the start menu.

| Field | Type | Notes |
| --- | --- | --- |
| `Label` | string | Display text, e.g. "Start Review". |
| `Action` | enum (`actionStartReview`, `actionViewStats`, `actionQuit`) | What Enter does when this item is highlighted. |

The menu itself is a fixed-order slice of three `MenuItem`s built at `Model` construction time —
not user-configurable, no persistence (spec.md Assumptions: no new settings/preferences).

## Model field additions (`internal/tui/model.go`)

| Field | Type | Purpose |
| --- | --- | --- |
| `screen` | `screen` (enum above) | Current active screen; zero value is `screenStartMenu`. |
| `menuItems` | `[]MenuItem` | The fixed 3-item menu, built in `New`. |
| `menuSelected` | `int` | Index into `menuItems` of the currently highlighted item; clamped to `[0, len(menuItems)-1]` on every move (research.md #3 — no wraparound). |
| `width`, `height` | `int` | Latest terminal size from `tea.WindowSizeMsg`; zero until the first message arrives. |
| `statsSvc` | `stats.Service` | New constructor dependency (research.md #4); used only when `screen == screenStats`. |
| `statsSummary` | `*stats.Summary` | Result of the last `statsSvc.Compute` call; `nil` while loading. |
| `statsErr` | `error` | Set if `statsSvc.Compute` fails; rendered in place of the summary, does not quit the program (unlike `errMsg` on the review path, since a failed stats fetch shouldn't end the whole session — the user can back out with Esc). |

No existing field (`card`, `revealed`, `submitting`, `noneDue`, `quitting`, `err`) changes
meaning; they remain scoped to `screenReview` exactly as today.

## New message types (`internal/tui/model.go`)

| Type | Carries | Sent when |
| --- | --- | --- |
| `statsMsg` | `stats.Summary` | `statsSvc.Compute` succeeds. |
| `statsErrMsg` | `error` | `statsSvc.Compute` fails. |

(`tea.WindowSizeMsg` is a Bubble Tea built-in, not a new type — just a new `case` in `Update`.)
