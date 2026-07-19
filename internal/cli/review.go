package cli

import (
	"fmt"
	"os"
	"strings"
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
	var deckFlag string

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run one review session against due cards",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReview(cmd, plainFlag, deckFlag)
		},
	}
	cmd.Flags().BoolVar(&plainFlag, "plain", false, "Force the linear, non-interactive renderer")
	cmd.Flags().StringVar(&deckFlag, "deck", "", "Scope the session to one deck by slug (default: every deck)")
	return cmd
}

// runReview performs the startup sequence from contracts/cli.md — open (with
// self-healing permissions), migrate, seed — then dispatches to the
// interactive TUI or the plain renderer.
func runReview(cmd *cobra.Command, plainFlag bool, deckFlag string) error {
	scope, err := resolveDeckFlag(deckFlag)
	if err != nil {
		return err
	}

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
		return plain.Run(cmd.Context(), svc, cmd.InOrStdin(), cmd.OutOrStdout(), scope)
	}

	statsSvc := stats.NewService(db)
	finalModel, err := tea.NewProgram(
		tui.New(cmd.Context(), svc, statsSvc, deckScopes(deck.BuiltinDecks()), scope),
		programOptions()...,
	).Run()
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

// resolveDeckFlag resolves --deck's raw value against the built-in deck
// registry (contracts/007-deck-filter/review-cli.md). An empty slug is the
// unfiltered case (FR-002); an unrecognized non-empty slug is a clear,
// side-effect-free error before any database work happens (FR-004).
func resolveDeckFlag(slug string) (review.DeckScope, error) {
	if slug == "" {
		return review.DeckScope{}, nil
	}
	for _, d := range deck.BuiltinDecks() {
		if d.Slug == slug {
			return review.DeckScope{Slug: d.Slug, Name: d.Name}, nil
		}
	}

	var choices []string
	for _, d := range deck.BuiltinDecks() {
		choices = append(choices, fmt.Sprintf("%s (%s)", d.Slug, d.Name))
	}
	return review.DeckScope{}, fmt.Errorf("unknown deck %q — valid decks: %s", slug, strings.Join(choices, ", "))
}

// deckScopes builds the interactive TUI's deck-picker list from the built-in
// deck registry (research.md #2: internal/tui itself never imports
// internal/deck).
func deckScopes(defs []deck.Definition) []review.DeckScope {
	scopes := make([]review.DeckScope, len(defs))
	for i, d := range defs {
		scopes[i] = review.DeckScope{Slug: d.Slug, Name: d.Name}
	}
	return scopes
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
