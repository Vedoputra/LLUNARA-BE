package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

const (
	defaultInsightMonths = 6

	regularityMaxStdDev   = 3.0 // <= this many days: regular
	irregularityMinStdDev = 7.0 // > this many days: irregular; between the two: moderate
	minCyclesForSummary   = 2
)

// insightRepository is the subset of *repository.InsightRepository that
// InsightService depends on.
type insightRepository interface {
	ListSymptomOccurrences(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]repository.SymptomOccurrence, error)
	CountLoggedDays(ctx context.Context, userID uuid.UUID, from, to time.Time) (int, error)
	ListMoodOccurrences(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]repository.MoodOccurrence, error)
}

type InsightService struct {
	cycleRepo   cycleRepository
	insightRepo insightRepository
}

func NewInsightService(cycleRepo cycleRepository, insightRepo insightRepository) *InsightService {
	return &InsightService{cycleRepo: cycleRepo, insightRepo: insightRepo}
}

// averageLengths returns the user's rolling average cycle/period length
// (same pure functions BE-4.1 uses for prediction), falling back to the
// schema defaults if the repository call fails or there's no history —
// insights should degrade gracefully, never fail outright, when history
// is thin.
func (s *InsightService) averageLengths(ctx context.Context, userID uuid.UUID) (cycleLength, periodLength int) {
	cycles, err := s.cycleRepo.ListByUser(ctx, userID, 100)
	if err != nil {
		return fallbackCycleLength, fallbackPeriodLength
	}
	return calculateAverageCycleLength(cycles, fallbackCycleLength), calculateAveragePeriodLength(cycles, fallbackPeriodLength)
}

// classifyOccurrencePhase determines which cycle phase date fell in, based
// on the cycle it was logged against — using that cycle's own recorded
// length when known (more accurate for history than an average), falling
// back to the user's rolling average otherwise. Returns ok=false when the
// log wasn't linked to any cycle at all, so it can't be classified.
func classifyOccurrencePhase(date time.Time, cycle *model.Cycle, fallbackCycleLength, fallbackPeriodLength int) (phase model.Phase, ok bool) {
	if cycle == nil {
		return "", false
	}
	effectiveCycleLength := fallbackCycleLength
	if cycle.CycleLength != nil {
		effectiveCycleLength = *cycle.CycleLength
	}
	effectivePeriodLength := fallbackPeriodLength
	if cycle.PeriodLength != nil {
		effectivePeriodLength = *cycle.PeriodLength
	}
	return determineCurrentPhase(date, *cycle, effectiveCycleLength, effectivePeriodLength), true
}

func occurrenceDayOfCycle(date time.Time, cycle *model.Cycle) (day int, ok bool) {
	if cycle == nil {
		return 0, false
	}
	return daysBetween(cycle.StartDate, date) + 1, true
}

func monthsOrDefault(months int) int {
	if months <= 0 {
		return defaultInsightMonths
	}
	return months
}

// GetCycleSummary computes descriptive statistics over the user's full
// cycle history — not the rolling-6 window prediction uses, since this is
// meant to honestly describe everything recorded, not forecast forward.
func (s *InsightService) GetCycleSummary(ctx context.Context, userID uuid.UUID) (*model.CycleSummary, error) {
	cycles, err := s.cycleRepo.ListByUser(ctx, userID, 100)
	if err != nil {
		return nil, fmt.Errorf("list cycles: %w", err)
	}

	withLength := make([]model.Cycle, 0, len(cycles))
	for _, c := range cycles {
		if c.CycleLength != nil {
			withLength = append(withLength, c)
		}
	}

	if len(withLength) < minCyclesForSummary {
		return &model.CycleSummary{
			HasSufficientData: false,
			Message: fmt.Sprintf(
				"Butuh minimal %d siklus dengan panjang tercatat untuk menampilkan ringkasan (saat ini %d).",
				minCyclesForSummary, len(withLength),
			),
			TotalCycles: len(cycles),
		}, nil
	}

	sort.Slice(withLength, func(i, j int) bool { return withLength[i].StartDate.Before(withLength[j].StartDate) })

	sum, shortest, longest := 0, *withLength[0].CycleLength, *withLength[0].CycleLength
	trend := make([]model.CycleLengthPoint, 0, len(withLength))
	for _, c := range withLength {
		length := *c.CycleLength
		sum += length
		if length < shortest {
			shortest = length
		}
		if length > longest {
			longest = length
		}
		trend = append(trend, model.CycleLengthPoint{StartDate: c.StartDate, CycleLength: length})
	}
	avgCycle := float64(sum) / float64(len(withLength))

	periodSum, periodCount := 0, 0
	for _, c := range cycles {
		if c.PeriodLength != nil {
			periodSum += *c.PeriodLength
			periodCount++
		}
	}
	avgPeriod := 0.0
	if periodCount > 0 {
		avgPeriod = float64(periodSum) / float64(periodCount)
	}

	stddev := cycleLengthStdDev(withLength)
	regularity := model.RegularityModerate
	switch {
	case stddev <= regularityMaxStdDev:
		regularity = model.RegularityRegular
	case stddev > irregularityMinStdDev:
		regularity = model.RegularityIrregular
	}

	return &model.CycleSummary{
		HasSufficientData:   true,
		AverageCycleLength:  avgCycle,
		ShortestCycle:       shortest,
		LongestCycle:        longest,
		AveragePeriodLength: avgPeriod,
		TotalCycles:         len(cycles),
		Regularity:          regularity,
		CycleLengthTrend:    trend,
	}, nil
}

