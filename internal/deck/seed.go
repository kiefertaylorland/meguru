package deck

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Seed loads every builtin deck (BuiltinDecks) into storage on first run, or
// updates an individual deck in place when that deck's embedded
// content_version has increased: existing notes are updated by their stable
// "expression" natural key within that deck, and newly added expressions are
// inserted as new notes/cards/srs_state rows. Existing cards/srs_state/
// review_log rows for already-existing expressions are never touched
// (FR-002, FR-003, FR-004; research.md §3). This seed/update-in-place logic
// is shared by every builtin deck: adding a new deck means adding one
// Definition in embed.go, not new seed code.
func Seed(ctx context.Context, db *sql.DB, now time.Time) error {
	type parsedDeck struct {
		def     Definition
		content Content
	}
	var parsed []parsedDeck
	for _, d := range BuiltinDecks() {
		content, err := d.Content()
		if err != nil {
			return err
		}
		parsed = append(parsed, parsedDeck{def: d, content: content})
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, p := range parsed {
		if err := seedDeckTx(ctx, tx, p.def, p.content, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func seedDeck(ctx context.Context, db *sql.DB, d Definition, content Content, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := seedDeckTx(ctx, tx, d, content, now); err != nil {
		return err
	}
	return tx.Commit()
}

func seedDeckTx(ctx context.Context, tx *sql.Tx, d Definition, content Content, now time.Time) error {
	var deckID int64
	var storedVersion int
	err := tx.QueryRowContext(ctx, `SELECT id, content_version FROM decks WHERE slug = ?`, d.Slug).
		Scan(&deckID, &storedVersion)

	switch {
	case err == sql.ErrNoRows:
		return seedFresh(ctx, tx, d, content, now)
	case err != nil:
		return fmt.Errorf("look up deck %s: %w", d.Slug, err)
	case content.ContentVersion > storedVersion:
		return updateInPlace(ctx, tx, deckID, content, now)
	default:
		return nil
	}
}

func seedFresh(ctx context.Context, tx *sql.Tx, d Definition, content Content, now time.Time) error {
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

	return nil
}

func updateInPlace(ctx context.Context, tx *sql.Tx, deckID int64, content Content, now time.Time) error {
	nowStr := now.UTC().Format(time.RFC3339)
	for _, note := range content.Notes {
		fields, err := json.Marshal(note)
		if err != nil {
			return err
		}
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
		if affected == 0 {
			if err := insertNote(ctx, tx, deckID, note, nowStr); err != nil {
				return err
			}
		} else if affected != 1 {
			return fmt.Errorf("update note %s: expected to update 1 row, updated %d", note.Expression, affected)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE decks SET content_version = ? WHERE id = ?`,
		content.ContentVersion, deckID); err != nil {
		return fmt.Errorf("bump content_version: %w", err)
	}

	return nil
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
