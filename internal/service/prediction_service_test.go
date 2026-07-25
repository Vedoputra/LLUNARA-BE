package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// buildCycleChain builds len(gaps)+1 chronological cycles starting at
// firstStart, where gaps[i] is the cycle_length closing cycle i (i.e. the
// number of days until cycle i+1 starts). The final cycle is left open
// (CycleLength nil), as the latest recorded one always is.
func buildCycleChain(firstStart time.Time, gaps []int) []model.Cycle {
	cycles := make([]model.Cycle, len(gaps)+1)
	start := firstStart
	for i, gap := range gaps {
		length := gap
		cycles[i] = model.Cycle{
			ID:          uuid.New(),
			StartDate:   start,
			CycleLength: &length,
			IsOutlier:   length < 21 || length > 45,
		}
		start = start.AddDate(0, 0, gap)
	}
	cycles[len(gaps)] = model.Cycle{ID: uuid.New(), StartDate: start}
	return cycles
}

func withPeriodLengths(cycles []model.Cycle, lengths ...int) []model.Cycle {
	for i, l := range lengths {
		if i >= len(cycles) {
			break
		}
		length := l
		cycles[i].PeriodLength = &length
	}
	return cycles
}

// --- calculateAverageCycleLength ---

func TestCalculateAverageCycleLength_NoCycles_UsesDefault(t *testing.T) {
	if got := calculateAverageCycleLength(nil, 28); got != 28 {
		t.Errorf("got %d, want default 28", got)
	}
}

func TestCalculateAverageCycleLength_OneUsableCycle_UsesDefault(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{28})
	if got := calculateAverageCycleLength(cycles, 28); got != 28 {
		t.Errorf("got %d, want default 28 (only 1 usable cycle_length)", got)
	}
}

func TestCalculateAverageCycleLength_AveragesUpToSixMostRecent(t *testing.T) {
	// 8 gaps recorded; only the most recent 6 should count.
	cycles := buildCycleChain(date(2026, 1, 1), []int{20, 20, 30, 30, 30, 30, 30, 30})
	// Sorted desc by start date, the 6 most recent closed cycles all have
	// length 30, so the average should be exactly 30, ignoring the two
	// oldest 20-day (outlier) gaps entirely.
	if got := calculateAverageCycleLength(cycles, 28); got != 30 {
		t.Errorf("got %d, want 30 (capped at 6 most recent, oldest excess ignored)", got)
	}
}

func TestCalculateAverageCycleLength_ExcludesOutliers(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{28, 60, 28}) // 60 is an outlier (>45)
	got := calculateAverageCycleLength(cycles, 28)
	if got != 28 {
		t.Errorf("got %d, want 28 (60-day outlier excluded, average of the two 28s)", got)
	}
}

func TestCalculateAverageCycleLength_RoundsToNearestInt(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{27, 28, 28}) // avg = 27.666...
	if got := calculateAverageCycleLength(cycles, 28); got != 28 {
		t.Errorf("got %d, want 28 (rounded)", got)
	}
}

// --- calculateAveragePeriodLength ---

func TestCalculateAveragePeriodLength_NoData_UsesDefault(t *testing.T) {
	if got := calculateAveragePeriodLength(nil, 5); got != 5 {
		t.Errorf("got %d, want default 5", got)
	}
}

func TestCalculateAveragePeriodLength_Averages(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{28, 28, 28})
	cycles = withPeriodLengths(cycles, 4, 6, 5)
	if got := calculateAveragePeriodLength(cycles, 5); got != 5 {
		t.Errorf("got %d, want 5 (average of 4,6,5)", got)
	}
}

// --- predictNextPeriod / calculateOvulation / calculateFertileWindow ---

func TestPredictNextPeriod(t *testing.T) {
	last := model.Cycle{StartDate: date(2026, 7, 15)}
	start, end := predictNextPeriod(last, 29, 5)

	wantStart := date(2026, 8, 13)
	wantEnd := date(2026, 8, 17)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
}

func TestCalculateOvulation(t *testing.T) {
	nextStart := date(2026, 8, 13)
	got := calculateOvulation(nextStart, 29)
	want := date(2026, 7, 30)
	if !got.Equal(want) {
		t.Errorf("ovulation = %v, want %v (14 days before next period)", got, want)
	}
}

