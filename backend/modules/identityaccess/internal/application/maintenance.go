package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// errMaintenanceOnly is what a flow the maintenance composition does not
// serve reports.
var errMaintenanceOnly = errors.New("identity access compose: the maintenance composition neither signs sessions nor checks passwords")

type UnsignedSessions struct{}

func (UnsignedSessions) IssueTokenPair(identityaccess.SessionClaims, identityaccess.RefreshClaims) (string, string, error) {
	return "", "", errMaintenanceOnly
}

func (UnsignedSessions) IssueMFAEnrollmentToken(int64, int64, string, time.Duration) (string, error) {
	return "", errMaintenanceOnly
}

func (UnsignedSessions) ParseAccessToken(string) (identityaccess.SessionClaims, error) {
	return identityaccess.SessionClaims{}, errMaintenanceOnly
}

func (UnsignedSessions) ParseRefreshToken(string) (identityaccess.RefreshClaims, error) {
	return identityaccess.RefreshClaims{}, errMaintenanceOnly
}

func (UnsignedSessions) RefreshExpiry() time.Duration { return 0 }

func (UnsignedSessions) IssueAccessToken(identityaccess.SessionClaims) (string, error) {
	return "", errMaintenanceOnly
}

func (UnsignedSessions) ParseAccessTokenAllowExpired(string) (identityaccess.SessionClaims, error) {
	return identityaccess.SessionClaims{}, errMaintenanceOnly
}

func (UnsignedSessions) AccessExpiry() time.Duration { return 0 }

type UnusablePasswords struct{}

func (UnusablePasswords) VerifyPassword(string, string) (bool, error) {
	return false, errMaintenanceOnly
}

func (UnusablePasswords) ValidatePasswordStrength(string) error { return errMaintenanceOnly }

func (UnusablePasswords) HashPassword(string) (string, error) { return "", errMaintenanceOnly }

type UnlockedMFAPolicy struct{}

func (UnlockedMFAPolicy) LockMFAPolicySharedForTenant(context.Context, int64) error { return nil }

type UnsentPasswordResets struct{}

func (UnsentPasswordResets) DispatchPasswordReset(context.Context, identityaccess.PasswordResetLink, string, identityaccess.PasswordResetScope, time.Duration) error {

	return nil
}
