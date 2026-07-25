package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

type fakeWellnessRepo struct {
	logs map[string]model.WellnessLog
}

func newFakeWellnessRepo() *fakeWellnessRepo {
	return &fakeWellnessRepo{logs: make(map[string]model.WellnessLog)}
}

func wellnessKey(userID uuid.UUID, date time.Time) string {
	return userID.String() + "|" + date.Format(model.DateLayout)
}

// Upsert mirrors the real repository's merge (not replace) semantics: a
// nil field means "not sent this time", so it keeps whatever was there.
func (f *fakeWellnessRepo) Upsert(_ context.Context, log model.WellnessLog) (*model.WellnessLog, error) {
	key := wellnessKey(log.UserID, log.Date)
	if existing, ok := f.logs[key]; ok {
		log.ID = existing.ID
		if log.WaterGlasses == nil {
			log.WaterGlasses = existing.WaterGlasses
		}
		if log.SleepHours == nil {
			log.SleepHours = existing.SleepHours
		}
		if log.WeightKg == nil {
			log.WeightKg = existing.WeightKg
		}
	} else {
		log.ID = uuid.New()
	}
	f.logs[key] = log
	saved := log
	return &saved, nil
}

func (f *fakeWellnessRepo) ListByRange(_ context.Context, userID uuid.UUID, from, to time.Time) ([]model.WellnessLog, error) {
	list := make([]model.WellnessLog, 0)
	for _, l := range f.logs {
		if l.UserID == userID && !l.Date.Before(from) && !l.Date.After(to) {
			list = append(list, l)
		}
	}
	return list, nil
}

func TestWellnessUpsertLog_PartialDataAllowed(t *testing.T) {
	svc := NewWellnessService(newFakeWellnessRepo())
	userID := uuid.New()
	glasses := 5

	saved, err := svc.UpsertLog(context.Background(), userID, model.UpsertWellnessLogRequest{
		Date:         "2026-01-01",
		WaterGlasses: &glasses,
		// SleepHours and WeightKg deliberately omitted.
	})
	if err != nil {
		t.Fatalf("UpsertLog: %v", err)
	}
	if saved.WaterGlasses == nil || *saved.WaterGlasses != 5 {
		t.Errorf("water_glasses = %v, want 5", saved.WaterGlasses)
	}
	if saved.SleepHours != nil || saved.WeightKg != nil {
		t.Error("expected omitted fields to remain nil")
	}
}

func TestWellnessUpsertLog_TwiceOnSameDate_Updates(t *testing.T) {
	repo := newFakeWellnessRepo()
	svc := NewWellnessService(repo)
	userID := uuid.New()

	glasses1 := 3
	first, err := svc.UpsertLog(context.Background(), userID, model.UpsertWellnessLogRequest{Date: "2026-01-01", WaterGlasses: &glasses1})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	glasses2 := 8
	second, err := svc.UpsertLog(context.Background(), userID, model.UpsertWellnessLogRequest{Date: "2026-01-01", WaterGlasses: &glasses2})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("expected same id on second upsert, got %v then %v", first.ID, second.ID)
	}
	if len(repo.logs) != 1 {
		t.Errorf("expected exactly 1 stored log, got %d", len(repo.logs))
	}
}

// TestWellnessUpsertLog_LoggingOneMetricPreservesAnother is the case that
// matters most for this feature: users log water, sleep, and weight
// independently through the day, so a later call sending only one metric
// must not erase an earlier one for the same date.
func TestWellnessUpsertLog_LoggingOneMetricPreservesAnother(t *testing.T) {
	svc := NewWellnessService(newFakeWellnessRepo())
	userID := uuid.New()

	glasses := 6
	if _, err := svc.UpsertLog(context.Background(), userID, model.UpsertWellnessLogRequest{
		Date: "2026-01-01", WaterGlasses: &glasses,
	}); err != nil {
		t.Fatalf("first upsert (water): %v", err)
	}

	sleep := 7.5
	second, err := svc.UpsertLog(context.Background(), userID, model.UpsertWellnessLogRequest{
		Date: "2026-01-01", SleepHours: &sleep,
	})
	if err != nil {
		t.Fatalf("second upsert (sleep): %v", err)
	}

	if second.WaterGlasses == nil || *second.WaterGlasses != 6 {
		t.Errorf("water_glasses = %v, want 6 to still be there (logged earlier the same day)", second.WaterGlasses)
	}
	if second.SleepHours == nil || *second.SleepHours != 7.5 {
		t.Errorf("sleep_hours = %v, want 7.5", second.SleepHours)
	}
}

func TestWellnessUpsertLog_RejectsInvalidDateFormat(t *testing.T) {
	svc := NewWellnessService(newFakeWellnessRepo())

	_, err := svc.UpsertLog(context.Background(), uuid.New(), model.UpsertWellnessLogRequest{Date: "not-a-date"})
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}