func TestCalculateFertileWindow(t *testing.T) {
	ovulation := date(2026, 7, 30)
	start, end := calculateFertileWindow(ovulation)
	wantStart := date(2026, 7, 25)
	wantEnd := date(2026, 7, 31)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Errorf("fertile window = [%v, %v], want [%v, %v]", start, end, wantStart, wantEnd)
	}
}

// --- determineCurrentPhase ---

func TestDetermineCurrentPhase(t *testing.T) {
	last := model.Cycle{StartDate: date(2026, 7, 1)} // avgCycleLength=29, avgPeriodLength=5
	// period: Jul 1-5, next start Jul 30, ovulation Jul 16, fertile [Jul 11, Jul 17]

	tests := []struct {
		name  string
		today time.Time
		want  model.Phase
	}{
		{"first day of period", date(2026, 7, 1), model.PhaseMenstrual},
		{"last day of period", date(2026, 7, 5), model.PhaseMenstrual},
		{"day after period, before fertile window", date(2026, 7, 6), model.PhaseFollicular},
		{"start of fertile window", date(2026, 7, 11), model.PhaseOvulation},
		{"end of fertile window", date(2026, 7, 17), model.PhaseOvulation},
		{"after fertile window, before next period", date(2026, 7, 20), model.PhaseLuteal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineCurrentPhase(tt.today, last, 29, 5)
			if got != tt.want {
				t.Errorf("phase = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineCurrentPhase_UsesRecordedEndDateOverEstimate(t *testing.T) {
	end := date(2026, 7, 8) // recorded end, longer than the 5-day estimate
	last := model.Cycle{StartDate: date(2026, 7, 1), EndDate: &end}

	// Day 7 would be "follicular" under the 5-day estimate, but the
	// recorded end_date says the period was still ongoing.
	got := determineCurrentPhase(date(2026, 7, 7), last, 29, 5)
	if got != model.PhaseMenstrual {
		t.Errorf("phase = %v, want menstrual (recorded end_date should take priority over the estimate)", got)
	}
}

// --- calculateConfidence ---

func TestCalculateConfidence_Low(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{28}) // only 1 usable
	if got := calculateConfidence(cycles); got != model.ConfidenceLow {
		t.Errorf("confidence = %v, want low", got)
	}
}

func TestCalculateConfidence_Medium(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{28, 28, 28}) // 3 usable
	if got := calculateConfidence(cycles); got != model.ConfidenceMedium {
		t.Errorf("confidence = %v, want medium", got)
	}
}

func TestCalculateConfidence_HighWithLowVariance(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{28, 29, 28, 27, 29, 28}) // 6 usable, low stddev
	if got := calculateConfidence(cycles); got != model.ConfidenceHigh {
		t.Errorf("confidence = %v, want high", got)
	}
}

func TestCalculateConfidence_SixCyclesHighVariance_StaysMedium(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{21, 45, 21, 45, 21, 45}) // 6 usable, high stddev
	if got := calculateConfidence(cycles); got != model.ConfidenceMedium {
		t.Errorf("confidence = %v, want medium (6+ cycles but too irregular for high)", got)
	}
}

// --- Predict(): the six required end-to-end scenarios ---

func TestPredict_NoHistoryAtAll(t *testing.T) {
	p := Predict(nil, date(2026, 7, 25), 28, 5)

	if p.Confidence != model.ConfidenceLow {
		t.Errorf("confidence = %v, want low", p.Confidence)
	}
	if p.BasedOnCycles != 0 {
		t.Errorf("based_on_cycles = %d, want 0", p.BasedOnCycles)
	}
	if p.AverageCycleLength != 28 {
		t.Errorf("average_cycle_length = %d, want default 28", p.AverageCycleLength)
	}
	if p.NextPeriodStart != nil || p.NextPeriodEnd != nil || p.EstimatedOvulation != nil ||
		p.FertileWindowStart != nil || p.CurrentPhase != nil || p.DayOfCycle != nil {
		t.Error("expected all prediction fields to be nil with zero history")
	}
}

