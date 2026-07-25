package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

type fakeReminderRepo struct {
	reminders     []model.Reminder
	upsertedWith  model.Reminder
	upsertCalled  bool
	deleteErr     error
	deleteCalledW struct {
		userID, id uuid.UUID
	}
}

func (f *fakeReminderRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]model.Reminder, error) {
	out := make([]model.Reminder, 0)
	for _, r := range f.reminders {
		if r.UserID == userID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeReminderRepo) UpsertByType(_ context.Context, rem model.Reminder) (*model.Reminder, error) {
	f.upsertCalled = true
	f.upsertedWith = rem
	rem.ID = uuid.New()
	return &rem, nil
}

func (f *fakeReminderRepo) Delete(_ context.Context, userID, id uuid.UUID) error {
	f.deleteCalledW.userID = userID
	f.deleteCalledW.id = id
	return f.deleteErr
}

func TestListReminders_FiltersByUser(t *testing.T) {
	otherUser := uuid.New()
	repo := &fakeReminderRepo{reminders: []model.Reminder{
		{UserID: testUUID, Type: model.ReminderPeriodUpcoming},
		{UserID: otherUser, Type: model.ReminderMedication},
	}}
	svc := NewReminderService(repo)

	reminders, err := svc.ListReminders(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("ListReminders: %v", err)
	}
	if len(reminders) != 1 {
		t.Fatalf("expected 1 reminder for testUUID, got %d", len(reminders))
	}
}

func TestListReminders_EmptyIsNotAnError(t *testing.T) {
	svc := NewReminderService(&fakeReminderRepo{})

	reminders, err := svc.ListReminders(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("ListReminders: %v", err)
	}
	if len(reminders) != 0 {
		t.Errorf("expected empty slice, got %v", reminders)
	}
}

func TestUpsertReminder_MedicationRequiresTimeOfDay(t *testing.T) {
	repo := &fakeReminderRepo{}
	svc := NewReminderService(repo)

	_, err := svc.UpsertReminder(context.Background(), testUUID, model.UpsertReminderRequest{
		Type:      string(model.ReminderMedication),
		IsEnabled: true,
	})
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
	if repo.upsertCalled {
		t.Error("expected repo.UpsertByType NOT to be called when validation fails")
	}
}

func TestUpsertReminder_MedicationWithTimeOfDaySucceeds(t *testing.T) {
	repo := &fakeReminderRepo{}
	svc := NewReminderService(repo)
	timeOfDay := "09:00"

	rem, err := svc.UpsertReminder(context.Background(), testUUID, model.UpsertReminderRequest{
		Type:      string(model.ReminderMedication),
		IsEnabled: true,
		TimeOfDay: &timeOfDay,
	})
	if err != nil {
		t.Fatalf("UpsertReminder: %v", err)
	}
	if rem.UserID != testUUID {
		t.Errorf("UserID = %v, want %v", rem.UserID, testUUID)
	}
	if rem.TimeOfDay == nil || *rem.TimeOfDay != "09:00" {
		t.Errorf("TimeOfDay = %v, want 09:00", rem.TimeOfDay)
	}
}

func TestUpsertReminder_NonMedicationDoesNotRequireTimeOfDay(t *testing.T) {
	repo := &fakeReminderRepo{}
	svc := NewReminderService(repo)
	daysBefore := 2

	rem, err := svc.UpsertReminder(context.Background(), testUUID, model.UpsertReminderRequest{
		Type:       string(model.ReminderPeriodUpcoming),
		IsEnabled:  true,
		DaysBefore: &daysBefore,
	})
	if err != nil {
		t.Fatalf("UpsertReminder: %v", err)
	}
	if rem.DaysBefore == nil || *rem.DaysBefore != 2 {
		t.Errorf("DaysBefore = %v, want 2", rem.DaysBefore)
	}
}

func TestUpsertReminder_PassesUserIDFromContextNotClient(t *testing.T) {
	repo := &fakeReminderRepo{}
	svc := NewReminderService(repo)

	_, err := svc.UpsertReminder(context.Background(), testUUID, model.UpsertReminderRequest{
		Type: string(model.ReminderCheckup),
	})
	if err != nil {
		t.Fatalf("UpsertReminder: %v", err)
	}
	if repo.upsertedWith.UserID != testUUID {
		t.Errorf("repo received UserID = %v, want %v", repo.upsertedWith.UserID, testUUID)
	}
}

func TestDeleteReminder_MapsNotFound(t *testing.T) {
	repo := &fakeReminderRepo{deleteErr: repository.ErrNotFound}
	svc := NewReminderService(repo)

	err := svc.DeleteReminder(context.Background(), testUUID, uuid.New())
	if code := apiErrCode(t, err); code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
}

func TestDeleteReminder_Success(t *testing.T) {
	repo := &fakeReminderRepo{}
	svc := NewReminderService(repo)
	id := uuid.New()

	if err := svc.DeleteReminder(context.Background(), testUUID, id); err != nil {
		t.Fatalf("DeleteReminder: %v", err)
	}
	if repo.deleteCalledW.userID != testUUID || repo.deleteCalledW.id != id {
		t.Errorf("Delete called with (%v, %v), want (%v, %v)", repo.deleteCalledW.userID, repo.deleteCalledW.id, testUUID, id)
	}
}
