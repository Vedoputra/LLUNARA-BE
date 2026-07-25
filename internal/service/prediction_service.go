package service

import (
	"math"
	"sort"
	"time"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

const (
	lutealPhaseDays  = 14
	fertileLeadDays  = 5
	fertileTrailDays = 1
	maxCyclesForAvg  = 6

	confidenceLowMaxCycles    = 2 // fewer than 3 cycles
	confidenceMediumMaxCycles = 5 // 3 to 5 cycles
	confidenceHighMaxStdDev   = 5.0
)

// Predict runs the full prediction algorithm against a user's cycle
// history. It is a pure function — no database access — so every rule can
// be tested directly against hand-built cycle slices.
func Predict(cycles []model.Cycle, today time.Time, defaultCycleLength, defaultPeriodLength int) model.Prediction {
	confidence := calculateConfidence(cycles)
	avgCycleLength := calculateAverageCycleLength(cycles, defaultCycleLength)
	usedCycles := usableCyclesForCycleLength(cycles)

	prediction := model.Prediction{
		Confidence:         confidence,
		BasedOnCycles:      len(usedCycles),
		AverageCycleLength: avgCycleLength,
	}

	if len(cycles) == 0 {
		return prediction
	}

	avgPeriodLength := calculateAveragePeriodLength(cycles, defaultPeriodLength)
	last := latestCycle(cycles)

	nextStart, nextEnd := predictNextPeriod(last, avgCycleLength, avgPeriodLength)
	ovulation := calculateOvulation(nextStart, avgCycleLength)
	fertileStart, fertileEnd := calculateFertileWindow(ovulation)
	phase := determineCurrentPhase(today, last, avgCycleLength, avgPeriodLength)
	dayOfCycle := daysBetween(last.StartDate, today) + 1

	prediction.NextPeriodStart = &nextStart
	prediction.NextPeriodEnd = &nextEnd
	prediction.EstimatedOvulation = &ovulation
	prediction.FertileWindowStart = &fertileStart
	prediction.FertileWindowEnd = &fertileEnd
	prediction.CurrentPhase = &phase
	prediction.DayOfCycle = &dayOfCycle

	return prediction
}

// usableCyclesForCycleLength returns up to the 6 most recent cycles that
// have a known cycle_length (i.e. a cycle started after them, closing
// them) and aren't flagged as outliers.
func usableCyclesForCycleLength(cycles []model.Cycle) []model.Cycle {
	sorted := sortedByStartDateDesc(cycles)

	usable := make([]model.Cycle, 0, maxCyclesForAvg)
	for _, c := range sorted {
		if c.CycleLength == nil || c.IsOutlier {
			continue
		}
		usable = append(usable, c)
		if len(usable) == maxCyclesForAvg {
			break
		}
	}
	return usable
}

// usableCyclesForPeriodLength returns up to the 6 most recent cycles with a
// known period_length. Unlike cycle length, period length has no outlier
// exclusion rule in the spec.
func usableCyclesForPeriodLength(cycles []model.Cycle) []model.Cycle {
	sorted := sortedByStartDateDesc(cycles)

	usable := make([]model.Cycle, 0, maxCyclesForAvg)
	for _, c := range sorted {
		if c.PeriodLength == nil {
			continue
		}
		usable = append(usable, c)
		if len(usable) == maxCyclesForAvg {
			break
		}
	}
	return usable
}

func sortedByStartDateDesc(cycles []model.Cycle) []model.Cycle {
	sorted := make([]model.Cycle, len(cycles))
	copy(sorted, cycles)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartDate.After(sorted[j].StartDate) })
	return sorted
}

func latestCycle(cycles []model.Cycle) model.Cycle {
	latest := cycles[0]
	for _, c := range cycles[1:] {
		if c.StartDate.After(latest.StartDate) {
			latest = c
		}
	}
	return latest
}

// calculateAverageCycleLength averages up to the 6 most recent non-outlier
// cycle lengths, falling back to defaultCycleLength when fewer than 2 are
// available.
func calculateAverageCycleLength(cycles []model.Cycle, defaultCycleLength int) int {
	usable := usableCyclesForCycleLength(cycles)
	if len(usable) < 2 {
		return defaultCycleLength
	}
	sum := 0
	for _, c := range usable {
		sum += *c.CycleLength
	}
	return roundDiv(sum, len(usable))
}

