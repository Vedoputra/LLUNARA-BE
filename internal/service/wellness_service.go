package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
)

// wellnessRepository is the subset of *repository.WellnessRepository that
// WellnessService depends on.
type wellnessRepository interface {
	Upsert(ctx context.Context, log model.WellnessLog) (*model.WellnessLog, error)
	ListByRange(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.WellnessLog, error)
}

type WellnessService struct {
	repo wellnessRepository
}

func NewWellnessService(repo wellnessRepository) *WellnessService {
	return &WellnessService{repo: repo}
}

// UpsertLog saves a day's wellness metrics. Value ranges are validated at
// the request-decoding layer (validator tags on the DTO) — deliberately no
// server-side targets or "recommended" values here; the server only stores
// numbers, per BE-6.1's explicit instruction.
func (s *WellnessService) UpsertLog(ctx context.Context, userID uuid.UUID, req model.UpsertWellnessLogRequest) (*model.WellnessLog, error) {
	date, err := req.ParseDate()
	if err != nil {
		return nil, apierror.ValidationError("Format tanggal tidak valid", map[string]any{"date": "harus format YYYY-MM-DD"})
	}

	log := model.WellnessLog{
		UserID:       userID,
		Date:         truncateToDate(date),
		WaterGlasses: req.WaterGlasses,
		SleepHours:   req.SleepHours,
		WeightKg:     req.WeightKg,
	}

	saved, err := s.repo.Upsert(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("upsert wellness log: %w", err)
	}
	return saved, nil
}

func (s *WellnessService) ListLogs(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.WellnessLog, error) {
	logs, err := s.repo.ListByRange(ctx, userID, truncateToDate(from), truncateToDate(to))
	if err != nil {
		return nil, fmt.Errorf("list wellness logs: %w", err)
	}
	return logs, nil
}
