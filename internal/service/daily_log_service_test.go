package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

type fakeDailyLogRepo struct {
	logs map[string]model.DailyLog
}

func newFakeDailyLogRepo() *fakeDailyLogRepo {
	return &fakeDailyLogRepo{logs: make(map[string]model.DailyLog)}
}

func dailyLogKey(userID uuid.UUID, date time.Time) string {
	return userID.String() + "|" + date.Format(model.DateLayout)
}

func (f *fakeDailyLogRepo) UpsertWithSymptoms(_ context.Context, log model.DailyLog, symptomIDs []uuid.UUID) (*model.DailyLog, error) {
	key := dailyLogKey(log.UserID, log.Date)
	if existing, ok := f.logs[key]; ok {
		log.ID = existing.ID
		log.CreatedAt = existing.CreatedAt
	} else {
		log.ID = uuid.New()
		log.CreatedAt = time.Now()
	}
	log.UpdatedAt = time.Now()
	log.SymptomIDs = symptomIDs
	f.logs[key] = log
	saved := log
	return &saved, nil
}

func (f *fakeDailyLogRepo) GetByDate(_ context.Context, userID uuid.UUID, date time.Time) (*model.DailyLog, error) {
	log, ok := f.logs[dailyLogKey(userID, date)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	l := log
	return &l, nil
}

func (f *fakeDailyLogRepo) ListByRange(_ context.Context, userID uuid.UUID, from, to time.Time) ([]model.DailyLog, error) {
	list := make([]model.DailyLog, 0)
	for _, l := range f.logs {
		if l.UserID == userID && !l.Date.Before(from) && !l.Date.After(to) {
			list = append(list, l)
		}
	}
	return list, nil
}

func (f *fakeDailyLogRepo) Delete(_ context.Context, userID uuid.UUID, date time.Time) error {
	key := dailyLogKey(userID, date)
	l, ok := f.logs[key]
	if !ok || l.UserID != userID {
		return repository.ErrNotFound
	}
	delete(f.logs, key)
	return nil
}

type fakeSymptomValidator struct {
	validIDs map[uuid.UUID]bool
}

func newFakeSymptomValidator(validIDs ...uuid.UUID) *fakeSymptomValidator {
	m := make(map[uuid.UUID]bool)
	for _, id := range validIDs {
		m[id] = true
	}
	return &fakeSymptomValidator{validIDs: m}
}

func (f *fakeSymptomValidator) ValidateIDs(_ context.Context, _ uuid.UUID, symptomIDs []uuid.UUID) (bool, error) {
	for _, id := range symptomIDs {
		if !f.validIDs[id] {
			return false, nil
		}
	}
	return true, nil
}

func newTestDailyLogService(dailyLogs *fakeDailyLogRepo, cycles *fakeCycleRepo, symptoms *fakeSymptomValidator) *DailyLogService {
	if dailyLogs == nil {
		dailyLogs = newFakeDailyLogRepo()
	}
	if cycles == nil {
		cycles = newFakeCycleRepo()
	}
	if symptoms == nil {
		symptoms = newFakeSymptomValidator()
	}
	return NewDailyLogService(dailyLogs, cycles, symptoms)
}

func TestUpsertLog_RejectsFutureDate(t *testing.T) {
	svc := newTestDailyLogService(nil, nil, nil)
	future := time.Now().AddDate(0, 0, 3).Format(model.DateLayout)

	_, err := svc.UpsertLog(context.Background(), uuid.New(), model.UpsertDailyLogRequest{Date: future})
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestUpsertLog_RejectsNotesTooLong(t *testing.T) {
	svc := newTestDailyLogService(nil, nil, nil)
	longNotes := make([]byte, 501)
	for i := range longNotes {
		longNotes[i] = 'a'
	}
	notes := string(longNotes)

	_, err := svc.UpsertLog(context.Background(), uuid.New(), model.UpsertDailyLogRequest{
		Date:  "2026-01-01",
		Notes: &notes,
	})
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestUpsertLog_RejectsInvalidSymptomIDFormat(t *testing.T) {
	svc := newTestDailyLogService(nil, nil, nil)

	_, err := svc.UpsertLog(context.Background(), uuid.New(), model.UpsertDailyLogRequest{
		Date:       "2026-01-01",
		SymptomIDs: []string{"not-a-uuid"},
	})
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestUpsertLog_RejectsSymptomNotOwnedOrPreset(t *testing.T) {
	ownedSymptom := uuid.New()
	notOwnedSymptom := uuid.New()
	symptoms := newFakeSymptomValidator(ownedSymptom) // notOwnedSymptom is NOT valid
	svc := newTestDailyLogService(nil, nil, symptoms)

	_, err := svc.UpsertLog(context.Background(), uuid.New(), model.UpsertDailyLogRequest{
		Date:       "2026-01-01",
		SymptomIDs: []string{ownedSymptom.String(), notOwnedSymptom.String()},
	})
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestUpsertLog_LinksCycleIDWhenDateFallsWithinCycle(t *testing.T) {
	userID := uuid.New()
	cycle := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1)}
	cycles := newFakeCycleRepo(cycle)
	svc := newTestDailyLogService(nil, cycles, nil)

	saved, err := svc.UpsertLog(context.Background(), userID, model.UpsertDailyLogRequest{Date: "2026-01-03"})
	if err != nil {
		t.Fatalf("UpsertLog: %v", err)
	}
	if saved.CycleID == nil || *saved.CycleID != cycle.ID {
		t.Errorf("cycle_id = %v, want %v", saved.CycleID, cycle.ID)
	}
}

func TestUpsertLog_NoCycleIDWhenNoMatch(t *testing.T) {
	userID := uuid.New()
	svc := newTestDailyLogService(nil, newFakeCycleRepo(), nil)

	saved, err := svc.UpsertLog(context.Background(), userID, model.UpsertDailyLogRequest{Date: "2026-01-03"})
	if err != nil {
		t.Fatalf("UpsertLog: %v", err)
	}
	if saved.CycleID != nil {
		t.Errorf("expected nil cycle_id, got %v", *saved.CycleID)
	}
}

// TestUpsertLog_LinksCycleIDForOlderClosedCycle is a regression test for
// the same underlying bug fixed in FindOverlapping: a log dated well past
// an older, already-closed cycle's menstrual flow (end_date) but still
// within its full cycle_length span must still resolve to that cycle —
// UpsertLog reuses FindOverlapping to do this linking, so it inherited the
// bug where such cycles were only ever matched during their few end_date
// days instead of their whole ~28-day span.
func TestUpsertLog_LinksCycleIDForOlderClosedCycle(t *testing.T) {
	userID := uuid.New()
	length := 28
	older := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 1), CycleLength: &length}
	latest := model.Cycle{ID: uuid.New(), UserID: userID, StartDate: date(2026, 1, 29)}
	cycles := newFakeCycleRepo(older, latest)
	svc := newTestDailyLogService(nil, cycles, nil)

	// Day 20 of the older cycle — well past any plausible end_date, but
	// still inside its 28-day span.
	saved, err := svc.UpsertLog(context.Background(), userID, model.UpsertDailyLogRequest{Date: "2026-01-20"})
	if err != nil {
		t.Fatalf("UpsertLog: %v", err)
	}
	if saved.CycleID == nil || *saved.CycleID != older.ID {
		t.Errorf("cycle_id = %v, want %v (the older cycle whose span covers day 20)", saved.CycleID, older.ID)
	}
}

func TestUpsertLog_SuccessWithValidSymptoms(t *testing.T) {
	presetSymptom := uuid.New()
	userID := uuid.New()
	symptoms := newFakeSymptomValidator(presetSymptom)
	svc := newTestDailyLogService(nil, nil, symptoms)

	mood := "senang"
	saved, err := svc.UpsertLog(context.Background(), userID, model.UpsertDailyLogRequest{
		Date:       "2026-01-01",
		Mood:       &mood,
		SymptomIDs: []string{presetSymptom.String()},
	})
	if err != nil {
		t.Fatalf("UpsertLog: %v", err)
	}
	if len(saved.SymptomIDs) != 1 || saved.SymptomIDs[0] != presetSymptom {
		t.Errorf("symptom_ids = %v, want [%v]", saved.SymptomIDs, presetSymptom)
	}
	if saved.Mood == nil || *saved.Mood != "senang" {
		t.Errorf("mood = %v, want senang", saved.Mood)
	}
}

func TestUpsertLog_TwiceOnSameDate_UpdatesSameRow(t *testing.T) {
	userID := uuid.New()
	repo := newFakeDailyLogRepo()
	svc := newTestDailyLogService(repo, nil, nil)

	mood1 := "senang"
	first, err := svc.UpsertLog(context.Background(), userID, model.UpsertDailyLogRequest{Date: "2026-01-01", Mood: &mood1})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	mood2 := "cemas"
	second, err := svc.UpsertLog(context.Background(), userID, model.UpsertDailyLogRequest{Date: "2026-01-01", Mood: &mood2})
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

func TestDeleteLog_NotFound(t *testing.T) {
	svc := newTestDailyLogService(nil, nil, nil)

	err := svc.DeleteLog(context.Background(), uuid.New(), date(2026, 1, 1))
	if code := apiErrCode(t, err); code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
}
