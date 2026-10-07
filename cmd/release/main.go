// Command release is the Release CLI's entry point: it hands the process's
// arguments and standard streams to the command tree internal/cli builds,
// and exits with the status that tree decided on.
//
// It sits in cmd/release/ so that
// `go install github.com/brettcallaghan/release/cmd/release@vX.Y.Z` installs a
// binary called `release`; the module is at the repository root, so release
// tags are plain `vX.Y.Z`.
package main

import (
	"os"

	"github.com/brettcallaghan/release/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
