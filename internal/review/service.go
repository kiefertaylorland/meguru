// Package review orchestrates one review action against storage: finding the
// next due card, or recording a rating and rescheduling it. It is the sole
// caller of scheduler.NextDue and the sole writer of review_log/srs_state.
package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"meguru/internal/scheduler"
)

// Card is one due study item as presented to a UI layer.
type Card struct {
	ID         int64
	Expression string
	Reading    string
	Meaning    string
}

// Service is what both the TUI and plain renderer call — neither UI layer
// talks to storage or the scheduler directly (contracts/cli.md).
type Service interface {
	// NextDueCard returns the next due card, or (nil, nil) if nothing is due.
	NextDueCard(ctx context.Context) (*Card, error)
	// Rate records a rating for cardID and reschedules it, atomically
	// (FR-007, FR-008, FR-015).
	Rate(ctx context.Context, cardID int64, rating scheduler.Rating, now time.Time) error
}

type service struct {
	db *sql.DB
}

// NewService builds a Service backed by db.
func NewService(db *sql.DB) Service {
	return &service{db: db}
}

func (s *service) NextDueCard(ctx context.Context) (*Card, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT c.id, n.fields
		FROM cards c
		JOIN notes n ON n.id = c.note_id
		JOIN srs_state st ON st.card_id = c.id
		WHERE st.due_at IS NOT NULL AND st.due_at <= ?
		ORDER BY st.due_at ASC
		LIMIT 1`, time.Now().UTC().Format(time.RFC3339))

	var cardID int64
	var fieldsJSON string
	err := row.Scan(&cardID, &fieldsJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query next due card: %w", err)
	}

	var fields struct {
		Expression string `json:"expression"`
		Reading    string `json:"reading"`
		Meaning    string `json:"meaning"`
	}
	if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
		return nil, fmt.Errorf("parse card fields: %w", err)
	}

	return &Card{
		ID:         cardID,
		Expression: fields.Expression,
		Reading:    fields.Reading,
		Meaning:    fields.Meaning,
	}, nil
}

func (s *service) Rate(ctx context.Context, cardID int64, rating scheduler.Rating, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var stateStr string
	var stability, difficulty float64
	var reps, lapses int
	var lastReviewAt sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT state, stability, difficulty, reps, lapses, last_review_at
		FROM srs_state WHERE card_id = ?`, cardID).
		Scan(&stateStr, &stability, &difficulty, &reps, &lapses, &lastReviewAt)
	if err != nil {
		return fmt.Errorf("read srs_state for card %d: %w", cardID, err)
	}

	state, err := stateFromString(stateStr)
	if err != nil {
		return fmt.Errorf("read srs_state for card %d: %w", cardID, err)
	}

	current := scheduler.CardState{
		State:      state,
		Stability:  stability,
		Difficulty: difficulty,
		Reps:       reps,
		Lapses:     lapses,
	}
	if lastReviewAt.Valid {
		prev, perr := time.Parse(time.RFC3339, lastReviewAt.String)
		if perr != nil {
			return fmt.Errorf("parse last_review_at for card %d: %w", cardID, perr)
		}
		current.LastReviewAt = &prev
	} else if state != scheduler.StateNew {
		return fmt.Errorf("read srs_state for card %d: %s card missing last_review_at", cardID, stateStr)
	}

	outcome := scheduler.Schedule(current, rating, now)

	nowStr := now.UTC().Format(time.RFC3339)
	dueStr := outcome.DueAt.UTC().Format(time.RFC3339)

	var elapsedDays sql.NullFloat64
	if current.LastReviewAt != nil {
		elapsedDays = sql.NullFloat64{Float64: outcome.ElapsedDays, Valid: true}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO review_log (card_id, rating, reviewed_at, state_before, stability_before, difficulty_before, elapsed_days, scheduled_days)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		cardID, int(rating), nowStr, stateStr, stability, difficulty, elapsedDays, outcome.ScheduledDays); err != nil {
		return fmt.Errorf("insert review_log: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE srs_state
		SET state = ?, stability = ?, difficulty = ?, due_at = ?, last_review_at = ?, reps = ?, lapses = ?
		WHERE card_id = ?`,
		stateToString(outcome.NextState), outcome.Stability, outcome.Difficulty, dueStr, nowStr,
		outcome.Reps, outcome.Lapses, cardID); err != nil {
		return fmt.Errorf("update srs_state: %w", err)
	}

	return tx.Commit()
}

// stateFromString maps srs_state.state's CHECK-constrained values to
// scheduler.State.
func stateFromString(s string) (scheduler.State, error) {
	switch s {
	case "new":
		return scheduler.StateNew, nil
	case "learning":
		return scheduler.StateLearning, nil
	case "review":
		return scheduler.StateReview, nil
	case "relearning":
		return scheduler.StateRelearning, nil
	default:
		return 0, fmt.Errorf("unknown srs_state.state %q", s)
	}
}

func stateToString(s scheduler.State) string {
	switch s {
	case scheduler.StateNew:
		return "new"
	case scheduler.StateLearning:
		return "learning"
	case scheduler.StateReview:
		return "review"
	case scheduler.StateRelearning:
		return "relearning"
	default:
		panic(fmt.Sprintf("unknown scheduler.State %d", s))
	}
}
