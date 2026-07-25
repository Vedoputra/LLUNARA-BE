package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccountRepository handles the hard-delete of everything a user owns.
// Unlike other repositories it needs the pool directly (not the narrower DB
// interface) because it must run every delete inside one transaction.
type AccountRepository struct {
	pool *pgxpool.Pool
}

func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

// DeleteAllUserData hard-deletes every row userID owns across every table,
// in a single transaction. Every FK back to auth.users is already ON DELETE
// CASCADE, so deleting the auth.users row alone would eventually clean all
// of this up too — but that delete goes through Supabase's Admin API
// (service.AccountService does that afterwards), which has proven flaky in
// testing. Deleting explicitly here, first, and through our own reliable DB
// connection means the sensitive health data is gone even if that second
// step fails or needs retrying.
//
// symptoms is filtered to is_custom = true only — preset symptoms have a
// null user_id and are shared across all users, never owned by one.
func (r *AccountRepository) DeleteAllUserData(ctx context.Context, userID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete account tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	stmts := []string{
		`delete from daily_log_symptoms where daily_log_id in (select id from daily_logs where user_id = $1)`,
		`delete from daily_logs where user_id = $1`,
		`delete from wellness_logs where user_id = $1`,
		`delete from cycles where user_id = $1`,
		`delete from symptoms where user_id = $1 and is_custom = true`,
		`delete from reminders where user_id = $1`,
		`delete from sharing_permissions where owner_user_id = $1 or partner_user_id = $1`,
		`delete from profiles where id = $1`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(ctx, stmt, userID); err != nil {
			return fmt.Errorf("delete account data: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete account tx: %w", err)
	}
	return nil
}
