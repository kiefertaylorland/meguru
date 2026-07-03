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

	var state string
	var lastReviewAt sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT state, last_review_at FROM srs_state WHERE card_id = ?`, cardID).
		Scan(&state, &lastReviewAt)
	if err != nil {
		return fmt.Errorf("read srs_state for card %d: %w", cardID, err)
	}

	var elapsedDays sql.NullFloat64
	if lastReviewAt.Valid {
		if prev, perr := time.Parse(time.RFC3339, lastReviewAt.String); perr == nil {
			elapsedDays = sql.NullFloat64{Float64: now.Sub(prev).Hours() / 24, Valid: true}
		}
	}

	due := scheduler.NextDue(rating, now)
	scheduledDays := due.Sub(now).Hours() / 24
	nowStr := now.UTC().Format(time.RFC3339)
	dueStr := due.UTC().Format(time.RFC3339)

	lapseIncrement := 0
	if rating == scheduler.Again {
		lapseIncrement = 1
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO review_log (card_id, rating, reviewed_at, state_before, elapsed_days, scheduled_days)
		VALUES (?, ?, ?, ?, ?, ?)`,
		cardID, int(rating), nowStr, state, elapsedDays, scheduledDays); err != nil {
		return fmt.Errorf("insert review_log: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE srs_state
		SET state = 'learning', due_at = ?, last_review_at = ?, reps = reps + 1, lapses = lapses + ?
		WHERE card_id = ?`,
		dueStr, nowStr, lapseIncrement, cardID); err != nil {
		return fmt.Errorf("update srs_state: %w", err)
	}

	return tx.Commit()
}
