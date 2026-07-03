package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"meguru/internal/scheduler"
)

// Init kicks off loading the first due card.
func (m Model) Init() tea.Cmd {
	return m.loadNextCard
}

func (m Model) loadNextCard() tea.Msg {
	card, err := m.svc.NextDueCard(m.ctx)
	if err != nil {
		return errMsg{err}
	}
	return cardMsg{card}
}

// Update handles one Bubble Tea message: loading due cards, revealing a
// card's answer, submitting a rating (FR-006), and clearly communicating
// when nothing is due (FR-005).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cardMsg:
		m.card = msg.card
		m.revealed = false
		m.submitting = false
		m.noneDue = msg.card == nil
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, tea.Quit

	case ratedMsg:
		m.submitting = false
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		return m, m.loadNextCard

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	}

	if m.noneDue {
		m.quitting = true
		return m, tea.Quit
	}
	if m.card == nil {
		return m, nil // still loading
	}

	if !m.revealed {
		if msg.String() == "space" || msg.String() == "enter" {
			m.revealed = true
		}
		return m, nil
	}

	if m.submitting {
		// A rating is already in flight for this card; ignore repeat
		// keypresses until ratedMsg lands, so a fast double-press can't
		// submit two ratings for the same card.
		return m, nil
	}

	rating, ok := ratingFromKey(msg.String())
	if !ok {
		return m, nil
	}
	m.submitting = true
	cardID := m.card.ID
	svc := m.svc
	ctx := m.ctx
	return m, func() tea.Msg {
		return ratedMsg{err: svc.Rate(ctx, cardID, rating, time.Now())}
	}
}

func ratingFromKey(key string) (scheduler.Rating, bool) {
	switch key {
	case "1", "a":
		return scheduler.Again, true
	case "2", "h":
		return scheduler.Hard, true
	case "3", "g":
		return scheduler.Good, true
	case "4", "e":
		return scheduler.Easy, true
	}
	return 0, false
}
