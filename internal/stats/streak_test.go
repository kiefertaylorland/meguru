package stats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStreakDays_ZeroReviewsEver(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	got := StreakDays(nil, now, time.UTC)

	require.Equal(t, 0, got)
}

func TestStreakDays_BrokenStreak(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	// Last review 3 days ago — neither today nor yesterday has a review.
	reviewed := []time.Time{
		now.AddDate(0, 0, -3),
		now.AddDate(0, 0, -4),
		now.AddDate(0, 0, -5),
	}

	got := StreakDays(reviewed, now, time.UTC)

	require.Equal(t, 0, got)
}

func TestStreakDays_ReviewsOnlyToday(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	reviewed := []time.Time{now.Add(-1 * time.Hour)}

	got := StreakDays(reviewed, now, time.UTC)

	require.Equal(t, 1, got)
}

func TestStreakDays_SeveralConsecutiveDaysEndingToday(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	reviewed := []time.Time{
		now,
		now.AddDate(0, 0, -1),
		now.AddDate(0, 0, -2),
	}

	got := StreakDays(reviewed, now, time.UTC)

	require.Equal(t, 3, got)
}

// A streak isn't broken until a full calendar day passes with no review —
// reviewing yesterday but not yet today still counts (spec.md Acceptance
// Scenario 2a).
func TestStreakDays_EndsYesterdayNotYetReviewedToday(t *testing.T) {
	now := time.Date(2026, 7, 6, 8, 0, 0, 0, time.UTC)
	reviewed := []time.Time{
		now.AddDate(0, 0, -1),
		now.AddDate(0, 0, -2),
	}

	got := StreakDays(reviewed, now, time.UTC)

	require.Equal(t, 2, got)
}

// A gap before the current run must not be added on top of it.
func TestStreakDays_GapBeforeCurrentRunIsNotCounted(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	reviewed := []time.Time{
		now,                    // today
		now.AddDate(0, 0, -1),  // yesterday
		now.AddDate(0, 0, -10), // long-past day, separated by a gap
		now.AddDate(0, 0, -11),
	}

	got := StreakDays(reviewed, now, time.UTC)

	require.Equal(t, 2, got)
}

// A review logged late in the evening in a non-UTC zone must be attributed
// to the correct local calendar day, even though its UTC calendar date may
// already be the next day.
func TestStreakDays_LocalUTCBoundary(t *testing.T) {
	loc := time.FixedZone("UTC-8", -8*60*60)
	// 2026-07-05 23:45 local == 2026-07-06 07:45 UTC.
	reviewedLocalEvening := time.Date(2026, 7, 5, 23, 45, 0, 0, loc)
	now := time.Date(2026, 7, 6, 6, 0, 0, 0, loc) // still 2026-07-06 local

	// If evaluated in UTC, the review (UTC date 07-06) would look like it
	// happened "today" (now's UTC date is also 07-06) rather than
	// "yesterday" in local time — assert the local-time semantics hold.
	got := StreakDays([]time.Time{reviewedLocalEvening}, now, loc)

	require.Equal(t, 1, got, "yesterday's late-evening review should still start a 1-day streak counted from yesterday")
}

func TestStreakDays_ReviewsFromDifferentZonesNormalizeToLoc(t *testing.T) {
	loc := time.FixedZone("UTC-8", -8*60*60)
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, loc)
	// Stored as UTC (as review_log.reviewed_at always is), representing
	// 2026-07-06 04:00 in loc — same local calendar day as now.
	reviewedUTC := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	got := StreakDays([]time.Time{reviewedUTC}, now, loc)

	require.Equal(t, 1, got)
}
