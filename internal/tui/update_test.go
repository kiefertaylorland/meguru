package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"meguru/internal/review"
	"meguru/internal/scheduler"
	"meguru/internal/stats"
)

type fakeService struct {
	card       *review.Card
	nextErr    error
	rateErr    error
	rateCalls  int
	lastCardID int64
	lastRating scheduler.Rating
}

func (f *fakeService) NextDueCard(ctx context.Context, scope review.DeckScope) (*review.Card, error) {
	if f.nextErr != nil {
		return nil, f.nextErr
	}
	return f.card, nil
}

func (f *fakeService) Rate(ctx context.Context, cardID int64, rating scheduler.Rating, now time.Time) error {
	f.rateCalls++
	f.lastCardID = cardID
	f.lastRating = rating
	return f.rateErr
}

// fakeStatsService mimics stats.Service for the start menu's "View Stats"
// screen (no fake for stats.Service existed anywhere in the repo before
// this feature — research.md #5).
type fakeStatsService struct {
	summary    stats.Summary
	computeErr error
	calls      int
}

func (f *fakeStatsService) Compute(ctx context.Context, now time.Time) (stats.Summary, error) {
	f.calls++
	if f.computeErr != nil {
		return stats.Summary{}, f.computeErr
	}
	return f.summary, nil
}

func asModel(t *testing.T, m tea.Model) Model {
	t.Helper()
	tm, ok := m.(Model)
	require.True(t, ok, "expected tui.Model, got %T", m)
	return tm
}

func isQuitCmd(t *testing.T, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestErr_ReturnsStoredError(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.err = errors.New("boom")
	require.EqualError(t, m.Err(), "boom")
}

func TestErr_NilWhenNoError(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	require.NoError(t, m.Err())
}

func TestUpdate_CardMsg_SetsCardAndResetsRevealAndSubmitting(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.revealed = true
	m.submitting = true

	card := &review.Card{ID: 1, Expression: "a"}
	updated, cmd := m.Update(cardMsg{card: card})

	got := asModel(t, updated)
	require.Equal(t, card, got.card)
	require.False(t, got.revealed)
	require.False(t, got.submitting)
	require.False(t, got.noneDue)
	require.Nil(t, cmd)
}

func TestUpdate_CardMsg_NilCardSetsNoneDue(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	updated, _ := m.Update(cardMsg{card: nil})

	require.True(t, asModel(t, updated).noneDue)
}

func TestUpdate_ErrMsg_SetsErrAndQuits(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	updated, cmd := m.Update(errMsg{err: errors.New("db exploded")})

	got := asModel(t, updated)
	require.EqualError(t, got.Err(), "db exploded")
	require.True(t, isQuitCmd(t, cmd))
}

func TestUpdate_RatedMsg_Success_ResetsSubmittingAndReloads(t *testing.T) {
	svc := &fakeService{card: &review.Card{ID: 1}}
	m := New(context.Background(), svc, &fakeStatsService{}, nil, review.DeckScope{})
	m.submitting = true

	updated, cmd := m.Update(ratedMsg{err: nil})

	got := asModel(t, updated)
	require.False(t, got.submitting)
	require.NotNil(t, cmd)
	// The returned cmd is loadNextCard; invoking it exercises the real
	// review.Service call and should hand back a cardMsg for svc.card.
	msg := cmd()
	cm, ok := msg.(cardMsg)
	require.True(t, ok)
	require.Equal(t, svc.card, cm.card)
}

func TestUpdate_RatedMsg_Error_SetsErrAndQuits(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.submitting = true

	updated, cmd := m.Update(ratedMsg{err: errors.New("write failed")})

	got := asModel(t, updated)
	require.False(t, got.submitting)
	require.EqualError(t, got.Err(), "write failed")
	require.True(t, isQuitCmd(t, cmd))
}

func quitKey(key rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: key, Text: string(key)}
}

func TestUpdate_KeyPressDispatchesToHandleKey(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.noneDue = true

	updated, cmd := m.Update(quitKey('q'))

	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))
}

