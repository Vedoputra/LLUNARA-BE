package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// ReminderRepository handles database access for reminder preferences.
type ReminderRepository struct {
	db DB
}

func NewReminderRepository(db DB) *ReminderRepository {
	return &ReminderRepository{db: db}
}

// reminderColumns formats time_of_day as "HH:MM" text directly in SQL —
// Postgres's `time` type doesn't scan into a Go string on its own, and
// "HH:MM" is what the API contract (and the datetime=15:04 validation on
// the request) expects back.
const reminderColumns = `id, user_id, type, is_enabled, to_char(time_of_day, 'HH24:MI'), days_before, custom_message, created_at, updated_at`

func scanReminder(row rowScanner) (*model.Reminder, error) {
	var rem model.Reminder
	var reminderType string
	err := row.Scan(
		&rem.ID, &rem.UserID, &reminderType, &rem.IsEnabled,
		&rem.TimeOfDay, &rem.DaysBefore, &rem.CustomMessage,
		&rem.CreatedAt, &rem.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	rem.Type = model.ReminderType(reminderType)
	return &rem, nil
}

// ListByUser returns every reminder preference the user has set.
func (r *ReminderRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Reminder, error) {
	q := `select ` + reminderColumns + ` from reminders where user_id = $1 order by created_at`

	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()

	reminders := make([]model.Reminder, 0)
	for rows.Next() {
		rem, err := scanReminder(rows)
		if err != nil {
			return nil, fmt.Errorf("list reminders: scan: %w", err)
		}
		reminders = append(reminders, *rem)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	return reminders, nil
}

func (r *ReminderRepository) getByUserAndType(ctx context.Context, userID uuid.UUID, t model.ReminderType) (*model.Reminder, error) {
	q := `select ` + reminderColumns + ` from reminders where user_id = $1 and type = $2 limit 1`

	row := r.db.QueryRow(ctx, q, userID, string(t))
	rem, err := scanReminder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reminder by type: %w", err)
	}
	return rem, nil
}

// UpsertByType saves rem as the single reminder row for (user_id, type).
// There's no unique constraint on (user_id, type) in the schema — the
// spec deliberately leaves room for `medication` to have multiple rows
// later — so this can't rely on `on conflict`. It looks up the existing
// row for that (user_id, type) pair first, then updates or inserts.
func (r *ReminderRepository) UpsertByType(ctx context.Context, rem model.Reminder) (*model.Reminder, error) {
	existing, err := r.getByUserAndType(ctx, rem.UserID, rem.Type)
	if err != nil {
		return nil, fmt.Errorf("upsert reminder: %w", err)
	}

	if existing != nil {
		q := `update reminders set is_enabled = $3, time_of_day = $4::time, days_before = $5, custom_message = $6
			where user_id = $1 and id = $2
			returning ` + reminderColumns

		row := r.db.QueryRow(ctx, q, rem.UserID, existing.ID, rem.IsEnabled, rem.TimeOfDay, rem.DaysBefore, rem.CustomMessage)
		saved, err := scanReminder(row)
		if err != nil {
			return nil, fmt.Errorf("upsert reminder: update: %w", err)
		}
		return saved, nil
	}

	q := `insert into reminders (user_id, type, is_enabled, time_of_day, days_before, custom_message)
		values ($1, $2, $3, $4::time, $5, $6)
		returning ` + reminderColumns

	row := r.db.QueryRow(ctx, q, rem.UserID, string(rem.Type), rem.IsEnabled, rem.TimeOfDay, rem.DaysBefore, rem.CustomMessage)
	saved, err := scanReminder(row)
	if err != nil {
		return nil, fmt.Errorf("upsert reminder: insert: %w", err)
	}
	return saved, nil
}

func (r *ReminderRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	q := `delete from reminders where user_id = $1 and id = $2`

	tag, err := r.db.Exec(ctx, q, userID, id)
	if err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
