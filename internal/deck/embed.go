// Package deck holds the embedded, built-in study decks and the
// seed/update-in-place logic that loads them into storage.
package deck

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed hiragana.json
var hiraganaJSON []byte

//go:embed katakana.json
var katakanaJSON []byte

//go:embed jlpt_n5_kanji.json
var jlptN5KanjiJSON []byte

//go:embed jlpt_n5_vocab.json
var jlptN5VocabJSON []byte

// Slugs of the embedded built-in decks. These are the stable identifiers
// Seed (seed.go) looks decks up by, and content-version bumps key updates
// against.
const (
	HiraganaSlug    = "kana-hiragana"
	KatakanaSlug    = "kana-katakana"
	JLPTN5KanjiSlug = "jlpt-n5-kanji"
	JLPTN5VocabSlug = "jlpt-n5-vocab"
)

// Note is one fact from an embedded deck's JSON envelope.
type Note struct {
	Expression string `json:"expression"`
	Reading    string `json:"reading"`
	Meaning    string `json:"meaning"`
}

// Content is an embedded deck's parsed JSON envelope: its own content
// version plus the notes it contains.
type Content struct {
	ContentVersion int    `json:"content_version"`
	Notes          []Note `json:"notes"`
}

// Definition identifies one embedded built-in deck's stable identity — the
// row Seed creates/updates in the decks table — and how to load its raw
// embedded JSON. Adding a new built-in deck means adding one Definition
// here (plus its embedded JSON file); Seed's generic seed/update-in-place
// logic in seed.go needs no per-deck changes.
type Definition struct {
	Slug string
	Name string
	Kind string
	raw  func() []byte
}

// Content parses this definition's embedded JSON envelope.
func (d Definition) Content() (Content, error) {
	if d.raw == nil {
		return Content{}, fmt.Errorf("invalid Definition for %s: missing content loader", d.Slug)
	}
	var c Content
	if err := json.Unmarshal(d.raw(), &c); err != nil {
		return Content{}, fmt.Errorf("parse embedded %s deck: %w", d.Slug, err)
	}
	return c, nil
}

// builtinDecks is the fixed set of decks Seed loads into storage on every
// startup.
var builtinDecks = []Definition{
	{Slug: HiraganaSlug, Name: "Hiragana", Kind: "kana", raw: func() []byte { return hiraganaJSON }},
	{Slug: KatakanaSlug, Name: "Katakana", Kind: "kana", raw: func() []byte { return katakanaJSON }},
	{Slug: JLPTN5KanjiSlug, Name: "JLPT N5 Kanji", Kind: "kanji", raw: func() []byte { return jlptN5KanjiJSON }},
	{Slug: JLPTN5VocabSlug, Name: "JLPT N5 Vocabulary", Kind: "vocab", raw: func() []byte { return jlptN5VocabJSON }},
}

// BuiltinDecks returns a copy of the fixed list of embedded deck definitions
// that Seed loads into storage on every startup. The unexported raw loaders are
// immutable package functions shared by the copied Definition values.
func BuiltinDecks() []Definition {
	return append([]Definition(nil), builtinDecks...)
}

// Hiragana returns the parsed embedded hiragana deck content.
func Hiragana() (Content, error) { return lookupBuiltin(HiraganaSlug) }

// Katakana returns the parsed embedded katakana deck content.
func Katakana() (Content, error) { return lookupBuiltin(KatakanaSlug) }

// JLPTN5Kanji returns the parsed embedded N5 kanji deck content.
func JLPTN5Kanji() (Content, error) { return lookupBuiltin(JLPTN5KanjiSlug) }

// JLPTN5Vocab returns the parsed embedded N5 vocabulary deck content.
func JLPTN5Vocab() (Content, error) { return lookupBuiltin(JLPTN5VocabSlug) }

func lookupBuiltin(slug string) (Content, error) {
	for _, d := range builtinDecks {
		if d.Slug == slug {
			return d.Content()
		}
	}
	return Content{}, fmt.Errorf("unknown builtin deck %s", slug)
}
