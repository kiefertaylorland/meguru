// Package tui implements the interactive Bubble Tea v2 review screen.
package tui

import (
	"context"

	"meguru/internal/review"
	"meguru/internal/stats"
)

// screen identifies which of the interactive TUI's screens is active.
type screen int

const (
	screenStartMenu screen = iota
	screenStats
	screenReview
	screenDeckPicker
)

// action identifies what happens when a start-menu item is activated.
type action int

const (
	actionStartReview action = iota
	actionStudyDeck
	actionViewStats
	actionQuit
)

// MenuItem is one selectable entry on the start menu.
type MenuItem struct {
	Label  string
	Action action
}

// Model is the Bubble Tea model for one interactive session: the start
// menu, the stats screen, and the review screen.
type Model struct {
	ctx      context.Context
	svc      review.Service
	statsSvc stats.Service

	screen       screen
	menuItems    []MenuItem
	menuSelected int

	width  int
	height int

	deckOptions  []review.DeckScope
	deckSelected int
	activeDeck   review.DeckScope

	card       *review.Card
	revealed   bool
	submitting bool
	noneDue    bool

	statsSummary *stats.Summary
	statsErr     error

	quitting bool
	err      error
}

// New builds the initial model for a review session against svc and
// statsSvc. The session opens on the start menu (data-model.md Screen
// transitions) — no card is loaded until "Start Review" or "Study a Deck"
// is selected. decks is the deck-picker's fixed list; initialScope seeds
// activeDeck (e.g. from a --deck flag) — its zero value means unfiltered.
func New(ctx context.Context, svc review.Service, statsSvc stats.Service, decks []review.DeckScope, initialScope review.DeckScope) Model {
	return Model{
		ctx:         ctx,
		svc:         svc,
		statsSvc:    statsSvc,
		screen:      screenStartMenu,
		deckOptions: decks,
		activeDeck:  initialScope,
		menuItems: []MenuItem{
			{Label: "Start Review", Action: actionStartReview},
			{Label: "Study a Deck", Action: actionStudyDeck},
			{Label: "View Stats", Action: actionViewStats},
			{Label: "Quit", Action: actionQuit},
		},
	}
}

// Err returns the error, if any, that ended the session. Bubble Tea's
// tea.Quit carries no error payload, so the caller of Program.Run() must
// inspect the returned final Model via this method to detect a failure and
// exit non-zero (contracts/cli.md: "1 = Unrecoverable error").
func (m Model) Err() error {
	return m.err
}

type cardMsg struct{ card *review.Card }
type errMsg struct{ err error }
type ratedMsg struct{ err error }
type statsMsg struct{ summary stats.Summary }
type statsErrMsg struct{ err error }
