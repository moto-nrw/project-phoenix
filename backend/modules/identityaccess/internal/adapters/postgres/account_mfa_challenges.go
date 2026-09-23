package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

const accountMFAChallengeColumns = "id, account_id, scope, tenant_id, code_hash, expires_at, consumed_at, ip_address, created_at, updated_at"

func (r *AccountMFARecords) CreateChallenge(ctx context.Context, challenge domain.AccountMFAChallenge) (domain.AccountMFAChallenge, error) {
	if challenge.AccountID == 0 || challenge.CodeHash == "" || challenge.Scope == "" || challenge.ExpiresAt.IsZero() {
		return domain.AccountMFAChallenge{}, fmt.Errorf("mfa challenge requires account, hash, scope and expiry")
	}
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountMFAChallenge{}, err
	}
	var createdAt, updatedAt *time.Time
	if !challenge.CreatedAt.IsZero() {
		createdAt = &challenge.CreatedAt
	}
	if !challenge.UpdatedAt.IsZero() {
		updatedAt = &challenge.UpdatedAt
	}
	var row domain.AccountMFAChallenge
	err = db.NewRaw(`INSERT INTO auth.mfa_email_challenges
 (account_id, scope, tenant_id, code_hash, expires_at, consumed_at, ip_address, created_at, updated_at)
 VALUES (?, ?, NULLIF(?, 0), ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), COALESCE(?, CURRENT_TIMESTAMP))
 RETURNING `+accountMFAChallengeColumns,
		challenge.AccountID, challenge.Scope, challenge.TenantID, challenge.CodeHash, challenge.ExpiresAt,
		challenge.ConsumedAt, nullableMFAIPAddress(challenge.IPAddress), createdAt, updatedAt).Scan(ctx, &row)
	return row, err
}

func (r *AccountMFARecords) ActivateChallenge(ctx context.Context, id int64) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	return requireMFATransition(db.NewRaw("UPDATE auth.mfa_email_challenges SET consumed_at = NULL WHERE id = ? AND consumed_at IS NOT NULL", id).Exec(ctx))
}

func (r *AccountMFARecords) ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	return requireMFATransition(db.NewRaw("UPDATE auth.mfa_email_challenges SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL", consumedAt, id).Exec(ctx))
}

func (r *AccountMFARecords) CountChallengesSince(ctx context.Context, accountID int64, since time.Time) (int, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	err = db.NewRaw("SELECT COUNT(*) FROM auth.mfa_email_challenges WHERE account_id = ? AND created_at >= ?", accountID, since).Scan(ctx, &count)
	return count, err
}

func (r *AccountMFARecords) FindActiveChallengeForAccount(ctx context.Context, id, accountID int64) (domain.AccountMFAChallenge, bool, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountMFAChallenge{}, false, err
	}
	var row domain.AccountMFAChallenge
	err = db.NewRaw("SELECT "+accountMFAChallengeColumns+" FROM auth.mfa_email_challenges WHERE id = ? AND account_id = ? AND consumed_at IS NULL AND expires_at > ? LIMIT 1", id, accountID, time.Now()).Scan(ctx, &row)
	return row, err == nil, err
}

func (r *AccountMFARecords) FindActiveChallengeInScope(ctx context.Context, accountID, tenantID int64, scope string) (domain.AccountMFAChallenge, bool, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountMFAChallenge{}, false, err
	}
	var row domain.AccountMFAChallenge
	// Zero retains the account-wide lookup for callers without a selected school.
	err = db.NewRaw("SELECT "+accountMFAChallengeColumns+` FROM auth.mfa_email_challenges
 WHERE account_id = ? AND scope = ? AND (? <= 0 OR tenant_id = ?)
 AND consumed_at IS NULL AND expires_at > ? ORDER BY expires_at DESC LIMIT 1`,
		accountID, scope, tenantID, tenantID, time.Now()).Scan(ctx, &row)
	return row, err == nil, err
}
