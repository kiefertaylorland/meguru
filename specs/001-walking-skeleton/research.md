# Phase 0 Research: Walking Skeleton (M1)

No unresolved `NEEDS CLARIFICATION` markers remain in the Technical Context — this milestone's
stack is already decided in `docs/product/TECH_STACK.md`. This file records the specific
implementation decisions needed to turn that stack decision into a working walking skeleton.

## 1. SQLite driver & concurrency mode

- **Decision**: `modernc.org/sqlite` (pure Go, no CGo), opened with `_pragma=journal_mode(WAL)`
  and `_pragma=foreign_keys(1)` in the DSN.
- **Rationale**: Matches TECH_STACK.md exactly; pure-Go keeps the 3-OS CI matrix and
  cross-compilation trivial (no per-OS CGo toolchain). WAL mode avoids writer-blocks-reader stalls
  for a single-process desktop app and is what TECH_STACK.md specifies.
- **Alternatives considered**: `mattn/go-sqlite3` (CGo) — rejected, breaks pure cross-compilation
  and complicates the network-denied/3-OS CI gate with per-platform build steps.

## 2. First-run directory/file creation with correct permissions

- **Decision**: Resolve paths via `adrg/xdg`. Create the data directory with `os.MkdirAll(path, 0700)`
  before opening the DB file; open/create the DB file via `os.OpenFile(path, O_RDWR|O_CREATE, 0600)`
  first (to fix the createmask), then hand the path to the SQLite driver. On every startup, `os.Stat`
  both paths and compare `Mode().Perm()` against the expected mask; if broader, `os.Chmod` to the
  expected value and print a one-line warning to stderr (FR-013).
- **Rationale**: Doing the create+chmod ourselves (rather than trusting the SQLite driver's own
  file creation mode) guarantees permissions are correct even if the driver's default create mode
  ever changes, and gives a single place to implement the startup self-heal check.
- **Windows caveat**: POSIX permission bits are only partially meaningful on Windows (no
  group/other bits in the same sense). Decision: apply the same `os.Chmod`/`os.MkdirAll` calls
  unconditionally (Go's `os` package maps 0600/0700 to the nearest ACL-based equivalent — owner
  read/write, no other accounts), and let the E2E suite assert "no error, file created" on Windows
  rather than asserting exact octal bits cross-platform. The _intent_ (owner-only) is honored via
  Go's stdlib mapping; a byte-for-byte permission assertion only runs on the two POSIX OSes.

## 3. Embedded deck format & seed/update-without-duplication strategy

- **Decision**: `go:embed hiragana.json` holding an array of note objects
  (`{"expression":"あ","reading":"a","meaning":"..."}`) plus a top-level `content_version` field
  for the deck's own JSON envelope. Seed logic: on startup, look up the `decks` row by
  `slug = 'kana-hiragana'`. If absent, insert the deck row + all notes/cards fresh. If present and
  `content_version` in the embedded JSON is greater than the stored row's `content_version`,
  update existing notes' `fields` **in place by a stable natural key** (the `expression` field,
  since hiragana characters are unique and stable across releases) rather than by row id, then
  bump the stored `content_version` — never touching `cards`/`srs_state`/`review_log` rows for
  notes that already exist.
- **Rationale**: FR-004 requires content updates to preserve scheduling progress; keying the
  update by a stable natural identifier (not insertion order or row id) is what makes "update
  in place, no duplication" possible across releases. Matches the `decks.content_version` column
  already defined in TECH_STACK.md's schema.
- **Alternatives considered**: Re-seeding by dropping and re-inserting the deck on every version
  bump — rejected outright, violates FR-004 (would reset scheduling state / orphan review_log
  rows via cascade).

## 4. Naive placeholder scheduler shape

- **Decision**: `internal/scheduler/naive.go` exports a single pure function
  `NextDue(rating Rating, now time.Time) time.Time` implementing exactly the four fixed intervals
  from the spec (Again → now+1m, Hard → now+1d, Good → now+3d, Easy → now+7d). No stability/
  difficulty math, no state machine beyond what `srs_state.state` already models generically.
- **Rationale**: FR-008 explicitly forbids making this "more sophisticated than this simple
  interval rule." Isolating it as one pure function with the same signature shape FSRS will need
  (`(state, rating, now) -> (new state, due date)`, per TECH_STACK.md §4) means M2's FSRS swap
  touches only this file's internals and its call site, not `internal/review` or storage.
