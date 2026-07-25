package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// ExportRepository handles the cross-table query behind CSV/PDF export —
// one row per logged day, with enough cycle context for the service to
// classify phase and cycle day, and every symptom name attached that day.
type ExportRepository struct {
	db DB
}

func NewExportRepository(db DB) *ExportRepository {
	return &ExportRepository{db: db}
}

// DailyLogExportRow is one day's log plus everything needed to describe it
// in an export: which cycle it fell in (for phase/day-of-cycle) and the
// names (not just ids) of every symptom logged that day.
type DailyLogExportRow struct {
	ID            uuid.UUID
	Date          time.Time
	Cycle         *model.Cycle
	FlowIntensity *string
	Mood          *string
	Notes         *string
	SymptomNames  []string
}

func (r *ExportRepository) ListDailyLogsForExport(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]DailyLogExportRow, error) {
	const q = `
		select dl.id, dl.date, dl.flow_intensity, dl.mood, dl.notes,
		       c.start_date, c.end_date, c.cycle_length, c.period_length
		from daily_logs dl
		left join cycles c on c.id = dl.cycle_id
		where dl.user_id = $1 and dl.date between $2 and $3
		order by dl.date`

	rows, err := r.db.Query(ctx, q, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list daily logs for export: %w", err)
	}
	defer rows.Close()

	logs := make([]DailyLogExportRow, 0)
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var row DailyLogExportRow
		var cycleStart, cycleEnd *time.Time
		var cycleLength, periodLength *int
		if err := rows.Scan(&row.ID, &row.Date, &row.FlowIntensity, &row.Mood, &row.Notes, &cycleStart, &cycleEnd, &cycleLength, &periodLength); err != nil {
			return nil, fmt.Errorf("list daily logs for export: scan: %w", err)
		}
		if cycleStart != nil {
			row.Cycle = &model.Cycle{StartDate: *cycleStart, EndDate: cycleEnd, CycleLength: cycleLength, PeriodLength: periodLength}
		}
		logs = append(logs, row)
		ids = append(ids, row.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list daily logs for export: %w", err)
	}

	names, err := r.symptomNamesForLogs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list daily logs for export: %w", err)
	}
	for i := range logs {
		logs[i].SymptomNames = names[logs[i].ID]
	}
	return logs, nil
}

func (r *ExportRepository) symptomNamesForLogs(ctx context.Context, logIDs []uuid.UUID) (map[uuid.UUID][]string, error) {
	result := make(map[uuid.UUID][]string)
	if len(logIDs) == 0 {
		return result, nil
	}

	idStrings := make([]string, len(logIDs))
	for i, id := range logIDs {
		idStrings[i] = id.String()
	}

	const q = `
		select dls.daily_log_id, s.name
		from daily_log_symptoms dls
		join symptoms s on s.id = dls.symptom_id
		where dls.daily_log_id = any($1::uuid[])
		order by s.name`

	rows, err := r.db.Query(ctx, q, idStrings)
	if err != nil {
		return nil, fmt.Errorf("symptom names for logs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var logID uuid.UUID
		var name string
		if err := rows.Scan(&logID, &name); err != nil {
			return nil, fmt.Errorf("symptom names for logs: scan: %w", err)
		}
		result[logID] = append(result[logID], name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("symptom names for logs: %w", err)
	}
	return result, nil
}
