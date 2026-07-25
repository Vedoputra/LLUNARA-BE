package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

func TestCycleRepository_CreateAndGetByID(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	created, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: start})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	if created.ID == uuid.Nil {
		t.Fatal("expected a generated ID")
	}
	if !created.StartDate.Equal(start) {
		t.Errorf("start date = %v, want %v", created.StartDate, start)
	}

	fetched, err := repo.GetByID(ctx, testUserID, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("fetched id = %v, want %v", fetched.ID, created.ID)
	}
}

func TestCycleRepository_GetByID_WrongUserIsNotFound(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	created, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	_, err = repo.GetByID(ctx, uuid.New(), created.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound when fetching another user's cycle, got %v", err)
	}
}

func TestCycleRepository_ListByUser_OrderedAndLimited(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	dates := []time.Time{
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	var ids []uuid.UUID
	for _, d := range dates {
		c, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: d})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		ids = append(ids, c.ID)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_ = repo.Delete(context.Background(), testUserID, id)
		}
	})

	list, err := repo.ListByUser(ctx, testUserID, 2)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 results (limit), got %d", len(list))
	}
	if !list[0].StartDate.After(list[1].StartDate) {
		t.Errorf("expected descending order by start_date, got %v then %v", list[0].StartDate, list[1].StartDate)
	}
}

func TestCycleRepository_GetLatest(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	older, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	newer, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.Delete(context.Background(), testUserID, older.ID)
		_ = repo.Delete(context.Background(), testUserID, newer.ID)
	})

	latest, err := repo.GetLatest(ctx, testUserID)
	if err != nil {
		t.Fatalf("GetLatest: %v", err)
	}
	if latest.ID != newer.ID {
		t.Errorf("expected latest cycle to be %v, got %v", newer.ID, latest.ID)
	}
}

func TestCycleRepository_FindOverlapping_BoundedByCycleLength(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	length := 28
	created, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: start, CycleLength: &length})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	// Day 3: well within the 28-day span.
	overlapping, err := repo.FindOverlapping(ctx, testUserID, time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FindOverlapping: %v", err)
	}
	if overlapping == nil || overlapping.ID != created.ID {
		t.Errorf("expected to find overlapping cycle %v, got %v", created.ID, overlapping)
	}

	// Exactly start_date + cycle_length: this is where the NEXT cycle
	// actually started, so it must NOT be considered part of this one —
	// otherwise every legitimate next-cycle start would false-positive.
	notOverlapping, err := repo.FindOverlapping(ctx, testUserID, start.AddDate(0, 0, length))
	if err != nil {
		t.Fatalf("FindOverlapping: %v", err)
	}
	if notOverlapping != nil {
		t.Errorf("expected no overlap exactly at start_date+cycle_length, got %+v", notOverlapping)
	}

	// Far beyond the span entirely.
	notOverlapping, err = repo.FindOverlapping(ctx, testUserID, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FindOverlapping: %v", err)
	}
	if notOverlapping != nil {
		t.Errorf("expected no overlap, got %+v", notOverlapping)
	}
}

func TestCycleRepository_FindOverlapping_OpenCycleHasNoBound(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	// A cycle with no cycle_length yet (the current latest, still open) —
	// even an end_date this far in the past doesn't close it; only a new
	// cycle starting does.
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	created, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: start, EndDate: &end})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	overlapping, err := repo.FindOverlapping(ctx, testUserID, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("FindOverlapping: %v", err)
	}
	if overlapping == nil || overlapping.ID != created.ID {
		t.Errorf("expected the still-open cycle to be treated as unbounded, got %v", overlapping)
	}
}

func TestCycleRepository_Update(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	created, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	end := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	periodLen := 5
	created.EndDate = &end
	created.PeriodLength = &periodLen

	updated, err := repo.Update(ctx, *created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.EndDate == nil || !updated.EndDate.Equal(end) {
		t.Errorf("end date = %v, want %v", updated.EndDate, end)
	}
	if updated.PeriodLength == nil || *updated.PeriodLength != periodLen {
		t.Errorf("period length = %v, want %d", updated.PeriodLength, periodLen)
	}
}

func TestCycleRepository_Update_WrongUserIsNotFound(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	created, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	attempt := *created
	attempt.UserID = uuid.New() // pretend to be a different user
	_, err = repo.Update(ctx, attempt)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound updating another user's cycle, got %v", err)
	}
}

func TestCycleRepository_Delete(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	created, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, testUserID, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.GetByID(ctx, testUserID, created.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCycleRepository_Delete_WrongUserIsNotFound(t *testing.T) {
	skipIfNoDB(t)
	repo := NewCycleRepository(testPool)
	ctx := context.Background()

	created, err := repo.Create(ctx, model.Cycle{UserID: testUserID, StartDate: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	err = repo.Delete(ctx, uuid.New(), created.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound deleting another user's cycle, got %v", err)
	}
}
