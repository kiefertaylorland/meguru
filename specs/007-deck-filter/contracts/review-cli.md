# Contract: `meguru review --deck` (extends `specs/001-walking-skeleton/contracts/cli.md`)

## `meguru review --deck <slug>`

**New flag**:

| Flag | Type | Default | Behavior |
| --- | --- | --- | --- |
| `--deck` | string | `""` (unfiltered) | Scopes the session to one built-in deck's due cards. Applies identically in `--plain` and interactive mode. |

**Valid values**: `kana-hiragana`, `kana-katakana`, `jlpt-n5-kanji`, `jlpt-n5-vocab` (the existing
stable slugs from `internal/deck`).

**Behavior — recognized slug**: every due card shown during the session belongs to that deck
only, until nothing more is due in it. Rating still writes `review_log`/`srs_state` exactly as
before (unaffected by scope).

**Behavior — unrecognized value**: prints an error naming every valid slug and display name, does
not open the database, does not start a session, exits `1`. Example:

```text
unknown deck "bogus" — valid decks: kana-hiragana (Hiragana), kana-katakana (Katakana),
jlpt-n5-kanji (JLPT N5 Kanji), jlpt-n5-vocab (JLPT N5 Vocabulary)
```

**Behavior — no `--deck` given**: identical to `meguru review` before this feature — due cards
pulled from every deck together (FR-002).

**Behavior — scoped deck has nothing due**: the "nothing due" output names the deck, e.g.
"Nothing due in Hiragana right now." instead of the generic message, in both plain and interactive
mode (FR-008).

**Exit codes**: unchanged from `specs/001-walking-skeleton/contracts/cli.md` — `0` on any clean
exit (including a scoped session's "nothing due"), `1` on an unrecoverable error (including an
unrecognized `--deck` value).
