package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Stable outcomes of the staff PIN self-service flow behind /api/staff/pin.
// The HTTP layer maps them to status codes; the messages are the ones the
// legacy handler produced verbatim.
var (
	ErrStaffPINAccountNotFound   = errors.New("account not found")
	ErrStaffPINSelfServiceLocked = errors.New("account is temporarily locked due to failed PIN attempts")
	ErrStaffPINCurrentRequired   = errors.New("current PIN is required when updating existing PIN")
	ErrStaffPINCurrentWrong      = errors.New("current PIN is incorrect")
	errStaffPINHash              = errors.New("failed to hash PIN")
)

// StaffPINStatus reports whether the account has a PIN and when it last
// changed (the account's UpdatedAt stands in for a dedicated timestamp).
func StaffPINStatus(ctx context.Context, service AuthService, accountID int64) (bool, *time.Time, error) {
	account, err := service.GetAccountByID(ctx, int(accountID))
	if err != nil {
		return false, nil, ErrStaffPINAccountNotFound
	}
	if !account.HasPIN() {
		return false, nil, nil
	}
	lastChanged := account.UpdatedAt
	return true, &lastChanged, nil
}

// StaffPINPreflight runs the account checks that answer before anything
// else on a PIN change: the account must exist and must not be locked.
func StaffPINPreflight(ctx context.Context, service AuthService, accountID int64) error {
	account, err := service.GetAccountByID(ctx, int(accountID))
	if err != nil {
		return ErrStaffPINAccountNotFound
	}
	if service.IsPINLocked(account, time.Now()) {
		return ErrStaffPINSelfServiceLocked
	}
	return nil
}

// ChangeStaffPIN verifies the current PIN (a wrong one counts towards the
// lockout), hashes the new one and persists it together with a lockout reset
// in one tenant transaction.
func ChangeStaffPIN(ctx context.Context, service AuthService, db *bun.DB, logger *slog.Logger, accountID int64, currentPIN *string, newPIN string) error {
	account, err := service.GetAccountByID(ctx, int(accountID))
	if err != nil {
		return ErrStaffPINAccountNotFound
	}
	if service.IsPINLocked(account, time.Now()) {
		return ErrStaffPINSelfServiceLocked
	}
	if account.HasPIN() {
		if currentPIN == nil || *currentPIN == "" {
			return ErrStaffPINCurrentRequired
		}
		if !account.VerifyPIN(*currentPIN) {
			if updateErr := service.RecordFailedPINAttempt(ctx, int64(account.ID)); updateErr != nil {
				logger.Error("failed to update account PIN attempts", slog.String("error", updateErr.Error()))
			}
			return ErrStaffPINCurrentWrong
		}
	}
	if account.HashPIN(newPIN) != nil {
		return errStaffPINHash
	}
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if updateErr := service.UpdateAccount(ctx, account); updateErr != nil {
			return updateErr
		}
		return service.ResetPINLockout(ctx, int64(account.ID))
	})
}