func TestHandleKey_ErrSuppressesAllInput(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.err = errors.New("already failed")

	updated, cmd := m.handleKey(quitKey('q'))

	require.False(t, asModel(t, updated).quitting)
	require.Nil(t, cmd)
}

func TestHandleKey_CtrlCQuits(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	updated, cmd := m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))
}

func TestHandleKey_NoneDueAnyKeyQuits(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.noneDue = true

	updated, cmd := m.handleKey(quitKey('x'))

	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))
}

func TestHandleKey_StillLoadingIgnoresKey(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	// card == nil, noneDue == false: still loading.

	updated, cmd := m.handleKey(quitKey('3'))

	got := asModel(t, updated)
	require.False(t, got.quitting)
	require.Nil(t, cmd)
}

func TestHandleKey_RevealsOnSpace(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.card = &review.Card{ID: 1}

	updated, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	require.True(t, asModel(t, updated).revealed)
	require.Nil(t, cmd)
}

func TestHandleKey_RevealsOnEnter(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.card = &review.Card{ID: 1}

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.True(t, asModel(t, updated).revealed)
}

func TestHandleKey_UnrecognizedKeyBeforeRevealDoesNothing(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.card = &review.Card{ID: 1}

	updated, cmd := m.handleKey(quitKey('z'))

	require.False(t, asModel(t, updated).revealed)
	require.Nil(t, cmd)
}

func TestHandleKey_SubmittingGuardBlocksRepeatRating(t *testing.T) {
	svc := &fakeService{}
	m := New(context.Background(), svc, &fakeStatsService{}, nil, review.DeckScope{})
	m.card = &review.Card{ID: 1}
	m.revealed = true
	m.submitting = true

	updated, cmd := m.handleKey(quitKey('3'))

	require.True(t, asModel(t, updated).submitting)
	require.Nil(t, cmd)
	require.Zero(t, svc.rateCalls, "a rating in flight must block a second submission")
}

func TestHandleKey_RatingFiresRateAndSetsSubmitting(t *testing.T) {
	svc := &fakeService{}
	m := New(context.Background(), svc, &fakeStatsService{}, nil, review.DeckScope{})
	m.card = &review.Card{ID: 42}
	m.revealed = true

	updated, cmd := m.handleKey(quitKey('3'))

	got := asModel(t, updated)
	require.True(t, got.submitting)
	require.NotNil(t, cmd)

	msg := cmd()
	rm, ok := msg.(ratedMsg)
	require.True(t, ok)
	require.NoError(t, rm.err)
	require.Equal(t, 1, svc.rateCalls)
	require.Equal(t, int64(42), svc.lastCardID)
	require.Equal(t, scheduler.Good, svc.lastRating)
}

func TestHandleKey_InvalidRatingKeyAfterRevealDoesNothing(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.card = &review.Card{ID: 1}
	m.revealed = true

	updated, cmd := m.handleKey(quitKey('z'))

	require.False(t, asModel(t, updated).submitting)
	require.Nil(t, cmd)
}

func TestRatingFromKey_AllVariants(t *testing.T) {
	cases := map[string]scheduler.Rating{
		"1": scheduler.Again, "a": scheduler.Again,
		"2": scheduler.Hard, "h": scheduler.Hard,
		"3": scheduler.Good, "g": scheduler.Good,
		"4": scheduler.Easy, "e": scheduler.Easy,
	}
	for key, want := range cases {
		got, ok := ratingFromKey(key)
		require.True(t, ok, "key %q", key)
		require.Equal(t, want, got, "key %q", key)
	}
}

func TestRatingFromKey_Invalid(t *testing.T) {
	_, ok := ratingFromKey("z")
	require.False(t, ok)
}

func TestInit_ReturnsNil_StartMenuIsFirstScreen(t *testing.T) {
	svc := &fakeService{card: &review.Card{ID: 7}}
	m := New(context.Background(), svc, &fakeStatsService{}, nil, review.DeckScope{})

	require.Nil(t, m.Init(), "the start menu is the first screen; Init no longer auto-loads a card")
	require.Equal(t, screenStartMenu, m.screen)
}

