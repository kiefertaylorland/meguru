// Command meguru is the CLI entrypoint: it builds the Cobra root command and
// wires dependencies.
package main

import (
	"fmt"
	"os"

	"meguru/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
