// Package profanitymask masks configured terms in UTF-8 text.
//
// The package folds case, strips combining marks, collapses whitespace, and
// maps common obfuscation characters to letters before it matches whole tokens
// and phrases. One asterisk replaces one matched rune, so multibyte characters
// never split.
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
// obfuscation characters map to letters. NormalizeTerm returns ErrInvalidTerm
// when the term is empty, shorter than 2 runes, longer than 48 runes, contains
// a control character, or contains no letter.
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

// Mask replaces every whole-token and whole-phrase match of terms in text with
// asterisks. Each masked rune becomes one '*', and Mask replaces the full
// original rune span of a match, so no matched part stays readable. Mask
// compares terms as given: pass terms from NormalizeTerm or NormalizeTerms.
func Mask(text string, terms []string) string {
	if text == "" || len(terms) == 0 {
		return text
	}
	runes := []rune(text)
	index := indexText(text)
	masked := make([]bool, len(runes))
	markMatches(index, terms, masked)
	var builder strings.Builder
	builder.Grow(len(text))
	for position, current := range runes {
		if masked[position] {
			builder.WriteRune('*')
			continue
		}
		builder.WriteRune(current)
	}
	return builder.String()
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

func foldRuneAt(runes []rune, index int) []rune {
	current := unicode.ToLower(runes[index])
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
			first := index.original[start]
			last := index.original[end-1]
			for position := first; position <= last; position++ {
				masked[position] = true
			}
			start = end - 1
		}
	}
	return false
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
