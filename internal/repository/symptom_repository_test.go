package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Vedoputra/LLUNARA-BE/internal/model"
)

func TestSymptomRepository_CreateAndGetByID(t *testing.T) {
	skipIfNoDB(t)
	repo := NewSymptomRepository(testPool)
	ctx := context.Background()

	created, err := repo.Create(ctx, model.Symptom{UserID: &testUserID, Name: "test-custom-symptom-1", Category: "physical"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	if !created.IsCustom {
		t.Error("expected is_custom = true for a user-created tag")
	}

	fetched, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.UserID == nil || *fetched.UserID != testUserID {
		t.Errorf("user_id = %v, want %v", fetched.UserID, testUserID)
	}
}

func TestSymptomRepository_ListForUser_IncludesPresetsAndOwnTags(t *testing.T) {
	skipIfNoDB(t)
	repo := NewSymptomRepository(testPool)
	ctx := context.Background()

	created, err := repo.Create(ctx, model.Symptom{UserID: &testUserID, Name: "test-custom-symptom-2", Category: "emotional"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	list, err := repo.ListForUser(ctx, testUserID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}

	var foundCustom, foundPreset bool
	for _, s := range list {
		if s.ID == created.ID {
			foundCustom = true
		}
		if s.UserID == nil {
			foundPreset = true // seeded presets from migration 003
		}
	}
	if !foundCustom {
		t.Error("expected the user's own custom tag to appear in the list")
	}
	if !foundPreset {
		t.Error("expected system presets to appear in the list")
	}
}

func TestSymptomRepository_ListForUser_DoesNotIncludeOtherUsersTags(t *testing.T) {
	skipIfNoDB(t)
	repo := NewSymptomRepository(testPool)
	ctx := context.Background()

	// user_id has a FK to auth.users, so this needs a real second user.
	otherUserID, err := createTestUser(testSupabaseURL, testSupabaseSecret)
	if err != nil {
		t.Fatalf("create second test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "delete from auth.users where id = $1", otherUserID)
	})

	created, err := repo.Create(ctx, model.Symptom{UserID: &otherUserID, Name: "test-custom-symptom-other-user", Category: "physical"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), otherUserID, created.ID) })

	list, err := repo.ListForUser(ctx, testUserID)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	for _, s := range list {
		if s.ID == created.ID {
			t.Error("expected another user's custom tag to NOT appear in this user's list")
		}
	}
}

func TestSymptomRepository_Delete_CannotDeletePreset(t *testing.T) {
	skipIfNoDB(t)
	repo := NewSymptomRepository(testPool)
	ctx := context.Background()

	var presetID uuid.UUID
	err := testPool.QueryRow(ctx, `select id from symptoms where user_id is null limit 1`).Scan(&presetID)
	if err != nil {
		t.Fatalf("find a preset symptom: %v", err)
	}

	err = repo.Delete(ctx, testUserID, presetID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound trying to delete a preset via user_id filter, got %v", err)
	}

	// confirm the preset really is untouched
	fetched, err := repo.GetByID(ctx, presetID)
	if err != nil {
		t.Fatalf("preset should still exist: %v", err)
	}
	if fetched.UserID != nil {
		t.Error("preset's user_id should still be nil")
	}
}

func TestSymptomRepository_Delete_WrongUserIsNotFound(t *testing.T) {
	skipIfNoDB(t)
	repo := NewSymptomRepository(testPool)
	ctx := context.Background()

	created, err := repo.Create(ctx, model.Symptom{UserID: &testUserID, Name: "test-custom-symptom-3", Category: "other"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	err = repo.Delete(ctx, uuid.New(), created.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound deleting another user's tag, got %v", err)
	}
}

func TestSymptomRepository_ExistsByName_CaseInsensitive(t *testing.T) {
	skipIfNoDB(t)
	repo := NewSymptomRepository(testPool)
	ctx := context.Background()

	// "kram" is a seeded preset (migration 003) — check case-insensitivity
	// against a preset, not just a custom tag.
	exists, err := repo.ExistsByName(ctx, testUserID, "KRAM")
	if err != nil {
		t.Fatalf("ExistsByName: %v", err)
	}
	if !exists {
		t.Error("expected case-insensitive match against preset 'kram'")
	}

	notExists, err := repo.ExistsByName(ctx, testUserID, "definitely-not-a-real-symptom-name")
	if err != nil {
		t.Fatalf("ExistsByName: %v", err)
	}
	if notExists {
		t.Error("expected no match for a nonexistent name")
	}
}

func TestSymptomRepository_CountCustomForUser(t *testing.T) {
	skipIfNoDB(t)
	repo := NewSymptomRepository(testPool)
	ctx := context.Background()

	before, err := repo.CountCustomForUser(ctx, testUserID)
	if err != nil {
		t.Fatalf("CountCustomForUser: %v", err)
	}

	created, err := repo.Create(ctx, model.Symptom{UserID: &testUserID, Name: "test-custom-symptom-4", Category: "physical"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), testUserID, created.ID) })

	after, err := repo.CountCustomForUser(ctx, testUserID)
	if err != nil {
		t.Fatalf("CountCustomForUser: %v", err)
	}
	if after != before+1 {
		t.Errorf("count = %d, want %d", after, before+1)
	}
}
