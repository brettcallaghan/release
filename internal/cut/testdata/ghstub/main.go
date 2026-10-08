// Command ghstub stands in for gh in tests, on every platform: it appends
// its arguments, one per line, to the file GHSTUB_ARGS names, then either
// fails with GHSTUB_FAIL on standard error or prints a pull request URL.
package main

import (
	"fmt"
	"os"
)

func main() {
	if path := os.Getenv("GHSTUB_ARGS"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		for _, a := range os.Args[1:] {
			fmt.Fprintln(f, a)
		}
		f.Close()
	}
	if msg := os.Getenv("GHSTUB_FAIL"); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	fmt.Println("https://github.com/o/r/pull/7")
}
