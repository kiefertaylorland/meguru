# Phase 1 Data Model: Katakana + JLPT N5 Kanji/Vocab Built-In Decks

No schema migration this feature (see research.md). This document describes the `internal/deck`
Go types this feature introduces/generalizes and how the new embedded JSON content maps onto the
existing SQLite columns from `specs/001-walking-skeleton/data-model.md` — it is a mapping and
generalization document, not a new schema.

## Go types (`internal/deck`)

### `Note` (unchanged from M1)

```go
type Note struct {
    Expression string `json:"expression"`
    Reading    string `json:"reading"`
    Meaning    string `json:"meaning"`
}
```

Reused verbatim for every deck kind in this feature (kana, kanji, vocab) — see research.md's
"keep `Content`/`Note` shape unchanged" decision.

### `Content` (unchanged from M1)

```go
type Content struct {
    ContentVersion int    `json:"content_version"`
    Notes          []Note `json:"notes"`
}
```

One `Content` value per embedded JSON file (`hiragana.json`, `katakana.json`,
`jlpt_n5_kanji.json`, `jlpt_n5_vocab.json`).

### `Definition` (new this feature)

```go
type Definition struct {
    Slug string
    Name string
    Kind string
    raw  func() []byte // unexported: returns this definition's current embedded JSON bytes
}

func (d Definition) Content() (Content, error)
```

Replaces the hiragana-only hardcoded constants/function from M1's `embed.go`. One `Definition`
value exists per built-in deck; `raw` is a closure over the deck's `//go:embed`'d package variable
so tests can still swap embedded content per-deck via the existing M1 pattern (reassigning the
package-level `[]byte` var), without `Definition` itself needing to be mutable.

### `BuiltinDecks()` (new this feature)

```go
func BuiltinDecks() []Definition
```

Returns the fixed four-entry registry:

| `Slug` | `Name` | `Kind` | Source file |
| --- | --- | --- | --- |
| `kana-hiragana` | Hiragana | `kana` | `hiragana.json` (unchanged from M1) |
| `kana-katakana` | Katakana | `kana` | `katakana.json` (new) |
| `jlpt-n5-kanji` | JLPT N5 Kanji | `kanji` | `jlpt_n5_kanji.json` (new) |
| `jlpt-n5-vocab` | JLPT N5 Vocabulary | `vocab` | `jlpt_n5_vocab.json` (new) |

## Relationship to existing schema

No new tables, no new columns, no changed CHECK constraints — `decks.kind` already allows `'kana'`,
`'kanji'`, `'vocab'` (`internal/storage/migrations/0001_init.sql`). This feature is purely:

1. A new Go-level `Definition` type + fixed `BuiltinDecks()` list, replacing the M1 hardcoded
   single-hiragana-deck constants.
2. `internal/deck/seed.go`'s existing per-hiragana functions (`seedFresh`, `updateInPlace`,
   `insertNote`) parameterized by `Definition` instead of closed over hiragana's fixed values, and
   called once per entry in `BuiltinDecks()` from `Seed`.
3. Three new embedded JSON files providing the `Content` each new `Definition` parses.

`notes.fields`, `notes.deck_id`, `cards`, `srs_state`, `review_log` are populated identically for
every deck kind — `insertNote` has no deck-kind branching and needs none, since `Note`'s three
fields already cover kana, kanji, and vocab content uniformly (research.md).

## Seed flow (generalized)

```
Seed(ctx, db, now)
  parsed := parse every Definition in BuiltinDecks() // fail before writing anything
  begin one transaction for the full built-in set
  for each parsed Definition/content:
    seedDeck(ctx, tx, d, content, now)
      SELECT id, content_version FROM decks WHERE slug = d.Slug
      case no row:            seedFresh(ctx, tx, d, content, now)   // INSERT deck + all notes/cards/srs_state
      case version increased: updateInPlace(ctx, tx, deckID, content, now) // UPDATE notes.fields in place, bump content_version
      case version unchanged: no-op
  commit transaction
```

`Seed` parses every built-in deck before opening the write transaction, so malformed embedded JSON
cannot leave a partially seeded fresh profile. The write pass then runs in one transaction for the
full built-in set: any failed insert/update rolls back all changes from that `Seed` call and returns
the error immediately (FR-004's no-duplication guarantee, edge case: "what happens if any single
deck's insert fails outright").

## Content-version update-in-place (generalized, per deck)

Unchanged from M1's mechanism, now scoped correctly per deck: `updateInPlace(ctx, db, deckID,
content, now)` matches existing notes by `deck_id = ? AND json_extract(fields, '$.expression') =
?` — the `deck_id` scoping (already present in M1's SQL, since M1 always had exactly one deck) is
what makes this safe to run independently per deck once more than one deck exists: two different
decks may reuse the same `expression` string (unlikely for real content but not schema-forbidden)
without colliding, because the `WHERE` clause is deck-scoped.
