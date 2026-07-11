package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"meguru/internal/stats"
	"meguru/internal/storage"
	"meguru/internal/textwidth"
)

func newStatsCommand() *cobra.Command {
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show due counts, streak, and retention",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStats(cmd, jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Emit machine-readable JSON instead of plain text")
	return cmd
}

// runStats mirrors runReview's startup sequence (contracts/stats-cli.md)
// minus deck seeding — stats only reads whatever data already exists.
func runStats(cmd *cobra.Command, jsonFlag bool) error {
	db, err := storage.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := storage.Migrate(db); err != nil {
		return err
	}

	svc := stats.NewService(db)
	summary, err := svc.Compute(cmd.Context(), time.Now())
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if jsonFlag {
		return writeStatsJSON(out, summary)
	}
	writeStatsPlain(out, summary)
	return nil
}

// statsJSON is the --json wire shape, per contracts/stats-cli.md. Field
// names and nullability (retention_percent/next_due_at) are part of the
// documented contract — do not rename without updating it.
type statsJSON struct {
	DueCards            int      `json:"due_cards"`
	TotalCards          int      `json:"total_cards"`
	StreakDays          int      `json:"streak_days"`
	RetentionPercent    *float64 `json:"retention_percent"`
	RetentionWindowDays int      `json:"retention_window_days"`
	NextDueAt           *string  `json:"next_due_at"`
}

func writeStatsJSON(out io.Writer, s stats.Summary) error {
	payload := statsJSON{
		DueCards:            s.DueCards,
		TotalCards:          s.TotalCards,
		StreakDays:          s.StreakDays,
		RetentionPercent:    s.RetentionPercent,
		RetentionWindowDays: s.RetentionWindowDays,
	}
	if s.NextDueAt != nil {
		v := s.NextDueAt.UTC().Format(time.RFC3339)
		payload.NextDueAt = &v
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// statsLabelWidth mirrors internal/plain/renderer.go's labelWidth
// convention: the column every field value aligns to.
const statsLabelWidth = 17

func writeStatsPlain(out io.Writer, s stats.Summary) {
	printStatsField(out, "Due now", fmt.Sprintf("%d", s.DueCards))
	printStatsField(out, "Total cards", fmt.Sprintf("%d", s.TotalCards))
	printStatsField(out, "Streak", fmt.Sprintf("%d day(s)", s.StreakDays))

	retentionLabel := fmt.Sprintf("Retention (%dd)", s.RetentionWindowDays)
	if s.RetentionPercent != nil {
		printStatsField(out, retentionLabel, fmt.Sprintf("%.0f%%", *s.RetentionPercent))
	} else {
		printStatsField(out, retentionLabel, "n/a (no reviews yet)")
	}

	if s.DueCards == 0 {
		if s.NextDueAt != nil {
			printStatsField(out, "Next due", s.NextDueAt.Local().Format("2006-01-02 15:04"))
		} else {
			printStatsField(out, "Next due", "no cards scheduled")
		}
	}
}

func printStatsField(out io.Writer, label, value string) {
	pad := strings.Repeat(" ", max(0, statsLabelWidth-textwidth.Width(label)))
	fmt.Fprintf(out, "%s:%s%s\n", label, pad, value)
}
