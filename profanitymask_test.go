package profanitymask

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeTermCanonicalizes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "  PÍČA  ", want: "pica"},
		{raw: "Kurva", want: "kurva"},
		{raw: "Hello World", want: "hello world"},
		{raw: "sh!t", want: "shit"},
		{raw: "Šíleně   dlouhý", want: "silene dlouhy"},
		{raw: "sh1t", want: "shit"},
		{raw: "a@b", want: "aab"},
		{raw: "HELLO\tWORLD", want: "hello world"},
		{raw: "Pi\u0301c\u030ca", want: "pica"},
		{raw: "ab\ncd", want: "ab cd"},
	}
	for _, test := range cases {
		got, err := NormalizeTerm(test.raw)
		if err != nil {
			t.Fatalf("NormalizeTerm(%q) returned error %v", test.raw, err)
		}
		if got != test.want {
			t.Fatalf("NormalizeTerm(%q) = %q, want %q", test.raw, got, test.want)
		}
	}
}

func TestNormalizeTermRejects(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		" ",
		"a",
		"??",
		"\u0301",
		"ab\u0000cd",
		"\u001bab",
		strings.Repeat("x", maxTermRunes+1),
	}
	for _, raw := range cases {
		got, err := NormalizeTerm(raw)
		if !errors.Is(err, ErrInvalidTerm) {
			t.Fatalf("NormalizeTerm(%q) = %q, %v, want ErrInvalidTerm", raw, got, err)
		}
	}
}

func TestNormalizeTermAcceptsLengthBoundaries(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"ab", strings.Repeat("x", maxTermRunes), "x y"} {
		if _, err := NormalizeTerm(raw); err != nil {
			t.Fatalf("NormalizeTerm(%q) returned error %v", raw, err)
		}
	}
}

func TestNormalizeTermsDeduplicatesAndSorts(t *testing.T) {
	t.Parallel()
	got, err := NormalizeTerms([]string{"Kurva", "PÍČA", "kurva", "hello world"})
	if err != nil {
		t.Fatalf("NormalizeTerms returned error %v", err)
	}
	want := []string{"hello world", "kurva", "pica"}
	if !slices.Equal(got, want) {
		t.Fatalf("NormalizeTerms = %v, want %v", got, want)
	}
}

func TestNormalizeTermsEmpty(t *testing.T) {
	t.Parallel()
	got, err := NormalizeTerms(nil)
	if err != nil {
		t.Fatalf("NormalizeTerms(nil) returned error %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("NormalizeTerms(nil) = %#v, want empty non-nil slice", got)
	}
}

func TestNormalizeTermsRejectsInvalidTerm(t *testing.T) {
	t.Parallel()
	got, err := NormalizeTerms([]string{"ok", "??"})
	if !errors.Is(err, ErrInvalidTerm) {
		t.Fatalf("NormalizeTerms returned %v, want ErrInvalidTerm", err)
	}
	if got != nil {
		t.Fatalf("NormalizeTerms = %v, want nil result", got)
	}
}

func TestNormalizeTermsRejectsTooManyTerms(t *testing.T) {
	t.Parallel()
	terms := make([]string, maxTerms+1)
	for index := range terms {
		terms[index] = "ab"
	}
	if _, err := NormalizeTerms(terms); !errors.Is(err, ErrTooManyTerms) {
		t.Fatalf("NormalizeTerms returned %v, want ErrTooManyTerms", err)
	}
}

func TestMaskWholeTokens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		terms []string
		text  string
		want  string
	}{
		{name: "diacritics and case", terms: []string{"pica"}, text: "Píča a PÍČA", want: "**** a ****"},
		{name: "decomposed diacritics", terms: []string{"pica"}, text: "Pi\u0301c\u030ca", want: "******"},
		{name: "trailing punctuation stays a boundary", terms: []string{"kurva"}, text: "kurva!", want: "*****!"},
		{name: "embedded symbol obfuscation", terms: []string{"shit"}, text: "sh!t", want: "****"},
		{name: "benign substring with same stem", terms: []string{"kurva"}, text: "kurvat zkurva", want: "kurvat zkurva"},
		{name: "safe substrings", terms: []string{"ass"}, text: "classic pass assert", want: "classic pass assert"},
		{name: "multiple words", terms: []string{"shit", "fuck"}, text: "oh shit and fuck", want: "oh **** and ****"},
		{name: "obfuscation", terms: []string{"shit"}, text: "sh1t happens", want: "**** happens"},
		{name: "phrases", terms: []string{"your mom"}, text: "kiss your mom now", want: "kiss ******** now"},
		{name: "changed term list disables masking", terms: []string{"other"}, text: "Píča a PÍČA", want: "Píča a PÍČA"},
		{name: "empty term list", terms: nil, text: "Píča", want: "Píča"},
		{name: "never leaks partial", terms: []string{"bastard"}, text: "BASTARD!", want: "*******!"},
	}
	for _, test := range cases {
		if got := Mask(test.text, test.terms); got != test.want {
			t.Fatalf("%s: Mask(%q) = %q, want %q", test.name, test.text, got, test.want)
		}
	}
}

func TestMaskMultipleOccurrences(t *testing.T) {
	t.Parallel()
	if got := Mask("shit shitty shit", []string{"shit"}); got != "**** shitty ****" {
		t.Fatalf("Mask = %q, want %q", got, "**** shitty ****")
	}
}

func TestMaskReturnedTextAlwaysContainsStars(t *testing.T) {
	t.Parallel()
	got := Mask("kurva", []string{"kurva"})
	if !strings.Contains(got, "*") || strings.Contains(got, "kurva") {
		t.Fatalf("Mask leaked the matched term: %q", got)
	}
}

