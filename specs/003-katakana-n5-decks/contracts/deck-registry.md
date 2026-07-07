# Contract: `internal/deck` built-in deck registry

Internal package contract only — `internal/deck` has no external (CLI/API) surface; this document
is the contract between `internal/deck`'s registry/seed logic and its two callers,
`internal/cli/review.go` (`deck.Seed`) and any test/tooling code that enumerates built-in decks
(`deck.BuiltinDecks`), replacing the M1-era single-hiragana-deck contract this package used to
expose.

## Public surface

```go
package deck

type Note struct {
    Expression string `json:"expression"`
    Reading    string `json:"reading"`
    Meaning    string `json:"meaning"`
}

type Content struct {
    ContentVersion int    `json:"content_version"`
    Notes          []Note `json:"notes"`
}

type Definition struct {
    Slug string
    Name string
    Kind string
    // raw func() []byte exists in implementation as an unexported, package-owned content loader.
}

func (d Definition) Content() (Content, error)

func BuiltinDecks() []Definition

const (
    HiraganaSlug    = "kana-hiragana"
    KatakanaSlug    = "kana-katakana"
    JLPTN5KanjiSlug = "jlpt-n5-kanji"
    JLPTN5VocabSlug = "jlpt-n5-vocab"
)

func Hiragana() (Content, error)
func Katakana() (Content, error)
func JLPTN5Kanji() (Content, error)
func JLPTN5Vocab() (Content, error)

func Seed(ctx context.Context, db *sql.DB, now time.Time) error
```

## Preconditions

- `BuiltinDecks()` MUST return a fixed, non-empty slice with no duplicate `Slug` values and no
  empty `Slug`/`Name`/`Kind` fields — enforced by `TestBuiltinDecks_AllDistinctAndValid`.
- Every `Definition.Kind` returned by `BuiltinDecks()` MUST be one of the values allowed by
  `decks.kind`'s CHECK constraint (`internal/storage/migrations/0001_init.sql`): `'kana'`,
  `'kanji'`, `'vocab'`, `'keigo'`, `'sentence'`. `Seed` does not itself validate this — an invalid
  `Kind` surfaces as a SQL CHECK-constraint failure from `seedFresh`'s `INSERT`, which is treated as
  a programmer error (a bad `Definition` in the registry), not a runtime input-validation concern.
- `db` passed to `Seed` MUST already be migrated (`storage.Migrate`) — `Seed` does not create
  tables itself, matching M1's existing contract with `internal/cli/review.go`'s startup sequence.

## Postconditions

- `Seed` MUST create exactly one `decks` row per `Definition` in `BuiltinDecks()` on a fresh
  database, with that deck's full note/card/srs_state set, and MUST NOT create more than one
  `decks` row per `Slug` on any number of subsequent calls (FR-004, SC-002).
- `Seed` MUST NOT touch (`UPDATE`) any row belonging to a deck whose stored `content_version`
  already equals its embedded `Content.ContentVersion` (FR-004's "Nth startup" edge case).
- `Seed` MUST update only the one deck whose embedded `Content.ContentVersion` has increased,
  leaving every other built-in deck's `decks`/`notes`/`cards`/`srs_state`/`review_log` rows
  untouched (FR-005, SC-004) — this includes not re-touching `updated_at` on notes whose content
  did not change.
- `Seed` MUST parse every built-in `Definition` before writing; if any embedded JSON fails to parse,
  no `decks`/`notes`/`cards`/`srs_state` rows are written. A write failure partway through seeding
  the built-in set (e.g., a bad `INSERT`) MUST roll back all changes from that `Seed` call and return
  the error immediately (data-model.md's "Seed flow" section).
- `Definition.Content()` MUST return an error (not panic, not silently return zero-value content)
  if its embedded JSON fails to parse — the error MUST name the failing deck's slug
  (`"parse embedded %s deck"`), matching M1's existing error-message contract for `Hiragana()`.

## Caller contract (`internal/cli/review.go`)

`runReview` MUST, exactly as it does today, call `deck.Seed(ctx, db, time.Now())` once at startup
after `storage.Migrate`, before constructing `review.NewService(db)` — no change to this call site
is required or made by this feature, since `Seed`'s exported signature is unchanged from M1.

## Caller contract (`internal/review/service.go`)

None — `internal/review` does not call anything in this contract directly. It reads
`cards`/`notes`/`srs_state` rows generically (no deck-slug or deck-kind filtering), which is what
makes every deck `Seed` creates automatically reviewable with zero changes to `internal/review`
(research.md's "no change to `internal/review`" decision).
