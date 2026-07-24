package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SymptomRepository handles database access for symptoms.
type SymptomRepository struct {
	db DB
}

func NewSymptomRepository(db DB) *SymptomRepository {
	return &SymptomRepository{db: db}
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
