package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

// reminderRepository is the subset of *repository.ReminderRepository that
// ReminderService depends on.
type reminderRepository interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Reminder, error)
	UpsertByType(ctx context.Context, rem model.Reminder) (*model.Reminder, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type ReminderService struct {
	repo reminderRepository
}

func NewReminderService(repo reminderRepository) *ReminderService {
	return &ReminderService{repo: repo}
}

func (s *ReminderService) ListReminders(ctx context.Context, userID uuid.UUID) ([]model.Reminder, error) {
	reminders, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	return reminders, nil
}

// UpsertReminder saves one reminder preference, one row per type (per
// docs/FEATURE_REMINDERS_AND_GARDEN.md 1.4 — v1 keeps this simple, no
// multi-row `medication` support). The only business rule enforced here:
// `medication` needs a time_of_day, since without one the FE has nothing to
// schedule the local notification against.
func (s *ReminderService) UpsertReminder(ctx context.Context, userID uuid.UUID, req model.UpsertReminderRequest) (*model.Reminder, error) {
	reminderType := model.ReminderType(req.Type)

	if reminderType == model.ReminderMedication && req.TimeOfDay == nil {
		return nil, apierror.ValidationError("Reminder obat butuh jam pengingat", map[string]any{"time_of_day": "wajib diisi untuk tipe medication"})
	}

	rem := model.Reminder{
		UserID:        userID,
		Type:          reminderType,
		IsEnabled:     req.IsEnabled,
		TimeOfDay:     req.TimeOfDay,
		DaysBefore:    req.DaysBefore,
		CustomMessage: req.CustomMessage,
	}

	saved, err := s.repo.UpsertByType(ctx, rem)
	if err != nil {
		return nil, fmt.Errorf("upsert reminder: %w", err)
	}
	return saved, nil
}

func (s *ReminderService) DeleteReminder(ctx context.Context, userID, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, userID, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apierror.NotFound("Reminder tidak ditemukan")
		}
		return fmt.Errorf("delete reminder: %w", err)
	}
	return nil
}
