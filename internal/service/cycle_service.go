// Package service contains business logic. It must not touch
// http.Request/http.ResponseWriter directly.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

const (
	minNormalCycleLength = 21
	maxNormalCycleLength = 45
	minNormalPeriodDays  = 1
	maxNormalPeriodDays  = 14
)

// cycleRepository is the subset of *repository.CycleRepository that
// CycleService depends on, so tests can supply an in-memory fake instead of
// hitting a real database.
type cycleRepository interface {
	Create(ctx context.Context, cycle model.Cycle) (*model.Cycle, error)
	GetByID(ctx context.Context, userID, cycleID uuid.UUID) (*model.Cycle, error)
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.Cycle, error)
	GetLatest(ctx context.Context, userID uuid.UUID) (*model.Cycle, error)
	FindOverlapping(ctx context.Context, userID uuid.UUID, startDate time.Time) (*model.Cycle, error)
	Update(ctx context.Context, cycle model.Cycle) (*model.Cycle, error)
	Delete(ctx context.Context, userID, cycleID uuid.UUID) error
}

type CycleService struct {
	repo cycleRepository
}

func NewCycleService(repo cycleRepository) *CycleService {
	return &CycleService{repo: repo}
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func daysBetween(a, b time.Time) int {
	return int(truncateToDate(b).Sub(truncateToDate(a)).Hours() / 24)
}

// ListCycles returns the user's cycle history, most recent first.
func (s *CycleService) ListCycles(ctx context.Context, userID uuid.UUID) ([]model.Cycle, error) {
	cycles, err := s.repo.ListByUser(ctx, userID, 100)
	if err != nil {
		return nil, fmt.Errorf("list cycles: %w", err)
	}
	return cycles, nil
}

// StartCycle records a new period start date. If the user has a previous
// cycle that hasn't been closed yet (its cycle_length is still unknown),
// starting a new one is what closes it: its cycle_length becomes the gap to
// this new start date, and it's flagged as an outlier if that gap falls
// outside the normal 21–45 day range.
func (s *CycleService) StartCycle(ctx context.Context, userID uuid.UUID, startDate time.Time) (*model.Cycle, error) {
	startDateOnly := truncateToDate(startDate)
	today := truncateToDate(time.Now().UTC())
	if startDateOnly.After(today) {
		return nil, apierror.ValidationError("Tanggal mulai tidak boleh di masa depan", map[string]any{"start_date": "tidak boleh di masa depan"})
	}

	latest, err := s.repo.GetLatest(ctx, userID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("get latest cycle: %w", err)
	}
	if errors.Is(err, repository.ErrNotFound) {
		latest = nil
	}

	if latest != nil {
		// A new cycle must start strictly after the latest recorded one,
		// and strictly after its recorded period end (if any).
		if !startDateOnly.After(latest.StartDate) {
			return nil, apierror.CycleOverlap("", map[string]any{"conflicting_cycle_id": latest.ID.String()})
		}
		if latest.EndDate != nil && !startDateOnly.After(*latest.EndDate) {
			return nil, apierror.CycleOverlap("", map[string]any{"conflicting_cycle_id": latest.ID.String()})
		}
	}

	// Guard against overlapping any older, already-closed cycle too (e.g. a
	// duplicate or out-of-order historical entry). The latest cycle is
	// exempted here because it was already vetted, precisely, above.
	overlapping, err := s.repo.FindOverlapping(ctx, userID, startDateOnly)
	if err != nil {
		return nil, fmt.Errorf("check overlap: %w", err)
	}
	if overlapping != nil && (latest == nil || overlapping.ID != latest.ID) {
		return nil, apierror.CycleOverlap("", map[string]any{"conflicting_cycle_id": overlapping.ID.String()})
	}

	if latest != nil && latest.CycleLength == nil {
		length := daysBetween(latest.StartDate, startDateOnly)
		isOutlier := length < minNormalCycleLength || length > maxNormalCycleLength
		latest.CycleLength = &length
		latest.IsOutlier = isOutlier
		if _, err := s.repo.Update(ctx, *latest); err != nil {
			return nil, fmt.Errorf("close previous cycle: %w", err)
		}
	}

	created, err := s.repo.Create(ctx, model.Cycle{UserID: userID, StartDate: startDateOnly})
	if err != nil {
		return nil, fmt.Errorf("create cycle: %w", err)
	}
	return created, nil
}

// EndCycle records when a period ended and computes its period_length. A
// duration outside the normal 1–14 day range is logged but still allowed —
// this is observational tracking, not a gate on what the user can record.
func (s *CycleService) EndCycle(ctx context.Context, userID, cycleID uuid.UUID, endDate time.Time) (*model.Cycle, error) {
	cycle, err := s.repo.GetByID(ctx, userID, cycleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apierror.NotFound("Siklus tidak ditemukan")
		}
		return nil, fmt.Errorf("get cycle: %w", err)
	}

	endDateOnly := truncateToDate(endDate)
	if endDateOnly.Before(cycle.StartDate) {
		return nil, apierror.ValidationError("Tanggal selesai tidak boleh sebelum tanggal mulai", map[string]any{"end_date": "sebelum tanggal mulai"})
	}

	periodLength := daysBetween(cycle.StartDate, endDateOnly) + 1
	if periodLength < minNormalPeriodDays || periodLength > maxNormalPeriodDays {
		slog.Warn("cycle service: period length outside typical range", "user_id", userID, "cycle_id", cycleID, "period_length", periodLength)
	}

	cycle.EndDate = &endDateOnly
	cycle.PeriodLength = &periodLength

	updated, err := s.repo.Update(ctx, *cycle)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apierror.NotFound("Siklus tidak ditemukan")
		}
		return nil, fmt.Errorf("update cycle: %w", err)
	}
	return updated, nil
}

