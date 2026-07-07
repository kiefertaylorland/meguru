package romaji

import "testing"

func TestToHiragana_TableDriven(t *testing.T) {
	cases := map[string]string{
		// Vowels
		"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お",

		// k / g rows
		"ka": "か", "ki": "き", "ku": "く", "ke": "け", "ko": "こ",
		"ga": "が", "gi": "ぎ", "gu": "ぐ", "ge": "げ", "go": "ご",

		// s / z rows (with alternate spellings)
		"sa": "さ", "shi": "し", "si": "し", "su": "す", "se": "せ", "so": "そ",
		"za": "ざ", "ji": "じ", "zi": "じ", "zu": "ず", "ze": "ぜ", "zo": "ぞ",

		// t / d rows (with alternate spellings)
		"ta": "た", "chi": "ち", "ti": "ち", "tsu": "つ", "tu": "つ", "te": "て", "to": "と",
		"da": "だ", "di": "ぢ", "du": "づ", "de": "で", "do": "ど",

		// n row
		"na": "な", "ni": "に", "nu": "ぬ", "ne": "ね", "no": "の",

		// h / b / p rows
		"ha": "は", "hi": "ひ", "fu": "ふ", "hu": "ふ", "he": "へ", "ho": "ほ",
		"ba": "ば", "bi": "び", "bu": "ぶ", "be": "べ", "bo": "ぼ",
		"pa": "ぱ", "pi": "ぴ", "pu": "ぷ", "pe": "ぺ", "po": "ぽ",

		// m row
		"ma": "ま", "mi": "み", "mu": "む", "me": "め", "mo": "も",

		// y row
		"ya": "や", "yu": "ゆ", "yo": "よ",

		// r row
		"ra": "ら", "ri": "り", "ru": "る", "re": "れ", "ro": "ろ",

		// w row + bare n
		"wa": "わ", "wo": "を", "n": "ん",

		// Yōon digraphs, one per row
		"kya": "きゃ", "kyu": "きゅ", "kyo": "きょ",
		"gya": "ぎゃ", "gyu": "ぎゅ", "gyo": "ぎょ",
		"sha": "しゃ", "shu": "しゅ", "sho": "しょ",
		"ja": "じゃ", "ju": "じゅ", "jo": "じょ",
		"cha": "ちゃ", "chu": "ちゅ", "cho": "ちょ",
		"nya": "にゃ", "nyu": "にゅ", "nyo": "にょ",
		"hya": "ひゃ", "hyu": "ひゅ", "hyo": "ひょ",
		"bya": "びゃ", "byu": "びゅ", "byo": "びょ",
		"pya": "ぴゃ", "pyu": "ぴゅ", "pyo": "ぴょ",
		"mya": "みゃ", "myu": "みゅ", "myo": "みょ",
		"rya": "りゃ", "ryu": "りゅ", "ryo": "りょ",

		// Whole words. Note: "konnichiwa" converts mora-by-mora to
		// こんにちわ, not the conventionally-written こんにちは — the topic
		// particle は/wa is a grammatical orthographic exception, not a
		// phonetic one, and is intentionally out of scope for a generic
		// mora-based converter (see contracts/romaji.md's non-goals).
		"konnichiwa": "こんにちわ",
		"arigatou":   "ありがとう",
		"sushi":      "すし",
		"toukyou":    "とうきょう",
	}

	for input, want := range cases {
		got := ToHiragana(input)
		if got != want {
			t.Errorf("ToHiragana(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestToHiragana_Sokuon(t *testing.T) {
	cases := map[string]string{
		"kka":    "っか",
		"kekkon": "けっこん",
		"matcha": "まっちゃ",
		"kitte":  "きって",
		"zassi":  "ざっし",
	}
	for input, want := range cases {
		got := ToHiragana(input)
		if got != want {
			t.Errorf("ToHiragana(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestToHiragana_NDisambiguation(t *testing.T) {
	cases := map[string]string{
		"annai":     "あんない", // doubled "n": ん + な-row, no special-casing needed
		"kon'ya":    "こんや",  // apostrophe forces ん before "ya" (not the にゃ digraph)
		"kin'youbi": "きんようび",
		// Documented ambiguity (research.md): without an apostrophe, n+y+vowel
		// is read as the nya/nyu/nyo digraph, matching standard IME convention.
		"kinyoubi": "きにょうび",
		"hon":      "ほん", // bare "n" at end of input
		"sannin":   "さんにん",
	}
	for input, want := range cases {
		got := ToHiragana(input)
		if got != want {
			t.Errorf("ToHiragana(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestToHiragana_CaseInsensitive(t *testing.T) {
	if got := ToHiragana("KA"); got != "か" {
		t.Errorf("ToHiragana(%q) = %q, want %q", "KA", got, "か")
	}
	if got := ToHiragana("Ka"); got != "か" {
		t.Errorf("ToHiragana(%q) = %q, want %q", "Ka", got, "か")
	}
}

func TestToHiragana_EmptyString(t *testing.T) {
	if got := ToHiragana(""); got != "" {
		t.Errorf("ToHiragana(\"\") = %q, want empty string", got)
	}
}

func TestToHiragana_UnrecognizedRunesPassThrough(t *testing.T) {
	cases := map[string]string{
		"ka ki": "か き", // the space is an unrecognized rune and passes through
		"1234":  "1234",
		"ka!":   "か!",
		"a'ka":  "あ'か", // apostrophes pass through unless part of n'
		"あいう":   "あいう", // already-kana input passes through unchanged
	}

	for input, want := range cases {
		got := ToHiragana(input)
		if got != want {
			t.Errorf("ToHiragana(%q) = %q, want %q", input, got, want)
		}
	}
}
