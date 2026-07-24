package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

const maxNotesLength = 500

// dailyLogRepository is the subset of *repository.DailyLogRepository that
// DailyLogService depends on.
type dailyLogRepository interface {
	UpsertWithSymptoms(ctx context.Context, log model.DailyLog, symptomIDs []uuid.UUID) (*model.DailyLog, error)
	GetByDate(ctx context.Context, userID uuid.UUID, date time.Time) (*model.DailyLog, error)
	ListByRange(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.DailyLog, error)
	Delete(ctx context.Context, userID uuid.UUID, date time.Time) error
}

// symptomValidator is the subset of *repository.SymptomRepository that
// DailyLogService depends on, to check symptom_ids belong to the user (or
// are system presets) before attaching them to a log.
type symptomValidator interface {
	ValidateIDs(ctx context.Context, userID uuid.UUID, symptomIDs []uuid.UUID) (bool, error)
}

type DailyLogService struct {
	repo        dailyLogRepository
	cycleRepo   cycleRepository
	symptomRepo symptomValidator
}

func NewDailyLogService(repo dailyLogRepository, cycleRepo cycleRepository, symptomRepo symptomValidator) *DailyLogService {
	return &DailyLogService{repo: repo, cycleRepo: cycleRepo, symptomRepo: symptomRepo}
}

// UpsertLog validates and saves a day's log, automatically linking it to
// whichever cycle's date range covers it (if any).
func (s *DailyLogService) UpsertLog(ctx context.Context, userID uuid.UUID, req model.UpsertDailyLogRequest) (*model.DailyLog, error) {
	date, err := req.ParseDate()
	if err != nil {
		return nil, apierror.ValidationError("Format tanggal tidak valid", map[string]any{"date": "harus format YYYY-MM-DD"})
	}
	dateOnly := truncateToDate(date)
	today := truncateToDate(time.Now().UTC())
	if dateOnly.After(today) {
		return nil, apierror.ValidationError("Tanggal tidak boleh di masa depan", map[string]any{"date": "tidak boleh di masa depan"})
	}

	if req.Notes != nil && len(*req.Notes) > maxNotesLength {
		return nil, apierror.ValidationError("Catatan melebihi batas maksimum", map[string]any{"notes": fmt.Sprintf("maksimal %d karakter", maxNotesLength)})
	}

	symptomIDs := make([]uuid.UUID, 0, len(req.SymptomIDs))
	for _, raw := range req.SymptomIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, apierror.ValidationError("Format symptom_id tidak valid", map[string]any{"symptom_ids": "harus berupa UUID yang valid"})
		}
		symptomIDs = append(symptomIDs, id)
	}

	if len(symptomIDs) > 0 {
		valid, err := s.symptomRepo.ValidateIDs(ctx, userID, symptomIDs)
		if err != nil {
			return nil, fmt.Errorf("validate symptom ids: %w", err)
		}
		if !valid {
			return nil, apierror.ValidationError("Ada symptom_id yang tidak valid atau bukan milikmu", map[string]any{"symptom_ids": "tidak valid"})
		}
	}

	var cycleID *uuid.UUID
	matchingCycle, err := s.cycleRepo.FindOverlapping(ctx, userID, dateOnly)
	if err != nil {
		return nil, fmt.Errorf("find matching cycle: %w", err)
	}
	if matchingCycle != nil {
		cycleID = &matchingCycle.ID
	}

	var flowIntensity *model.FlowIntensity
	if req.FlowIntensity != nil {
		fi := model.FlowIntensity(*req.FlowIntensity)
		flowIntensity = &fi
	}

	log := model.DailyLog{
		UserID:        userID,
		CycleID:       cycleID,
		Date:          dateOnly,
		FlowIntensity: flowIntensity,
		Mood:          req.Mood,
		Notes:         req.Notes,
	}

	saved, err := s.repo.UpsertWithSymptoms(ctx, log, symptomIDs)
	if err != nil {
		return nil, fmt.Errorf("upsert daily log: %w", err)
	}
	return saved, nil
}

// ListLogs returns the user's logs within [from, to], inclusive.
func (s *DailyLogService) ListLogs(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.DailyLog, error) {
	logs, err := s.repo.ListByRange(ctx, userID, truncateToDate(from), truncateToDate(to))
	if err != nil {
		return nil, fmt.Errorf("list daily logs: %w", err)
	}
	return logs, nil
}

// DeleteLog removes the log for a given date, if any.
func (s *DailyLogService) DeleteLog(ctx context.Context, userID uuid.UUID, date time.Time) error {
	if err := s.repo.Delete(ctx, userID, truncateToDate(date)); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apierror.NotFound("Log tidak ditemukan")
		}
		return fmt.Errorf("delete daily log: %w", err)
	}
	return nil
}
