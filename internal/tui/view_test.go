package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"meguru/internal/review"
)

func TestView_ErrorState(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.err = errors.New("kaboom")

	view := m.View()

	require.Contains(t, view.Content, "error: kaboom")
}

func TestView_Quitting(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.quitting = true

	view := m.View()

	require.Equal(t, "", view.Content)
}

func TestView_NoneDue(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.noneDue = true

	view := m.View()

	require.Contains(t, view.Content, "Nothing due right now.")
}

func TestView_Loading(t *testing.T) {
	m := New(context.Background(), &fakeService{})

	view := m.View()

	require.Contains(t, view.Content, "Loading...")
}

func TestView_CardFrontOnly(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.card = &review.Card{Expression: "あ", Reading: "a", Meaning: "a"}

	view := m.View()

	require.Contains(t, view.Content, "あ")
	require.Contains(t, view.Content, "press space/enter to reveal")
	require.NotContains(t, view.Content, "1=Again")
}

func TestView_CardRevealed(t *testing.T) {
	m := New(context.Background(), &fakeService{})
	m.card = &review.Card{Expression: "あ", Reading: "a", Meaning: "a"}
	m.revealed = true

	view := m.View()

	require.Contains(t, view.Content, "a — a")
	require.Contains(t, view.Content, "1=Again 2=Hard 3=Good 4=Easy")
}