// DeleteCycle removes a cycle and, if it had an older neighbor, recomputes
// that neighbor's cycle_length so it still reflects the gap to whatever is
// now the next recorded cycle — or reopens it (cycle_length back to nil) if
// the deleted cycle was the most recent one.
func (s *CycleService) DeleteCycle(ctx context.Context, userID, cycleID uuid.UUID) error {
	cycle, err := s.repo.GetByID(ctx, userID, cycleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apierror.NotFound("Siklus tidak ditemukan")
		}
		return fmt.Errorf("get cycle: %w", err)
	}

	all, err := s.repo.ListByUser(ctx, userID, 100)
	if err != nil {
		return fmt.Errorf("list cycles: %w", err)
	}

	// all is ordered start_date desc, so the entry right before the target
	// (lower index) is chronologically newer, and right after (higher
	// index) is chronologically older.
	var newerNeighbor, olderNeighbor *model.Cycle
	for i, c := range all {
		if c.ID != cycle.ID {
			continue
		}
		if i > 0 {
			n := all[i-1]
			newerNeighbor = &n
		}
		if i+1 < len(all) {
			o := all[i+1]
			olderNeighbor = &o
		}
		break
	}

	if err := s.repo.Delete(ctx, userID, cycleID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apierror.NotFound("Siklus tidak ditemukan")
		}
		return fmt.Errorf("delete cycle: %w", err)
	}

	if olderNeighbor == nil {
		return nil
	}

	if newerNeighbor != nil {
		length := daysBetween(olderNeighbor.StartDate, newerNeighbor.StartDate)
		isOutlier := length < minNormalCycleLength || length > maxNormalCycleLength
		olderNeighbor.CycleLength = &length
		olderNeighbor.IsOutlier = isOutlier
	} else {
		olderNeighbor.CycleLength = nil
		olderNeighbor.IsOutlier = false
	}

	if _, err := s.repo.Update(ctx, *olderNeighbor); err != nil {
		return fmt.Errorf("recalculate neighboring cycle: %w", err)
	}
	return nil
}
