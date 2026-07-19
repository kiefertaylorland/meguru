package tui

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	_ "modernc.org/sqlite"

	"meguru/internal/deck"
	"meguru/internal/review"
	"meguru/internal/stats"
	"meguru/internal/storage"
)

func newTestModel(t *testing.T) Model {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "meguru.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := storage.Migrate(db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	if err := deck.Seed(context.Background(), db, time.Now()); err != nil {
		t.Fatalf("seed test db: %v", err)
	}

	return New(context.Background(), review.NewService(db), stats.NewService(db))
}

// Golden-frame coverage for the interactive session: start menu, navigating
// into the review screen, card render, reveal, and a rating keypress
// (plan.md Testing strategy). 100x30 is at or above the 80x24 minimum
// supported terminal size (view.go minWidth/minHeight) — unlike the
// pre-full-window-layout version of this test, which used a compact 60x12
// purely to keep test output small; that size would now trigger the
// "terminal too small" message instead of exercising the real screens.
func TestReviewScreen_RendersCardRevealAndAcceptsRating(t *testing.T) {
	tm := teatest.NewTestModel(t, newTestModel(t), teatest.WithInitialTermSize(100, 30))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Start Review"))
	})

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("press space/enter to reveal"))
	})

	tm.Send(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("1=Again 2=Hard 3=Good 4=Easy"))
	})

	tm.Send(tea.KeyPressMsg{Code: '3', Text: "3"})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
