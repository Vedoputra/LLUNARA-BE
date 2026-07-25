package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

type fakeInsightRepo struct {
	symptomOccurrences []repository.SymptomOccurrence
	moodOccurrences    []repository.MoodOccurrence
	loggedDays         int
}

func (f *fakeInsightRepo) ListSymptomOccurrences(context.Context, uuid.UUID, time.Time, time.Time) ([]repository.SymptomOccurrence, error) {
	return f.symptomOccurrences, nil
}

func (f *fakeInsightRepo) CountLoggedDays(context.Context, uuid.UUID, time.Time, time.Time) (int, error) {
	return f.loggedDays, nil
}

func (f *fakeInsightRepo) ListMoodOccurrences(context.Context, uuid.UUID, time.Time, time.Time) ([]repository.MoodOccurrence, error) {
	return f.moodOccurrences, nil
}

// --- GetCycleSummary ---

func TestGetCycleSummary_InsufficientData(t *testing.T) {
	cycles := newFakeCycleRepo(model.Cycle{ID: uuid.New(), UserID: testUUID, StartDate: date(2026, 1, 1)}) // 1 cycle, no cycle_length
	svc := NewInsightService(cycles, &fakeInsightRepo{})

	summary, err := svc.GetCycleSummary(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetCycleSummary: %v", err)
	}
	if summary.HasSufficientData {
		t.Error("expected has_sufficient_data = false")
	}
	if summary.Message == "" {
		t.Error("expected a message explaining how much more data is needed")
	}
}

func TestGetCycleSummary_ComputesStats(t *testing.T) {
	all := withUserID(buildCycleChain(date(2026, 1, 1), []int{28, 30, 26}), testUUID) // 3 usable: 28,30,26
	cycles := newFakeCycleRepo(all...)
	svc := NewInsightService(cycles, &fakeInsightRepo{})

	summary, err := svc.GetCycleSummary(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetCycleSummary: %v", err)
	}
	if !summary.HasSufficientData {
		t.Fatal("expected has_sufficient_data = true")
	}
	if summary.ShortestCycle != 26 {
		t.Errorf("shortest = %d, want 26", summary.ShortestCycle)
	}
	if summary.LongestCycle != 30 {
		t.Errorf("longest = %d, want 30", summary.LongestCycle)
	}
	wantAvg := float64(28+30+26) / 3
	if summary.AverageCycleLength != wantAvg {
		t.Errorf("average = %v, want %v", summary.AverageCycleLength, wantAvg)
	}
	if summary.TotalCycles != len(all) {
		t.Errorf("total_cycles = %d, want %d", summary.TotalCycles, len(all))
	}
	if len(summary.CycleLengthTrend) != 3 {
		t.Errorf("trend points = %d, want 3", len(summary.CycleLengthTrend))
	}
}

func TestGetCycleSummary_Regularity(t *testing.T) {
	tests := []struct {
		name string
		gaps []int
		want model.Regularity
	}{
		{"very consistent", []int{28, 29, 28, 27}, model.RegularityRegular},
		{"wildly inconsistent", []int{21, 45, 21, 45}, model.RegularityIrregular},
		{"somewhat variable", []int{22, 34, 24, 32}, model.RegularityModerate}, // stddev ~5.1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cycles := newFakeCycleRepo(withUserID(buildCycleChain(date(2026, 1, 1), tt.gaps), testUUID)...)
			svc := NewInsightService(cycles, &fakeInsightRepo{})

			summary, err := svc.GetCycleSummary(context.Background(), testUUID)
			if err != nil {
				t.Fatalf("GetCycleSummary: %v", err)
			}
			if summary.Regularity != tt.want {
				t.Errorf("regularity = %v, want %v", summary.Regularity, tt.want)
			}
		})
	}
}

// --- GetSymptomInsights ---