func TestLoadNextCard_PropagatesError(t *testing.T) {
	svc := &fakeService{nextErr: errors.New("query failed")}
	m := New(context.Background(), svc, &fakeStatsService{}, nil, review.DeckScope{})

	msg := m.loadNextCard()

	em, ok := msg.(errMsg)
	require.True(t, ok)
	require.EqualError(t, em.err, "query failed")
}

// --- Start menu navigation (US1) ---

func downKey() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyDown} }
func upKey() tea.KeyPressMsg   { return tea.KeyPressMsg{Code: tea.KeyUp} }

func TestHandleStartMenuKey_DownMovesSelectionAndClampsAtLast(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	updated, _ := m.handleStartMenuKey(downKey())
	m = asModel(t, updated)
	require.Equal(t, 1, m.menuSelected)

	updated, _ = m.handleStartMenuKey(downKey())
	m = asModel(t, updated)
	require.Equal(t, 2, m.menuSelected)

	updated, _ = m.handleStartMenuKey(downKey())
	m = asModel(t, updated)
	require.Equal(t, 3, m.menuSelected)

	updated, _ = m.handleStartMenuKey(downKey())
	m = asModel(t, updated)
	require.Equal(t, 3, m.menuSelected, "clamps at the last item")
}

func TestHandleStartMenuKey_JKMoveSelectionSameAsArrows(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	updated, _ := m.handleStartMenuKey(quitKey('j'))
	require.Equal(t, 1, asModel(t, updated).menuSelected)

	updated, _ = asModel(t, updated).handleStartMenuKey(quitKey('k'))
	require.Equal(t, 0, asModel(t, updated).menuSelected)
}

func TestHandleStartMenuKey_UpClampsAtFirst(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	updated, _ := m.handleStartMenuKey(upKey())

	require.Equal(t, 0, asModel(t, updated).menuSelected, "clamps at the first item")
}

func TestHandleStartMenuKey_EnterOnStartReview_TransitionsToReviewAndLoadsCard(t *testing.T) {
	svc := &fakeService{card: &review.Card{ID: 7}}
	m := New(context.Background(), svc, &fakeStatsService{}, nil, review.DeckScope{})

	updated, cmd := m.handleStartMenuKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	got := asModel(t, updated)
	require.Equal(t, screenReview, got.screen)
	require.NotNil(t, cmd)
	msg := cmd()
	cm, ok := msg.(cardMsg)
	require.True(t, ok)
	require.Equal(t, svc.card, cm.card)
}

func TestHandleStartMenuKey_EnterOnViewStats_TransitionsToStatsAndFetches(t *testing.T) {
	statsSvc := &fakeStatsService{summary: stats.Summary{DueCards: 2}}
	m := New(context.Background(), &fakeService{}, statsSvc, nil, review.DeckScope{})
	m.menuSelected = 2 // Start Review, Study a Deck, View Stats, Quit

	updated, cmd := m.handleStartMenuKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	got := asModel(t, updated)
	require.Equal(t, screenStats, got.screen)
	require.NotNil(t, cmd)
	msg := cmd()
	sm, ok := msg.(statsMsg)
	require.True(t, ok)
	require.Equal(t, 2, sm.summary.DueCards)
	require.Equal(t, 1, statsSvc.calls)
}

func TestHandleStartMenuKey_EnterOnQuit_Quits(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.menuSelected = 3 // Start Review, Study a Deck, View Stats, Quit

	updated, cmd := m.handleStartMenuKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))
}

func TestHandleStartMenuKey_QAndCtrlCQuit(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	updated, cmd := m.handleStartMenuKey(quitKey('q'))
	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))

	m2 := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	updated2, cmd2 := m2.handleStartMenuKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	require.True(t, asModel(t, updated2).quitting)
	require.True(t, isQuitCmd(t, cmd2))
}

// --- Stats screen (US2) ---

