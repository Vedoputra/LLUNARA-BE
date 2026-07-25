package repository

import (
	"context"
	"testing"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

func TestReminderRepository_UpsertByType_InsertsThenUpdatesSameRow(t *testing.T) {
	skipIfNoDB(t)
	repo := NewReminderRepository(testPool)
	ctx := context.Background()

	daysBefore := 2
	created, err := repo.UpsertByType(ctx, model.Reminder{
		UserID: testUserID, Type: model.ReminderPeriodUpcoming, IsEnabled: true, DaysBefore: &daysBefore,
	})
	if err != nil {
		t.Fatalf("UpsertByType (insert): %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	if created.DaysBefore == nil || *created.DaysBefore != 2 {
		t.Errorf("days_before = %v, want 2", created.DaysBefore)
	}

	newDaysBefore := 3
	updated, err := repo.UpsertByType(ctx, model.Reminder{
		UserID: testUserID, Type: model.ReminderPeriodUpcoming, IsEnabled: false, DaysBefore: &newDaysBefore,
	})
	if err != nil {
		t.Fatalf("UpsertByType (update): %v", err)
	}

	if updated.ID != created.ID {
		t.Errorf("expected the same row to be updated (id %v), got a different id %v", created.ID, updated.ID)
	}
	if updated.IsEnabled {
		t.Error("expected is_enabled to be updated to false")
	}
	if updated.DaysBefore == nil || *updated.DaysBefore != 3 {
		t.Errorf("days_before = %v, want 3", updated.DaysBefore)
	}

	list, err := repo.ListByUser(ctx, testUserID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	count := 0
	for _, r := range list {
		if r.Type == model.ReminderPeriodUpcoming {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row for type period_upcoming, got %d", count)
	}
}

func TestReminderRepository_UpsertByType_StoresAndReturnsTimeOfDay(t *testing.T) {
	skipIfNoDB(t)
	repo := NewReminderRepository(testPool)
	ctx := context.Background()

	timeOfDay := "09:00"
	created, err := repo.UpsertByType(ctx, model.Reminder{
		UserID: testUserID, Type: model.ReminderMedication, IsEnabled: true, TimeOfDay: &timeOfDay,
	})
	if err != nil {
		t.Fatalf("UpsertByType: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	if created.TimeOfDay == nil || *created.TimeOfDay != "09:00" {
		t.Errorf("time_of_day = %v, want 09:00", created.TimeOfDay)
	}
}

func TestReminderRepository_ListByUser_EmptyIsEmptySliceNotNil(t *testing.T) {
	skipIfNoDB(t)
	repo := NewReminderRepository(testPool)

	otherUserID, err := createTestUser(testSupabaseURL, testSupabaseSecret)
	if err != nil {
		t.Fatalf("create second test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "delete from auth.users where id = $1", otherUserID)
	})

	list, err := repo.ListByUser(context.Background(), otherUserID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if list == nil {
		t.Error("expected an empty slice, got nil")
	}
	if len(list) != 0 {
		t.Errorf("expected 0 reminders for a fresh user, got %d", len(list))
	}
}

func TestReminderRepository_Delete_CrossUserIsNotFound(t *testing.T) {
	skipIfNoDB(t)
	repo := NewReminderRepository(testPool)
	ctx := context.Background()

	created, err := repo.UpsertByType(ctx, model.Reminder{UserID: testUserID, Type: model.ReminderCheckup, IsEnabled: true})
	if err != nil {
		t.Fatalf("UpsertByType: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	otherUserID, err := createTestUser(testSupabaseURL, testSupabaseSecret)
	if err != nil {
		t.Fatalf("create second test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "delete from auth.users where id = $1", otherUserID)
	})

	if err := repo.Delete(ctx, otherUserID, created.ID); err != ErrNotFound {
		t.Errorf("expected ErrNotFound when deleting another user's reminder, got %v", err)
	}
}

func TestReminderRepository_Delete_RemovesRow(t *testing.T) {
	skipIfNoDB(t)
	repo := NewReminderRepository(testPool)
	ctx := context.Background()

	created, err := repo.UpsertByType(ctx, model.Reminder{UserID: testUserID, Type: model.ReminderFertileWindow, IsEnabled: true})
	if err != nil {
		t.Fatalf("UpsertByType: %v", err)
	}

	if err := repo.Delete(ctx, testUserID, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, testUserID, created.ID); err != ErrNotFound {
		t.Errorf("expected ErrNotFound deleting an already-deleted row, got %v", err)
	}
}
