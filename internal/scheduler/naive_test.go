package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var allRatings = []Rating{Again, Hard, Good, Easy}

// NextDue never returns a time <= now (contracts/scheduler.md invariant).
func TestNextDue_NeverReturnsPastOrPresent(t *testing.T) {
	now := time.Now()
	for _, r := range allRatings {
		due := NextDue(r, now)
		require.True(t, due.After(now), "rating %d: due %v must be after now %v", r, due, now)
	}
}

// NextDue is deterministic: same (rating, now) always yields the same result.
func TestNextDue_IsDeterministic(t *testing.T) {
	now := time.Now()
	for _, r := range allRatings {
		require.Equal(t, NextDue(r, now), NextDue(r, now))
	}
}

// NextDue must not panic for any of the four valid Rating values, and must
// implement the exact fixed intervals from contracts/scheduler.md.
func TestNextDue_FixedIntervals(t *testing.T) {
	now := time.Now()
	require.NotPanics(t, func() {
		require.Equal(t, now.Add(1*time.Minute), NextDue(Again, now))
		require.Equal(t, now.Add(24*time.Hour), NextDue(Hard, now))
		require.Equal(t, now.Add(3*24*time.Hour), NextDue(Good, now))
		require.Equal(t, now.Add(7*24*time.Hour), NextDue(Easy, now))
	})
}

// An invalid Rating is a caller programming error, not a crash.
func TestNextDue_InvalidRatingDoesNotPanic(t *testing.T) {
	now := time.Now()
	require.NotPanics(t, func() {
		due := NextDue(Rating(99), now)
		require.True(t, due.After(now))
	})
}