- **Alternatives considered**: Wiring in `go-fsrs` now with a "simple" parameter preset — rejected
  per spec Out-of-Scope ("FSRS itself" is excluded from this milestone) and CON-2 (no speculative
  complexity now for a later milestone).

## 5. TUI vs. plain-mode dispatch

- **Decision**: Cobra's `review` command checks `--plain` flag OR `!term.IsTerminal(os.Stdout.Fd())`
  OR `os.Getenv("NO_COLOR") != ""` at startup. `--plain` (or a non-TTY stdout) routes to
  `internal/plain`, a simple sequential `fmt.Println`-based renderer with a blocking
  `bufio.Scanner` prompt for the rating. `NO_COLOR` alone (TTY still interactive) routes to the
  normal Bubble Tea program but with Lip Gloss styles constructed via a no-color `lipgloss.Renderer`
  (`lipgloss.NewRenderer(os.Stdout, termenv.WithColorCache(true))` forced into `termenv.Ascii`
  profile) so interactive redraws remain but zero escape codes for color/style are emitted.
- **Rationale**: FR-010 and FR-011 are separate requirements (plain disables redraws entirely;
  NO_COLOR only suppresses color) and the spec's Assumptions section states they're independent
  and combinable — this dispatch keeps them as two independent checks rather than one combined
  flag.
- **Alternatives considered**: Forcing plain mode whenever `NO_COLOR` is set — rejected, spec
  Acceptance Scenario (User Story 3, #2) explicitly requires NO_COLOR alone to preserve
  interactive layout.

## 6. Network-denied CI proof (SEC-8 / FR-017 / SC-005)

- **Decision**: A dedicated Go integration test (build-tagged, e.g. `//go:build networkdenied`)
  runs the full core loop (temp dir, migrate, seed, review, rate, reschedule) inside a process
  whose network access is blocked at the OS/CI level — on Linux via `unshare --net` (or an
  equivalent sandboxed CI step), invoked from a dedicated GitHub Actions job/step that only runs
  this test on the ubuntu leg of the matrix. The test itself makes no network calls by construction
  (no `internal/ai` import); the OS-level block is the actual proof requested by FR-017, not just a
  code review claim.
- **Rationale**: FR-017 requires the test to _fail if network egress is attempted_, which means
  the enforcement must happen outside the Go process (a mock or code-level check could pass
  vacuously). Restricting the sandboxed network-denied job to one OS (ubuntu) matches SC-005's
  wording ("on at least one supported operating system") and avoids fighting Windows/macOS
  sandboxing primitives for a proof that only needs to run once.
- **Alternatives considered**: A `net.Dial` interceptor / monkey-patched transport inside the test
  binary — rejected, verifies the test author's assumptions rather than actual OS-level egress,
  weaker proof than what FR-017 asks for.

## 7. Two additional small dependencies not itemized in TECH_STACK.md (CON-2 justification)

- **Decision**: `github.com/mattn/go-isatty` (TTY detection for the `--plain`/non-TTY dispatch,
  FR-010) and `github.com/charmbracelet/colorprofile` (forcing an ASCII/no-color output profile
  for `NO_COLOR`, FR-011) are added as direct dependencies. Neither is named by that exact package
  path in `docs/product/TECH_STACK.md`, so this section is the CON-2-required stated justification.
- **Rationale**: FR-010 and FR-011 are binding functional requirements this milestone, and both
  require a real mechanism — not just intent — to detect a non-interactive stdout and to strip
  color codes from Bubble Tea's own render output. `go-isatty` is the same TTY-detection dependency
  Cobra/Bubble Tea's own ecosystem already pulls in transitively (`github.com/mattn/go-isatty` is a
  dependency of `github.com/mattn/go-runewidth`/terminal libraries already approved for this
  milestone); `colorprofile` is `charmbracelet`'s own companion package to `bubbletea/v2`
  (`charm.land/bubbletea/v2`'s `WithColorProfile` option is typed on `colorprofile.Profile`), so
  it ships from the same vendor and major-version family already approved in TECH_STACK.md's TUI
  stack, not an unrelated new supply-chain surface.
- **Alternatives considered**: `golang.org/x/term.IsTerminal` in place of `go-isatty` — equally
  valid and no more "approved" by name in TECH_STACK.md than `go-isatty`; not switched to it since
  it would not change the justification need, only the package name. Hand-rolling ASCII-downgrade
  color stripping instead of `colorprofile` — rejected, `tea.WithColorProfile` already types its
  parameter on `colorprofile.Profile`, so avoiding the dependency would mean reimplementing logic
  Bubble Tea already depends on and re-exports the type for.