func TestLoadStats_ReturnsStatsMsgOnSuccess(t *testing.T) {
	statsSvc := &fakeStatsService{summary: stats.Summary{DueCards: 5, StreakDays: 3}}
	m := New(context.Background(), &fakeService{}, statsSvc, nil, review.DeckScope{})

	msg := m.loadStats()

	sm, ok := msg.(statsMsg)
	require.True(t, ok)
	require.Equal(t, 5, sm.summary.DueCards)
	require.Equal(t, 3, sm.summary.StreakDays)
}

func TestLoadStats_ReturnsStatsErrMsgOnFailure(t *testing.T) {
	statsSvc := &fakeStatsService{computeErr: errors.New("query failed")}
	m := New(context.Background(), &fakeService{}, statsSvc, nil, review.DeckScope{})

	msg := m.loadStats()

	em, ok := msg.(statsErrMsg)
	require.True(t, ok)
	require.EqualError(t, em.err, "query failed")
}

func TestUpdate_StatsMsg_StoresSummaryAndClearsErr(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.statsErr = errors.New("stale error")

	updated, cmd := m.Update(statsMsg{summary: stats.Summary{DueCards: 9}})

	got := asModel(t, updated)
	require.NotNil(t, got.statsSummary)
	require.Equal(t, 9, got.statsSummary.DueCards)
	require.NoError(t, got.statsErr)
	require.Nil(t, cmd)
}

func TestUpdate_StatsErrMsg_StoresErr(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	updated, cmd := m.Update(statsErrMsg{err: errors.New("db exploded")})

	require.EqualError(t, asModel(t, updated).statsErr, "db exploded")
	require.Nil(t, cmd)
}

func TestHandleStatsKey_EscReturnsToStartMenu(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenStats

	updated, cmd := m.handleStatsKey(tea.KeyPressMsg{Code: tea.KeyEsc})

	require.Equal(t, screenStartMenu, asModel(t, updated).screen)
	require.Nil(t, cmd)
}

func TestHandleStatsKey_QAndCtrlCQuit(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenStats

	updated, cmd := m.handleStatsKey(quitKey('q'))

	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))
}

func TestUpdate_RoutesKeyPressByScreen(t *testing.T) {
	menu := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	updated, _ := menu.Update(downKey())
	require.Equal(t, 1, asModel(t, updated).menuSelected, "start menu screen routes to handleStartMenuKey")

	statsScreen := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	statsScreen.screen = screenStats
	updated, _ = statsScreen.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	require.Equal(t, screenStartMenu, asModel(t, updated).screen, "stats screen routes to handleStatsKey")

	reviewScreen := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	reviewScreen.screen = screenReview
	reviewScreen.card = &review.Card{ID: 1}
	updated, _ = reviewScreen.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	require.True(t, asModel(t, updated).revealed, "review screen routes to handleKey")
}

// --- Deck picker (007-deck-filter US2) ---

func testDecks() []review.DeckScope {
	return []review.DeckScope{
		{Slug: "kana-hiragana", Name: "Hiragana"},
		{Slug: "kana-katakana", Name: "Katakana"},
		{Slug: "jlpt-n5-kanji", Name: "JLPT N5 Kanji"},
	}
}

func TestHandleStartMenuKey_EnterOnStudyADeck_TransitionsToDeckPicker(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, testDecks(), review.DeckScope{})
	m.menuSelected = 1

	updated, cmd := m.handleStartMenuKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	got := asModel(t, updated)
	require.Equal(t, screenDeckPicker, got.screen)
	require.Equal(t, 0, got.deckSelected)
	require.Nil(t, cmd)
}

func TestHandleDeckPickerKey_DownMovesSelectionAndClampsAtLast(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, testDecks(), review.DeckScope{})

	updated, _ := m.handleDeckPickerKey(downKey())
	m = asModel(t, updated)
	require.Equal(t, 1, m.deckSelected)

	updated, _ = m.handleDeckPickerKey(downKey())
	m = asModel(t, updated)
	require.Equal(t, 2, m.deckSelected)

	updated, _ = m.handleDeckPickerKey(downKey())
	m = asModel(t, updated)
	require.Equal(t, 2, m.deckSelected, "clamps at the last deck")
}

