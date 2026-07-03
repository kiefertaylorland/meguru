// Package cli wires Meguru's Cobra command tree. TTY detection here decides
// whether the review command dispatches to the interactive TUI or the plain
// renderer.
package cli

import "github.com/spf13/cobra"

// NewRootCommand builds the Cobra root command tree. With no subcommand,
// root prints help and exits 0 (M1 scope — no default-to-review behavior).
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "meguru",
		Short: "Meguru is an offline-first, terminal-native Japanese SRS",
		// A runtime error (DB failure, seed failure, etc.) is not a
		// flag-parsing mistake — don't dump the usage block for it, and
		// don't let Cobra print the error itself; main.go is the single
		// place that prints it (contracts/cli.md's exit-code contract).
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newReviewCommand())
	return root
}
