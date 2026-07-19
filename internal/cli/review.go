package cli

import (
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"meguru/internal/deck"
	"meguru/internal/plain"
	"meguru/internal/review"
	"meguru/internal/stats"
	"meguru/internal/storage"
	"meguru/internal/tui"
)

func newReviewCommand() *cobra.Command {
	var plainFlag bool

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run one review session against due cards",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReview(cmd, plainFlag)
		},
	}
	cmd.Flags().BoolVar(&plainFlag, "plain", false, "Force the linear, non-interactive renderer")
	return cmd
}

// runReview performs the startup sequence from contracts/cli.md — open (with
// self-healing permissions), migrate, seed — then dispatches to the
// interactive TUI or the plain renderer.
func runReview(cmd *cobra.Command, plainFlag bool) error {
	db, err := storage.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := storage.Migrate(db); err != nil {
		return err
	}
	if err := deck.Seed(cmd.Context(), db, time.Now()); err != nil {
		return err
	}

	svc := review.NewService(db)

	// --plain, or a non-TTY stdout, forces the linear renderer (FR-010).
	if shouldUsePlain(plainFlag, isatty.IsTerminal(os.Stdout.Fd())) {
		return plain.Run(cmd.Context(), svc, cmd.InOrStdin(), cmd.OutOrStdout())
	}

	statsSvc := stats.NewService(db)
	finalModel, err := tea.NewProgram(tui.New(cmd.Context(), svc, statsSvc), programOptions()...).Run()
	if err != nil {
		return err
	}
	// tea.Quit carries no error payload, so a failure inside the TUI (e.g.
	// review.Service.Rate erroring) must be read back off the final model —
	// otherwise it would be swallowed and the process would exit 0 despite
	// an unrecoverable error (contracts/cli.md).
	if m, ok := finalModel.(tui.Model); ok {
		return m.Err()
	}
	return nil
}

// shouldUsePlain is the FR-010 dispatch decision, isolated from cobra/OS
// wiring so it's directly unit-testable.
func shouldUsePlain(plainFlag, stdoutIsTTY bool) bool {
	return plainFlag || !stdoutIsTTY
}

// programOptions builds the Bubble Tea program options for the interactive
// TUI. NO_COLOR alone still uses the interactive TUI — it only forces
// Bubble Tea's output profile to emit no color/style codes (FR-011).
func programOptions() []tea.ProgramOption {
	if os.Getenv("NO_COLOR") != "" {
		return []tea.ProgramOption{tea.WithColorProfile(colorprofile.Ascii)}
	}
	return nil
}
