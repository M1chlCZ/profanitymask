package profanitymask

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

func TestNewDictionaryNormalizesTerms(t *testing.T) {
	t.Parallel()
	dictionary, err := NewDictionary("Kurva", "PÍČA", "sh!t", "kurva")
	if err != nil {
		t.Fatalf("NewDictionary returned error %v", err)
	}
	if got := dictionary.Len(); got != 3 {
		t.Fatalf("Len = %d, want 3", got)
	}
	want := []string{"kurva", "pica", "shit"}
	if got := dictionary.Terms(); !slices.Equal(got, want) {
		t.Fatalf("Terms = %v, want %v", got, want)
	}
}

func TestNewDictionaryEmpty(t *testing.T) {
	t.Parallel()
	dictionary, err := NewDictionary()
	if err != nil {
		t.Fatalf("NewDictionary returned error %v", err)
	}
	if dictionary.Len() != 0 {
		t.Fatalf("Len = %d, want 0", dictionary.Len())
	}
	if dictionary.Terms() == nil {
		t.Fatalf("Terms = nil, want empty non-nil slice")
	}
}

func TestNewDictionaryErrors(t *testing.T) {
	t.Parallel()
	if _, err := NewDictionary("ok", "??"); !errors.Is(err, ErrInvalidTerm) {
		t.Fatalf("NewDictionary returned %v, want ErrInvalidTerm", err)
	}
	terms := make([]string, maxTerms+1)
	for index := range terms {
		terms[index] = fmt.Sprintf("term%03d", index)
	}
	if _, err := NewDictionary(terms...); !errors.Is(err, ErrTooManyTerms) {
		t.Fatalf("NewDictionary returned %v, want ErrTooManyTerms", err)
	}
}

func TestDictionaryLookupNormalizes(t *testing.T) {
	t.Parallel()
	dictionary, err := NewDictionary("Kurva", "PÍČA")
	if err != nil {
		t.Fatalf("NewDictionary returned error %v", err)
	}
	cases := []struct {
		name string
		term string
		want bool
	}{
		{name: "exact", term: "kurva", want: true},
		{name: "case", term: "KURVA", want: true},
		{name: "diacritics", term: "pica", want: true},
		{name: "decomposed diacritics", term: "Pi\u0301c\u030ca", want: true},
		{name: "obfuscation", term: "p!ca", want: true},
		{name: "missing", term: "kurvat", want: false},
		{name: "invalid", term: "?", want: false},
		{name: "empty", term: "", want: false},
	}
	for _, test := range cases {
		if got := dictionary.Lookup(test.term); got != test.want {
			t.Fatalf("%s: Lookup(%q) = %t, want %t", test.name, test.term, got, test.want)
		}
	}
}

func TestDictionaryAdd(t *testing.T) {
	t.Parallel()
	dictionary, err := NewDictionary("Kurva")
	if err != nil {
		t.Fatalf("NewDictionary returned error %v", err)
	}
	extended, err := dictionary.Add("PÍČA", "kurva")
	if err != nil {
		t.Fatalf("Add returned error %v", err)
	}
	if got := dictionary.Len(); got != 1 {
		t.Fatalf("receiver Len = %d, want 1", got)
	}
	if got := extended.Len(); got != 2 {
		t.Fatalf("extended Len = %d, want 2", got)
	}
	if !extended.Lookup("pica") || !extended.Lookup("kurva") {
		t.Fatalf("extended dictionary lost a term: %v", extended.Terms())
	}
	unchanged, err := extended.Add("??")
	if !errors.Is(err, ErrInvalidTerm) {
		t.Fatalf("Add returned %v, want ErrInvalidTerm", err)
	}
	if unchanged.Len() != extended.Len() {
		t.Fatalf("Add on error returned Len %d, want %d", unchanged.Len(), extended.Len())
	}
}

func TestDictionaryAddRejectsTooManyTerms(t *testing.T) {
	t.Parallel()
	terms := make([]string, maxTerms)
	for index := range terms {
		terms[index] = fmt.Sprintf("term%03d", index)
	}
	dictionary, err := NewDictionary(terms...)
	if err != nil {
		t.Fatalf("NewDictionary returned error %v", err)
	}
	if _, err := dictionary.Add("extra"); !errors.Is(err, ErrTooManyTerms) {
		t.Fatalf("Add returned %v, want ErrTooManyTerms", err)
	}
}

