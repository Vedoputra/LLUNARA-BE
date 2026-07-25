package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// WellnessRepository handles database access for wellness logs.
type WellnessRepository struct {
	db DB
}

func NewWellnessRepository(db DB) *WellnessRepository {
	return &WellnessRepository{db: db}
}

const wellnessColumns = `id, user_id, date, water_glasses, sleep_hours, weight_kg`

func scanWellnessLog(row rowScanner) (*model.WellnessLog, error) {
	var w model.WellnessLog
	if err := row.Scan(&w.ID, &w.UserID, &w.Date, &w.WaterGlasses, &w.SleepHours, &w.WeightKg); err != nil {
		return nil, err
	}
	return &w, nil
}

// Upsert saves a day's wellness metrics, merging rather than replacing: a
// field left nil in log means "not sent this time", not "clear it" — users
// log each metric independently through the day (water in the morning,
// sleep in the evening), so a later call must not wipe out an earlier one.
func (r *WellnessRepository) Upsert(ctx context.Context, log model.WellnessLog) (*model.WellnessLog, error) {
	q := `insert into wellness_logs (user_id, date, water_glasses, sleep_hours, weight_kg)
		values ($1, $2, $3, $4, $5)
		on conflict (user_id, date) do update set
			water_glasses = coalesce(excluded.water_glasses, wellness_logs.water_glasses),
			sleep_hours = coalesce(excluded.sleep_hours, wellness_logs.sleep_hours),
			weight_kg = coalesce(excluded.weight_kg, wellness_logs.weight_kg)
		returning ` + wellnessColumns

	row := r.db.QueryRow(ctx, q, log.UserID, log.Date, log.WaterGlasses, log.SleepHours, log.WeightKg)
	w, err := scanWellnessLog(row)
	if err != nil {
		return nil, fmt.Errorf("upsert wellness log: %w", err)
	}
	return w, nil
}

func (r *WellnessRepository) ListByRange(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.WellnessLog, error) {
	q := `select ` + wellnessColumns + ` from wellness_logs where user_id = $1 and date between $2 and $3 order by date`
	rows, err := r.db.Query(ctx, q, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list wellness logs: %w", err)
	}
	defer rows.Close()

	logs := make([]model.WellnessLog, 0)
	for rows.Next() {
		w, err := scanWellnessLog(rows)
		if err != nil {
			return nil, fmt.Errorf("list wellness logs: scan: %w", err)
		}
		logs = append(logs, *w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list wellness logs: %w", err)
	}
	return logs, nil
}