// GetSymptomInsights returns every symptom logged in the last `months`
// months, ranked by frequency, each with its phase distribution and most
// common cycle day.
func (s *InsightService) GetSymptomInsights(ctx context.Context, userID uuid.UUID, months int) (*model.SymptomInsights, error) {
	months = monthsOrDefault(months)
	to := truncateToDate(time.Now().UTC())
	from := to.AddDate(0, -months, 0)

	occurrences, err := s.insightRepo.ListSymptomOccurrences(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list symptom occurrences: %w", err)
	}
	sampleSize, err := s.insightRepo.CountLoggedDays(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("count logged days: %w", err)
	}

	fallbackCycle, fallbackPeriod := s.averageLengths(ctx, userID)

	type accumulator struct {
		name        string
		count       int
		phaseCounts map[model.Phase]int
		dayCounts   map[int]int
	}
	acc := make(map[uuid.UUID]*accumulator)
	var order []uuid.UUID

	for _, o := range occurrences {
		a, ok := acc[o.SymptomID]
		if !ok {
			a = &accumulator{name: o.Name, phaseCounts: map[model.Phase]int{}, dayCounts: map[int]int{}}
			acc[o.SymptomID] = a
			order = append(order, o.SymptomID)
		}
		a.count++
		if phase, ok := classifyOccurrencePhase(o.Date, o.Cycle, fallbackCycle, fallbackPeriod); ok {
			a.phaseCounts[phase]++
		}
		if day, ok := occurrenceDayOfCycle(o.Date, o.Cycle); ok {
			a.dayCounts[day]++
		}
	}

	insights := make([]model.SymptomInsight, 0, len(order))
	for _, id := range order {
		a := acc[id]

		days := make([]int, 0, len(a.dayCounts))
		for d := range a.dayCounts {
			days = append(days, d)
		}
		sort.Ints(days)

		var mostCommonDay *int
		bestCount := 0
		for _, d := range days {
			if a.dayCounts[d] > bestCount {
				bestCount = a.dayCounts[d]
				day := d
				mostCommonDay = &day
			}
		}

		insights = append(insights, model.SymptomInsight{
			SymptomID:          id,
			Name:               a.name,
			Count:              a.count,
			PhaseDistribution:  a.phaseCounts,
			MostCommonCycleDay: mostCommonDay,
			SampleSize:         a.count,
		})
	}

	sort.SliceStable(insights, func(i, j int) bool { return insights[i].Count > insights[j].Count })

	return &model.SymptomInsights{Symptoms: insights, Months: months, SampleSize: sampleSize}, nil
}

// GetMoodInsights returns the mood distribution for each cycle phase over
// the last `months` months.
func (s *InsightService) GetMoodInsights(ctx context.Context, userID uuid.UUID, months int) (*model.MoodInsights, error) {
	months = monthsOrDefault(months)
	to := truncateToDate(time.Now().UTC())
	from := to.AddDate(0, -months, 0)

	occurrences, err := s.insightRepo.ListMoodOccurrences(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list mood occurrences: %w", err)
	}

	fallbackCycle, fallbackPeriod := s.averageLengths(ctx, userID)

	phases := []model.Phase{model.PhaseMenstrual, model.PhaseFollicular, model.PhaseOvulation, model.PhaseLuteal}
	counts := make(map[model.Phase]map[string]int, len(phases))
	for _, p := range phases {
		counts[p] = map[string]int{}
	}

	for _, o := range occurrences {
		phase, ok := classifyOccurrencePhase(o.Date, o.Cycle, fallbackCycle, fallbackPeriod)
		if !ok {
			continue
		}
		counts[phase][o.Mood]++
	}

	byPhase := make([]model.MoodPhaseDistribution, 0, len(phases))
	for _, phase := range phases {
		moodCounts := counts[phase]

		sampleSize := 0
		moodNames := make([]string, 0, len(moodCounts))
		for m, c := range moodCounts {
			sampleSize += c
			moodNames = append(moodNames, m)
		}
		sort.Strings(moodNames)

		percentages := make(map[string]float64, len(moodCounts))
		dominantMood := ""
		dominantCount := 0
		for _, m := range moodNames {
			c := moodCounts[m]
			if sampleSize > 0 {
				percentages[m] = math.Round(float64(c)/float64(sampleSize)*1000) / 10
			}
			if c > dominantCount {
				dominantCount = c
				dominantMood = m
			}
		}

		byPhase = append(byPhase, model.MoodPhaseDistribution{
			Phase:          phase,
			MoodCounts:     moodCounts,
			MoodPercentage: percentages,
			DominantMood:   dominantMood,
			SampleSize:     sampleSize,
		})
	}

	return &model.MoodInsights{ByPhase: byPhase, Months: months}, nil
}
