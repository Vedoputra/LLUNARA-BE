package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

func createTestSymptom(t *testing.T, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := testPool.QueryRow(context.Background(),
		`insert into symptoms (user_id, name, category, is_custom) values (null, $1, 'physical', false) returning id`,
		name).Scan(&id)
	if err != nil {
		t.Fatalf("create test symptom: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `delete from symptoms where id = $1`, id)
	})
	return id
}

func TestDailyLogRepository_UpsertTwice_UpdatesNotDuplicates(t *testing.T) {
	skipIfNoDB(t)
	repo := NewDailyLogRepository(testPool)
	ctx := context.Background()

	d := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, d) })

	mood1 := "senang"
	first, err := repo.UpsertWithSymptoms(ctx, model.DailyLog{UserID: testUserID, Date: d, Mood: &mood1}, nil)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	mood2 := "cemas"
	second, err := repo.UpsertWithSymptoms(ctx, model.DailyLog{UserID: testUserID, Date: d, Mood: &mood2}, nil)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("expected the same row id on second upsert (update, not duplicate), got %v then %v", first.ID, second.ID)
	}
	if second.Mood == nil || *second.Mood != "cemas" {
		t.Errorf("mood = %v, want cemas (should reflect the update)", second.Mood)
	}

	fetched, err := repo.GetByDate(ctx, testUserID, d)
	if err != nil {
		t.Fatalf("GetByDate: %v", err)
	}
	if fetched.ID != first.ID {
		t.Error("expected only one row to exist for (user_id, date)")
	}
}

func TestDailyLogRepository_UpsertWithSymptoms_ReplacesSymptoms(t *testing.T) {
	skipIfNoDB(t)
	repo := NewDailyLogRepository(testPool)
	ctx := context.Background()

	symptomA := createTestSymptom(t, "test-symptom-a")
	symptomB := createTestSymptom(t, "test-symptom-b")

	d := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, d) })

	saved, err := repo.UpsertWithSymptoms(ctx, model.DailyLog{UserID: testUserID, Date: d}, []uuid.UUID{symptomA})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	fetched, err := repo.GetByDate(ctx, testUserID, d)
	if err != nil {
		t.Fatalf("GetByDate: %v", err)
	}
	if len(fetched.SymptomIDs) != 1 || fetched.SymptomIDs[0] != symptomA {
		t.Fatalf("expected symptom_ids = [%v], got %v", symptomA, fetched.SymptomIDs)
	}

	// Replace with a different symptom entirely.
	_, err = repo.UpsertWithSymptoms(ctx, model.DailyLog{UserID: testUserID, Date: d}, []uuid.UUID{symptomB})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	fetched, err = repo.GetByDate(ctx, testUserID, d)
	if err != nil {
		t.Fatalf("GetByDate: %v", err)
	}
	if len(fetched.SymptomIDs) != 1 || fetched.SymptomIDs[0] != symptomB {
		t.Fatalf("expected symptom_ids = [%v] after replace, got %v", symptomB, fetched.SymptomIDs)
	}
	if saved.ID != fetched.ID {
		t.Error("expected the same log row across upserts")
	}
}

func TestDailyLogRepository_UpsertWithSymptoms_RollsBackOnFailure(t *testing.T) {
	skipIfNoDB(t)
	repo := NewDailyLogRepository(testPool)
	ctx := context.Background()

	d := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, d) })

	// A symptom id that doesn't exist violates the FK on
	// daily_log_symptoms.symptom_id, so ReplaceSymptoms fails inside the
	// transaction. The whole operation — including the daily_logs upsert
	// itself — must be rolled back, not partially committed.
	bogusSymptomID := uuid.New()
	_, err := repo.UpsertWithSymptoms(ctx, model.DailyLog{UserID: testUserID, Date: d}, []uuid.UUID{bogusSymptomID})
	if err == nil {
		t.Fatal("expected an error from an invalid symptom_id, got nil")
	}

	_, err = repo.GetByDate(ctx, testUserID, d)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected no daily_logs row to have been committed after rollback, got err=%v", err)
	}
}

func TestDailyLogRepository_ListByRange(t *testing.T) {
	skipIfNoDB(t)
	repo := NewDailyLogRepository(testPool)
	ctx := context.Background()

	dates := []time.Time{
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), // outside the queried range
	}
	for _, d := range dates {
		if _, err := repo.UpsertWithSymptoms(ctx, model.DailyLog{UserID: testUserID, Date: d}, nil); err != nil {
			t.Fatalf("upsert: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, d := range dates {
			_ = repo.Delete(context.Background(), testUserID, d)
		}
	})

	logs, err := repo.ListByRange(ctx, testUserID, dates[0], dates[1])
	if err != nil {
		t.Fatalf("ListByRange: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs in range, got %d", len(logs))
	}
	if !logs[0].Date.Before(logs[1].Date) && !logs[0].Date.Equal(logs[1].Date) {
		t.Errorf("expected ascending date order, got %v then %v", logs[0].Date, logs[1].Date)
	}
}

func TestDailyLogRepository_Delete_WrongUserIsNotFound(t *testing.T) {
	skipIfNoDB(t)
	repo := NewDailyLogRepository(testPool)
	ctx := context.Background()

	d := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if _, err := repo.UpsertWithSymptoms(ctx, model.DailyLog{UserID: testUserID, Date: d}, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, d) })

	err := repo.Delete(ctx, uuid.New(), d)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound deleting another user's log, got %v", err)
	}
}

func TestDailyLogRepository_GetByDate_WrongUserIsNotFound(t *testing.T) {
	skipIfNoDB(t)
	repo := NewDailyLogRepository(testPool)
	ctx := context.Background()

	d := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	if _, err := repo.UpsertWithSymptoms(ctx, model.DailyLog{UserID: testUserID, Date: d}, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, d) })

	_, err := repo.GetByDate(ctx, uuid.New(), d)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound reading another user's log, got %v", err)
	}
}
