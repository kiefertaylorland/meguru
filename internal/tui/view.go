package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

var (
	cardStyle = lipgloss.NewStyle().Bold(true).Padding(1, 2).Border(lipgloss.RoundedBorder())
	hintStyle = lipgloss.NewStyle().Faint(true)
)

// View renders the current card (front-only until revealed, then with its
// reading/meaning and rating hint), the "nothing due" message, or an error.
func (m Model) View() tea.View {
	if m.err != nil {
		return tea.NewView(fmt.Sprintf("error: %v\n", m.err))
	}
	if m.quitting {
		return tea.NewView("")
	}
	if m.noneDue {
		return tea.NewView("Nothing due right now.\n")
	}
	if m.card == nil {
		return tea.NewView("Loading...\n")
	}

	var body string
	if !m.revealed {
		body = fmt.Sprintf("%s\n\n%s", m.card.Expression, hintStyle.Render("press space/enter to reveal"))
	} else {
		body = fmt.Sprintf("%s\n%s — %s\n\n%s",
			m.card.Expression, m.card.Reading, m.card.Meaning,
			hintStyle.Render("1=Again 2=Hard 3=Good 4=Easy"))
	}

	return tea.NewView(cardStyle.Render(body) + "\n")
}
