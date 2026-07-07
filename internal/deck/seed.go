package deck

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Seed loads every builtin deck (BuiltinDecks) into storage on first run, or
// updates an individual deck's existing notes' fields in place — keyed by
// their stable "expression" natural key within that deck — when that deck's
// embedded content_version has increased. Existing cards/srs_state/
// review_log rows are never touched (FR-002, FR-003, FR-004; research.md
// §3). This seed/update-in-place logic is shared by every builtin deck:
// adding a new deck means adding one Definition in embed.go, not new seed
// code.
func Seed(ctx context.Context, db *sql.DB, now time.Time) error {
	for _, d := range BuiltinDecks() {
		content, err := d.Content()
		if err != nil {
			return err
		}
		if err := seedDeck(ctx, db, d, content, now); err != nil {
			return err
		}
	}
	return nil
}

func seedDeck(ctx context.Context, db *sql.DB, d Definition, content Content, now time.Time) error {
	var deckID int64
	var storedVersion int
	err := db.QueryRowContext(ctx, `SELECT id, content_version FROM decks WHERE slug = ?`, d.Slug).
		Scan(&deckID, &storedVersion)

	switch {
	case err == sql.ErrNoRows:
		return seedFresh(ctx, db, d, content, now)
	case err != nil:
		return fmt.Errorf("look up deck %s: %w", d.Slug, err)
	case content.ContentVersion > storedVersion:
		return updateInPlace(ctx, db, deckID, content, now)
	default:
		return nil
	}
}

func seedFresh(ctx context.Context, db *sql.DB, d Definition, content Content, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO decks (slug, name, kind, source, content_version) VALUES (?, ?, ?, 'builtin', ?)`,
		d.Slug, d.Name, d.Kind, content.ContentVersion)
	if err != nil {
		return fmt.Errorf("insert deck: %w", err)
	}
	deckID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	nowStr := now.UTC().Format(time.RFC3339)
	for _, note := range content.Notes {
		if err := insertNote(ctx, tx, deckID, note, nowStr); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func updateInPlace(ctx context.Context, db *sql.DB, deckID int64, content Content, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	nowStr := now.UTC().Format(time.RFC3339)
	for _, note := range content.Notes {
		fields, err := json.Marshal(note)
		if err != nil {
			return err
		}
		// FR-004 only requires updating existing notes' content in place;
		// this milestone's single fixed hiragana deck never introduces new
		// notes via a content-version bump, so an unmatched expression is
		// not handled here.
		res, err := tx.ExecContext(ctx,
			`UPDATE notes SET fields = ?, updated_at = ?
			 WHERE deck_id = ? AND json_extract(fields, '$.expression') = ?`,
			string(fields), nowStr, deckID, note.Expression)
		if err != nil {
			return fmt.Errorf("update note %s: %w", note.Expression, err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("update note %s: %w", note.Expression, err)
		}
		if affected != 1 {
			return fmt.Errorf("update note %s: expected to update 1 row, updated %d", note.Expression, affected)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE decks SET content_version = ? WHERE id = ?`,
		content.ContentVersion, deckID); err != nil {
		return fmt.Errorf("bump content_version: %w", err)
	}

	return tx.Commit()
}

func insertNote(ctx context.Context, tx *sql.Tx, deckID int64, note Note, nowStr string) error {
	fields, err := json.Marshal(note)
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO notes (deck_id, fields, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		deckID, string(fields), nowStr, nowStr)
	if err != nil {
		return fmt.Errorf("insert note %s: %w", note.Expression, err)
	}
	noteID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	res, err = tx.ExecContext(ctx, `INSERT INTO cards (note_id, direction) VALUES (?, 'recognition')`, noteID)
	if err != nil {
		return fmt.Errorf("insert card for note %s: %w", note.Expression, err)
	}
	cardID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO srs_state (card_id, state, due_at) VALUES (?, 'new', ?)`,
		cardID, nowStr); err != nil {
		return fmt.Errorf("insert srs_state for note %s: %w", note.Expression, err)
	}
	return nil
}
