# Phase 1 Data Model: Romaji Answer Input

**N/A — no schema migration, no new persisted table or column.**

This feature introduces no new storage concept. It adds one pure conversion function
(`internal/romaji.ToHiragana`) and one pure comparison function
(`internal/review.CheckAnswer`), both operating on values that already exist in memory for the
duration of a single review interaction:

- The typed romaji string (never persisted — it lives only as a local variable in
  `internal/plain.Run`'s loop body for the duration of one card's interaction).
- `review.Card.Reading`, already loaded from `notes.fields` by the existing
  `Service.NextDueCard` (see `specs/001-walking-skeleton/data-model.md` for that field's
  schema origin — unchanged by this feature).

No new column, table, migration, or index is added to `internal/storage/migrations/`. The
persisted record of a review interaction remains exactly what M1/M2 already write: a
`review_log` row created by `Service.Rate`, keyed on the rating the learner ultimately submits —
this feature does not add a "was the typed answer correct" column to that row, since spec.md's
Key Entities section defines the typed answer and its check result as ephemeral, not part of the
durable review history.

## In-memory-only shapes introduced

### `review.AnswerResult`

```go
type AnswerResult struct {
    Kana    string // typedRomaji converted to hiragana via romaji.ToHiragana
    Correct bool   // whether the typed answer matched card.Reading
}
```

Produced by `review.CheckAnswer(card *Card, typedRomaji string) AnswerResult` and consumed
immediately by the calling UI layer (`internal/plain` this slice) to decide what feedback line to
print. Never serialized, never stored.
