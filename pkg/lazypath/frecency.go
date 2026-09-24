package lazypath

import "time"

// Frecency weights, in the spirit of the ones z and zoxide settled on: how
// often you go somewhere matters, but how recently matters more, and a project
// you have not opened in a month should not outrank the one you were in an
// hour ago just because you once lived in it.
const (
	frecencyWithinHour = 4.0
	frecencyWithinDay  = 2.0
	frecencyWithinWeek = 0.5
	frecencyOlder      = 0.25
)

// Score rates a visit for ranking: the number of visits, weighted by how long
// ago the last one was.
//
// A project never visited scores zero, which puts it behind everything that
// has been, and leaves the rest of the ordering to whatever the caller uses to
// break ties.
func (v Visit) Score(now time.Time) float64 {
	if v.Count <= 0 || v.At.IsZero() {
		return 0
	}

	elapsed := now.Sub(v.At)
	count := float64(v.Count)

	switch {
	case elapsed < time.Hour:
		return count * frecencyWithinHour
	case elapsed < 24*time.Hour:
		return count * frecencyWithinDay
	case elapsed < 7*24*time.Hour:
		return count * frecencyWithinWeek
	default:
		return count * frecencyOlder
	}
}

// Scores returns the frecency of every project in the history, by short name.
//
// It is read once and handed around, because ranking a list of candidates
// should not read the history once per candidate.
func Scores() map[string]float64 {
	now := time.Now()

	visits := ReadHistory()
	scores := make(map[string]float64, len(visits))
	for _, visit := range visits {
		scores[visit.Name] = visit.Score(now)
	}
	return scores
}
