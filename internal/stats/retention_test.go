package stats

import (
	"testing"

	"github.com/stretchr/testify/require"

	"meguru/internal/scheduler"
)

func TestRetention_EmptyInputIsUnavailable(t *testing.T) {
	percent, ok := Retention(nil)

	require.False(t, ok)
	require.Zero(t, percent)
}

func TestRetention_AllAgainIsZeroPercent(t *testing.T) {
	ratings := []int{int(scheduler.Again), int(scheduler.Again), int(scheduler.Again)}

	percent, ok := Retention(ratings)

	require.True(t, ok)
	require.Equal(t, 0.0, percent)
}

func TestRetention_AllNonAgainIsHundredPercent(t *testing.T) {
	ratings := []int{int(scheduler.Hard), int(scheduler.Good), int(scheduler.Easy)}

	percent, ok := Retention(ratings)

	require.True(t, ok)
	require.Equal(t, 100.0, percent)
}

func TestRetention_MixedRatingsComputesExpectedPercentage(t *testing.T) {
	// 1 Again out of 4 -> 75% retained.
	ratings := []int{int(scheduler.Again), int(scheduler.Hard), int(scheduler.Good), int(scheduler.Easy)}

	percent, ok := Retention(ratings)

	require.True(t, ok)
	require.Equal(t, 75.0, percent)
}
