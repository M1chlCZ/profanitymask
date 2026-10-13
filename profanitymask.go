// Package profanitymask masks configured terms in UTF-8 text.
//
// The package folds case, strips combining marks, collapses whitespace, and
// maps common obfuscation characters and homoglyphs to letters before it
// matches whole tokens and phrases. One asterisk replaces one matched rune, so
// multibyte characters never split.
package profanitymask

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	maxTerms     = 500
	minTermRunes = 2
	maxTermRunes = 48
)

var (
	// ErrInvalidTerm reports a term that is empty, shorter than 2 runes,
	// longer than 48 runes, contains a control character, or contains no
	// letter.
	ErrInvalidTerm = errors.New("profanitymask: invalid term")

	// ErrTooManyTerms reports a term list that holds more than 500 terms.
	ErrTooManyTerms = errors.New("profanitymask: too many terms")
)

// NormalizeTerm validates one term and returns its canonical form. The
// canonical form is lowercase, diacritic-free, and single-spaced. Common
// obfuscation characters and homoglyphs map to letters. NormalizeTerm returns
// ErrInvalidTerm when the term is empty, shorter than 2 runes, longer than 48
// runes, contains a control character, or contains no letter.
func NormalizeTerm(term string) (string, error) {
	normalized := normalizeText(strings.TrimSpace(term))
	runes := []rune(normalized)
	if len(runes) < minTermRunes || len(runes) > maxTermRunes {
		return "", ErrInvalidTerm
	}
	if !slices.ContainsFunc(runes, unicode.IsLetter) {
		return "", ErrInvalidTerm
	}
	if slices.ContainsFunc(runes, unicode.IsControl) {
		return "", ErrInvalidTerm
	}
	return normalized, nil
}

