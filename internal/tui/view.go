package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// Minimum supported terminal size (research.md #2) — below this, every
// screen shows a "terminal too small" message instead of its own content.
const (
	minWidth  = 80
	minHeight = 24
)

var (
	cardStyle     = lipgloss.NewStyle().Bold(true).Padding(1, 2).Border(lipgloss.RoundedBorder())
	hintStyle     = lipgloss.NewStyle().Faint(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
)

// View renders the current screen (start menu, stats, or review), filling
// the terminal's current width/height, or the "nothing due"/error/too-small
// message in its place.
func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Model) render() string {
	if m.err != nil {
		return fmt.Sprintf("error: %v\n", m.err)
	}
	if m.quitting {
		return ""
	}

	// width/height are zero until the first tea.WindowSizeMsg arrives (and
	// in tests that construct a Model directly without one) — render
	// unclamped in that transient case rather than collapsing content into
	// a zero-size box.
	knownSize := m.width > 0 && m.height > 0
	if knownSize && (m.width < minWidth || m.height < minHeight) {
		return fmt.Sprintf("Terminal window is too small.\nResize to at least %dx%d and try again.\n", minWidth, minHeight)
	}

	var body string
	switch m.screen {
	case screenStartMenu:
		body = m.renderStartMenu()
	case screenStats:
		body = m.renderStats()
	default:
		body = m.renderReview()
	}

	if !knownSize {
		return body
	}
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(body)
}

func (m Model) renderStartMenu() string {
	var b strings.Builder
	for i, item := range m.menuItems {
		line := item.Label
		if i == m.menuSelected {
			line = selectedStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line)
		if i < len(m.menuItems)-1 {
			b.WriteString("\n")
		}
	}
	body := b.String() + "\n\n" + hintStyle.Render("↑/k ↓/j move  enter select  q quit")
	return cardStyle.Render(body)
}

func (m Model) renderStats() string {
	if m.statsErr != nil {
		return cardStyle.Render(fmt.Sprintf("error loading stats: %v\n\n%s",
			m.statsErr, hintStyle.Render("esc back  q quit")))
	}
	if m.statsSummary == nil {
		return cardStyle.Render("Loading stats...")
	}

	retention := "unavailable"
	if m.statsSummary.RetentionPercent != nil {
		retention = fmt.Sprintf("%.0f%%", *m.statsSummary.RetentionPercent)
	}
	body := fmt.Sprintf(
		"Due: %d    Total: %d\nStreak: %d day(s)\nRetention (%dd): %s\n\n%s",
		m.statsSummary.DueCards, m.statsSummary.TotalCards, m.statsSummary.StreakDays,
		m.statsSummary.RetentionWindowDays, retention,
		hintStyle.Render("esc back  q quit"),
	)
	return cardStyle.Render(body)
}

func (m Model) renderReview() string {
	if m.noneDue {
		return "Nothing due right now.\n"
	}
	if m.card == nil {
		return "Loading...\n"
	}

	var body string
	if !m.revealed {
		body = fmt.Sprintf("%s\n\n%s", m.card.Expression, hintStyle.Render("press space/enter to reveal"))
	} else {
		body = fmt.Sprintf("%s\n%s — %s\n\n%s",
			m.card.Expression, m.card.Reading, m.card.Meaning,
			hintStyle.Render("1=Again 2=Hard 3=Good 4=Easy"))
	}
	return cardStyle.Render(body) + "\n"
}
