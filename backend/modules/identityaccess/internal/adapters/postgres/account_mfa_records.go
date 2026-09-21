package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// AccountMFARecords persists the account second factor on the caller's connection.
// It never starts or elevates a transaction.
type AccountMFARecords struct{ store *Store }

func NewAccountMFARecords(store *Store) *AccountMFARecords {
	return &AccountMFARecords{store: store}
}

// Raw arguments do not honor a model field's nullzero tag. Preserve the
// nullable inet contract explicitly, including a zero-length net.IP.
func nullableMFAIPAddress(address net.IP) *string {
	if len(address) == 0 {
		return nil
	}
	value := address.String()
	return &value
}

func (r *AccountMFARecords) FindAccountIdentity(ctx context.Context, accountID int64) (domain.AccountIdentity, bool, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountIdentity{}, false, err
	}
	var row domain.AccountIdentity
	err = db.NewRaw("SELECT id, email, active, mfa_attempts, mfa_locked_until FROM auth.accounts WHERE id = ?", accountID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccountIdentity{}, false, nil
	}
	return row, err == nil, err
}

func (r *AccountMFARecords) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return false, err
	}
	var exists bool
	err = db.NewRaw("SELECT EXISTS (SELECT 1 FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ? AND status = 'active')", accountID, tenantID).Scan(ctx, &exists)
	return exists, err
}

// Override writes serialize with session minting on the account row.
func (r *AccountMFARecords) LockAccountForOverrideWrite(ctx context.Context, accountID int64) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	var id int64
	err = db.NewRaw("SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE", accountID).Scan(ctx, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func (r *AccountMFARecords) IncrementMFAAttempts(ctx context.Context, accountID int64, threshold int, duration time.Duration) (domain.AccountLockout, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountLockout{}, err
	}
	var lockout domain.AccountLockout
	// Compute the deadline with the same application clock that evaluates it.
	err = db.NewRaw(`UPDATE auth.accounts SET mfa_attempts = mfa_attempts + 1,
 mfa_locked_until = CASE WHEN mfa_attempts + 1 >= ? THEN ? ELSE mfa_locked_until END
 WHERE id = ? RETURNING mfa_attempts AS attempts, mfa_locked_until AS locked_until`,
		threshold, time.Now().Add(duration), accountID).Scan(ctx, &lockout)
	return lockout, err
}

func (r *AccountMFARecords) ResetMFAAttempts(ctx context.Context, accountID int64) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw("UPDATE auth.accounts SET mfa_attempts = 0, mfa_locked_until = NULL WHERE id = ?", accountID).Exec(ctx)
	return err
}

func (r *AccountMFARecords) FindCredential(ctx context.Context, accountID int64) (domain.AccountMFACredential, bool, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountMFACredential{}, false, err
	}
	var row domain.AccountMFACredential
	err = db.NewRaw("SELECT id, account_id, method, enrolled_at, last_used_at, created_at, updated_at FROM auth.mfa_credentials WHERE account_id = ? LIMIT 1", accountID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccountMFACredential{}, false, nil
	}
	return row, err == nil, err
}

func (r *AccountMFARecords) CreateCredential(ctx context.Context, credential domain.AccountMFACredential) error {
	if credential.AccountID == 0 {
		return fmt.Errorf("account_id is required")
	}
	if credential.Method != "email" {
		return fmt.Errorf("unsupported MFA method")
	}
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	var enrolledAt *time.Time
	if !credential.EnrolledAt.IsZero() {
		enrolledAt = &credential.EnrolledAt
	}
	_, err = db.NewRaw(`INSERT INTO auth.mfa_credentials (account_id, method, enrolled_at, last_used_at)
 VALUES (?, ?, COALESCE(?, CURRENT_TIMESTAMP), ?)`, credential.AccountID, credential.Method, enrolledAt, credential.LastUsedAt).Exec(ctx)
	return err
}

func (r *AccountMFARecords) TouchCredential(ctx context.Context, id int64, usedAt time.Time) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw("UPDATE auth.mfa_credentials SET last_used_at = ? WHERE id = ?", usedAt, id).Exec(ctx)
	return err
}

func (r *AccountMFARecords) DeleteCredentials(ctx context.Context, accountID int64) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw("DELETE FROM auth.mfa_credentials WHERE account_id = ?", accountID).Exec(ctx)
	return err
}

// A compare-and-set transition must affect one row, including under races.
func requireMFATransition(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("mfa transition: expected one affected row, got %d", rows)
	}
	return nil
}
