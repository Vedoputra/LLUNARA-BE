package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// InsightRepository handles the cross-table queries behind FASE 5's
// insight endpoints. Phase classification is business logic (it depends on
// average cycle/period length, which cycle a log fell in, etc.) and stays
// in the service layer — this repository only joins and filters by date
// range, keeping the expensive part (bounding the row count) in SQL.
type InsightRepository struct {
	db DB
}

func NewInsightRepository(db DB) *InsightRepository {
	return &InsightRepository{db: db}
}

// SymptomOccurrence is one (symptom, day) pair the user logged, with
// enough cycle context for the service to classify its phase and cycle
// day. Cycle is nil when the log wasn't linked to any recorded cycle.
type SymptomOccurrence struct {
	SymptomID uuid.UUID
	Name      string
	Date      time.Time
	Cycle     *model.Cycle
}

// ListSymptomOccurrences returns every symptom logged by userID within
// [from, to], joined with whatever cycle each log belonged to.
func (r *InsightRepository) ListSymptomOccurrences(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]SymptomOccurrence, error) {
	const q = `
		select s.id, s.name, dl.date,
		       c.start_date, c.end_date, c.cycle_length, c.period_length
		from daily_log_symptoms dls
		join daily_logs dl on dl.id = dls.daily_log_id
		join symptoms s on s.id = dls.symptom_id
		left join cycles c on c.id = dl.cycle_id
		where dl.user_id = $1 and dl.date between $2 and $3
		order by dl.date`

	rows, err := r.db.Query(ctx, q, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list symptom occurrences: %w", err)
	}
	defer rows.Close()

	occurrences := make([]SymptomOccurrence, 0)
	for rows.Next() {
		var o SymptomOccurrence
		var cycleStart, cycleEnd *time.Time
		var cycleLength, periodLength *int

		if err := rows.Scan(&o.SymptomID, &o.Name, &o.Date, &cycleStart, &cycleEnd, &cycleLength, &periodLength); err != nil {
			return nil, fmt.Errorf("list symptom occurrences: scan: %w", err)
		}
		if cycleStart != nil {
			o.Cycle = &model.Cycle{
				StartDate:    *cycleStart,
				EndDate:      cycleEnd,
				CycleLength:  cycleLength,
				PeriodLength: periodLength,
			}
		}
		occurrences = append(occurrences, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list symptom occurrences: %w", err)
	}
	return occurrences, nil
}

// CountLoggedDays returns how many distinct days the user logged anything
// within [from, to] — used as the shared sample_size denominator for
// symptom insights.
func (r *InsightRepository) CountLoggedDays(ctx context.Context, userID uuid.UUID, from, to time.Time) (int, error) {
	const q = `select count(*) from daily_logs where user_id = $1 and date between $2 and $3`
	var count int
	if err := r.db.QueryRow(ctx, q, userID, from, to).Scan(&count); err != nil {
		return 0, fmt.Errorf("count logged days: %w", err)
	}
	return count, nil
}

// MoodOccurrence is one day's mood, with enough cycle context for the
// service to classify its phase.
type MoodOccurrence struct {
	Mood  string
	Date  time.Time
	Cycle *model.Cycle
}

// ListMoodOccurrences returns every mood logged by userID within
// [from, to], joined with whatever cycle each log belonged to.
func (r *InsightRepository) ListMoodOccurrences(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]MoodOccurrence, error) {
	const q = `
		select dl.mood, dl.date,
		       c.start_date, c.end_date, c.cycle_length, c.period_length
		from daily_logs dl
		left join cycles c on c.id = dl.cycle_id
		where dl.user_id = $1 and dl.date between $2 and $3 and dl.mood is not null`

	rows, err := r.db.Query(ctx, q, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list mood occurrences: %w", err)
	}
	defer rows.Close()

	occurrences := make([]MoodOccurrence, 0)
	for rows.Next() {
		var o MoodOccurrence
		var cycleStart, cycleEnd *time.Time
		var cycleLength, periodLength *int

		if err := rows.Scan(&o.Mood, &o.Date, &cycleStart, &cycleEnd, &cycleLength, &periodLength); err != nil {
			return nil, fmt.Errorf("list mood occurrences: scan: %w", err)
		}
		if cycleStart != nil {
			o.Cycle = &model.Cycle{
				StartDate:    *cycleStart,
				EndDate:      cycleEnd,
				CycleLength:  cycleLength,
				PeriodLength: periodLength,
			}
		}
		occurrences = append(occurrences, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list mood occurrences: %w", err)
	}
	return occurrences, nil
}
