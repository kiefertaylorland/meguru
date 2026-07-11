package scheduler

import (
	"math"
	"reflect"
	"testing"
	"time"
)

// These two tests pin published upstream go-fsrs test vectors (from that
// module's own fsrs_test.go, TestBasicSchedulerExample and
// TestBasicSchedulerMemoState) through our own CardState/Outcome mapping.
// They exist to prove the mapping in fsrs.go introduces no drift from the
// reference implementation — the property tests in fsrs_test.go only check
// invariants, not exact values.

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

// advance simulates one real review: apply rating to current at now, and
// return the resulting CardState (with LastReviewAt set to this now) plus
// the Outcome for inspection.
func advance(current CardState, rating Rating, now time.Time) (CardState, Outcome) {
	out := Schedule(current, rating, now)
	last := now
	next := CardState{
		State:        out.NextState,
		Stability:    out.Stability,
		Difficulty:   out.Difficulty,
		Reps:         out.Reps,
		Lapses:       out.Lapses,
		LastReviewAt: &last,
	}
	return next, out
}

func TestSchedule_ReferenceVectorScheduledDaysAndStateHistory(t *testing.T) {
	now := time.Date(2022, 11, 29, 12, 30, 0, 0, time.UTC)
	ratings := []Rating{Good, Good, Good, Good, Good, Good, Again, Again, Good, Good, Good, Good, Good}
	wantScheduledDays := []float64{0, 4, 14, 44, 125, 328, 0, 0, 7, 16, 34, 71, 142}
	wantStateBeforeRating := []State{
		StateNew, StateLearning, StateReview, StateReview, StateReview, StateReview, StateReview,
		StateRelearning, StateRelearning, StateReview, StateReview, StateReview, StateReview,
	}

	current := CardState{State: StateNew}
	var gotScheduledDays []float64
	var gotStateBeforeRating []State

	for _, rating := range ratings {
		gotStateBeforeRating = append(gotStateBeforeRating, current.State)
		next, out := advance(current, rating, now)
		gotScheduledDays = append(gotScheduledDays, out.ScheduledDays)
		now = out.DueAt
		current = next
	}

	if !reflect.DeepEqual(gotScheduledDays, wantScheduledDays) {
		t.Errorf("scheduled days: want %v, got %v", wantScheduledDays, gotScheduledDays)
	}
	if !reflect.DeepEqual(gotStateBeforeRating, wantStateBeforeRating) {
		t.Errorf("state before each rating: want %v, got %v", wantStateBeforeRating, gotStateBeforeRating)
	}
}

func TestSchedule_ReferenceVectorMemoState(t *testing.T) {
	now := time.Date(2022, 11, 29, 12, 30, 0, 0, time.UTC)
	ratings := []Rating{Again, Good, Good, Good, Good, Good}
	gapDays := []float64{0, 0, 1, 3, 8, 21}

	current := CardState{State: StateNew}
	for i, rating := range ratings {
		next, _ := advance(current, rating, now)
		current = next
		now = now.Add(time.Duration(gapDays[i]) * 24 * time.Hour)
	}

	final := Schedule(current, Good, now)

	const wantStability = 48.4848
	const wantDifficulty = 7.0866
	if got := roundFloat(final.Stability, 4); got != wantStability {
		t.Errorf("stability: want %v, got %v", wantStability, got)
	}
	if got := roundFloat(final.Difficulty, 4); got != wantDifficulty {
		t.Errorf("difficulty: want %v, got %v", wantDifficulty, got)
	}
}
