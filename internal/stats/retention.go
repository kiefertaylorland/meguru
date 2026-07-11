package stats

import "meguru/internal/scheduler"

// Retention returns the percentage of ratings that were not Again, and ok
// reports whether there was any data to compute from — an empty ratings
// slice returns (0, false) rather than a misleading 0% (spec.md SC-003;
// data-model.md "Retention derivation").
func Retention(ratings []int) (percent float64, ok bool) {
	if len(ratings) == 0 {
		return 0, false
	}

	nonAgain := 0
	for _, r := range ratings {
		if scheduler.Rating(r) != scheduler.Again {
			nonAgain++
		}
	}
	return float64(nonAgain) / float64(len(ratings)) * 100, true
}