func TestPredict_OneCycleRecorded(t *testing.T) {
	cycles := []model.Cycle{{ID: uuid.New(), StartDate: date(2026, 7, 1)}}
	p := Predict(cycles, date(2026, 7, 10), 28, 5)

	if p.Confidence != model.ConfidenceLow {
		t.Errorf("confidence = %v, want low", p.Confidence)
	}
	if p.AverageCycleLength != 28 {
		t.Errorf("average_cycle_length = %d, want default 28 (only 1 cycle, no cycle_length yet)", p.AverageCycleLength)
	}
	if p.NextPeriodStart == nil {
		t.Fatal("expected a prediction to still be produced from a single cycle")
	}
	wantNextStart := date(2026, 7, 29) // Jul 1 + 28 default days
	if !p.NextPeriodStart.Equal(wantNextStart) {
		t.Errorf("next_period_start = %v, want %v", p.NextPeriodStart, wantNextStart)
	}
	if p.DayOfCycle == nil || *p.DayOfCycle != 10 {
		t.Errorf("day_of_cycle = %v, want 10", p.DayOfCycle)
	}
}

func TestPredict_SixRegularCycles(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{28, 28, 28, 28, 28, 28})
	last := cycles[len(cycles)-1]

	p := Predict(cycles, last.StartDate, 28, 5)

	if p.Confidence != model.ConfidenceHigh {
		t.Errorf("confidence = %v, want high", p.Confidence)
	}
	if p.BasedOnCycles != 6 {
		t.Errorf("based_on_cycles = %d, want 6", p.BasedOnCycles)
	}
	if p.AverageCycleLength != 28 {
		t.Errorf("average_cycle_length = %d, want 28", p.AverageCycleLength)
	}
}

func TestPredict_SixCyclesHighVariance(t *testing.T) {
	cycles := buildCycleChain(date(2026, 1, 1), []int{21, 45, 22, 44, 23, 43})
	last := cycles[len(cycles)-1]

	p := Predict(cycles, last.StartDate, 28, 5)

	if p.Confidence == model.ConfidenceHigh {
		t.Error("expected confidence NOT to be high with a highly variable history")
	}
	if p.BasedOnCycles != 6 {
		t.Errorf("based_on_cycles = %d, want 6", p.BasedOnCycles)
	}
}

func TestPredict_HistoryWithOutlier(t *testing.T) {
	// One 60-day outlier mixed in with otherwise-regular 28-day cycles.
	cycles := buildCycleChain(date(2026, 1, 1), []int{28, 60, 28, 28})
	last := cycles[len(cycles)-1]

	p := Predict(cycles, last.StartDate, 28, 5)

	if p.AverageCycleLength != 28 {
		t.Errorf("average_cycle_length = %d, want 28 (outlier excluded)", p.AverageCycleLength)
	}
	if p.BasedOnCycles != 3 {
		t.Errorf("based_on_cycles = %d, want 3 (4 recorded, 1 excluded as outlier)", p.BasedOnCycles)
	}

	// The outlier cycle itself must still be present in the raw history,
	// just excluded from the average — Predict doesn't mutate its input.
	var stillPresent bool
	for _, c := range cycles {
		if c.IsOutlier {
			stillPresent = true
		}
	}
	if !stillPresent {
		t.Error("outlier cycle should remain in history, only excluded from the average")
	}
}

func TestPredict_CycleCrossingYearBoundary(t *testing.T) {
	// Last recorded cycle starts Dec 20, 2026; a 28-day average predicts
	// the next one landing in mid-January 2027.
	cycles := buildCycleChain(date(2026, 11, 22), []int{28})
	last := cycles[len(cycles)-1]
	if last.StartDate.Year() != 2026 || last.StartDate.Month() != time.December {
		t.Fatalf("test setup: expected latest cycle in December 2026, got %v", last.StartDate)
	}

	p := Predict(cycles, last.StartDate, 28, 5)

	wantNextStart := date(2027, 1, 17)
	if p.NextPeriodStart == nil || !p.NextPeriodStart.Equal(wantNextStart) {
		t.Errorf("next_period_start = %v, want %v (crossing into 2027)", p.NextPeriodStart, wantNextStart)
	}

	wantOvulation := date(2027, 1, 3)
	if p.EstimatedOvulation == nil || !p.EstimatedOvulation.Equal(wantOvulation) {
		t.Errorf("estimated_ovulation = %v, want %v", p.EstimatedOvulation, wantOvulation)
	}
}
