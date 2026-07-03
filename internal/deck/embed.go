// Package deck holds the embedded, built-in study decks (M1: hiragana only)
// and the seed/update-in-place logic that loads them into storage.
package deck

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed hiragana.json
var hiraganaJSON []byte

// HiraganaSlug identifies the single built-in hiragana deck's row in the
// decks table.
const HiraganaSlug = "kana-hiragana"

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

// Hiragana returns the parsed embedded hiragana deck content.
func Hiragana() (Content, error) {
	var c Content
	if err := json.Unmarshal(hiraganaJSON, &c); err != nil {
		return Content{}, fmt.Errorf("parse embedded hiragana deck: %w", err)
	}
	return c, nil
}
