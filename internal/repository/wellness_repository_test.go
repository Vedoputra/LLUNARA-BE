package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

func TestWellnessRepository_UpsertPartialData(t *testing.T) {
	skipIfNoDB(t)
	repo := NewWellnessRepository(testPool)
	ctx := context.Background()

	d := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	glasses := 6
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "delete from wellness_logs where user_id = $1 and date = $2", testUserID, d)
	})

	saved, err := repo.Upsert(ctx, model.WellnessLog{UserID: testUserID, Date: d, WaterGlasses: &glasses})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if saved.WaterGlasses == nil || *saved.WaterGlasses != 6 {
		t.Errorf("water_glasses = %v, want 6", saved.WaterGlasses)
	}
	if saved.SleepHours != nil || saved.WeightKg != nil {
		t.Error("expected omitted fields to be nil")
	}
}

func TestWellnessRepository_UpsertTwice_Updates(t *testing.T) {
	skipIfNoDB(t)
	repo := NewWellnessRepository(testPool)
	ctx := context.Background()

	d := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "delete from wellness_logs where user_id = $1 and date = $2", testUserID, d)
	})

	sleep1 := 6.5
	first, err := repo.Upsert(ctx, model.WellnessLog{UserID: testUserID, Date: d, SleepHours: &sleep1})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	sleep2 := 8.0
	second, err := repo.Upsert(ctx, model.WellnessLog{UserID: testUserID, Date: d, SleepHours: &sleep2})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("expected same row id, got %v then %v", first.ID, second.ID)
	}
	if second.SleepHours == nil || *second.SleepHours != 8.0 {
		t.Errorf("sleep_hours = %v, want 8.0", second.SleepHours)
	}
}

func TestWellnessRepository_UpsertMergesInsteadOfReplacing(t *testing.T) {
	skipIfNoDB(t)
	repo := NewWellnessRepository(testPool)
	ctx := context.Background()

	d := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "delete from wellness_logs where user_id = $1 and date = $2", testUserID, d)
	})

	glasses := 6
	if _, err := repo.Upsert(ctx, model.WellnessLog{UserID: testUserID, Date: d, WaterGlasses: &glasses}); err != nil {
		t.Fatalf("first upsert (water): %v", err)
	}

	sleep := 7.5
	second, err := repo.Upsert(ctx, model.WellnessLog{UserID: testUserID, Date: d, SleepHours: &sleep})
	if err != nil {
		t.Fatalf("second upsert (sleep): %v", err)
	}

	if second.WaterGlasses == nil || *second.WaterGlasses != 6 {
		t.Errorf("water_glasses = %v, want 6 to be preserved from the earlier call", second.WaterGlasses)
	}
	if second.SleepHours == nil || *second.SleepHours != 7.5 {
		t.Errorf("sleep_hours = %v, want 7.5", second.SleepHours)
	}
}

func TestWellnessRepository_ListByRange(t *testing.T) {
	skipIfNoDB(t)
	repo := NewWellnessRepository(testPool)
	ctx := context.Background()

	dates := []time.Time{
		time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), // outside queried range
	}
	weight := 55.5
	for _, d := range dates {
		if _, err := repo.Upsert(ctx, model.WellnessLog{UserID: testUserID, Date: d, WeightKg: &weight}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, d := range dates {
			_, _ = testPool.Exec(context.Background(), "delete from wellness_logs where user_id = $1 and date = $2", testUserID, d)
		}
	})

	logs, err := repo.ListByRange(ctx, testUserID, dates[0], dates[1])
	if err != nil {
		t.Fatalf("ListByRange: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs in range, got %d", len(logs))
	}
}
