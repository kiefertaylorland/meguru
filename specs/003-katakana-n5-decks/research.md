# Phase 0 Research: Katakana + JLPT N5 Kanji/Vocab Built-In Decks

No `NEEDS CLARIFICATION` markers remain in the Technical Context — the design was validated
against the actual M1 `internal/deck`/`internal/review` code before this spec was written. This
document records the decisions and rationale for the record.

## Decision: Generalize via a `Definition` registry, not a per-deck function

**Decision**: Replace the hardcoded `HiraganaSlug`/`Hiragana()`/single-embed shape in `embed.go`
with a `Definition{Slug, Name, Kind, raw func() []byte}` struct, a fixed `builtinDecks []Definition`
slice, and an exported `BuiltinDecks() []Definition` accessor. `seed.go`'s `Seed` becomes a loop
over `BuiltinDecks()`, calling one shared `seedDeck`/`seedFresh`/`updateInPlace`/`insertNote` set
of functions parameterized by `Definition` instead of the hiragana-specific constants they used to
close over.

**Rationale**: This is the minimal generalization that satisfies FR-003 ("one shared seed/
update-in-place implementation... adding or updating a built-in deck's content MUST NOT require
writing new per-deck seeding code") without introducing a plugin system, config file format, or
runtime deck-loading mechanism — none of which any current requirement asks for. Every field
`seedFresh`'s `INSERT INTO decks` statement needs (`slug`, `name`, `kind`) was already a per-call
argument in disguise (hardcoded to hiragana's values); making them `Definition` fields is a
mechanical extraction, not a new concept.

**Alternatives considered**: A `map[string]Content` keyed by slug — rejected, loses the `Name`/
`Kind` metadata `seedFresh` needs without a parallel lookup structure, and a struct is simpler to
extend than two structures kept in sync. A directory-scanning/dynamic deck-discovery mechanism
(e.g., globbing `*.json` under `internal/deck/`) — rejected as speculative scope: this feature adds
exactly three known files, and Go's `//go:embed` doesn't support runtime-discovered file lists
without extra tooling that four fixed files don't justify (Simplicity First).

## Decision: Keep `Content`/`Note` shape unchanged; reuse verbatim for kanji/vocab

**Decision**: Katakana, N5 kanji, and N5 vocab decks use the exact same
`{content_version, notes: [{expression, reading, meaning}]}` JSON envelope hiragana already uses —
no new fields, no per-kind schema variant.

**Rationale**: `expression`/`reading`/`meaning` already generalizes cleanly to kanji (character /
romaji reading / English meaning) and vocab (word / romaji reading / English meaning) without
strain — `internal/review/service.go`'s `Card` struct and rendering in `internal/tui`/
`internal/plain` already only ever display these three fields regardless of deck kind, so no
kind-specific field (e.g., stroke order, part of speech) is needed to satisfy this feature's scope
(guided kana + built-in N5 kanji/vocab content, not a full dictionary feature).

**Alternatives considered**: A richer per-kind schema (e.g., separate on'yomi/kun'yomi fields for
kanji, part-of-speech for vocab) — rejected as speculative scope beyond what US-1/US-5 or any
current UI surface consumes; `internal/tui`/`internal/plain` would need matching changes to
display richer fields, which is explicitly out of scope for this slice ("No UI or CLI changes").

## Decision: Curated starter subsets for N5 kanji/vocab, not full lists

**Decision**: Ship 30 N5 kanji entries and 30 N5 vocab entries — a curated, correctness-first
starter subset — rather than the full ~100-entry N5 kanji list or full N5 vocabulary list.

**Rationale**: The task scope explicitly calls for "a modest curated starter subset (roughly
20-40 entries each)... do not attempt to embed the full list in this slice." Prioritizing a smaller
set of entries the author is confident are correct (numbers 1-10, days/time vocabulary, common
verbs/adjectives, basic nouns — all extremely well-established N5 content with no ambiguity in
reading or meaning) over a larger set risks introducing subtle errors under this review's time
budget. Expansion is a pure content-version bump using the exact update-in-place mechanism this
feature generalizes (research.md's first decision) — no new pipeline work, just a future PR editing
the JSON file and bumping `content_version`.

**Alternatives considered**: Scraping or generating a full N5 list programmatically — rejected;
CON-3 requires curated, correct fixtures, and mass-generating vocabulary/kanji data without manual
verification risks incorrect readings/meanings shipping in a learning tool where "accuracy
matters" per the task brief. Splitting into a separate "expand N5 decks" feature immediately after
this one — deferred to actual future work rather than done speculatively now, consistent with
Simplicity First (no unrequested scope).

## Decision: No change to `internal/review`

**Decision**: `internal/review/service.go`'s `NextDueCard` and `Rate` are left untouched.

**Rationale**: `NextDueCard`'s query (`SELECT c.id, n.fields FROM cards c JOIN notes n ... JOIN
srs_state st ... WHERE st.due_at <= ? ORDER BY st.due_at ASC LIMIT 1`) joins across all decks
generically — there is no `deck_id`/`slug` filter anywhere in the query or in `Rate`. Any note
seeded into any deck automatically becomes eligible to be the next due card the moment its
`srs_state.due_at` row exists, which `insertNote` (unchanged by this feature) already sets on
every note it creates, regardless of deck. This was confirmed by reading the file before writing
this spec, per the task's explicit instruction to verify rather than assume.

**Alternatives considered**: None — this is a confirmation of existing behavior, not a design
choice requiring alternatives.
