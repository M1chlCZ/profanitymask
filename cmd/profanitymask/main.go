// Command profanitymask masks configured terms in text.
//
// The command reads lines from the named files, or from standard input when
// no file is given, and writes masked lines to standard output. A dictionary
// file supplies one term per line; blank lines and comment lines that start
// with '#' are skipped.
package main

import (
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
