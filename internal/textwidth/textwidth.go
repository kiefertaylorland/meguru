// Package textwidth is the sole place display-width math happens across the
// TUI and plain renderers, so CJK-wide characters (e.g. hiragana) are always
// measured correctly. No user-visible string should be measured with len()
// or utf8.RuneCountInString.
package textwidth

import (
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// Width returns the terminal display width of s, accounting for East-Asian
// wide characters and grapheme cluster boundaries.
func Width(s string) int {
	total := 0
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		total += clusterWidth(gr.Str())
	}
	return total
}

// Truncate shortens s to fit within maxWidth display columns without
// splitting a grapheme cluster.
func Truncate(s string, maxWidth int) string {
	if Width(s) <= maxWidth {
		return s
	}
	var result []rune
	width := 0
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		w := clusterWidth(gr.Str())
		if width+w > maxWidth {
			break
		}
		result = append(result, gr.Runes()...)
		width += w
	}
	return string(result)
}

func clusterWidth(cluster string) int {
	w := runewidth.StringWidth(cluster)
	if w == 0 && cluster != "" {
		return 1
	}
	return w
}
