package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

// SymptomRepository handles database access for symptoms.
type SymptomRepository struct {
	db DB
}

func NewSymptomRepository(db DB) *SymptomRepository {
	return &SymptomRepository{db: db}
}

func scanSymptom(row rowScanner) (*model.Symptom, error) {
	var s model.Symptom
	if err := row.Scan(&s.ID, &s.UserID, &s.Name, &s.Category, &s.IsCustom); err != nil {
		return nil, err
	}
	return &s, nil
}

const symptomColumns = `id, user_id, name, category, is_custom`

// GetByID fetches a symptom regardless of ownership — callers decide what
// to do based on UserID (nil means it's a system preset).
func (r *SymptomRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Symptom, error) {
	q := `select ` + symptomColumns + ` from symptoms where id = $1`
	row := r.db.QueryRow(ctx, q, id)
	s, err := scanSymptom(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get symptom: %w", err)
	}
	return s, nil
}

// ListForUser returns system presets combined with the user's own custom
// tags, presets first.
func (r *SymptomRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]model.Symptom, error) {
	q := `select ` + symptomColumns + ` from symptoms where user_id is null or user_id = $1 order by is_custom, name`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list symptoms: %w", err)
	}
	defer rows.Close()

	symptoms := make([]model.Symptom, 0)
	for rows.Next() {
		s, err := scanSymptom(rows)
		if err != nil {
			return nil, fmt.Errorf("list symptoms: scan: %w", err)
		}
		symptoms = append(symptoms, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list symptoms: %w", err)
	}
	return symptoms, nil
}

// Create inserts a new custom symptom tag owned by symptom.UserID.
func (r *SymptomRepository) Create(ctx context.Context, symptom model.Symptom) (*model.Symptom, error) {
	q := `insert into symptoms (user_id, name, category, is_custom) values ($1, $2, $3, true) returning ` + symptomColumns
	row := r.db.QueryRow(ctx, q, symptom.UserID, symptom.Name, symptom.Category)
	s, err := scanSymptom(row)
	if err != nil {
		return nil, fmt.Errorf("create symptom: %w", err)
	}
	return s, nil
}

// Delete removes a custom tag. The user_id filter means this can never
// match a system preset (user_id is null) or another user's tag.
func (r *SymptomRepository) Delete(ctx context.Context, userID, symptomID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `delete from symptoms where id = $1 and user_id = $2`, symptomID, userID)
	if err != nil {
		return fmt.Errorf("delete symptom: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ExistsByName reports whether name (case-insensitive) is already used by a
// system preset or by userID's own tags.
func (r *SymptomRepository) ExistsByName(ctx context.Context, userID uuid.UUID, name string) (bool, error) {
	const q = `select exists(select 1 from symptoms where lower(name) = lower($1) and (user_id is null or user_id = $2))`
	var exists bool
	if err := r.db.QueryRow(ctx, q, name, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check symptom name: %w", err)
	}
	return exists, nil
}

// CountCustomForUser returns how many custom tags userID has created.
func (r *SymptomRepository) CountCustomForUser(ctx context.Context, userID uuid.UUID) (int, error) {
	const q = `select count(*) from symptoms where user_id = $1 and is_custom = true`
	var count int
	if err := r.db.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count custom symptoms: %w", err)
	}
	return count, nil
}

// ValidateIDs reports whether every id in symptomIDs exists and is either a
// system preset (user_id null) or owned by userID. Used by daily log
// upserts to make sure a user can't attach someone else's custom tag.
func (r *SymptomRepository) ValidateIDs(ctx context.Context, userID uuid.UUID, symptomIDs []uuid.UUID) (bool, error) {
	if len(symptomIDs) == 0 {
		return true, nil
	}

	idStrings := make([]string, len(symptomIDs))
	for i, id := range symptomIDs {
		idStrings[i] = id.String()
	}

	const q = `select count(*) from symptoms where id = any($1::uuid[]) and (user_id is null or user_id = $2)`
	var count int
	if err := r.db.QueryRow(ctx, q, idStrings, userID).Scan(&count); err != nil {
		return false, fmt.Errorf("validate symptom ids: %w", err)
	}
	return count == len(symptomIDs), nil
}
