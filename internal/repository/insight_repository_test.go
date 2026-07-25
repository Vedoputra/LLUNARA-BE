package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

func TestInsightRepository_ListSymptomOccurrences_JoinsCycleContext(t *testing.T) {
	skipIfNoDB(t)
	ctx := context.Background()

	cycleRepo := NewCycleRepository(testPool)
	dailyLogRepo := NewDailyLogRepository(testPool)
	insightRepo := NewInsightRepository(testPool)

	cycleStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	cycle, err := cycleRepo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: cycleStart})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	t.Cleanup(func() { _ = cycleRepo.Delete(context.Background(), testUserID, cycle.ID) })

	symptom := createTestSymptom(t, "test-insight-symptom")

	logDate := cycleStart.AddDate(0, 0, 3) // day 4 of the cycle
	if _, err := dailyLogRepo.UpsertWithSymptoms(ctx, model.DailyLog{
		UserID: testUserID, CycleID: &cycle.ID, Date: logDate,
	}, []uuid.UUID{symptom}); err != nil {
		t.Fatalf("upsert daily log: %v", err)
	}
	t.Cleanup(func() { _ = dailyLogRepo.Delete(context.Background(), testUserID, logDate) })

	occurrences, err := insightRepo.ListSymptomOccurrences(ctx, testUserID, cycleStart, cycleStart.AddDate(0, 1, 0))
	if err != nil {
		t.Fatalf("ListSymptomOccurrences: %v", err)
	}

	var found bool
	for _, o := range occurrences {
		if o.SymptomID != symptom {
			continue
		}
		found = true
		if o.Cycle == nil {
			t.Fatal("expected occurrence to carry cycle context")
		}
		if !o.Cycle.StartDate.Equal(cycleStart) {
			t.Errorf("cycle start = %v, want %v", o.Cycle.StartDate, cycleStart)
		}
		if !o.Date.Equal(logDate) {
			t.Errorf("date = %v, want %v", o.Date, logDate)
		}
	}
	if !found {
		t.Error("expected to find the logged symptom occurrence")
	}
}

func TestInsightRepository_ListMoodOccurrences(t *testing.T) {
	skipIfNoDB(t)
	ctx := context.Background()

	dailyLogRepo := NewDailyLogRepository(testPool)
	insightRepo := NewInsightRepository(testPool)

	logDate := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	mood := "tenang"
	if _, err := dailyLogRepo.UpsertWithSymptoms(ctx, model.DailyLog{
		UserID: testUserID, Date: logDate, Mood: &mood,
	}, nil); err != nil {
		t.Fatalf("upsert daily log: %v", err)
	}
	t.Cleanup(func() { _ = dailyLogRepo.Delete(context.Background(), testUserID, logDate) })

	occurrences, err := insightRepo.ListMoodOccurrences(ctx, testUserID, logDate, logDate)
	if err != nil {
		t.Fatalf("ListMoodOccurrences: %v", err)
	}
	if len(occurrences) != 1 {
		t.Fatalf("expected 1 mood occurrence, got %d", len(occurrences))
	}
	if occurrences[0].Mood != "tenang" {
		t.Errorf("mood = %q, want tenang", occurrences[0].Mood)
	}
}

func TestInsightRepository_CountLoggedDays(t *testing.T) {
	skipIfNoDB(t)
	ctx := context.Background()

	dailyLogRepo := NewDailyLogRepository(testPool)
	insightRepo := NewInsightRepository(testPool)

	dates := []time.Time{
		time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
	}
	for _, d := range dates {
		if _, err := dailyLogRepo.UpsertWithSymptoms(ctx, model.DailyLog{UserID: testUserID, Date: d}, nil); err != nil {
			t.Fatalf("upsert daily log: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, d := range dates {
			_ = dailyLogRepo.Delete(context.Background(), testUserID, d)
		}
	})

	count, err := insightRepo.CountLoggedDays(ctx, testUserID, dates[0], dates[1])
	if err != nil {
		t.Fatalf("CountLoggedDays: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}
