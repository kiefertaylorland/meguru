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
)

type fakeService struct {
	card       *review.Card
	nextErr    error
	rateErr    error
	rateCalls  int
	lastCardID int64
	lastRating scheduler.Rating
}

func (f *fakeService) NextDueCard(ctx context.Context) (*review.Card, error) {
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
	m := New(context.Background(), &fakeService{})
	m.err = errors.New("boom")
	require.EqualError(t, m.Err(), "boom")
}

func TestErr_NilWhenNoError(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	require.NoError(t, m.Err())
}

func TestUpdate_CardMsg_SetsCardAndResetsRevealAndSubmitting(t *testing.T) {
	m := New(context.Background(), &fakeService{})
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
	m := New(context.Background(), &fakeService{})

	updated, _ := m.Update(cardMsg{card: nil})

	require.True(t, asModel(t, updated).noneDue)
}

func TestUpdate_ErrMsg_SetsErrAndQuits(t *testing.T) {
	m := New(context.Background(), &fakeService{})

	updated, cmd := m.Update(errMsg{err: errors.New("db exploded")})

	got := asModel(t, updated)
	require.EqualError(t, got.Err(), "db exploded")
	require.True(t, isQuitCmd(t, cmd))
}

func TestUpdate_RatedMsg_Success_ResetsSubmittingAndReloads(t *testing.T) {
	svc := &fakeService{card: &review.Card{ID: 1}}
	m := New(context.Background(), svc)
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
	m := New(context.Background(), &fakeService{})
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
	m := New(context.Background(), &fakeService{})
	m.noneDue = true

	updated, cmd := m.Update(quitKey('q'))

	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))
}

func TestHandleKey_ErrSuppressesAllInput(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.err = errors.New("already failed")

	updated, cmd := m.handleKey(quitKey('q'))

	require.False(t, asModel(t, updated).quitting)
	require.Nil(t, cmd)
}

func TestHandleKey_CtrlCQuits(t *testing.T) {
	m := New(context.Background(), &fakeService{})

	updated, cmd := m.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))
}

func TestHandleKey_NoneDueAnyKeyQuits(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.noneDue = true

	updated, cmd := m.handleKey(quitKey('x'))

	require.True(t, asModel(t, updated).quitting)
	require.True(t, isQuitCmd(t, cmd))
}

func TestHandleKey_StillLoadingIgnoresKey(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	// card == nil, noneDue == false: still loading.

	updated, cmd := m.handleKey(quitKey('3'))

	got := asModel(t, updated)
	require.False(t, got.quitting)
	require.Nil(t, cmd)
}

func TestHandleKey_RevealsOnSpace(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.card = &review.Card{ID: 1}

	updated, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	require.True(t, asModel(t, updated).revealed)
	require.Nil(t, cmd)
}

func TestHandleKey_RevealsOnEnter(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.card = &review.Card{ID: 1}

	updated, _ := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.True(t, asModel(t, updated).revealed)
}

func TestHandleKey_UnrecognizedKeyBeforeRevealDoesNothing(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.card = &review.Card{ID: 1}

	updated, cmd := m.handleKey(quitKey('z'))

	require.False(t, asModel(t, updated).revealed)
	require.Nil(t, cmd)
}

func TestHandleKey_SubmittingGuardBlocksRepeatRating(t *testing.T) {
	svc := &fakeService{}
	m := New(context.Background(), svc)
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
	m := New(context.Background(), svc)
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
	m := New(context.Background(), &fakeService{})
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

func TestInit_ReturnsLoadNextCardCmd(t *testing.T) {
	svc := &fakeService{card: &review.Card{ID: 7}}
	m := New(context.Background(), svc)

	cmd := m.Init()
	require.NotNil(t, cmd)

	msg := cmd()
	cm, ok := msg.(cardMsg)
	require.True(t, ok)
	require.Equal(t, svc.card, cm.card)
}

func TestLoadNextCard_PropagatesError(t *testing.T) {
	svc := &fakeService{nextErr: errors.New("query failed")}
	m := New(context.Background(), svc)

	msg := m.loadNextCard()

	em, ok := msg.(errMsg)
	require.True(t, ok)
	require.EqualError(t, em.err, "query failed")
}
