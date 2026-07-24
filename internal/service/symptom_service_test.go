package service

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
	"github.com/Vedoputra/LLUNARA-BE/internal/repository"
)

type fakeSymptomRepo struct {
	symptoms map[uuid.UUID]model.Symptom
}

func newFakeSymptomRepo(symptoms ...model.Symptom) *fakeSymptomRepo {
	m := make(map[uuid.UUID]model.Symptom)
	for _, s := range symptoms {
		m[s.ID] = s
	}
	return &fakeSymptomRepo{symptoms: m}
}

func (f *fakeSymptomRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Symptom, error) {
	s, ok := f.symptoms[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	ss := s
	return &ss, nil
}

func (f *fakeSymptomRepo) ListForUser(_ context.Context, userID uuid.UUID) ([]model.Symptom, error) {
	list := make([]model.Symptom, 0)
	for _, s := range f.symptoms {
		if s.UserID == nil || *s.UserID == userID {
			list = append(list, s)
		}
	}
	return list, nil
}

func (f *fakeSymptomRepo) Create(_ context.Context, symptom model.Symptom) (*model.Symptom, error) {
	symptom.ID = uuid.New()
	f.symptoms[symptom.ID] = symptom
	s := symptom
	return &s, nil
}

func (f *fakeSymptomRepo) Delete(_ context.Context, userID, symptomID uuid.UUID) error {
	s, ok := f.symptoms[symptomID]
	if !ok || s.UserID == nil || *s.UserID != userID {
		return repository.ErrNotFound
	}
	delete(f.symptoms, symptomID)
	return nil
}

func (f *fakeSymptomRepo) ExistsByName(_ context.Context, userID uuid.UUID, name string) (bool, error) {
	for _, s := range f.symptoms {
		if (s.UserID == nil || *s.UserID == userID) && strings.EqualFold(s.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeSymptomRepo) CountCustomForUser(_ context.Context, userID uuid.UUID) (int, error) {
	count := 0
	for _, s := range f.symptoms {
		if s.IsCustom && s.UserID != nil && *s.UserID == userID {
			count++
		}
	}
	return count, nil
}

func TestListSymptoms_IncludesPresetsAndOwnTags(t *testing.T) {
	userID := uuid.New()
	otherUserID := uuid.New()
	preset := model.Symptom{ID: uuid.New(), Name: "kram", Category: "physical"}
	own := model.Symptom{ID: uuid.New(), UserID: &userID, Name: "my-tag", Category: "other", IsCustom: true}
	other := model.Symptom{ID: uuid.New(), UserID: &otherUserID, Name: "their-tag", Category: "other", IsCustom: true}

	repo := newFakeSymptomRepo(preset, own, other)
	svc := NewSymptomService(repo)

	list, err := svc.ListSymptoms(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListSymptoms: %v", err)
	}

	var foundPreset, foundOwn, foundOther bool
	for _, s := range list {
		switch s.ID {
		case preset.ID:
			foundPreset = true
		case own.ID:
			foundOwn = true
		case other.ID:
			foundOther = true
		}
	}
	if !foundPreset || !foundOwn {
		t.Errorf("expected preset and own tag in list: preset=%v own=%v", foundPreset, foundOwn)
	}
	if foundOther {
		t.Error("expected another user's tag to NOT appear in the list")
	}
}

func TestCreateSymptom_RejectsDuplicateNameCaseInsensitive(t *testing.T) {
	userID := uuid.New()
	repo := newFakeSymptomRepo(model.Symptom{ID: uuid.New(), Name: "kram", Category: "physical"}) // preset
	svc := NewSymptomService(repo)

	_, err := svc.CreateSymptom(context.Background(), userID, model.CreateSymptomRequest{Name: "KRAM", Category: "physical"})
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestCreateSymptom_RejectsDuplicateAgainstOwnTag(t *testing.T) {
	userID := uuid.New()
	repo := newFakeSymptomRepo(model.Symptom{ID: uuid.New(), UserID: &userID, Name: "my custom tag", Category: "other", IsCustom: true})
	svc := NewSymptomService(repo)

	_, err := svc.CreateSymptom(context.Background(), userID, model.CreateSymptomRequest{Name: "My Custom Tag", Category: "other"})
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestCreateSymptom_EnforcesMaxCustomTags(t *testing.T) {
	userID := uuid.New()
	symptoms := make([]model.Symptom, 0, maxCustomSymptomsPerUser)
	for i := 0; i < maxCustomSymptomsPerUser; i++ {
		symptoms = append(symptoms, model.Symptom{ID: uuid.New(), UserID: &userID, Name: uuid.New().String(), Category: "other", IsCustom: true})
	}
	repo := newFakeSymptomRepo(symptoms...)
	svc := NewSymptomService(repo)

	_, err := svc.CreateSymptom(context.Background(), userID, model.CreateSymptomRequest{Name: "one-too-many", Category: "other"})
	if code := apiErrCode(t, err); code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", code)
	}
}

func TestCreateSymptom_Success(t *testing.T) {
	userID := uuid.New()
	repo := newFakeSymptomRepo()
	svc := NewSymptomService(repo)

	created, err := svc.CreateSymptom(context.Background(), userID, model.CreateSymptomRequest{Name: "unique-tag", Category: "physical"})
	if err != nil {
		t.Fatalf("CreateSymptom: %v", err)
	}
	if !created.IsCustom {
		t.Error("expected is_custom = true")
	}
	if created.UserID == nil || *created.UserID != userID {
		t.Errorf("user_id = %v, want %v", created.UserID, userID)
	}
}

func TestDeleteSymptom_RejectsDeletingPreset(t *testing.T) {
	userID := uuid.New()
	preset := model.Symptom{ID: uuid.New(), Name: "kram", Category: "physical"} // UserID nil
	repo := newFakeSymptomRepo(preset)
	svc := NewSymptomService(repo)

	err := svc.DeleteSymptom(context.Background(), userID, preset.ID)
	if code := apiErrCode(t, err); code != "FORBIDDEN" {
		t.Errorf("code = %q, want FORBIDDEN", code)
	}
	if _, err := repo.GetByID(context.Background(), preset.ID); err != nil {
		t.Error("preset should NOT have been deleted")
	}
}

func TestDeleteSymptom_RejectsDeletingOtherUsersTag(t *testing.T) {
	ownerID := uuid.New()
	attackerID := uuid.New()
	tag := model.Symptom{ID: uuid.New(), UserID: &ownerID, Name: "owner-tag", Category: "other", IsCustom: true}
	repo := newFakeSymptomRepo(tag)
	svc := NewSymptomService(repo)

	err := svc.DeleteSymptom(context.Background(), attackerID, tag.ID)
	if code := apiErrCode(t, err); code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
	if _, err := repo.GetByID(context.Background(), tag.ID); err != nil {
		t.Error("tag should NOT have been deleted by a non-owner")
	}
}

func TestDeleteSymptom_Success(t *testing.T) {
	userID := uuid.New()
	tag := model.Symptom{ID: uuid.New(), UserID: &userID, Name: "my-tag", Category: "other", IsCustom: true}
	repo := newFakeSymptomRepo(tag)
	svc := NewSymptomService(repo)

	if err := svc.DeleteSymptom(context.Background(), userID, tag.ID); err != nil {
		t.Fatalf("DeleteSymptom: %v", err)
	}
	if _, err := repo.GetByID(context.Background(), tag.ID); err == nil {
		t.Error("expected tag to be deleted")
	}
}

func TestDeleteSymptom_NotFound(t *testing.T) {
	repo := newFakeSymptomRepo()
	svc := NewSymptomService(repo)

	err := svc.DeleteSymptom(context.Background(), uuid.New(), uuid.New())
	if code := apiErrCode(t, err); code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
}
