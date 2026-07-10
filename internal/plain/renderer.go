// Package plain implements the linear, non-interactive review renderer used
// by --plain and whenever stdout is not a TTY (FR-010).
package plain

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"meguru/internal/review"
	"meguru/internal/scheduler"
	"meguru/internal/textwidth"
)

// Run drives one full review session using sequential fmt.Println output and
// a blocking line-based rating prompt — no interactive redraws, no
// color/style escape sequences.
func Run(ctx context.Context, svc review.Service, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)

	for {
		card, err := svc.NextDueCard(ctx)
		if err != nil {
			return err
		}
		if card == nil {
			fmt.Fprintln(out, "Nothing due right now.")
			return nil
		}

		printField(out, "Expression", card.Expression)
		cardReading := strings.TrimSpace(card.Reading)
		fmt.Fprintln(out, "Type the reading (romaji):")
		fmt.Fprint(out, "> ")

		if !scanner.Scan() {
			return scanner.Err()
		}
		answerInput := scanner.Text()
		if rating, ok := parseWordRatingShortcut(answerInput, cardReading); ok {
			if err := svc.Rate(ctx, card.ID, rating, time.Now()); err != nil {
				return err
			}
			fmt.Fprintf(out, "Recorded: %s\n\n", ratingName(rating))
			continue
		}

		result := review.CheckAnswer(card, answerInput)
		if result.Correct {
			fmt.Fprintf(out, "Correct! (%s)\n", result.Kana)
		} else {
			fmt.Fprintf(out, "Not quite — you typed: %s\n", result.Kana)
		}

		printField(out, "Reading", card.Reading)
		printField(out, "Meaning", card.Meaning)
		fmt.Fprintln(out, "Rate: (a)gain / (h)ard / (g)ood / (e)asy")
		fmt.Fprint(out, "> ")

		if !scanner.Scan() {
			return scanner.Err()
		}
		rating, ok := parseRating(scanner.Text())
		if !ok {
			fmt.Fprintln(out, "Unrecognized rating — answer again/hard/good/easy (or a/h/g/e).")
			continue
		}

		if err := svc.Rate(ctx, card.ID, rating, time.Now()); err != nil {
			return err
		}
		fmt.Fprintf(out, "Recorded: %s\n\n", ratingName(rating))
	}
}

// labelWidth is the column all field values align to, measured via
// textwidth so label alignment is correct even if a label contains
// wide characters.
const labelWidth = 10

func printField(out io.Writer, label, value string) {
	pad := strings.Repeat(" ", max(0, labelWidth-textwidth.Width(label)))
	fmt.Fprintf(out, "%s:%s%s\n", label, pad, value)
}

func parseRating(input string) (scheduler.Rating, bool) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "a", "again":
		return scheduler.Again, true
	case "h", "hard":
		return scheduler.Hard, true
	case "g", "good":
		return scheduler.Good, true
	case "e", "easy":
		return scheduler.Easy, true
	}
	return 0, false
}

func parseWordRating(input string) (scheduler.Rating, bool) {
	switch strings.ToLower(input) {
	case "again":
		return scheduler.Again, true
	case "hard":
		return scheduler.Hard, true
	case "good":
		return scheduler.Good, true
	case "easy":
		return scheduler.Easy, true
	}
	return 0, false
}

func parseWordRatingShortcut(answerInput, cardReading string) (scheduler.Rating, bool) {
	trimmedAnswer := strings.TrimSpace(answerInput)
	rating, ok := parseWordRating(trimmedAnswer)
	if !ok {
		return 0, false
	}
	if strings.EqualFold(trimmedAnswer, cardReading) {
		return 0, false
	}
	return rating, true
}

func ratingName(r scheduler.Rating) string {
	switch r {
	case scheduler.Again:
		return "Again"
	case scheduler.Hard:
		return "Hard"
	case scheduler.Good:
		return "Good"
	case scheduler.Easy:
		return "Easy"
	default:
		return "?"
	}
}
