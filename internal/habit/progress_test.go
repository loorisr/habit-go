package habit

import (
	"testing"
	"time"

	"habit-go/internal/entry"
)

// Entries around base 2025-06-10 (today). Each period sees a different subset:
//
//	daily   (06-10)                  -> 2
//	weekly  (06-04, 06-10)           -> 5 + 2 = 7
//	monthly (05-20, 06-04, 06-10)    -> 4 + 5 + 2 = 11
//
// Boolean habits count entries with value >= 1, so all three entries count once each.
var progressEntries = []entry.Entry{
	{Date: "2025-05-20", Value: 4},
	{Date: "2025-06-04", Value: 5},
	{Date: "2025-06-10", Value: 2},
}

var expectedCurrent = map[string]map[string]float64{
	"numerical": {"daily": 2, "weekly": 7, "monthly": 11},
	"boolean":   {"daily": 1, "weekly": 2, "monthly": 3},
}

// The whole matrix: type x positive/negative x daily/weekly/monthly.
func TestProgressMatrix(t *testing.T) {
	base, _ := time.Parse("2006-01-02", "2025-06-10")
	for typ, byPeriod := range expectedCurrent {
		for period, cur := range byPeriod {
			for _, neg := range []bool{false, true} {
				name := typ + "/" + period + "/" + map[bool]string{false: "positive", true: "negative"}[neg]
				t.Run(name, func(t *testing.T) {
					if neg {
						// goal = current: "at most" is met. goal = current-1: exceeded.
						met := &Habit{Type: typ, GoalPeriod: period, IsNegative: true, GoalValue: cur}
						p := computeProgress(met, progressEntries, base)
						if p.Current != cur || !p.Success {
							t.Errorf("at limit: %+v, want current %v and success", p, cur)
						}
						over := &Habit{Type: typ, GoalPeriod: period, IsNegative: true, GoalValue: cur - 1}
						p = computeProgress(over, progressEntries, base)
						if p.Success {
							t.Errorf("over limit: %+v, want failure", p)
						}
						return
					}
					// goal = current: reached. goal = current+1: missed.
					reached := &Habit{Type: typ, GoalPeriod: period, GoalValue: cur}
					p := computeProgress(reached, progressEntries, base)
					if p.Current != cur || !p.Success || p.Percentage != 100 {
						t.Errorf("reached: %+v, want current %v, success, 100%%", p, cur)
					}
					missed := &Habit{Type: typ, GoalPeriod: period, GoalValue: cur + 1}
					p = computeProgress(missed, progressEntries, base)
					if p.Success {
						t.Errorf("missed: %+v, want failure", p)
					}
				})
			}
		}
	}
}

// Entries outside the window and in the future must never count.
func TestProgressIgnoresOutOfWindowEntries(t *testing.T) {
	base, _ := time.Parse("2006-01-02", "2025-06-10")
	entries := []entry.Entry{
		{Date: "2025-05-11", Value: 100}, // 31 days back: outside monthly
		{Date: "2025-06-11", Value: 100}, // tomorrow
	}
	for _, period := range []string{"daily", "weekly", "monthly"} {
		hab := &Habit{Type: "numerical", GoalValue: 1, GoalPeriod: period}
		if p := computeProgress(hab, entries, base); p.Current != 0 {
			t.Errorf("%s: current = %v, want 0", period, p.Current)
		}
	}
}
