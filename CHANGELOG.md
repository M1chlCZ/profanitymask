# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-10-13

### Added

- `Dictionary` with `NewDictionary`, `Add`, `Lookup`, `Len` and `Terms`. A
  dictionary holds normalized terms for repeated masking and extends the
  caller's built-in term list; the folding rules of the package stay the same.
- `MaskWithDictionary` and `ContainsWithDictionary`, which mask and check text
  with a dictionary instead of a raw term slice. `MaskWithDictionary` also
  reports whether it masked at least one match.
- Homoglyph folding: the Cyrillic and Greek lookalike letters in the new
  confusable table, plus the fullwidth ASCII range (U+FF01 to U+FF5E), fold to
  their ASCII counterparts before matching.
- `cmd/profanitymask`, a bulk command that masks lines from files or standard
  input. Flags: `-dictionary` (`#` comments and blank lines are skipped),
  `-check` (exit 1 on a match) and `-quiet` (no summary).
