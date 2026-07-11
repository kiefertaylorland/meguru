package stats

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// retentionWindow is the trailing duration retention is computed over
// (specs/005-dashboard-stats/research.md "Retention window = 30 days").
const (
	retentionWindow     = 30 * 24 * time.Hour
	retentionWindowDays = 30
)

// Summary is everything `meguru stats` reports, computed fresh on every
// invocation — see specs/005-dashboard-stats/data-model.md.
type Summary struct {
	DueCards            int
	TotalCards          int
	StreakDays          int
	RetentionPercent    *float64
	RetentionWindowDays int
	NextDueAt           *time.Time
}

// Service is what internal/cli calls — it never talks to storage directly,
// mirroring internal/review.Service's convention.
type Service interface {
	// Compute returns a fresh Summary as of now.
	Compute(ctx context.Context, now time.Time) (Summary, error)
}

type service struct {
	db *sql.DB
}

// NewService builds a Service backed by db.
func NewService(db *sql.DB) Service {
	return &service{db: db}
}

func (s *service) Compute(ctx context.Context, now time.Time) (Summary, error) {
	totalCards, err := s.countCards(ctx)
	if err != nil {
		return Summary{}, err
	}

	dueCards, err := s.countDueCards(ctx, now)
	if err != nil {
		return Summary{}, err
	}

	nextDueAt, err := s.nextDueAt(ctx)
	if err != nil {
		return Summary{}, err
	}

	reviewedAt, err := s.reviewedAtTimestamps(ctx)
	if err != nil {
		return Summary{}, err
	}
	streak := StreakDays(reviewedAt, now, time.Local)

	ratings, err := s.ratingsSince(ctx, now.UTC().Add(-retentionWindow))
	if err != nil {
		return Summary{}, err
	}
	retentionPercent, ok := Retention(ratings)
	var retentionPtr *float64
	if ok {
		retentionPtr = &retentionPercent
	}

	return Summary{
		DueCards:            dueCards,
		TotalCards:          totalCards,
		StreakDays:          streak,
		RetentionPercent:    retentionPtr,
		RetentionWindowDays: retentionWindowDays,
		NextDueAt:           nextDueAt,
	}, nil
}

func (s *service) countCards(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cards`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count cards: %w", err)
	}
	return count, nil
}

func (s *service) countDueCards(ctx context.Context, now time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM srs_state WHERE due_at IS NOT NULL AND due_at <= ?`,
		now.UTC().Format(time.RFC3339)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count due cards: %w", err)
	}
	return count, nil
}

func (s *service) nextDueAt(ctx context.Context) (*time.Time, error) {
	var dueAt sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT MIN(due_at) FROM srs_state WHERE due_at IS NOT NULL`).
		Scan(&dueAt)
	if err != nil {
		return nil, fmt.Errorf("find next due: %w", err)
	}
	if !dueAt.Valid {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, dueAt.String)
	if err != nil {
		return nil, fmt.Errorf("parse next due_at %q: %w", dueAt.String, err)
	}
	return &t, nil
}

func (s *service) reviewedAtTimestamps(ctx context.Context) ([]time.Time, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT reviewed_at FROM review_log`)
	if err != nil {
		return nil, fmt.Errorf("query review_log: %w", err)
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan reviewed_at: %w", err)
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, fmt.Errorf("parse reviewed_at %q: %w", raw, err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review_log: %w", err)
	}
	return out, nil
}

func (s *service) ratingsSince(ctx context.Context, cutoff time.Time) ([]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT rating FROM review_log WHERE reviewed_at >= ?`, cutoff.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("query review_log ratings: %w", err)
	}
	defer rows.Close()

	var out []int
	for rows.Next() {
		var rating int
		if err := rows.Scan(&rating); err != nil {
			return nil, fmt.Errorf("scan rating: %w", err)
		}
		out = append(out, rating)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review_log ratings: %w", err)
	}
	return out, nil
}
