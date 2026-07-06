# Phase 1 Data Model: Walking Skeleton (M1)

Scope note: this milestone uses a subset of the full schema in `docs/product/TECH_STACK.md` §3.
Tables below are exactly what M1 needs; columns present in TECH_STACK.md but not exercised this
milestone (e.g. FSRS-specific fields on `srs_state`, the entire `ai_cache` table) are noted but
not required to be dropped — the full schema may be created now (matches the canonical spec) as
long as only the M1 subset is populated/exercised.

## Entities

### Deck

One row per deck; M1 has exactly one row (`slug = 'kana-hiragana'`).

| Field             | Type                       | Notes                                                                                       |
| ----------------- | -------------------------- | ------------------------------------------------------------------------------------------- |
| `id`              | INTEGER PK                 |                                                                                             |
| `slug`            | TEXT UNIQUE NOT NULL       | `'kana-hiragana'` for M1's only deck                                                        |
| `name`            | TEXT NOT NULL              | Display name, e.g. "Hiragana"                                                               |
| `kind`            | TEXT NOT NULL              | CHECK IN ('kana','kanji','vocab','keigo','sentence'); M1 uses `'kana'`                      |
| `source`          | TEXT NOT NULL              | CHECK IN ('builtin','user','ai'); M1 uses `'builtin'`                                       |
| `content_version` | INTEGER NOT NULL DEFAULT 1 | Bumped when embedded `hiragana.json`'s own version changes; drives update-in-place (FR-004) |
| `created_at`      | TEXT NOT NULL DEFAULT now  |                                                                                             |

**Validation**: `slug` unique; `kind`/`source` constrained by CHECK. `content_version` only
increases, never decreases (enforced in application code, not a DB constraint).

### Note

One fact from the deck; for hiragana, one character + reading + meaning.

| Field                       | Type                                             | Notes                                                   |
| --------------------------- | ------------------------------------------------ | ------------------------------------------------------- |
| `id`                        | INTEGER PK                                       |                                                         |
| `deck_id`                   | INTEGER NOT NULL FK → decks.id ON DELETE CASCADE |                                                         |
| `fields`                    | TEXT NOT NULL                                    | JSON: `{"expression":"あ","reading":"a","meaning":"a"}` |
| `tags`                      | TEXT NOT NULL DEFAULT '[]'                       | Unused by M1 logic; present for schema parity           |
| `created_at` / `updated_at` | TEXT NOT NULL                                    | `updated_at` bumped on content-version update-in-place  |

**Natural key for update-in-place**: `fields->>'expression'` within a given `deck_id` (see
research.md §3). Not a DB-level unique constraint in M1 (avoids JSON-path index complexity for a
one-deck milestone) — enforced by the seed/update code path.

### Card

Note × study direction. M1 hiragana deck uses a single direction per note (recognition: see kana,
recall its reading) — no multi-direction fan-out needed this milestone, but the table supports it
per the canonical schema.

| Field       | Type                                             | Notes                                                                   |
| ----------- | ------------------------------------------------ | ----------------------------------------------------------------------- |
| `id`        | INTEGER PK                                       |                                                                         |
| `note_id`   | INTEGER NOT NULL FK → notes.id ON DELETE CASCADE |                                                                         |
| `direction` | TEXT NOT NULL                                    | CHECK IN ('recognition','recall','production'); M1 uses `'recognition'` |
| —           | UNIQUE(note_id, direction)                       | Prevents duplicate cards per note+direction on re-seed                  |

### Study/Scheduling State (`srs_state`)

Per-card state; drives due-card selection.

