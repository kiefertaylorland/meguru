// Package romaji converts Hepburn-style romaji input to hiragana. It is a
// pure string transformation with no dependency on storage, TUI, or CLI
// packages (contracts/romaji.md), used to auto-check a learner's typed
// answer against a card's expected kana reading (PRD US-3).
package romaji

import "strings"

// digraphs maps three-rune yōon spellings (a consonant + small y + vowel,
// e.g. "kya") to their single kana character.
var digraphs = map[string]string{
	"kya": "きゃ", "kyu": "きゅ", "kyo": "きょ",
	"gya": "ぎゃ", "gyu": "ぎゅ", "gyo": "ぎょ",
	"sha": "しゃ", "shu": "しゅ", "sho": "しょ",
	"sya": "しゃ", "syu": "しゅ", "syo": "しょ",
	"ja": "じゃ", "ju": "じゅ", "jo": "じょ",
	"jya": "じゃ", "jyu": "じゅ", "jyo": "じょ",
	"zya": "じゃ", "zyu": "じゅ", "zyo": "じょ",
	"cha": "ちゃ", "chu": "ちゅ", "cho": "ちょ",
	"tya": "ちゃ", "tyu": "ちゅ", "tyo": "ちょ",
	"cya": "ちゃ", "cyu": "ちゅ", "cyo": "ちょ",
	"nya": "にゃ", "nyu": "にゅ", "nyo": "にょ",
	"hya": "ひゃ", "hyu": "ひゅ", "hyo": "ひょ",
	"bya": "びゃ", "byu": "びゅ", "byo": "びょ",
	"pya": "ぴゃ", "pyu": "ぴゅ", "pyo": "ぴょ",
	"mya": "みゃ", "myu": "みゅ", "myo": "みょ",
	"rya": "りゃ", "ryu": "りゅ", "ryo": "りょ",
	"dya": "ぢゃ", "dyu": "ぢゅ", "dyo": "ぢょ",
}

// monographs maps single-mora romaji spellings (one or two romaji letters,
// occasionally three for e.g. "shi"/"chi"/"tsu") to their kana character.
// Alternate spellings for the same mora (si/shi, ti/chi, tu/tsu, di/zi/ji)
// are included so learners can type either convention.
var monographs = map[string]string{
	"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お",

	"ka": "か", "ki": "き", "ku": "く", "ke": "け", "ko": "こ",
	"ga": "が", "gi": "ぎ", "gu": "ぐ", "ge": "げ", "go": "ご",

	"sa": "さ", "shi": "し", "si": "し", "su": "す", "se": "せ", "so": "そ",
	"za": "ざ", "ji": "じ", "zi": "じ", "zu": "ず", "ze": "ぜ", "zo": "ぞ",

	"ta": "た", "chi": "ち", "ti": "ち", "tsu": "つ", "tu": "つ", "te": "て", "to": "と",
	"da": "だ", "di": "ぢ", "du": "づ", "de": "で", "do": "ど",

	"na": "な", "ni": "に", "nu": "ぬ", "ne": "ね", "no": "の",

	"ha": "は", "hi": "ひ", "fu": "ふ", "hu": "ふ", "he": "へ", "ho": "ほ",
	"ba": "ば", "bi": "び", "bu": "ぶ", "be": "べ", "bo": "ぼ",
	"pa": "ぱ", "pi": "ぴ", "pu": "ぷ", "pe": "ぺ", "po": "ぽ",

	"ma": "ま", "mi": "み", "mu": "む", "me": "め", "mo": "も",

	"ya": "や", "yu": "ゆ", "yo": "よ",

	"ra": "ら", "ri": "り", "ru": "る", "re": "れ", "ro": "ろ",

	"wa": "わ", "wo": "を",

	"n": "ん",
}

// ToHiragana converts a romaji (Hepburn-style) string to hiragana, per
// contracts/romaji.md. It lowercases the input, then walks it left to
// right using a greedy longest-match tokenizer: the n' disambiguator,
// sokuon (doubled consonants / "tch"), a 3-rune match, a 2-rune match, then
// a 1-rune match (vowels and bare "n"). Any rune that matches nothing is
// copied to the output unchanged, so this function never errors or panics
// on any input, including the empty string.
func ToHiragana(input string) string {
	runes := []rune(strings.ToLower(input))
	n := len(runes)
	var out strings.Builder

	for i := 0; i < n; {
		// n' disambiguator: the only case where an apostrophe is consumed,
		// forcing "n" to stand alone rather than extend into a na-row/nya-row
		// mora. Other apostrophes fall through to the unrecognized-rune passthrough.
		if i+1 < n && runes[i] == 'n' && runes[i+1] == '\'' {
			out.WriteString("ん")
			i += 2
			continue
		}

		// Sokuon: a doubled consonant (excluding vowels and "n") signals a
		// geminate consonant, rendered as small tsu before the following
		// mora. "tch" is the Hepburn spelling for a geminate before the
		// ch-row (e.g. "matcha") and is special-cased the same way.
		if i+1 < n && isSokuonPair(runes[i], runes[i+1]) {
			out.WriteString("っ")
			i++
			continue
		}
		if i+2 < n && runes[i] == 't' && runes[i+1] == 'c' && runes[i+2] == 'h' {
			out.WriteString("っ")
			i++
			continue
		}

		if i+2 < n {
			if kana, matched := lookup3(runes, i); matched {
				out.WriteString(kana)
				i += 3
				continue
			}
		}
		if i+1 < n {
			if kana, matched := lookup2(runes, i); matched {
				out.WriteString(kana)
				i += 2
				continue
			}
		}
		if kana, ok := monographs[string(runes[i])]; ok {
			out.WriteString(kana)
			i++
			continue
		}

		// Unrecognized rune: pass through unchanged (FR-006 — never error).
		out.WriteRune(runes[i])
		i++
	}

	return out.String()
}

func lookup3(runes []rune, i int) (string, bool) {
	key := string(runes[i : i+3])
	if kana, ok := digraphs[key]; ok {
		return kana, true
	}
	if kana, ok := monographs[key]; ok {
		return kana, true
	}
	return "", false
}

func lookup2(runes []rune, i int) (string, bool) {
	key := string(runes[i : i+2])
	if kana, ok := digraphs[key]; ok {
		return kana, true
	}
	if kana, ok := monographs[key]; ok {
		return kana, true
	}
	return "", false
}

func isSokuonPair(a, b rune) bool {
	if a != b || a == 'n' {
		return false
	}
	switch a {
	case 'a', 'i', 'u', 'e', 'o':
		return false
	}
	return a >= 'a' && a <= 'z'
}
