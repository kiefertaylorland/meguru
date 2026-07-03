// Package scheduler implements the M1 naive interval-bump scheduler. This is
// an explicitly temporary placeholder (FR-008) — it MUST NOT be made more
// sophisticated than the four fixed intervals below; go-fsrs replaces it
// wholesale in M2.
package scheduler

import "time"

// Rating is a self-graded review outcome, worst to best.
type Rating int

const (
	Again Rating = 1
	Hard  Rating = 2
	Good  Rating = 3
	Easy  Rating = 4
)

// NextDue is the entire M1 scheduling contract: a pure function from a
// rating and the current time to the next due timestamp. It has no side
// effects and is not aware of FSRS-style stability/difficulty.
func NextDue(rating Rating, now time.Time) time.Time {
	switch rating {
	case Again:
		return now.Add(1 * time.Minute)
	case Hard:
		return now.Add(24 * time.Hour)
	case Good:
		return now.Add(3 * 24 * time.Hour)
	case Easy:
		return now.Add(7 * 24 * time.Hour)
	default:
		// Caller programming error (contracts/scheduler.md) — never panic;
		// fall back to the middle-ground interval.
		return now.Add(24 * time.Hour)
	}
}
