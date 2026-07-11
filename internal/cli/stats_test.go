package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"meguru/internal/stats"
)

func TestNewStatsCommand_RegistersJSONFlag(t *testing.T) {
	cmd := newStatsCommand()

	flag := cmd.Flags().Lookup("json")
	require.NotNil(t, flag)
	require.Equal(t, "false", flag.DefValue)
}

func TestWriteStatsJSON_ValidAndMatchesContractFields(t *testing.T) {
	retention := 92.5
	nextDue := time.Date(2026, 7, 8, 9, 15, 0, 0, time.UTC)
	summary := stats.Summary{
		DueCards:            3,
		TotalCards:          46,
		StreakDays:          4,
		RetentionPercent:    &retention,
		RetentionWindowDays: 30,
		NextDueAt:           &nextDue,
	}
	var out bytes.Buffer

	require.NoError(t, writeStatsJSON(&out, summary))

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &decoded))
	require.Equal(t, float64(3), decoded["due_cards"])
	require.Equal(t, float64(46), decoded["total_cards"])
	require.Equal(t, float64(4), decoded["streak_days"])
	require.Equal(t, 92.5, decoded["retention_percent"])
	require.Equal(t, float64(30), decoded["retention_window_days"])
	require.Equal(t, "2026-07-08T09:15:00Z", decoded["next_due_at"])
}

func TestWriteStatsJSON_NullRetentionAndNextDueWhenAbsent(t *testing.T) {
	summary := stats.Summary{RetentionWindowDays: 30}
	var out bytes.Buffer

	require.NoError(t, writeStatsJSON(&out, summary))

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &decoded))
	require.Nil(t, decoded["retention_percent"])
	require.Nil(t, decoded["next_due_at"])
}

func TestWriteStatsPlain_ShowsCoreFields(t *testing.T) {
	retention := 100.0
	summary := stats.Summary{
		DueCards:            0,
		TotalCards:          10,
		StreakDays:          2,
		RetentionPercent:    &retention,
		RetentionWindowDays: 30,
	}
	var out bytes.Buffer

	writeStatsPlain(&out, summary)

	text := out.String()
	require.Contains(t, text, "Due now")
	require.Contains(t, text, "0")
	require.Contains(t, text, "Total cards")
	require.Contains(t, text, "10")
	require.Contains(t, text, "Streak")
	require.Contains(t, text, "2 day(s)")
	require.Contains(t, text, "Retention (30d)")
	require.Contains(t, text, "100%")
}

func TestWriteStatsPlain_NoReviewsShowsUnavailableRetention(t *testing.T) {
	summary := stats.Summary{RetentionWindowDays: 30}
	var out bytes.Buffer

	writeStatsPlain(&out, summary)

	require.Contains(t, out.String(), "n/a (no reviews yet)")
}

func TestWriteStatsPlain_NothingDueWithNoScheduledCardsShowsFallback(t *testing.T) {
	summary := stats.Summary{DueCards: 0, RetentionWindowDays: 30}
	var out bytes.Buffer

	writeStatsPlain(&out, summary)

	require.Contains(t, out.String(), "Next due")
	require.Contains(t, out.String(), "no cards scheduled")
}

func TestWriteStatsPlain_NothingDueWithScheduledCardShowsNextDueTime(t *testing.T) {
	next := time.Date(2026, 7, 8, 9, 15, 0, 0, time.UTC)
	summary := stats.Summary{DueCards: 0, RetentionWindowDays: 30, NextDueAt: &next}
	var out bytes.Buffer

	writeStatsPlain(&out, summary)

	require.Contains(t, out.String(), "Next due")
}

func TestWriteStatsPlain_DueNowOmitsNextDueLine(t *testing.T) {
	summary := stats.Summary{DueCards: 1, RetentionWindowDays: 30}
	var out bytes.Buffer

	writeStatsPlain(&out, summary)

	require.NotContains(t, out.String(), "Next due")
}
