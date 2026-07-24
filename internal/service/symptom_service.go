package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/pkg/apierror"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

const maxCustomSymptomsPerUser = 30

// symptomRepository is the subset of *repository.SymptomRepository that
// SymptomService depends on.
type symptomRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Symptom, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]model.Symptom, error)
	Create(ctx context.Context, symptom model.Symptom) (*model.Symptom, error)
	Delete(ctx context.Context, userID, symptomID uuid.UUID) error
	ExistsByName(ctx context.Context, userID uuid.UUID, name string) (bool, error)
	CountCustomForUser(ctx context.Context, userID uuid.UUID) (int, error)
}

type SymptomService struct {
	repo symptomRepository
}

func NewSymptomService(repo symptomRepository) *SymptomService {
	return &SymptomService{repo: repo}
}

// ListSymptoms returns system presets combined with the user's own custom
// tags.
func (s *SymptomService) ListSymptoms(ctx context.Context, userID uuid.UUID) ([]model.Symptom, error) {
	symptoms, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list symptoms: %w", err)
	}
	return symptoms, nil
}

// CreateSymptom adds a custom tag for the user, rejecting a name that
// already exists (case-insensitive, checked against presets and the user's
// own tags) and enforcing the 30-tag-per-user cap.
func (s *SymptomService) CreateSymptom(ctx context.Context, userID uuid.UUID, req model.CreateSymptomRequest) (*model.Symptom, error) {
	exists, err := s.repo.ExistsByName(ctx, userID, req.Name)
	if err != nil {
		return nil, fmt.Errorf("check existing symptom name: %w", err)
	}
	if exists {
		return nil, apierror.ValidationError("Nama gejala sudah ada", map[string]any{"name": "sudah digunakan"})
	}

	count, err := s.repo.CountCustomForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count custom symptoms: %w", err)
	}
	if count >= maxCustomSymptomsPerUser {
		return nil, apierror.ValidationError("Batas tag gejala kustom tercapai", map[string]any{"symptom": fmt.Sprintf("maksimal %d tag kustom", maxCustomSymptomsPerUser)})
	}

	created, err := s.repo.Create(ctx, model.Symptom{UserID: &userID, Name: req.Name, Category: req.Category, IsCustom: true})
	if err != nil {
		return nil, fmt.Errorf("create symptom: %w", err)
	}
	return created, nil
}

// DeleteSymptom removes a user's custom tag. Deleting a system preset is
// rejected with FORBIDDEN; deleting another user's tag looks like
// NOT_FOUND, same as any other cross-user access attempt.
func (s *SymptomService) DeleteSymptom(ctx context.Context, userID, symptomID uuid.UUID) error {
	symptom, err := s.repo.GetByID(ctx, symptomID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apierror.NotFound("Gejala tidak ditemukan")
		}
		return fmt.Errorf("get symptom: %w", err)
	}

	if symptom.UserID == nil {
		return apierror.Forbidden("Preset sistem tidak dapat dihapus")
	}
	if *symptom.UserID != userID {
		return apierror.NotFound("Gejala tidak ditemukan")
	}

	if err := s.repo.Delete(ctx, userID, symptomID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apierror.NotFound("Gejala tidak ditemukan")
		}
		return fmt.Errorf("delete symptom: %w", err)
	}
	return nil
}
