package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// gardenRepository is the subset of *repository.GardenRepository that
// GardenService depends on.
type gardenRepository interface {
	CountDistinctLoggedDays(ctx context.Context, userID uuid.UUID) (int, error)
	CountDistinctLoggedDaysInRange(ctx context.Context, userID uuid.UUID, from, to time.Time) (int, error)
	DistinctMoods(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// gardenMessage is Luna's line on every response. Positive-only per PRD 4.5
// / DESIGN.md — it must never reference missed days or a broken streak, so
// it's kept static rather than computed from any absence-measuring data.
const gardenMessage = "Setiap hari kecil berarti. Tidak apa-apa kalau ada yang terlewat."

type GardenService struct {
	repo gardenRepository
}

func NewGardenService(repo gardenRepository) *GardenService {
	return &GardenService{repo: repo}
}

// GetGarden aggregates everything Taman Luna shows, entirely derived from
// daily_logs — see docs/FEATURE_REMINDERS_AND_GARDEN.md 2.2 for why there's
// no dedicated gamification table. Always succeeds (never an error) for a
// user with zero logs — same pattern as GetPrediction: an empty garden is a
// valid state, not a failure.
func (s *GardenService) GetGarden(ctx context.Context, userID uuid.UUID) (*model.Garden, error) {
	total, err := s.repo.CountDistinctLoggedDays(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count total logged days: %w", err)
	}

	today := truncateToDate(time.Now().UTC())
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	thisMonth, err := s.repo.CountDistinctLoggedDaysInRange(ctx, userID, monthStart, today)
	if err != nil {
		return nil, fmt.Errorf("count logged days this month: %w", err)
	}

	weekStart := today.AddDate(0, 0, -6)
	thisWeek, err := s.repo.CountDistinctLoggedDaysInRange(ctx, userID, weekStart, today)
	if err != nil {
		return nil, fmt.Errorf("count logged days this week: %w", err)
	}

	collected, err := s.repo.DistinctMoods(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("distinct moods: %w", err)
	}

	return &model.Garden{
		TotalLoggedDays:     total,
		LoggedDaysThisMonth: thisMonth,
		NewThisWeek:         thisWeek,
		CollectedMoods:      collected,
		UncollectedMoods:    uncollectedMoods(collected),
		Message:             gardenMessage,
	}, nil
}

// uncollectedMoods returns every mood preset not present in collected, in
// preset order — the FE renders these as not-yet-collected sticker
// placeholders.
func uncollectedMoods(collected []string) []string {
	collectedSet := make(map[string]bool, len(collected))
	for _, m := range collected {
		collectedSet[m] = true
	}

	uncollected := make([]string, 0, len(model.MoodPresets))
	for _, preset := range model.MoodPresets {
		if !collectedSet[preset] {
			uncollected = append(uncollected, preset)
		}
	}
	return uncollected
}
