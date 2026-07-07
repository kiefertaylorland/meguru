package scheduler

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// genCardState produces a CardState respecting the precondition documented
// in contracts/scheduler.md: LastReviewAt is nil if and only if State is
// StateNew.
func genCardState(t *rapid.T, now time.Time) CardState {
	state := State(rapid.IntRange(0, 3).Draw(t, "state"))
	cs := CardState{
		State:      state,
		Stability:  rapid.Float64Range(0, 1000).Draw(t, "stability"),
		Difficulty: rapid.Float64Range(0, 10).Draw(t, "difficulty"),
		Reps:       rapid.IntRange(0, 500).Draw(t, "reps"),
		Lapses:     rapid.IntRange(0, 500).Draw(t, "lapses"),
	}
	if state != StateNew {
		daysAgo := rapid.IntRange(1, 3650).Draw(t, "daysAgo")
		last := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)
		cs.LastReviewAt = &last
	}
	return cs
}

func TestSchedule_DueAtAlwaysAfterNow(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now().UTC()
		current := genCardState(t, now)
		rating := Rating(rapid.IntRange(1, 4).Draw(t, "rating"))

		out := Schedule(current, rating, now)

		if !out.DueAt.After(now) {
			t.Fatalf("DueAt %v is not after now %v (rating=%d, current=%+v)", out.DueAt, now, rating, current)
		}
	})
}

func TestSchedule_RatingOrderingIsNonDecreasing(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now().UTC()
		current := genCardState(t, now)

		again := Schedule(current, Again, now)
		hard := Schedule(current, Hard, now)
		good := Schedule(current, Good, now)
		easy := Schedule(current, Easy, now)

		if again.DueAt.After(hard.DueAt) {
			t.Fatalf("Again due (%v) after Hard due (%v)", again.DueAt, hard.DueAt)
		}
		if hard.DueAt.After(good.DueAt) {
			t.Fatalf("Hard due (%v) after Good due (%v)", hard.DueAt, good.DueAt)
		}
		if good.DueAt.After(easy.DueAt) {
			t.Fatalf("Good due (%v) after Easy due (%v)", good.DueAt, easy.DueAt)
		}
	})
}

func TestSchedule_StabilityAndDifficultyStayInBounds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now().UTC()
		current := genCardState(t, now)
		rating := Rating(rapid.IntRange(1, 4).Draw(t, "rating"))

		out := Schedule(current, rating, now)

		if out.Stability <= 0 {
			t.Fatalf("Stability %v is not positive", out.Stability)
		}
		if out.Difficulty < 1 || out.Difficulty > 10 {
			t.Fatalf("Difficulty %v out of documented [1,10] bounds", out.Difficulty)
		}
	})
}

func TestSchedule_StateTransitionsFollowValidGraph(t *testing.T) {
	valid := map[State]map[State]bool{
		StateNew:        {StateLearning: true, StateReview: true},
		StateLearning:   {StateLearning: true, StateReview: true},
		StateReview:     {StateReview: true, StateRelearning: true},
		StateRelearning: {StateRelearning: true, StateReview: true},
	}

	rapid.Check(t, func(t *rapid.T) {
		now := time.Now().UTC()
		current := genCardState(t, now)
		rating := Rating(rapid.IntRange(1, 4).Draw(t, "rating"))

		out := Schedule(current, rating, now)

		if !valid[current.State][out.NextState] {
			t.Fatalf("invalid transition %v -[%d]-> %v", current.State, rating, out.NextState)
		}
	})
}

func TestSchedule_IsDeterministic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now().UTC()
		current := genCardState(t, now)
		rating := Rating(rapid.IntRange(1, 4).Draw(t, "rating"))

		first := Schedule(current, rating, now)
		second := Schedule(current, rating, now)

		if first != second {
			t.Fatalf("Schedule is not deterministic: %+v != %+v", first, second)
		}
	})
}

func TestSchedule_LapseOnlyOnReviewStateAgain(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		now := time.Now().UTC()
		current := genCardState(t, now)

		out := Schedule(current, Again, now)

		if current.State == StateReview {
			if out.Lapses != current.Lapses+1 {
				t.Fatalf("expected lapses to increment from %d, got %d", current.Lapses, out.Lapses)
			}
		} else {
			if out.Lapses != current.Lapses {
				t.Fatalf("expected lapses to stay at %d for state %v, got %d", current.Lapses, current.State, out.Lapses)
			}
		}
	})
}
