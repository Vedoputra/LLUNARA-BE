package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// TestAccountRepository_DeleteAllUserData_RemovesEverything uses its own
// dedicated user (rather than the package's shared testUserID) because it
// wipes every row that user owns — running it against the shared user would
// break every other test in this package that depends on that data still
// existing.
func TestAccountRepository_DeleteAllUserData_RemovesEverything(t *testing.T) {
	skipIfNoDB(t)
	ctx := context.Background()

	userID, err := createTestUser(testSupabaseURL, testSupabaseSecret)
	if err != nil {
		t.Fatalf("create dedicated test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "delete from auth.users where id = $1", userID)
	})

	d := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cycleRepo := NewCycleRepository(testPool)
	cycle, err := cycleRepo.Create(ctx, model.Cycle{UserID: userID, StartDate: d})
	if err != nil {
		t.Fatalf("create cycle: %v", err)
	}

	symptomRepo := NewSymptomRepository(testPool)
	symptom, err := symptomRepo.Create(ctx, model.Symptom{UserID: &userID, Name: "account-delete-test-symptom", Category: "physical"})
	if err != nil {
		t.Fatalf("create symptom: %v", err)
	}

	dailyLogRepo := NewDailyLogRepository(testPool)
	log, err := dailyLogRepo.UpsertWithSymptoms(ctx, model.DailyLog{
		UserID: userID, CycleID: &cycle.ID, Date: d,
	}, []uuid.UUID{symptom.ID})
	if err != nil {
		t.Fatalf("create daily log: %v", err)
	}

	wellnessRepo := NewWellnessRepository(testPool)
	glasses := 5
	if _, err := wellnessRepo.Upsert(ctx, model.WellnessLog{UserID: userID, Date: d, WaterGlasses: &glasses}); err != nil {
		t.Fatalf("create wellness log: %v", err)
	}

	accountRepo := NewAccountRepository(testPool)
	if err := accountRepo.DeleteAllUserData(ctx, userID); err != nil {
		t.Fatalf("DeleteAllUserData: %v", err)
	}

	assertNoRowsForUser(t, ctx, "cycles", userID)
	assertNoRowsForUser(t, ctx, "daily_logs", userID)
	assertNoRowsForUser(t, ctx, "wellness_logs", userID)
	assertNoRowsForUser(t, ctx, "reminders", userID)

	var symptomCount int
	if err := testPool.QueryRow(ctx, "select count(*) from symptoms where user_id = $1", userID).Scan(&symptomCount); err != nil {
		t.Fatalf("count symptoms: %v", err)
	}
	if symptomCount != 0 {
		t.Errorf("expected 0 remaining custom symptoms for user, got %d", symptomCount)
	}

	var junctionCount int
	if err := testPool.QueryRow(ctx, "select count(*) from daily_log_symptoms where daily_log_id = $1", log.ID).Scan(&junctionCount); err != nil {
		t.Fatalf("count daily_log_symptoms: %v", err)
	}
	if junctionCount != 0 {
		t.Errorf("expected the daily_log's symptom links to cascade-delete, got %d remaining", junctionCount)
	}

	// Preset symptoms (user_id is null) must never be touched.
	var presetCount int
	if err := testPool.QueryRow(ctx, "select count(*) from symptoms where user_id is null").Scan(&presetCount); err != nil {
		t.Fatalf("count preset symptoms: %v", err)
	}
	if presetCount == 0 {
		t.Error("expected seeded preset symptoms to still exist")
	}
}

func assertNoRowsForUser(t *testing.T, ctx context.Context, table string, userID uuid.UUID) {
	t.Helper()
	var count int
	q := "select count(*) from " + table + " where user_id = $1"
	if err := testPool.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if count != 0 {
		t.Errorf("expected 0 remaining rows in %s for user, got %d", table, count)
	}
}
