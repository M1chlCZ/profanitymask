package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m1chlcz/profanitymask"
)

func writeFile(t *testing.T, directory, name, content string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) returned error %v", path, err)
	}
	return path
}

func TestRunMasksLines(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	dictionaryPath := writeFile(t, directory, "terms.txt", "# profanity\n\nKurva\nPÍČA\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dictionary", dictionaryPath}, strings.NewReader("To je Píča, kurva!\nnic tu není\n"), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d; stderr %q", code, exitOK, stderr.String())
	}
	if want := "To je ****, *****!\nnic tu není\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if want := "profanitymask: masked 1 of 2 lines\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunMasksHomoglyphsAndObfuscation(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	dictionaryPath := writeFile(t, directory, "terms.txt", "kurva\nshit\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dictionary", dictionaryPath}, strings.NewReader("kurv\u0430\nsh1t\n"), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d; stderr %q", code, exitOK, stderr.String())
	}
	if want := "*****\n****\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunMultipleFiles(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	dictionaryPath := writeFile(t, directory, "terms.txt", "kurva\npica\n")
	first := writeFile(t, directory, "first.txt", "kurva\n")
	second := writeFile(t, directory, "second.txt", "pica kurva\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dictionary", dictionaryPath, first, second}, strings.NewReader(""), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d; stderr %q", code, exitOK, stderr.String())
	}
	if want := "*****\n**** *****\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if want := "profanitymask: masked 2 of 2 lines\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunCheckMode(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	dictionaryPath := writeFile(t, directory, "terms.txt", "kurva\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dictionary", dictionaryPath, "-check"}, strings.NewReader("vše je v pořádku\n"), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d", code, exitOK)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if want := "profanitymask: matched 0 of 1 lines\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"-dictionary", dictionaryPath, "-check"}, strings.NewReader("to je kurva\nnic\n"), &stdout, &stderr)
	if code != exitMatch {
		t.Fatalf("run = %d, want %d", code, exitMatch)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if want := "profanitymask: matched 1 of 2 lines\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunCheckQuiet(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	dictionaryPath := writeFile(t, directory, "terms.txt", "kurva\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dictionary", dictionaryPath, "-check", "-quiet"}, strings.NewReader("kurva\n"), &stdout, &stderr)
	if code != exitMatch {
		t.Fatalf("run = %d, want %d", code, exitMatch)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q, want both empty", stdout.String(), stderr.String())
	}
}

func TestRunQuietSuppressesSummary(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	dictionaryPath := writeFile(t, directory, "terms.txt", "kurva\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dictionary", dictionaryPath, "-quiet"}, strings.NewReader("kurva\n"), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d", code, exitOK)
	}
	if want := "*****\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunWithoutDictionaryPassesThrough(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run(nil, strings.NewReader("kurva\n"), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d", code, exitOK)
	}
	if want := "kurva\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if want := "profanitymask: masked 0 of 1 lines\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunWithoutTrailingNewline(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run(nil, strings.NewReader("one line"), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d", code, exitOK)
	}
	if want := "one line\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing dictionary file", args: []string{"-dictionary", filepath.Join(directory, "missing.txt")}},
		{name: "missing input file", args: []string{filepath.Join(directory, "missing.txt")}},
		{name: "invalid dictionary term", args: []string{"-dictionary", writeFile(t, directory, "bad.txt", "??\n")}},
		{name: "unknown flag", args: []string{"-unknown"}},
	}
	for _, test := range cases {
		var stdout, stderr bytes.Buffer
		if code := run(test.args, strings.NewReader("kurva\n"), &stdout, &stderr); code != exitError {
			t.Fatalf("%s: run = %d, want %d; stderr %q", test.name, code, exitError, stderr.String())
		}
		if stderr.Len() == 0 {
			t.Fatalf("%s: stderr is empty, want an error message", test.name)
		}
	}
}

func TestRunHelp(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-h"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stderr.String(), "usage: profanitymask") {
		t.Fatalf("stderr = %q, want usage text", stderr.String())
	}
}

func TestReadDictionaryParsesFiles(t *testing.T) {
	t.Parallel()
	dictionary, err := readDictionary(strings.NewReader("# comment\n\n  Kurva  \nPÍČA\n#end\n"))
	if err != nil {
		t.Fatalf("readDictionary returned error %v", err)
	}
	if got := dictionary.Len(); got != 2 {
		t.Fatalf("Len = %d, want 2", got)
	}
	if !dictionary.Lookup("KURVA") || !dictionary.Lookup("pica") {
		t.Fatalf("dictionary lost a term: %v", dictionary.Terms())
	}
}

func TestReadDictionaryRejectsInvalidTerm(t *testing.T) {
	t.Parallel()
	_, err := readDictionary(strings.NewReader("ok\n??\n"))
	if !errors.Is(err, profanitymask.ErrInvalidTerm) {
		t.Fatalf("readDictionary returned %v, want ErrInvalidTerm", err)
	}
}

func TestReadDictionaryRejectsTooManyTerms(t *testing.T) {
	t.Parallel()
	_, err := readDictionary(strings.NewReader(strings.Repeat("ab\n", 501)))
	if !errors.Is(err, profanitymask.ErrTooManyTerms) {
		t.Fatalf("readDictionary returned %v, want ErrTooManyTerms", err)
	}
}

func TestEachLineHandlesBoundaries(t *testing.T) {
	t.Parallel()
	var lines []string
	err := eachLine(strings.NewReader("one\r\ntwo\n\nlast"), func(line string) error {
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatalf("eachLine returned error %v", err)
	}
	want := []string{"one", "two", "", "last"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("eachLine = %q, want %q", lines, want)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("read boom")
}

func TestEachLinePropagatesReadError(t *testing.T) {
	t.Parallel()
	err := eachLine(failingReader{}, func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "read boom") {
		t.Fatalf("eachLine returned %v, want read error", err)
	}
}

func TestEachLinePropagatesVisitorError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("visit boom")
	err := eachLine(strings.NewReader("line\n"), func(string) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("eachLine returned %v, want %v", err, wantErr)
	}
}

func TestReadDictionaryPropagatesReadError(t *testing.T) {
	t.Parallel()
	if _, err := readDictionary(failingReader{}); err == nil || !strings.Contains(err.Error(), "read boom") {
		t.Fatalf("readDictionary returned %v, want read error", err)
	}
}

func TestRunPropagatesReadError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run(nil, failingReader{}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("run = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "read boom") {
		t.Fatalf("stderr = %q, want read error", stderr.String())
	}
}
