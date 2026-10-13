# profanitymask

A UTF-8 aware profanity masker for Go with diacritic and obfuscation folding.

## What it is

`profanitymask` replaces configured terms in text with asterisks. The package
folds case, diacritics, common obfuscation, and homoglyphs before it compares
text with a term. The output keeps multibyte characters intact.

## Why

Most Go profanity filters target English ASCII. They miss `píča`, decomposed
`pi\u0301c\u030ca`, and leet spellings such as `sh1t` or `sh!t`. `profanitymask`
folds these forms to one canonical key before the match. A rune-to-rune index
keeps every match on rune boundaries, so masking never splits a multibyte
character.

## Install

```
go get github.com/m1chlcz/profanitymask
```

Go 1.27 or later is required.

## Quick start

```go
terms, err := profanitymask.NormalizeTerms([]string{"Kurva", "PÍČA", "sh!t"})
if err != nil {
    return err
}

masked := profanitymask.Mask("To je Píča, kurva!", terms)
// masked == "To je ****, *****!"

blocked := profanitymask.Contains("sh1t happens", terms)
// blocked == true
```

## Dictionary

A `Dictionary` holds normalized terms for repeated masking:

```go
dictionary, err := profanitymask.NewDictionary("Kurva", "sh!t")
if err != nil {
    return err
}
dictionary, err = dictionary.Add("PÍČA")
if err != nil {
    return err
}

masked, matched := profanitymask.MaskWithDictionary("To je Píča, kurva!", dictionary)
// masked == "To je ****, *****!", matched == true

blocked := profanitymask.ContainsWithDictionary("sh1t happens", dictionary)
// blocked == true

known := dictionary.Lookup("KURVA")
// known == true
```

A dictionary extends the caller's built-in term list. It changes which words
the caller masks, not how the package folds text. `Mask` and `Contains` keep
working with plain slices; call `dictionary.Terms()` when a plain slice is
needed.

## API

| Function | Result |
| --- | --- |
| `NormalizeTerm(term string) (string, error)` | Returns the canonical form of one term, or `ErrInvalidTerm`. |
| `NormalizeTerms(terms []string) ([]string, error)` | Normalizes a term list, removes duplicates, and sorts the result. Returns `ErrTooManyTerms` above 500 terms. |
| `Mask(text string, terms []string) string` | Replaces every whole-token or whole-phrase match with `*`. |
| `Contains(text string, terms []string) bool` | Reports whether at least one whole-token or whole-phrase match exists in the text. |
| `NewDictionary(terms ...string) (Dictionary, error)` | Normalizes a term list into a `Dictionary`. Returns the same errors as `NormalizeTerms`. |
| `Dictionary.Add(terms ...string) (Dictionary, error)` | Returns a copy of the dictionary with terms added. |
| `Dictionary.Lookup(term string) bool` | Reports whether a term is in the dictionary after normalization. |
| `Dictionary.Len() int` | Returns the number of dictionary terms. |
| `Dictionary.Terms() []string` | Returns a copy of the normalized, sorted terms. |
| `MaskWithDictionary(text string, dictionary Dictionary) (string, bool)` | Works like `Mask` and also reports whether it masked at least one match. |
| `ContainsWithDictionary(text string, dictionary Dictionary) bool` | Works like `Contains`. |

Pass normalized terms to `Mask` and `Contains`. These functions compare terms
as given, so call `NormalizeTerm` or `NormalizeTerms` first. `Dictionary`
normalizes on construction, so the dictionary functions need no extra step.

## Behavior

- One `*` replaces one matched rune. The rune length of the text does not change.
- A match covers whole tokens and whole phrases. The term `kurva` does not match `kurvatur` or `zkurva`.
- Case does not matter. `PÍČA` matches the term `pica`.
- Precomposed and decomposed diacritics fold to the same base letters.
- A run of whitespace collapses to one space. The term `hello world` matches `hello   world` and `hello\nworld`.
- Digits always fold: `0` to `o`, `1` to `i`, `3` to `e`, `4` to `a`, `5` to `s`, and `7` to `t`.
- Symbols fold only between word characters: `@` to `a`, `$` to `s`, and `!` or `|` to `i`. A trailing `!` stays a token boundary.
- Cyrillic and Greek homoglyphs fold to the ASCII letter they imitate: `kurvа` with a Cyrillic `а` matches `kurva`. The table covers the Cyrillic letters `а в е к м н о р с т у х і ј ѕ` and the Greek letters `α γ ε ι κ ν ο ρ τ υ χ`. Uppercase and accented forms fold through the same letters.
- Fullwidth ASCII (U+FF01 to U+FF5E) folds to its ASCII counterpart: `ｓｈｉｔ` matches `shit`, and a fullwidth `１` folds like `1`.
- The full rune span of a match becomes `*`. No part of a matched term stays readable.
- The output is the same when the term list holds duplicates or when two matches overlap.
- Invalid UTF-8 text becomes the Unicode replacement character in the output. The package uses Go rune semantics.

## Limits

- The package masks configured terms only. It is not a general moderation model.
- The package does not detect unknown insults, images, or context.
- The package does not add spelling variants. Add each variant to the term list.
- The homoglyph table is intentionally small. It covers the listed Cyrillic, Greek, and fullwidth forms, not every Unicode confusable.
- A term holds 2 to 48 runes and at least one letter. A term list holds at most 500 terms.
- Control characters are invalid in a term. Line breaks, tabs, and other whitespace become spaces before this check.
- `Mask` and `Contains` do not normalize terms. A term with uppercase letters matches nothing.

## Command line

```
go run ./cmd/profanitymask -dictionary terms.txt < input.txt > masked.txt
profanitymask -dictionary terms.txt -check files/*.md
```

The command reads lines from the files named as arguments, or from standard
input when no file is given. It writes masked lines to standard output and a
summary to standard error.

| Flag | Meaning |
| --- | --- |
| `-dictionary <file>` | Term list, one term per line. Blank lines and lines that start with `#` are skipped. |
| `-check` | Write nothing and exit 1 when a line matches. |
| `-quiet` | Suppress the summary line. |

Exit codes: 0 success, 1 a term matched with `-check`, 2 a usage, file, or
dictionary error.

## Development

```
go test -race -count=1 -cover ./...
```

## License

MIT. See `LICENSE`.
