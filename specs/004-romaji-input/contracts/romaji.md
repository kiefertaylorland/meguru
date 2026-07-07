# Contract: `internal/romaji`

Internal package contract only — no external (CLI/API) surface. `internal/romaji` has zero
imports outside the Go standard library and zero dependency on storage, TUI, or CLI packages
(FR-007).

## Public surface

```go
package romaji

// ToHiragana converts a romaji (Hepburn-style) string to hiragana.
func ToHiragana(input string) string
```

That is the entire public surface. No types, no options, no configuration — a single pure
function.

## Behavior

- Input is lowercased before matching (so `"KA"`, `"Ka"`, `"ka"` all convert identically).
- Matching is greedy longest-match-first at each position: sokuon detection, then a 3-rune
  digraph/monograph, then a 2-rune digraph/monograph, then a 1-rune monograph (vowels, bare `n`).
- Recognized mora: all five vowels (`a i u e o`); the k/s/t/n/h/m/y/r/w consonant rows including
  alternate spellings `si→し`, `ti→ち`, `tu→つ`, `di→ぢ`, `zi→じ`; the dakuten/handakuten rows
  g/z/d/b/p; `wo→を`; bare `n→ん`; and the yōon digraphs for every row that has them
  (kya/sha(=sya)/cha(=tya=cya)/nya/hya/mya/rya/gya/ja(=jya=zya)/bya/pya, and their `-yu`/`-yo`
  counterparts).
- Sokuon: a doubled consonant letter (excluding vowels and `n`) converts to a small tsu (`っ`)
  followed by the mora starting at the second occurrence, e.g. `"kka"` → `"っか"`. The `tch`
  spelling (used for a geminate before the ch-row, e.g. `"matcha"`) is special-cased to the same
  effect: `っ` + the ch-row mora, e.g. `"matcha"` → `"まっちゃ"`.
- `n` disambiguation: `n` immediately followed by a vowel or `y` is matched as part of the na-row
  or nya-row digraph (longest-match-first already guarantees this); `n` in any other position
  (before another consonant, or at end of input) converts to `ん`. An apostrophe immediately
  after `n` (`n'`) forces the `ん` interpretation and is itself consumed (produces no output
  character), e.g. `"kon'ya"` → `"こんや"` vs. `"kinyoubi"` → `"きにょうび"` (documented
  ambiguity — see spec.md Edge Cases; use `"kin'youbi"` → `"きんようび"` to disambiguate).
- Unrecognized runes (whitespace, punctuation, digits, already-kana text, anything not matched
  above) are copied to the output unchanged — `ToHiragana` never errors and never panics on any
  input, including the empty string (`ToHiragana("") == ""`).

## Non-goals (explicitly out of scope, see research.md)

- Katakana output.
- Long-vowel contraction (macrons); long vowels are rendered mora-by-mora (e.g. `"ou"` →
  `"おう"`, two characters — standard hiragana orthography for most such words).
- Streaming/incremental conversion (partial-input-per-keystroke); this is a whole-string
  transformation called once per submitted line in this slice's integration.