func TestGetSymptomInsights_RanksByFrequency(t *testing.T) {
	kram := uuid.New()
	sakitKepala := uuid.New()

	insightRepo := &fakeInsightRepo{
		loggedDays: 3,
		symptomOccurrences: []repository.SymptomOccurrence{
			{SymptomID: sakitKepala, Name: "sakit kepala", Date: date(2026, 1, 1)},
			{SymptomID: kram, Name: "kram", Date: date(2026, 1, 1)},
			{SymptomID: kram, Name: "kram", Date: date(2026, 1, 2)},
			{SymptomID: kram, Name: "kram", Date: date(2026, 1, 3)},
		},
	}
	svc := NewInsightService(newFakeCycleRepo(), insightRepo)

	insights, err := svc.GetSymptomInsights(context.Background(), testUUID, 6)
	if err != nil {
		t.Fatalf("GetSymptomInsights: %v", err)
	}
	if len(insights.Symptoms) != 2 {
		t.Fatalf("expected 2 distinct symptoms, got %d", len(insights.Symptoms))
	}
	if insights.Symptoms[0].SymptomID != kram || insights.Symptoms[0].Count != 3 {
		t.Errorf("expected kram (count 3) first, got %+v", insights.Symptoms[0])
	}
	if insights.SampleSize != 3 {
		t.Errorf("sample_size = %d, want 3", insights.SampleSize)
	}
}

func TestGetSymptomInsights_ClassifiesPhaseWhenCycleKnown(t *testing.T) {
	symptomID := uuid.New()
	cycle := &model.Cycle{StartDate: date(2026, 1, 1)} // no recorded length -> uses fallback

	insightRepo := &fakeInsightRepo{
		symptomOccurrences: []repository.SymptomOccurrence{
			{SymptomID: symptomID, Name: "kram", Date: date(2026, 1, 1), Cycle: cycle}, // day 1 -> menstrual
		},
	}
	svc := NewInsightService(newFakeCycleRepo(), insightRepo)

	insights, err := svc.GetSymptomInsights(context.Background(), testUUID, 6)
	if err != nil {
		t.Fatalf("GetSymptomInsights: %v", err)
	}
	if len(insights.Symptoms) != 1 {
		t.Fatalf("expected 1 symptom, got %d", len(insights.Symptoms))
	}
	got := insights.Symptoms[0]
	if got.PhaseDistribution[model.PhaseMenstrual] != 1 {
		t.Errorf("expected 1 menstrual-phase occurrence, got %v", got.PhaseDistribution)
	}
	if got.MostCommonCycleDay == nil || *got.MostCommonCycleDay != 1 {
		t.Errorf("most_common_cycle_day = %v, want 1", got.MostCommonCycleDay)
	}
}

func TestGetSymptomInsights_OccurrenceWithoutCycle_CountedButNotPhased(t *testing.T) {
	symptomID := uuid.New()
	insightRepo := &fakeInsightRepo{
		symptomOccurrences: []repository.SymptomOccurrence{
			{SymptomID: symptomID, Name: "kram", Date: date(2026, 1, 1), Cycle: nil},
		},
	}
	svc := NewInsightService(newFakeCycleRepo(), insightRepo)

	insights, err := svc.GetSymptomInsights(context.Background(), testUUID, 6)
	if err != nil {
		t.Fatalf("GetSymptomInsights: %v", err)
	}
	got := insights.Symptoms[0]
	if got.Count != 1 {
		t.Errorf("count = %d, want 1 (should still be counted)", got.Count)
	}
	if len(got.PhaseDistribution) != 0 {
		t.Errorf("expected no phase classification without a cycle, got %v", got.PhaseDistribution)
	}
	if got.MostCommonCycleDay != nil {
		t.Errorf("expected no cycle day without a cycle, got %v", *got.MostCommonCycleDay)
	}
}

func TestGetSymptomInsights_MostCommonDayBreaksTiesByEarliestDay(t *testing.T) {
	symptomID := uuid.New()
	cycle := &model.Cycle{StartDate: date(2026, 1, 1)}

	insightRepo := &fakeInsightRepo{
		symptomOccurrences: []repository.SymptomOccurrence{
			// day 5 once, day 2 once -> tie; day 2 (earlier) should win deterministically.
			{SymptomID: symptomID, Name: "kram", Date: date(2026, 1, 5), Cycle: cycle},
			{SymptomID: symptomID, Name: "kram", Date: date(2026, 1, 2), Cycle: cycle},
		},
	}
	svc := NewInsightService(newFakeCycleRepo(), insightRepo)

	insights, err := svc.GetSymptomInsights(context.Background(), testUUID, 6)
	if err != nil {
		t.Fatalf("GetSymptomInsights: %v", err)
	}
	got := insights.Symptoms[0].MostCommonCycleDay
	if got == nil || *got != 2 {
		t.Errorf("most_common_cycle_day = %v, want 2 (earliest day on a tie)", got)
	}
}