func TestHandleDeckPickerKey_UpClampsAtFirst(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, testDecks(), review.DeckScope{})

	updated, _ := m.handleDeckPickerKey(upKey())

	require.Equal(t, 0, asModel(t, updated).deckSelected)
}

func TestHandleDeckPickerKey_JKMoveSelectionSameAsArrows(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, testDecks(), review.DeckScope{})

	updated, _ := m.handleDeckPickerKey(quitKey('j'))
	require.Equal(t, 1, asModel(t, updated).deckSelected)

	updated, _ = asModel(t, updated).handleDeckPickerKey(quitKey('k'))
	require.Equal(t, 0, asModel(t, updated).deckSelected)
}

func TestHandleDeckPickerKey_EnterSetsActiveDeckAndStartsReview(t *testing.T) {
	svc := &fakeService{card: &review.Card{ID: 9}}
	m := New(context.Background(), svc, &fakeStatsService{}, testDecks(), review.DeckScope{})
	m.deckSelected = 2 // JLPT N5 Kanji

	updated, cmd := m.handleDeckPickerKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	got := asModel(t, updated)
	require.Equal(t, screenReview, got.screen)
	require.Equal(t, review.DeckScope{Slug: "jlpt-n5-kanji", Name: "JLPT N5 Kanji"}, got.activeDeck)
	require.NotNil(t, cmd)
	msg := cmd()
	cm, ok := msg.(cardMsg)
	require.True(t, ok)
	require.Equal(t, svc.card, cm.card)
}

func TestHandleDeckPickerKey_EscReturnsToStartMenuWithoutChangingScope(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, testDecks(), review.DeckScope{})
	m.deckSelected = 1

	updated, cmd := m.handleDeckPickerKey(tea.KeyPressMsg{Code: tea.KeyEsc})

	got := asModel(t, updated)
	require.Equal(t, screenStartMenu, got.screen)
	require.Equal(t, review.DeckScope{}, got.activeDeck)
	require.Nil(t, cmd)
}

func TestHandleDeckPickerKey_QAndCtrlCQuit(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, testDecks(), review.DeckScope{})

	updated, cmd := m.handleDeckPickerKey(quitKey('q'))
	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))
}

func TestUpdate_RoutesDeckPickerKeyPress(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, testDecks(), review.DeckScope{})
	m.screen = screenDeckPicker

	updated, _ := m.Update(downKey())

	require.Equal(t, 1, asModel(t, updated).deckSelected, "deck-picker screen routes to handleDeckPickerKey")
}

func TestHandleStartMenuKey_StartReview_UsesWhateverActiveDeckAlreadyIs(t *testing.T) {
	// Start Review honors a scope already set (e.g. by --deck), it doesn't
	// reset it (research.md #5).
	svc := &fakeService{card: &review.Card{ID: 1}}
	scope := review.DeckScope{Slug: "kana-katakana", Name: "Katakana"}
	m := New(context.Background(), svc, &fakeStatsService{}, testDecks(), scope)

	updated, cmd := m.handleStartMenuKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	got := asModel(t, updated)
	require.Equal(t, screenReview, got.screen)
	require.Equal(t, scope, got.activeDeck)
	require.NotNil(t, cmd)
}

// --- Full-window layout (US3) ---

func TestUpdate_WindowSizeMsg_StoresWidthAndHeight(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	got := asModel(t, updated)
	require.Equal(t, 120, got.width)
	require.Equal(t, 40, got.height)
	require.Nil(t, cmd)
}

func TestUpdate_WindowSizeMsg_MidReviewPreservesCardAndRevealed(t *testing.T) {
	m := New(context.Background(), &fakeService{}, &fakeStatsService{}, nil, review.DeckScope{})
	m.screen = screenReview
	m.card = &review.Card{ID: 1, Expression: "あ"}
	m.revealed = true

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})

	got := asModel(t, updated)
	require.Equal(t, m.card, got.card)
	require.True(t, got.revealed)
}
