// Package stats computes the read-only figures reported by `meguru stats`
// (due count, total count, streak, retention) from the existing local
// database. It introduces no new schema and never writes to the database —
// see specs/005-dashboard-stats/data-model.md.
package stats

import "time"

// StreakDays computes the number of consecutive local calendar days, ending
// at today or yesterday (in loc), that have at least one entry in
// reviewedAt. Returns 0 if neither today nor yesterday has a review — the
// streak is broken (specs/005-dashboard-stats/data-model.md "Streak
// derivation").
func StreakDays(reviewedAt []time.Time, now time.Time, loc *time.Location) int {
	days := make(map[time.Time]bool, len(reviewedAt))
	for _, t := range reviewedAt {
		days[civilDay(t.In(loc))] = true
	}

	today := civilDay(now.In(loc))
	cursor := today
	if !days[cursor] {
		cursor = cursor.AddDate(0, 0, -1)
		if !days[cursor] {
			return 0
		}
	}

	count := 0
	for days[cursor] {
		count++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return count
}

// civilDay truncates t to midnight in its own location, giving a stable
// map key for "which calendar day is this timestamp on."
func civilDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