// NormalizeTerms validates and canonicalizes a term list. The result holds no
// duplicates in sorted order. NormalizeTerms returns ErrTooManyTerms when the
// list holds more than 500 terms, and wraps ErrInvalidTerm when one term is
// invalid. An empty input returns an empty, non-nil slice.
func NormalizeTerms(terms []string) ([]string, error) {
	if len(terms) > maxTerms {
		return nil, ErrTooManyTerms
	}
	normalized := make([]string, 0, len(terms))
	seen := make(map[string]struct{}, len(terms))
	for index, raw := range terms {
		term, err := NormalizeTerm(raw)
		if err != nil {
			return nil, fmt.Errorf("profanitymask: term at index %d: %w", index, err)
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		normalized = append(normalized, term)
	}
	slices.Sort(normalized)
	return normalized, nil
}

// Dictionary holds normalized terms for repeated masking. Build it with
// NewDictionary, extend it with Add, and pass it to MaskWithDictionary or
// ContainsWithDictionary. The zero value holds no terms and matches nothing.
//
// A dictionary extends the caller's built-in term list. It decides which
// words the caller masks; it does not extend or replace the folding rules of
// this package. MaskWithDictionary and ContainsWithDictionary fold text
// exactly like Mask and Contains.
type Dictionary struct {
	terms []string
}

// NewDictionary normalizes terms and returns a dictionary that holds them.
// The result follows the NormalizeTerms contract: sorted, without duplicates,
// and validated the same way. NewDictionary returns ErrTooManyTerms above 500
// terms and a wrapped ErrInvalidTerm when one term is invalid. An empty input
// returns an empty dictionary.
func NewDictionary(terms ...string) (Dictionary, error) {
	normalized, err := NormalizeTerms(terms)
	if err != nil {
		return Dictionary{}, err
	}
	return Dictionary{terms: normalized}, nil
}

// Add returns a copy of the dictionary with terms added in normalized form.
// Add reports the same errors as NewDictionary and returns the receiver
// unchanged on error.
func (d Dictionary) Add(terms ...string) (Dictionary, error) {
	combined := make([]string, 0, len(d.terms)+len(terms))
	combined = append(combined, d.terms...)
	combined = append(combined, terms...)
	normalized, err := NormalizeTerms(combined)
	if err != nil {
		return d, err
	}
	return Dictionary{terms: normalized}, nil
}

// Lookup reports whether term matches one of the dictionary terms after
// normalization. Lookup returns false when term is invalid.
func (d Dictionary) Lookup(term string) bool {
	normalized, err := NormalizeTerm(term)
	if err != nil {
		return false
	}
	_, found := slices.BinarySearch(d.terms, normalized)
	return found
}

// Len returns the number of terms in the dictionary.
func (d Dictionary) Len() int {
	return len(d.terms)
}

// Terms returns a copy of the normalized terms, sorted and without
// duplicates. Pass the result to Mask or Contains when a plain term slice is
// needed.
func (d Dictionary) Terms() []string {
	return slices.Clone(d.terms)
}

// Mask replaces every whole-token and whole-phrase match of terms in text with
// asterisks. Each masked rune becomes one '*', and Mask replaces the full
// original rune span of a match, so no matched part stays readable. Mask
// compares terms as given: pass terms from NormalizeTerm or NormalizeTerms.
func Mask(text string, terms []string) string {
	masked, _ := maskText(text, terms)
	return masked
}

// MaskWithDictionary replaces every whole-token and whole-phrase match of the
// dictionary terms with asterisks. It returns the masked text and reports
// whether it masked at least one match. MaskWithDictionary folds text exactly
// like Mask.
func MaskWithDictionary(text string, dictionary Dictionary) (string, bool) {
	return maskText(text, dictionary.terms)
}

// Contains reports whether text holds at least one whole-token or whole-phrase
// match. Contains compares terms as given: pass terms from NormalizeTerm or
// NormalizeTerms.
func Contains(text string, terms []string) bool {
	if text == "" || len(terms) == 0 {
		return false
	}
	return markMatches(indexText(text), terms, nil)
}

// ContainsWithDictionary reports whether text holds at least one whole-token
// or whole-phrase match of a dictionary term.
func ContainsWithDictionary(text string, dictionary Dictionary) bool {
	return Contains(text, dictionary.terms)
}

func normalizeText(raw string) string {
	runes := []rune(raw)
	var builder strings.Builder
	builder.Grow(len(raw))
	pendingSpace := false
	for index := range runes {
		for _, folded := range foldRuneAt(runes, index) {
			if unicode.IsSpace(folded) {
				pendingSpace = builder.Len() > 0
				continue
			}
			if pendingSpace {
				builder.WriteRune(' ')
				pendingSpace = false
			}
			builder.WriteRune(folded)
		}
	}
	return builder.String()
}

// foldRuneAt folds one source rune to zero or more output runes. Confusables
// fold to ASCII first, so fullwidth symbols and digits still reach the
// contextual obfuscation rules; numeric stand-ins fold always; combining
// marks strip last, so decomposed confusables fold as well.
func foldRuneAt(runes []rune, index int) []rune {
	current := unicode.ToLower(runes[index])
	if replacement, found := foldConfusable(current); found {
		current = replacement
	}
	if replacement, found := obfuscatedSymbol(current); found {
		if index > 0 && index+1 < len(runes) &&
			isSourceWordRune(runes[index-1]) && isSourceWordRune(runes[index+1]) {
			return []rune{replacement}
		}
	}
	if replacement, found := obfuscatedDigit(current); found {
		return []rune{replacement}
	}
	decomposed := norm.NFD.String(string(current))
	folded := make([]rune, 0, len(decomposed))
	for _, part := range decomposed {
		if unicode.Is(unicode.Mn, part) {
			continue
		}
		if replacement, found := foldConfusable(part); found {
			part = replacement
		}
		folded = append(folded, part)
	}
	return folded
}

func obfuscatedDigit(current rune) (rune, bool) {
	switch current {
	case '0':
		return 'o', true
	case '1':
		return 'i', true
	case '3':
		return 'e', true
	case '4':
		return 'a', true
	case '5':
		return 's', true
	case '7':
		return 't', true
	}
	return 0, false
}

func obfuscatedSymbol(current rune) (rune, bool) {
	switch current {
	case '@':
		return 'a', true
	case '$':
		return 's', true
	case '!', '|':
		return 'i', true
	}
	return 0, false
}

// confusableFold maps homoglyph runes to the ASCII letter they imitate. The
// keys cover the Cyrillic and Greek lowercase letters that attackers mix into
// Latin words; uppercase and accented forms fold through unicode.ToLower and
// combining mark removal first. This table is the single mapping to mirror
// when the folding is ported into another codebase.
var confusableFold = map[rune]rune{
	'а': 'a', 'в': 'b', 'е': 'e', 'к': 'k', 'м': 'm', 'н': 'h',
	'о': 'o', 'р': 'p', 'с': 'c', 'т': 't', 'у': 'y', 'х': 'x',
	'і': 'i', 'ј': 'j', 'ѕ': 's',
	'α': 'a', 'γ': 'y', 'ε': 'e', 'ι': 'i', 'κ': 'k', 'ν': 'v',
	'ο': 'o', 'ρ': 'p', 'τ': 't', 'υ': 'u', 'χ': 'x',
}

const (
	fullwidthASCIIFirst = '\uFF01'
	fullwidthASCIILast  = '\uFF5E'
	fullwidthASCIIGap   = 0xFEE0
)

// foldConfusable returns the ASCII counterpart of a homoglyph. Fullwidth
// ASCII, U+FF01 to U+FF5E, folds to the ASCII rune 0xFEE0 below it, so a
// fullwidth digit or symbol reaches the obfuscation rules as its ASCII form.
func foldConfusable(current rune) (rune, bool) {
	if replacement, found := confusableFold[current]; found {
		return replacement, true
	}
	if current >= fullwidthASCIIFirst && current <= fullwidthASCIILast {
		return current - fullwidthASCIIGap, true
	}
	return 0, false
}

func isSourceWordRune(current rune) bool {
	return unicode.IsLetter(current) || unicode.IsDigit(current)
}

type textIndex struct {
	normalized []rune
	original   []int
}

func indexText(text string) textIndex {
	originals := []rune(text)
	index := textIndex{
		normalized: make([]rune, 0, len(originals)),
		original:   make([]int, 0, len(originals)),
	}
	for position := range originals {
		for _, folded := range foldRuneAt(originals, position) {
			if unicode.IsSpace(folded) {
				if len(index.normalized) == 0 || index.normalized[len(index.normalized)-1] == ' ' {
					continue
				}
				index.normalized = append(index.normalized, ' ')
				index.original = append(index.original, position)
				continue
			}
			index.normalized = append(index.normalized, folded)
			index.original = append(index.original, position)
		}
	}
	return index
}

func markMatches(index textIndex, terms []string, masked []bool) bool {
	found := false
	for _, term := range terms {
		termRunes := []rune(term)
		if len(termRunes) == 0 {
			continue
		}
		for start := 0; start+len(termRunes) <= len(index.normalized); start++ {
			if !wordBoundaryBefore(index.normalized, start) {
				continue
			}
			if !runesMatchAt(index.normalized, start, termRunes) {
				continue
			}
			end := start + len(termRunes)
			if !wordBoundaryAfter(index.normalized, end) {
				continue
			}
			if masked == nil {
				return true
			}
			found = true
			first := index.original[start]
			last := index.original[end-1]
			for position := first; position <= last; position++ {
				masked[position] = true
			}
			start = end - 1
		}
	}
	return found
}

// maskText applies the shared masking pipeline. It always rebuilds non-empty
// text from runes, so invalid UTF-8 becomes the Unicode replacement
// character even when nothing matches, and reports whether it masked at least
// one match.
func maskText(text string, terms []string) (string, bool) {
	if text == "" || len(terms) == 0 {
		return text, false
	}
	runes := []rune(text)
	index := indexText(text)
	masked := make([]bool, len(runes))
	found := markMatches(index, terms, masked)
	var builder strings.Builder
	builder.Grow(len(text))
	for position, current := range runes {
		if masked[position] {
			builder.WriteRune('*')
			continue
		}
		builder.WriteRune(current)
	}
	return builder.String(), found
}

func wordBoundaryBefore(normalized []rune, start int) bool {
	return start == 0 || !isWordRune(normalized[start-1])
}

func wordBoundaryAfter(normalized []rune, end int) bool {
	return end == len(normalized) || !isWordRune(normalized[end])
}

func isWordRune(current rune) bool {
	return unicode.IsLetter(current) || unicode.IsDigit(current)
}

func runesMatchAt(normalized []rune, start int, term []rune) bool {
	for offset, expected := range term {
		if normalized[start+offset] != expected {
			return false
		}
	}
	return true
}
