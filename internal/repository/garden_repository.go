package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GardenRepository provides the aggregate queries behind Taman Luna
// (GET /api/v1/garden) — everything is derived from daily_logs, there is no
// dedicated gamification table (see docs/FEATURE_REMINDERS_AND_GARDEN.md).
type GardenRepository struct {
	db DB
}

func NewGardenRepository(db DB) *GardenRepository {
	return &GardenRepository{db: db}
}

// CountDistinctLoggedDays counts every unique date userID has a daily log
// for, across all time.
func (r *GardenRepository) CountDistinctLoggedDays(ctx context.Context, userID uuid.UUID) (int, error) {
	const q = `select count(distinct date) from daily_logs where user_id = $1`

	var count int
	if err := r.db.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count distinct logged days: %w", err)
	}
	return count, nil
}

// CountDistinctLoggedDaysInRange counts unique logged dates within [from, to].
func (r *GardenRepository) CountDistinctLoggedDaysInRange(ctx context.Context, userID uuid.UUID, from, to time.Time) (int, error) {
	const q = `select count(distinct date) from daily_logs where user_id = $1 and date between $2 and $3`

	var count int
	if err := r.db.QueryRow(ctx, q, userID, from, to).Scan(&count); err != nil {
		return 0, fmt.Errorf("count distinct logged days in range: %w", err)
	}
	return count, nil
}

// DistinctMoods returns every distinct non-null mood value userID has ever
// logged, alphabetically.
func (r *GardenRepository) DistinctMoods(ctx context.Context, userID uuid.UUID) ([]string, error) {
	const q = `select distinct mood from daily_logs where user_id = $1 and mood is not null order by mood`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("distinct moods: %w", err)
	}
	defer rows.Close()

	moods := make([]string, 0)
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, fmt.Errorf("distinct moods: scan: %w", err)
		}
		moods = append(moods, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("distinct moods: %w", err)
	}
	return moods, nil
}
