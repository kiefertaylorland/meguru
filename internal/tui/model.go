// Package tui implements the interactive Bubble Tea v2 review screen.
package tui

import (
	"context"

	"meguru/internal/review"
)

// Model is the Bubble Tea model for one review session.
type Model struct {
	ctx        context.Context
	svc        review.Service
	card       *review.Card
	revealed   bool
	submitting bool
	noneDue    bool
	quitting   bool
	err        error
}

// New builds the initial model for a review session against svc.
func New(ctx context.Context, svc review.Service) Model {
	return Model{ctx: ctx, svc: svc}
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
