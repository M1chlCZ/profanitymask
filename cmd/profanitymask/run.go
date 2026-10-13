package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/m1chlcz/profanitymask"
)

const (
	exitOK    = 0
	exitMatch = 1
	exitError = 2
)

type streamStats struct {
	lines   int
	matches int
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("profanitymask", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: profanitymask [-dictionary file] [-check] [-quiet] [file ...]")
		flags.PrintDefaults()
	}
	dictionaryPath := flags.String("dictionary", "", "term list file, one term per line")
	check := flags.Bool("check", false, "write nothing and exit 1 when a term matches")
	quiet := flags.Bool("quiet", false, "suppress the summary line")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}

	var dictionary profanitymask.Dictionary
	if *dictionaryPath != "" {
		file, err := os.Open(*dictionaryPath)
		if err != nil {
			return fail(stderr, err)
		}
		dictionary, err = readDictionary(file)
		file.Close()
		if err != nil {
			return fail(stderr, err)
		}
	}

	total := streamStats{}
	paths := flags.Args()
	if len(paths) == 0 {
		stats, err := processStream(stdin, stdout, dictionary, *check)
		if err != nil {
			return fail(stderr, err)
		}
		total = stats
	} else {
		for _, path := range paths {
			file, err := os.Open(path)
			if err != nil {
				return fail(stderr, err)
			}
			stats, err := processStream(file, stdout, dictionary, *check)
			file.Close()
			if err != nil {
				return fail(stderr, err)
			}
			total.lines += stats.lines
			total.matches += stats.matches
		}
	}

	if !*quiet {
		verb := "masked"
		if *check {
			verb = "matched"
		}
		fmt.Fprintf(stderr, "profanitymask: %s %d of %d lines\n", verb, total.matches, total.lines)
	}
	if *check && total.matches > 0 {
		return exitMatch
	}
	return exitOK
}

func fail(stderr io.Writer, err error) int {
	message := err.Error()
	if !strings.HasPrefix(message, "profanitymask:") {
		message = "profanitymask: " + message
	}
	fmt.Fprintln(stderr, message)
	return exitError
}

func readDictionary(source io.Reader) (profanitymask.Dictionary, error) {
	var terms []string
	err := eachLine(source, func(line string) error {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			return nil
		}
		terms = append(terms, line)
		return nil
	})
	if err != nil {
		return profanitymask.Dictionary{}, err
	}
	return profanitymask.NewDictionary(terms...)
}

func processStream(source io.Reader, output io.Writer, dictionary profanitymask.Dictionary, check bool) (streamStats, error) {
	stats := streamStats{}
	writer := bufio.NewWriter(output)
	err := eachLine(source, func(line string) error {
		stats.lines++
		if check {
			if profanitymask.ContainsWithDictionary(line, dictionary) {
				stats.matches++
			}
			return nil
		}
		masked, matched := profanitymask.MaskWithDictionary(line, dictionary)
		if matched {
			stats.matches++
		}
		_, err := writer.WriteString(masked + "\n")
		return err
	})
	if err != nil {
		return stats, err
	}
	if err := writer.Flush(); err != nil {
		return stats, err
	}
	return stats, nil
}

func eachLine(source io.Reader, visit func(line string) error) error {
	reader := bufio.NewReader(source)
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			if err := visit(line); err != nil {
				return err
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}