func TestMaskPhrasesAcrossWhitespace(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		terms []string
		text  string
		want  string
	}{
		{name: "extra spaces", terms: []string{"your mom"}, text: "kiss your  mom now", want: "kiss ********* now"},
		{name: "newline", terms: []string{"your mom"}, text: "kiss your\nmom now", want: "kiss ******** now"},
		{name: "mixed case", terms: []string{"your mom"}, text: "Your  MOM", want: "*********"},
		{name: "leading whitespace", terms: []string{"kurva"}, text: "  kurva", want: "  *****"},
		{name: "trailing whitespace", terms: []string{"kurva"}, text: "kurva  ", want: "*****  "},
	}
	for _, test := range cases {
		if got := Mask(test.text, test.terms); got != test.want {
			t.Fatalf("%s: Mask(%q) = %q, want %q", test.name, test.text, got, test.want)
		}
	}
}

func TestMaskObfuscationFolding(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		terms []string
		text  string
		want  string
	}{
		{name: "all digits", terms: []string{"o", "i", "e", "a", "s", "t"}, text: "0 1 3 4 5 7", want: "* * * * * *"},
		{name: "digits fold anywhere", terms: []string{"shit"}, text: "sh1t 5hit", want: "**** ****"},
		{name: "at symbol between words", terms: []string{"xax"}, text: "x@x", want: "***"},
		{name: "dollar between words", terms: []string{"xsx"}, text: "x$x", want: "***"},
		{name: "bang between words", terms: []string{"xix"}, text: "x!x", want: "***"},
		{name: "pipe between words", terms: []string{"xix"}, text: "x|x", want: "***"},
		{name: "leading dollar stays", terms: []string{"shit"}, text: "$hit", want: "$hit"},
		{name: "leading bang stays", terms: []string{"shit"}, text: "!shit", want: "!****"},
		{name: "trailing bang stays", terms: []string{"shit"}, text: "shit!", want: "****!"},
		{name: "embedded dollar folds", terms: []string{"ashit"}, text: "a$hit", want: "*****"},
	}
	for _, test := range cases {
		if got := Mask(test.text, test.terms); got != test.want {
			t.Fatalf("%s: Mask(%q) = %q, want %q", test.name, test.text, got, test.want)
		}
	}
}

func TestMaskWordBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		terms []string
		text  string
		want  string
	}{
		{name: "digit folding keeps the boundary", terms: []string{"shit"}, text: "sh1tty", want: "sh1tty"},
		{name: "leading digit blocks the match", terms: []string{"shit"}, text: "1shit", want: "1shit"},
		{name: "substring inside a word", terms: []string{"ab"}, text: "abc", want: "abc"},
		{name: "match after a symbol", terms: []string{"shit"}, text: "(shit)", want: "(****)"},
		{name: "empty term is ignored", terms: []string{""}, text: "shit", want: "shit"},
		{name: "duplicate terms are safe", terms: []string{"shit", "shit"}, text: "shit", want: "****"},
		{name: "overlapping phrases", terms: []string{"aa bb", "bb cc"}, text: "aa bb cc", want: "********"},
	}
	for _, test := range cases {
		if got := Mask(test.text, test.terms); got != test.want {
			t.Fatalf("%s: Mask(%q) = %q, want %q", test.name, test.text, got, test.want)
		}
	}
}

func TestMaskPreservesMultibyteRunes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		terms []string
		text  string
		want  string
	}{
		{name: "diacritic prefix and suffix", terms: []string{"kurva"}, text: "Příliš kurva žluťoučký", want: "Příliš ***** žluťoučký"},
		{name: "emoji boundary", terms: []string{"kurva"}, text: "🎉kurva🎉", want: "🎉*****🎉"},
		{name: "one star per rune", terms: []string{"pica"}, text: "PÍČA", want: "****"},
	}
	for _, test := range cases {
		if got := Mask(test.text, test.terms); got != test.want {
			t.Fatalf("%s: Mask(%q) = %q, want %q", test.name, test.text, got, test.want)
		}
	}
}

func TestMaskInvalidUTF8(t *testing.T) {
	t.Parallel()
	got := Mask("kurva\xff", []string{"kurva"})
	want := "*****\ufffd"
	if got != want {
		t.Fatalf("Mask = %q, want %q", got, want)
	}
}

func TestMaskShortInputs(t *testing.T) {
	t.Parallel()
	if got := Mask("", []string{"kurva"}); got != "" {
		t.Fatalf("Mask on empty text = %q, want empty", got)
	}
	if got := Mask("kurva", nil); got != "kurva" {
		t.Fatalf("Mask without terms = %q, want unchanged", got)
	}
}

func TestContains(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		terms []string
		text  string
		want  bool
	}{
		{name: "diacritics and punctuation", terms: []string{"pica"}, text: "PÍČA!", want: true},
		{name: "obfuscation", terms: []string{"shit"}, text: "sh!t", want: true},
		{name: "partial word", terms: []string{"kurva"}, text: "kurvat", want: false},
		{name: "leading digit", terms: []string{"shit"}, text: "1shit", want: false},
		{name: "leading symbol", terms: []string{"shit"}, text: "$hit", want: false},
		{name: "empty text", terms: []string{"kurva"}, text: "", want: false},
		{name: "empty terms", terms: nil, text: "kurva", want: false},
		{name: "empty term", terms: []string{""}, text: "kurva", want: false},
	}
	for _, test := range cases {
		if got := Contains(test.text, test.terms); got != test.want {
			t.Fatalf("%s: Contains(%q) = %t, want %t", test.name, test.text, got, test.want)
		}
	}
}
