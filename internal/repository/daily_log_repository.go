package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

const dailyLogColumns = `id, user_id, cycle_id, date, flow_intensity, mood, notes, created_at, updated_at`

// DailyLogRepository handles database access for daily logs. Upsert and
// ReplaceSymptoms exist standalone (as specced), but UpsertWithSymptoms is
// what the service actually calls — it runs both in one transaction so a
// failure on either side leaves nothing committed.
type DailyLogRepository struct {
	db   DB
	pool *pgxpool.Pool
}

func NewDailyLogRepository(pool *pgxpool.Pool) *DailyLogRepository {
	return &DailyLogRepository{db: pool, pool: pool}
}

func scanDailyLog(row rowScanner) (*model.DailyLog, error) {
	var d model.DailyLog
	var flow *string
	err := row.Scan(&d.ID, &d.UserID, &d.CycleID, &d.Date, &flow, &d.Mood, &d.Notes, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if flow != nil {
		fi := model.FlowIntensity(*flow)
		d.FlowIntensity = &fi
	}
	return &d, nil
}

func upsertDailyLog(ctx context.Context, db DB, log model.DailyLog) (*model.DailyLog, error) {
	var flow *string
	if log.FlowIntensity != nil {
		s := string(*log.FlowIntensity)
		flow = &s
	}

	q := `insert into daily_logs (user_id, cycle_id, date, flow_intensity, mood, notes)
		values ($1, $2, $3, $4, $5, $6)
		on conflict (user_id, date) do update set
			cycle_id = excluded.cycle_id,
			flow_intensity = excluded.flow_intensity,
			mood = excluded.mood,
			notes = excluded.notes
		returning ` + dailyLogColumns

	row := db.QueryRow(ctx, q, log.UserID, log.CycleID, log.Date, flow, log.Mood, log.Notes)
	return scanDailyLog(row)
}

func replaceSymptomsTx(ctx context.Context, db DB, logID uuid.UUID, symptomIDs []uuid.UUID) error {
	if _, err := db.Exec(ctx, `delete from daily_log_symptoms where daily_log_id = $1`, logID); err != nil {
		return fmt.Errorf("delete old symptom relations: %w", err)
	}
	for _, sid := range symptomIDs {
		if _, err := db.Exec(ctx, `insert into daily_log_symptoms (daily_log_id, symptom_id) values ($1, $2)`, logID, sid); err != nil {
			return fmt.Errorf("insert symptom relation: %w", err)
		}
	}
	return nil
}

func (r *DailyLogRepository) Upsert(ctx context.Context, log model.DailyLog) (*model.DailyLog, error) {
	saved, err := upsertDailyLog(ctx, r.db, log)
	if err != nil {
		return nil, fmt.Errorf("upsert daily log: %w", err)
	}
	return saved, nil
}

func (r *DailyLogRepository) ReplaceSymptoms(ctx context.Context, logID uuid.UUID, symptomIDs []uuid.UUID) error {
	if err := replaceSymptomsTx(ctx, r.db, logID, symptomIDs); err != nil {
		return fmt.Errorf("replace symptoms: %w", err)
	}
	return nil
}

// UpsertWithSymptoms saves the log and replaces its symptom associations
// inside a single transaction — if either step fails, both are rolled back.
func (r *DailyLogRepository) UpsertWithSymptoms(ctx context.Context, log model.DailyLog, symptomIDs []uuid.UUID) (*model.DailyLog, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	saved, err := upsertDailyLog(ctx, tx, log)
	if err != nil {
		return nil, fmt.Errorf("upsert daily log: %w", err)
	}

	if err := replaceSymptomsTx(ctx, tx, saved.ID, symptomIDs); err != nil {
		return nil, fmt.Errorf("replace symptoms: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	saved.SymptomIDs = symptomIDs
	return saved, nil
}

func (r *DailyLogRepository) GetByDate(ctx context.Context, userID uuid.UUID, date time.Time) (*model.DailyLog, error) {
	q := `select ` + dailyLogColumns + ` from daily_logs where user_id = $1 and date = $2`
	row := r.db.QueryRow(ctx, q, userID, date)
	d, err := scanDailyLog(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get daily log: %w", err)
	}

	symptomIDs, err := r.symptomIDsForLogs(ctx, []uuid.UUID{d.ID})
	if err != nil {
		return nil, fmt.Errorf("get daily log: %w", err)
	}
	d.SymptomIDs = symptomIDs[d.ID]
	return d, nil
}

func (r *DailyLogRepository) ListByRange(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.DailyLog, error) {
	q := `select ` + dailyLogColumns + ` from daily_logs where user_id = $1 and date between $2 and $3 order by date`
	rows, err := r.db.Query(ctx, q, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list daily logs: %w", err)
	}
	defer rows.Close()

	logs := make([]model.DailyLog, 0)
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		d, err := scanDailyLog(rows)
		if err != nil {
			return nil, fmt.Errorf("list daily logs: scan: %w", err)
		}
		logs = append(logs, *d)
		ids = append(ids, d.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list daily logs: %w", err)
	}

	symptomIDs, err := r.symptomIDsForLogs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list daily logs: %w", err)
	}
	for i := range logs {
		logs[i].SymptomIDs = symptomIDs[logs[i].ID]
	}
	return logs, nil
}

func (r *DailyLogRepository) symptomIDsForLogs(ctx context.Context, logIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	result := make(map[uuid.UUID][]uuid.UUID)
	if len(logIDs) == 0 {
		return result, nil
	}

	idStrings := make([]string, len(logIDs))
	for i, id := range logIDs {
		idStrings[i] = id.String()
	}

	rows, err := r.db.Query(ctx, `select daily_log_id, symptom_id from daily_log_symptoms where daily_log_id = any($1::uuid[])`, idStrings)
	if err != nil {
		return nil, fmt.Errorf("get symptom relations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var logID, symptomID uuid.UUID
		if err := rows.Scan(&logID, &symptomID); err != nil {
			return nil, fmt.Errorf("get symptom relations: scan: %w", err)
		}
		result[logID] = append(result[logID], symptomID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get symptom relations: %w", err)
	}
	return result, nil
}

func (r *DailyLogRepository) Delete(ctx context.Context, userID uuid.UUID, date time.Time) error {
	q := `delete from daily_logs where user_id = $1 and date = $2`
	tag, err := r.db.Exec(ctx, q, userID, date)
	if err != nil {
		return fmt.Errorf("delete daily log: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
