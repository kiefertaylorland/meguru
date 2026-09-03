package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"meguru/internal/scheduler"
)

// Init no longer auto-loads a card: the start menu is the first screen, and
// loading only begins once "Start Review" is selected (handleStartMenuKey).
func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) loadNextCard() tea.Msg {
	card, err := m.svc.NextDueCard(m.ctx, m.activeDeck)
	if err != nil {
		return errMsg{err}
	}
	return cardMsg{card}
}

func (m Model) loadStats() tea.Msg {
	summary, err := m.statsSvc.Compute(m.ctx, time.Now())
	if err != nil {
		return statsErrMsg{err}
	}
	return statsMsg{summary}
}

// Update handles one Bubble Tea message: terminal resize, loading due cards,
// revealing a card's answer, submitting a rating (FR-006), fetching stats,
// and clearly communicating when nothing is due (FR-005).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case cardMsg:
		m.card = msg.card
		m.revealed = false
		m.submitting = false
		m.noneDue = msg.card == nil
		if m.noneDue {
			return m, tea.Quit
		}
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

	case statsMsg:
		m.statsSummary = &msg.summary
		m.statsErr = nil
		return m, nil

	case statsErrMsg:
		m.statsErr = msg.err
		return m, nil

	case tea.KeyPressMsg:
		switch m.screen {
		case screenStartMenu:
			return m.handleStartMenuKey(msg)
		case screenStats:
			return m.handleStatsKey(msg)
		case screenDeckPicker:
			return m.handleDeckPickerKey(msg)
		default:
			return m.handleKey(msg)
		}
	}
	return m, nil
}

// handleStartMenuKey handles a keypress on the start menu (contracts/tui-screens.md).
func (m Model) handleStartMenuKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.menuSelected > 0 {
			m.menuSelected--
		}
		return m, nil

	case "down", "j":
		if m.menuSelected < len(m.menuItems)-1 {
			m.menuSelected++
		}
		return m, nil

	case "enter":
		switch m.menuItems[m.menuSelected].Action {
		case actionStartReview:
			m.screen = screenReview
			return m, m.loadNextCard
		case actionStudyDeck:
			m.screen = screenDeckPicker
			m.deckSelected = 0
			return m, nil
		case actionViewStats:
			m.screen = screenStats
			return m, m.loadStats
		case actionQuit:
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// handleDeckPickerKey handles a keypress on the deck-picker screen
// (contracts/tui-deck-picker.md).
func (m Model) handleDeckPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.deckSelected > 0 {
			m.deckSelected--
		}
		return m, nil

	case "down", "j":
		if m.deckSelected < len(m.deckOptions)-1 {
			m.deckSelected++
		}
		return m, nil

	case "enter":
		if len(m.deckOptions) == 0 {
			return m, nil
		}
		m.activeDeck = m.deckOptions[m.deckSelected]
		m.screen = screenReview
		return m, m.loadNextCard

	case "esc":
		m.screen = screenStartMenu
		return m, nil
	}
	return m, nil
}

// handleStatsKey handles a keypress on the stats screen (contracts/tui-screens.md).
func (m Model) handleStatsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.screen = screenStartMenu
		return m, nil
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
