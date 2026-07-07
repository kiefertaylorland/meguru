// Package scheduler implements Meguru's spaced-repetition scheduling using
// FSRS (Free Spaced Repetition Scheduler) via
// github.com/open-spaced-repetition/go-fsrs/v3. It replaces the M1 naive,
// fixed-interval placeholder wholesale (per that package's own header
// comment) and is the sole scheduling contract internal/review depends on.
package scheduler

import (
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

// go-fsrs v3.3.1 API, confirmed via `go doc` and reading the vendored source
// before writing the mapping below (specs/002-fsrs-scheduler/research.md's
// verification step):
//
//   - Rating int8: Again=1, Hard=2, Good=3, Easy=4 — matches this package's
//     own Rating below exactly, so a direct numeric cast is safe.
//   - State int8: New=0, Learning=1, Review=2, Relearning=3 — matches this
//     package's own State below exactly.
//   - Card{Due, Stability, Difficulty, ElapsedDays uint64, ScheduledDays
//     uint64, Reps uint64, Lapses uint64, State, LastReview}. ElapsedDays,
//     ScheduledDays, Reps, and LastReview are all (re)computed internally
//     from the input Card + now; callers only need to supply State,
//     Stability, Difficulty, Reps, Lapses, LastReview as they stood before
//     this review.
//   - FSRS.Next(card Card, now time.Time, grade Rating) SchedulingInfo{Card,
//     ReviewLog} computes the single outcome for one grade (as opposed to
//     Repeat, which previews all four).
//   - A card only accrues a lapse (Lapses++) when an Again rating drops it
//     out of the Review state; New/Learning/Relearning Again ratings do not
//     touch Lapses (confirmed by reading scheduler_basic.go) — this is
//     exactly the FR-007 semantics this feature requires.

// Rating is a self-graded review outcome, worst to best.
type Rating int

const (
	Again Rating = 1
	Hard  Rating = 2
	Good  Rating = 3
	Easy  Rating = 4
)

// State mirrors srs_state.state's CHECK constraint values.
type State int

const (
	StateNew State = iota
	StateLearning
	StateReview
	StateRelearning
)

// CardState is a card's FSRS-relevant memory state, read out of srs_state
// before scheduling.
type CardState struct {
	State        State
	Stability    float64
	Difficulty   float64
	Reps         int
	Lapses       int
	LastReviewAt *time.Time // nil means "never reviewed"
}

// Outcome is everything review.Service needs to persist after scheduling.
type Outcome struct {
	NextState     State
	Stability     float64
	Difficulty    float64
	Reps          int
	Lapses        int
	DueAt         time.Time
	ElapsedDays   float64
	ScheduledDays float64
}

// defaultFSRS uses the library's default parameters. Per-user parameter
// optimization is post-MVP (docs/product/TECH_STACK.md §4), not a schema
// migration, so this stays a package-level constant until that lands.
//
// TODO(M2.x): expose Parameters once per-user optimization lands.
var defaultFSRS = fsrs.NewFSRS(fsrs.DefaultParam())

// Schedule is the entire scheduling contract: a pure function from a card's
// current FSRS state, a rating, and "now" to a full FSRS outcome. It has no
// side effects and never reaches storage — internal/review is the sole
// caller and sole writer of the persisted result.
func Schedule(current CardState, rating Rating, now time.Time) Outcome {
	info := defaultFSRS.Next(toFSRSCard(current), now, fsrs.Rating(rating))
	return fromSchedulingInfo(info)
}

func toFSRSCard(s CardState) fsrs.Card {
	card := fsrs.NewCard()
	card.State = fsrs.State(s.State)
	card.Stability = s.Stability
	card.Difficulty = s.Difficulty
	if s.State != StateNew {
		if card.Stability < 0.1 {
			card.Stability = 0.1
		}
		if card.Difficulty < 1 {
			card.Difficulty = 1
		}
	}
	card.Reps = uint64(s.Reps)
	card.Lapses = uint64(s.Lapses)
	if s.LastReviewAt != nil {
		card.LastReview = *s.LastReviewAt
	}
	return card
}

func fromSchedulingInfo(info fsrs.SchedulingInfo) Outcome {
	c := info.Card
	return Outcome{
		NextState:     State(c.State),
		Stability:     c.Stability,
		Difficulty:    c.Difficulty,
		Reps:          int(c.Reps),
		Lapses:        int(c.Lapses),
		DueAt:         c.Due,
		ElapsedDays:   float64(c.ElapsedDays),
		ScheduledDays: float64(c.ScheduledDays),
	}
}