func TestDictionaryTermsReturnsCopy(t *testing.T) {
	t.Parallel()
	dictionary, err := NewDictionary("kurva", "pica")
	if err != nil {
		t.Fatalf("NewDictionary returned error %v", err)
	}
	terms := dictionary.Terms()
	terms[0] = "mutated"
	if dictionary.Lookup("mutated") {
		t.Fatalf("Terms returned a live slice: %v", dictionary.Terms())
	}
}

func TestMaskWithDictionary(t *testing.T) {
	t.Parallel()
	dictionary, err := NewDictionary("Kurva", "PÍČA", "sh!t")
	if err != nil {
		t.Fatalf("NewDictionary returned error %v", err)
	}
	cases := []struct {
		name      string
		text      string
		want      string
		wantMatch bool
	}{
		{name: "whole tokens", text: "To je Píča, kurva!", want: "To je ****, *****!", wantMatch: true},
		{name: "obfuscation", text: "sh1t happens", want: "**** happens", wantMatch: true},
		{name: "homoglyph", text: "kurv\u0430", want: "*****", wantMatch: true},
		{name: "no match", text: "clean text", want: "clean text", wantMatch: false},
		{name: "empty text", text: "", want: "", wantMatch: false},
		{name: "invalid utf8 with match", text: "kurva\xff", want: "*****\ufffd", wantMatch: true},
		{name: "invalid utf8 without match", text: "z\xff", want: "z\ufffd", wantMatch: false},
	}
	for _, test := range cases {
		got, matched := MaskWithDictionary(test.text, dictionary)
		if got != test.want || matched != test.wantMatch {
			t.Fatalf("%s: MaskWithDictionary(%q) = %q, %t, want %q, %t",
				test.name, test.text, got, matched, test.want, test.wantMatch)
		}
	}
}

func TestMaskWithDictionaryMatchesMask(t *testing.T) {
	t.Parallel()
	terms := []string{"kurva", "pica", "shit"}
	dictionary, err := NewDictionary(terms...)
	if err != nil {
		t.Fatalf("NewDictionary returned error %v", err)
	}
	texts := []string{"To je Píča, kurva!", "sh1t", "kurv\u0430", "clean", "", "kurva\xff", "pica safe shit"}
	for _, text := range texts {
		want := Mask(text, terms)
		got, _ := MaskWithDictionary(text, dictionary)
		if got != want {
			t.Fatalf("MaskWithDictionary(%q) = %q, want Mask %q", text, got, want)
		}
	}
}

func TestMaskWithEmptyDictionary(t *testing.T) {
	t.Parallel()
	var dictionary Dictionary
	masked, matched := MaskWithDictionary("kurva", dictionary)
	if matched || masked != "kurva" {
		t.Fatalf("MaskWithDictionary with zero dictionary = %q, %t, want %q, false", masked, matched, "kurva")
	}
	if dictionary.Len() != 0 || dictionary.Lookup("kurva") {
		t.Fatalf("zero dictionary is not empty")
	}
	if ContainsWithDictionary("kurva", dictionary) {
		t.Fatalf("ContainsWithDictionary with zero dictionary = true, want false")
	}
}

func TestContainsWithDictionary(t *testing.T) {
	t.Parallel()
	dictionary, err := NewDictionary("Kurva", "PÍČA", "sh!t")
	if err != nil {
		t.Fatalf("NewDictionary returned error %v", err)
	}
	cases := []struct {
		name string
		text string
		want bool
	}{
		{name: "diacritics", text: "PÍČA!", want: true},
		{name: "obfuscation", text: "sh1t", want: true},
		{name: "homoglyph", text: "kurv\u0430", want: true},
		{name: "partial word", text: "kurvat", want: false},
		{name: "empty text", text: "", want: false},
	}
	for _, test := range cases {
		if got := ContainsWithDictionary(test.text, dictionary); got != test.want {
			t.Fatalf("%s: ContainsWithDictionary(%q) = %t, want %t", test.name, test.text, got, test.want)
		}
	}
}
