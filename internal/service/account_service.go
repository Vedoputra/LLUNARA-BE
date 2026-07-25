package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// accountDataRepository is the subset of *repository.AccountRepository that
// AccountService depends on.
type accountDataRepository interface {
	DeleteAllUserData(ctx context.Context, userID uuid.UUID) error
}

// authAdminClient is the subset of *supabaseadmin.Client that AccountService
// depends on.
type authAdminClient interface {
	DeleteUser(ctx context.Context, userID uuid.UUID) error
}

type AccountService struct {
	repo       accountDataRepository
	authClient authAdminClient
}

func NewAccountService(repo accountDataRepository, authClient authAdminClient) *AccountService {
	return &AccountService{repo: repo, authClient: authClient}
}

// DeleteAccount is a hard delete, per BE-6.4 — this is a health-data app,
// so retaining anything after a user asks to leave is not acceptable. App
// data is wiped first, through our own DB transaction, before the less
// reliable Supabase Admin API call removes the auth.users row — see
// AccountRepository.DeleteAllUserData for why that ordering matters.
func (s *AccountService) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	if err := s.repo.DeleteAllUserData(ctx, userID); err != nil {
		return fmt.Errorf("delete account data: %w", err)
	}
	if err := s.authClient.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("delete auth user: %w", err)
	}
	return nil
}
