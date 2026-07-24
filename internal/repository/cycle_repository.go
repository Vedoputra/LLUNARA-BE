package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// ErrNotFound is returned when a query filtered by user_id matches no rows —
// either the row doesn't exist, or it belongs to a different user.
var ErrNotFound = errors.New("repository: not found")

// rowScanner is satisfied by both pgx.Row and pgx.Rows, letting a single
// scan function serve single-row and multi-row queries alike.
type rowScanner interface {
	Scan(dest ...any) error
}

// CycleRepository handles database access for cycles. It has no business
// logic — that belongs in service.CycleService.
type CycleRepository struct {
	db DB
}

func NewCycleRepository(db DB) *CycleRepository {
	return &CycleRepository{db: db}
}

func scanCycle(row rowScanner) (*model.Cycle, error) {
	var c model.Cycle
	err := row.Scan(
		&c.ID, &c.UserID, &c.StartDate, &c.EndDate,
		&c.CycleLength, &c.PeriodLength, &c.IsOutlier,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

const cycleColumns = `id, user_id, start_date, end_date, cycle_length, period_length, is_outlier, created_at, updated_at`

func (r *CycleRepository) Create(ctx context.Context, cycle model.Cycle) (*model.Cycle, error) {
	q := `insert into cycles (user_id, start_date, end_date, cycle_length, period_length, is_outlier)
		values ($1, $2, $3, $4, $5, $6)
		returning ` + cycleColumns

	row := r.db.QueryRow(ctx, q, cycle.UserID, cycle.StartDate, cycle.EndDate, cycle.CycleLength, cycle.PeriodLength, cycle.IsOutlier)
	c, err := scanCycle(row)
	if err != nil {
		return nil, fmt.Errorf("create cycle: %w", err)
	}
	return c, nil
}

func (r *CycleRepository) GetByID(ctx context.Context, userID, cycleID uuid.UUID) (*model.Cycle, error) {
	q := `select ` + cycleColumns + ` from cycles where user_id = $1 and id = $2`

	row := r.db.QueryRow(ctx, q, userID, cycleID)
	c, err := scanCycle(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get cycle: %w", err)
	}
	return c, nil
}

// ListByUser returns the user's cycle history, most recent first. limit is
// clamped to the range (0, 100] to prevent unbounded queries.
func (r *CycleRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]model.Cycle, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	q := `select ` + cycleColumns + ` from cycles where user_id = $1 order by start_date desc limit $2`

	rows, err := r.db.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list cycles: %w", err)
	}
	defer rows.Close()

	cycles := make([]model.Cycle, 0)
	for rows.Next() {
		c, err := scanCycle(rows)
		if err != nil {
			return nil, fmt.Errorf("list cycles: scan: %w", err)
		}
		cycles = append(cycles, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list cycles: %w", err)
	}
	return cycles, nil
}

func (r *CycleRepository) GetLatest(ctx context.Context, userID uuid.UUID) (*model.Cycle, error) {
	q := `select ` + cycleColumns + ` from cycles where user_id = $1 order by start_date desc limit 1`

	row := r.db.QueryRow(ctx, q, userID)
	c, err := scanCycle(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest cycle: %w", err)
	}
	return c, nil
}

// FindOverlapping returns the cycle whose [start_date, end_date] span
// contains startDate (an open cycle, end_date null, extends indefinitely),
// or nil if none overlaps.
func (r *CycleRepository) FindOverlapping(ctx context.Context, userID uuid.UUID, startDate time.Time) (*model.Cycle, error) {
	q := `select ` + cycleColumns + ` from cycles
		where user_id = $1 and start_date <= $2 and (end_date is null or end_date >= $2)
		limit 1`

	row := r.db.QueryRow(ctx, q, userID, startDate)
	c, err := scanCycle(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find overlapping cycle: %w", err)
	}
	return c, nil
}

func (r *CycleRepository) Update(ctx context.Context, cycle model.Cycle) (*model.Cycle, error) {
	q := `update cycles set end_date = $3, cycle_length = $4, period_length = $5, is_outlier = $6
		where user_id = $1 and id = $2
		returning ` + cycleColumns

	row := r.db.QueryRow(ctx, q, cycle.UserID, cycle.ID, cycle.EndDate, cycle.CycleLength, cycle.PeriodLength, cycle.IsOutlier)
	c, err := scanCycle(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update cycle: %w", err)
	}
	return c, nil
}

func (r *CycleRepository) Delete(ctx context.Context, userID, cycleID uuid.UUID) error {
	q := `delete from cycles where user_id = $1 and id = $2`

	tag, err := r.db.Exec(ctx, q, userID, cycleID)
	if err != nil {
		return fmt.Errorf("delete cycle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