func TestGetSymptomInsights_DefaultsMonthsToSix(t *testing.T) {
	svc := NewInsightService(newFakeCycleRepo(), &fakeInsightRepo{})
	insights, err := svc.GetSymptomInsights(context.Background(), testUUID, 0)
	if err != nil {
		t.Fatalf("GetSymptomInsights: %v", err)
	}
	if insights.Months != 6 {
		t.Errorf("months = %d, want default 6", insights.Months)
	}
}

// --- GetMoodInsights ---

func TestGetMoodInsights_DistributionAndDominantMood(t *testing.T) {
	cycle := &model.Cycle{StartDate: date(2026, 1, 1)} // day 1 -> menstrual with fallback lengths

	insightRepo := &fakeInsightRepo{
		moodOccurrences: []repository.MoodOccurrence{
			{Mood: "senang", Date: date(2026, 1, 1), Cycle: cycle},
			{Mood: "senang", Date: date(2026, 1, 1), Cycle: cycle},
			{Mood: "cemas", Date: date(2026, 1, 1), Cycle: cycle},
		},
	}
	svc := NewInsightService(newFakeCycleRepo(), insightRepo)

	insights, err := svc.GetMoodInsights(context.Background(), testUUID, 6)
	if err != nil {
		t.Fatalf("GetMoodInsights: %v", err)
	}

	var menstrual *model.MoodPhaseDistribution
	for i := range insights.ByPhase {
		if insights.ByPhase[i].Phase == model.PhaseMenstrual {
			menstrual = &insights.ByPhase[i]
		}
	}
	if menstrual == nil {
		t.Fatal("expected a menstrual phase entry")
	}
	if menstrual.SampleSize != 3 {
		t.Errorf("sample_size = %d, want 3", menstrual.SampleSize)
	}
	if menstrual.DominantMood != "senang" {
		t.Errorf("dominant_mood = %q, want senang", menstrual.DominantMood)
	}
	if menstrual.MoodPercentage["senang"] < 66.0 || menstrual.MoodPercentage["senang"] > 67.0 {
		t.Errorf("senang percentage = %v, want ~66.7", menstrual.MoodPercentage["senang"])
	}
}

func TestGetMoodInsights_AlwaysReturnsAllFourPhases(t *testing.T) {
	svc := NewInsightService(newFakeCycleRepo(), &fakeInsightRepo{})
	insights, err := svc.GetMoodInsights(context.Background(), testUUID, 6)
	if err != nil {
		t.Fatalf("GetMoodInsights: %v", err)
	}
	if len(insights.ByPhase) != 4 {
		t.Fatalf("expected 4 phase entries even with no data, got %d", len(insights.ByPhase))
	}
	for _, p := range insights.ByPhase {
		if p.SampleSize != 0 {
			t.Errorf("expected 0 sample size for phase %v with no data, got %d", p.Phase, p.SampleSize)
		}
	}
}

func TestGetMoodInsights_OccurrenceWithoutCycleIsExcluded(t *testing.T) {
	insightRepo := &fakeInsightRepo{
		moodOccurrences: []repository.MoodOccurrence{
			{Mood: "senang", Date: date(2026, 1, 1), Cycle: nil},
		},
	}
	svc := NewInsightService(newFakeCycleRepo(), insightRepo)

	insights, err := svc.GetMoodInsights(context.Background(), testUUID, 6)
	if err != nil {
		t.Fatalf("GetMoodInsights: %v", err)
	}
	for _, p := range insights.ByPhase {
		if p.SampleSize != 0 {
			t.Errorf("expected moods without a cycle to be excluded, got sample_size=%d for %v", p.SampleSize, p.Phase)
		}
	}
}

// testUUID is a fixed user id reused across insight tests.
var testUUID = uuid.New()

// withUserID stamps userID onto each cycle — buildCycleChain (from
// prediction_service_test.go) doesn't set one, since Predict() operates on
// a raw slice and never filters by it, but fakeCycleRepo.ListByUser does.
func withUserID(cycles []model.Cycle, userID uuid.UUID) []model.Cycle {
	for i := range cycles {
		cycles[i].UserID = userID
	}
	return cycles
}
