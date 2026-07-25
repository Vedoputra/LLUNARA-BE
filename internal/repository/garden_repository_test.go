package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// TestGardenRepository_Aggregations uses its own dedicated user (not the
// package's shared testUserID) because it asserts exact counts — sharing
// the user with other tests in this package (which also write daily_logs
// for testUserID) would make those counts unpredictable.
func TestGardenRepository_Aggregations(t *testing.T) {
	skipIfNoDB(t)
	ctx := context.Background()

	userID, err := createTestUser(testSupabaseURL, testSupabaseSecret)
	if err != nil {
		t.Fatalf("create dedicated test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "delete from auth.users where id = $1", userID)
	})

	dailyLogRepo := NewDailyLogRepository(testPool)
	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	moods := []string{"senang", "tenang"}
	dates := []time.Time{
		today,
		today.AddDate(0, 0, -1),
		today.AddDate(0, 0, -40), // outside this month/week, still counts toward the all-time total
	}
	for i, d := range dates {
		mood := moods[i%len(moods)]
		if _, err := dailyLogRepo.Upsert(ctx, model.DailyLog{UserID: userID, Date: d, Mood: &mood}); err != nil {
			t.Fatalf("seed daily log %v: %v", d, err)
		}
	}
	t.Cleanup(func() {
		for _, d := range dates {
			_ = dailyLogRepo.Delete(context.Background(), userID, d)
		}
	})

	repo := NewGardenRepository(testPool)

	total, err := repo.CountDistinctLoggedDays(ctx, userID)
	if err != nil {
		t.Fatalf("CountDistinctLoggedDays: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	inMonth, err := repo.CountDistinctLoggedDaysInRange(ctx, userID, monthStart, today)
	if err != nil {
		t.Fatalf("CountDistinctLoggedDaysInRange (month): %v", err)
	}
	wantInMonth := 1
	if !today.AddDate(0, 0, -1).Before(monthStart) {
		wantInMonth = 2
	}
	if inMonth != wantInMonth {
		t.Errorf("logged days this month = %d, want %d", inMonth, wantInMonth)
	}

	weekStart := today.AddDate(0, 0, -6)
	inWeek, err := repo.CountDistinctLoggedDaysInRange(ctx, userID, weekStart, today)
	if err != nil {
		t.Fatalf("CountDistinctLoggedDaysInRange (week): %v", err)
	}
	if inWeek != 2 {
		t.Errorf("logged days this week = %d, want 2 (today + yesterday)", inWeek)
	}

	distinctMoods, err := repo.DistinctMoods(ctx, userID)
	if err != nil {
		t.Fatalf("DistinctMoods: %v", err)
	}
	if len(distinctMoods) != 2 {
		t.Fatalf("expected 2 distinct moods, got %v", distinctMoods)
	}
}

func TestGardenRepository_NoLogs_ReturnsZeroAndEmpty(t *testing.T) {
	skipIfNoDB(t)
	ctx := context.Background()

	userID, err := createTestUser(testSupabaseURL, testSupabaseSecret)
	if err != nil {
		t.Fatalf("create dedicated test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "delete from auth.users where id = $1", userID)
	})

	repo := NewGardenRepository(testPool)

	total, err := repo.CountDistinctLoggedDays(ctx, userID)
	if err != nil {
		t.Fatalf("CountDistinctLoggedDays: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}

	moods, err := repo.DistinctMoods(ctx, userID)
	if err != nil {
		t.Fatalf("DistinctMoods: %v", err)
	}
	if len(moods) != 0 {
		t.Errorf("expected no moods, got %v", moods)
	}
}