| Field                      | Type                                       | Notes                                                                                                                                                                                                 |
| -------------------------- | ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `card_id`                  | INTEGER PK FK → cards.id ON DELETE CASCADE |                                                                                                                                                                                                       |
| `state`                    | TEXT NOT NULL DEFAULT 'new'                | CHECK IN ('new','learning','review','relearning'); M1 sets to `'learning'` after first review, `'review'` is not semantically load-bearing for the naive scheduler but kept for schema parity with M2 |
| `stability` / `difficulty` | REAL NOT NULL DEFAULT 0                    | **Unused by the naive scheduler** — left at 0, present only so the column exists for M2's FSRS swap without a migration                                                                               |
| `due_at`                   | TEXT                                       | Next-due timestamp; NULL until first review, then set by `scheduler.NextDue`                                                                                                                          |
| `last_review_at`           | TEXT                                       | Set on every review                                                                                                                                                                                   |
| `reps`                     | INTEGER NOT NULL DEFAULT 0                 | Incremented on every review                                                                                                                                                                           |
| `lapses`                   | INTEGER NOT NULL DEFAULT 0                 | Incremented when rating = Again                                                                                                                                                                       |
| Index                      | `idx_srs_due` on `due_at`                  | Powers "find next due card" query                                                                                                                                                                     |

**Initial state**: on seed, every new card gets an `srs_state` row with `due_at = NULL` (or
`now`, so it's immediately due) — decision: `due_at` set to seed time so first run has due cards
immediately (required for SC-001/User Story 1).

**Transitions** (M1, driven only by `scheduler.NextDue`):

```
new/learning --[rating]--> due_at = NextDue(rating, now); reps += 1; lapses += 1 if rating == Again
```

No other state machine complexity in M1 — `state` field transitions to `'learning'` on first
review and stays there (FSRS's fuller state machine is M2 scope).

### Review Record (`review_log`)

Append-only; one row per submitted rating. Never updated or deleted by normal operation.

| Field                                    | Type                                             | Notes                                                                                                                        |
| ---------------------------------------- | ------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------- |
| `id`                                     | INTEGER PK                                       |                                                                                                                              |
| `card_id`                                | INTEGER NOT NULL FK → cards.id ON DELETE CASCADE |                                                                                                                              |
| `rating`                                 | INTEGER NOT NULL                                 | CHECK BETWEEN 1 AND 4 (1=Again..4=Easy)                                                                                      |
| `reviewed_at`                            | TEXT NOT NULL                                    |                                                                                                                              |
| `state_before`                           | TEXT NOT NULL                                    | Copy of `srs_state.state` immediately before this review                                                                     |
| `stability_before` / `difficulty_before` | REAL                                             | **Unused/NULL in M1** — naive scheduler doesn't compute these; columns kept for schema parity so M2 doesn't need a migration |
| `elapsed_days`                           | REAL                                             | Days since `last_review_at`; NULL on first review                                                                            |
| `scheduled_days`                         | REAL                                             | Days between old `due_at` and new `due_at` computed by the naive rule                                                        |
| `duration_ms`                            | INTEGER                                          | **Unused in M1** — no timing UI yet; may be left NULL                                                                        |

**Write ordering (FR-015, interrupted-session safety)**: the review write path is: (1) user
submits rating → (2) single transaction: INSERT `review_log` row + UPDATE `srs_state` row,
committed atomically. A card is only ever considered "answered" once that transaction commits. If
the process dies after showing the card but before the rating is submitted, no transaction was
opened — the card's `due_at` is unchanged and it simply remains due on next run. No separate
"card shown" bookkeeping table is needed to satisfy FR-015.

### App State (`app_state`)

Generic key/value table from TECH_STACK.md; M1 uses it for exactly one purpose:

| Key              | Value (JSON) | Purpose                                                                            |
| ---------------- | ------------ | ---------------------------------------------------------------------------------- |
| `schema_version` | integer      | Read by `internal/storage/migrate.go` to decide which migrations to apply (FR-014) |

No other `app_state` keys are needed this milestone (no consent records/streaks yet — those are
later-milestone/AI-feature concerns).

## Entity Relationship Summary

```
Deck (1) ──< Note (many) ──< Card (many, per direction) ──1:1── SRS_State
                                          │
                                          └──< Review_Log (many, append-only)
```

This mirrors `docs/product/TECH_STACK.md` §3 exactly — M1 introduces no new tables or columns,
only a scoped-down usage of the existing canonical schema.
