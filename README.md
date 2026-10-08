# profanitymask

A UTF-8 aware profanity masker for Go with diacritic and obfuscation folding.

## What it is

`profanitymask` replaces configured terms in text with asterisks. The package
folds case, diacritics, and common obfuscation before it compares text with a
term. The output keeps multibyte characters intact.

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

## API

| Function | Result |
| --- | --- |
| `NormalizeTerm(term string) (string, error)` | Returns the canonical form of one term, or `ErrInvalidTerm`. |
| `NormalizeTerms(terms []string) ([]string, error)` | Normalizes a term list, removes duplicates, and sorts the result. Returns `ErrTooManyTerms` above 500 terms. |
| `Mask(text string, terms []string) string` | Replaces every whole-token or whole-phrase match with `*`. |
| `Contains(text string, terms []string) bool` | Reports whether at least one whole-token or whole-phrase match exists in the text. |

Pass normalized terms to `Mask` and `Contains`. These functions compare terms as
given, so call `NormalizeTerm` or `NormalizeTerms` first.

## Behavior

- One `*` replaces one matched rune. The rune length of the text does not change.
- A match covers whole tokens and whole phrases. The term `kurva` does not match `kurvatur` or `zkurva`.
- Case does not matter. `PÍČA` matches the term `pica`.
- Precomposed and decomposed diacritics fold to the same base letters.
- A run of whitespace collapses to one space. The term `hello world` matches `hello   world` and `hello\nworld`.
- Digits always fold: `0` to `o`, `1` to `i`, `3` to `e`, `4` to `a`, `5` to `s`, and `7` to `t`.
- Symbols fold only between word characters: `@` to `a`, `$` to `s`, and `!` or `|` to `i`. A trailing `!` stays a token boundary.
- The full rune span of a match becomes `*`. No part of a matched term stays readable.
- The output is the same when the term list holds duplicates or when two matches overlap.
- Invalid UTF-8 text becomes the Unicode replacement character in the output. The package uses Go rune semantics.

## Limits

- The package masks configured terms only. It is not a general moderation model.
- The package does not detect unknown insults, images, or context.
- The package does not add spelling variants. Add each variant to the term list.
- A term holds 2 to 48 runes and at least one letter. A term list holds at most 500 terms.
- Control characters are invalid in a term. Line breaks, tabs, and other whitespace become spaces before this check.
- `Mask` and `Contains` do not normalize terms. A term with uppercase letters matches nothing.

## Development

```
go test -race -count=1 -cover ./...
```

## License

MIT. See `LICENSE`.
