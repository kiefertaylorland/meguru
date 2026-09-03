package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"meguru/internal/review"
	"meguru/internal/stats"
)

func TestView_ErrorState(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.err = errors.New("kaboom")

	view := m.View()

	require.Contains(t, view.Content, "error: kaboom")
}

func TestView_Quitting(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.quitting = true

	view := m.View()

	require.Equal(t, "", view.Content)
}

func TestView_NoneDue(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenReview
	m.noneDue = true

	view := m.View()

	require.Contains(t, view.Content, "Nothing due right now.")
}

func TestView_Loading(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenReview

	view := m.View()

	require.Contains(t, view.Content, "Loading...")
}

func TestView_CardFrontOnly(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenReview
	m.card = &review.Card{Expression: "あ", Reading: "a", Meaning: "a"}

	view := m.View()

	require.Contains(t, view.Content, "あ")
	require.Contains(t, view.Content, "press space/enter to reveal")
	require.NotContains(t, view.Content, "1=Again")
}

func TestView_CardRevealed(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenReview
	m.card = &review.Card{Expression: "あ", Reading: "a", Meaning: "a"}
	m.revealed = true

	view := m.View()

	require.Contains(t, view.Content, "a — a")
	require.Contains(t, view.Content, "1=Again 2=Hard 3=Good 4=Easy")
}

func TestView_StartMenu_ListsAllActionsAndMarksSelection(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	view := m.View()

	require.Contains(t, view.Content, "Start Review")
	require.Contains(t, view.Content, "Study a Deck")
	require.Contains(t, view.Content, "View Stats")
	require.Contains(t, view.Content, "Quit")
	require.Contains(t, view.Content, "> Start Review", "first item is selected by default")
}

func TestView_DeckPicker_ListsAllDecksAndMarksSelection(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{},
		[]review.DeckScope{{Slug: "kana-hiragana", Name: "Hiragana"}, {Slug: "kana-katakana", Name: "Katakana"}},
		review.DeckScope{})
	m.screen = screenDeckPicker

	view := m.View()

	require.Contains(t, view.Content, "Hiragana")
	require.Contains(t, view.Content, "Katakana")
	require.Contains(t, view.Content, "> Hiragana", "first deck is selected by default")
}

func TestView_DeckPicker_MovingSelectionMarksNewDeck(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{},
		[]review.DeckScope{{Slug: "kana-hiragana", Name: "Hiragana"}, {Slug: "kana-katakana", Name: "Katakana"}},
		review.DeckScope{})
	m.screen = screenDeckPicker
	m.deckSelected = 1

	view := m.View()

	require.Contains(t, view.Content, "> Katakana")
	require.NotContains(t, view.Content, "> Hiragana")
}

func TestView_Review_ScopedShowsStudyingLine(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil,
		review.DeckScope{Slug: "kana-hiragana", Name: "Hiragana"})
	m.screen = screenReview
	m.card = &review.Card{Expression: "あ", Reading: "a", Meaning: "a"}

	view := m.View()

	require.Contains(t, view.Content, "Studying: Hiragana")
}

func TestView_Review_UnscopedHasNoStudyingLine(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenReview
	m.card = &review.Card{Expression: "あ", Reading: "a", Meaning: "a"}

	view := m.View()

	require.NotContains(t, view.Content, "Studying:")
}

func TestView_Review_ScopedNoneDueNamesTheDeck(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil,
		review.DeckScope{Slug: "jlpt-n5-kanji", Name: "JLPT N5 Kanji"})
	m.screen = screenReview
	m.noneDue = true

	view := m.View()

	require.Contains(t, view.Content, "Nothing due in JLPT N5 Kanji right now.")
}

func TestView_StartMenu_MovingSelectionMarksNewItem(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.menuSelected = 1

	view := m.View()

	require.Contains(t, view.Content, "> Study a Deck")
	require.NotContains(t, view.Content, "> Start Review")
}

func TestView_Stats_ShowsSummaryFigures(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenStats
	retention := 87.0
	m.statsSummary = &stats.Summary{
		DueCards: 3, TotalCards: 42, StreakDays: 5,
		RetentionPercent: &retention, RetentionWindowDays: 30,
	}

	view := m.View()

	require.Contains(t, view.Content, "Due: 3")
	require.Contains(t, view.Content, "Total: 42")
	require.Contains(t, view.Content, "Streak: 5")
	require.Contains(t, view.Content, "87%")
}

func TestView_Stats_NoHistoryShowsUnavailableRetention(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenStats
	m.statsSummary = &stats.Summary{RetentionPercent: nil}

	view := m.View()

	require.Contains(t, view.Content, "unavailable")
}

func TestView_Stats_ErrorShowsMessageNotSummary(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenStats
	m.statsErr = errors.New("db exploded")

	view := m.View()

	require.Contains(t, view.Content, "db exploded")
}

func TestView_BelowMinimumSize_ShowsTooSmallMessage(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.width, m.height = 40, 10

	view := m.View()

	require.Contains(t, view.Content, "too small")
	require.NotContains(t, view.Content, "Start Review")
}

func TestView_AtOrAboveMinimumSize_RendersScreenContent(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.width, m.height = minWidth, minHeight

	view := m.View()

	require.Contains(t, view.Content, "Start Review")
}

func TestView_ReflowsBetweenDifferentKnownSizes(t *testing.T) {
	small := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	small.width, small.height = 80, 24

	large := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	large.width, large.height = 200, 60

	require.NotEqual(t, small.View().Content, large.View().Content)
}