// calculateAveragePeriodLength averages up to the 6 most recent period
// lengths, falling back to defaultPeriodLength when fewer than 2 are
// available.
func calculateAveragePeriodLength(cycles []model.Cycle, defaultPeriodLength int) int {
	usable := usableCyclesForPeriodLength(cycles)
	if len(usable) < 2 {
		return defaultPeriodLength
	}
	sum := 0
	for _, c := range usable {
		sum += *c.PeriodLength
	}
	return roundDiv(sum, len(usable))
}

func roundDiv(sum, count int) int {
	if count == 0 {
		return 0
	}
	return int(math.Round(float64(sum) / float64(count)))
}

// predictNextPeriod estimates the next period's [start, end] from the most
// recent cycle's start date.
func predictNextPeriod(lastCycle model.Cycle, avgCycleLength, avgPeriodLength int) (start, end time.Time) {
	start = lastCycle.StartDate.AddDate(0, 0, avgCycleLength)
	end = start.AddDate(0, 0, avgPeriodLength-1)
	return start, end
}

// calculateOvulation estimates ovulation as 14 days before the next
// predicted period start — the luteal phase is treated as a constant.
func calculateOvulation(nextStart time.Time, _ int) time.Time {
	return nextStart.AddDate(0, 0, -lutealPhaseDays)
}

// calculateFertileWindow spans 5 days before ovulation to 1 day after.
func calculateFertileWindow(ovulation time.Time) (start, end time.Time) {
	start = ovulation.AddDate(0, 0, -fertileLeadDays)
	end = ovulation.AddDate(0, 0, fertileTrailDays)
	return start, end
}

// determineCurrentPhase classifies today relative to the most recent
// cycle: menstrual while the period is ongoing (its recorded end_date, or
// an estimate from avgPeriodLength if it hasn't ended yet), ovulation
// during the fertile window, follicular between the two, luteal after.
func determineCurrentPhase(today time.Time, lastCycle model.Cycle, avgCycleLength, avgPeriodLength int) model.Phase {
	periodEnd := lastCycle.StartDate.AddDate(0, 0, avgPeriodLength-1)
	if lastCycle.EndDate != nil {
		periodEnd = *lastCycle.EndDate
	}
	if !today.Before(lastCycle.StartDate) && !today.After(periodEnd) {
		return model.PhaseMenstrual
	}

	nextStart := lastCycle.StartDate.AddDate(0, 0, avgCycleLength)
	ovulation := calculateOvulation(nextStart, avgCycleLength)
	fertileStart, fertileEnd := calculateFertileWindow(ovulation)

	if !today.Before(fertileStart) && !today.After(fertileEnd) {
		return model.PhaseOvulation
	}
	if today.Before(fertileStart) {
		return model.PhaseFollicular
	}
	return model.PhaseLuteal
}

// calculateConfidence reflects how much (and how consistent) history backs
// the prediction: low under 3 usable cycles, medium for 3-5, high for 6+
// with a cycle-length standard deviation of at most 5 days (otherwise it
// stays medium — six irregular cycles is still more than three, just not
// enough to call it "high confidence").
func calculateConfidence(cycles []model.Cycle) model.Confidence {
	usable := usableCyclesForCycleLength(cycles)
	switch {
	case len(usable) <= confidenceLowMaxCycles:
		return model.ConfidenceLow
	case len(usable) <= confidenceMediumMaxCycles:
		return model.ConfidenceMedium
	default:
		if cycleLengthStdDev(usable) <= confidenceHighMaxStdDev {
			return model.ConfidenceHigh
		}
		return model.ConfidenceMedium
	}
}

func cycleLengthStdDev(cycles []model.Cycle) float64 {
	n := len(cycles)
	if n == 0 {
		return 0
	}

	mean := 0.0
	for _, c := range cycles {
		mean += float64(*c.CycleLength)
	}
	mean /= float64(n)

	variance := 0.0
	for _, c := range cycles {
		diff := float64(*c.CycleLength) - mean
		variance += diff * diff
	}
	variance /= float64(n)

	return math.Sqrt(variance)
}
