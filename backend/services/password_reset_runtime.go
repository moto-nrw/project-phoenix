package services

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The school and parents portals may not name the Identity & Access
// contract, so the root hands each of them a runtime of plain-typed
// closures over the reset capability — the shape the People Directory
// guardian routes already use (#3332). The scope is fixed per portal here:
// a guardian resets on the parents portal and a Lehrkraft on the school
// portal, and neither may issue the other's link.

// PasswordResetRuntime is the plain-typed reset runtime a portal consumes.
// Its field names and signatures match the runtime structs the portals
// declare, so the HTTP composition maps it across without either side
// importing the other.
type PasswordResetRuntime struct {
	Initiate     func(ctx context.Context, email string) error
	Reset        func(ctx context.Context, token, newPassword string) error
	LinkUnusable func(error) bool
	TooWeak      func(error) bool
	RetryAfter   func(error) (int, bool)
}

// PasswordResetRuntimeFor binds the reset runtime of one portal. A nil
// capability yields the zero runtime, which the portals report as
// unavailable.
func PasswordResetRuntimeFor(resets identityaccess.PasswordResets, scope identityaccess.PasswordResetScope) PasswordResetRuntime {
	if resets == nil {
		return PasswordResetRuntime{}
	}
	return PasswordResetRuntime{
		Initiate: func(ctx context.Context, email string) error {
			// The link itself never leaves the owner: the portal answers the
			// same body whether or not one was issued.
			_, err := resets.InitiatePasswordReset(ctx, email, scope)
			return err
		},
		Reset: func(ctx context.Context, token, newPassword string) error {
			return resets.ResetPassword(ctx, token, newPassword)
		},
		LinkUnusable: passwordResetLinkUnusable,
		TooWeak:      passwordResetTooWeak,
		RetryAfter:   passwordResetRetryAfter,
	}
}

// passwordResetLinkUnusable reports a link that is unknown, expired or
// already spent. A missing row counts: the portals never say which of the
// three it was.
func passwordResetLinkUnusable(err error) bool {
	cause := passwordResetCause(err)
	return errors.Is(cause, identityaccess.ErrInvalidToken) || errors.Is(cause, sql.ErrNoRows)
}

// passwordResetTooWeak reports a credential the password policy refused.
func passwordResetTooWeak(err error) bool {
	return errors.Is(passwordResetCause(err), identityaccess.ErrPasswordTooWeak)
}

// passwordResetRetryAfter reports the rate-limit rejection and the seconds
// until the address may request again.
func passwordResetRetryAfter(err error) (int, bool) {
	var limited *identityaccess.PasswordResetRateLimitError
	if !errors.As(err, &limited) {
		return 0, false
	}
	return limited.RetryAfterSeconds(time.Now()), true
}

// passwordResetCause unwraps the operation envelope the owner reports so the
// classification reads the cause, not the wrapper.
func passwordResetCause(err error) error {
	var operation *identityaccess.AuthenticationError
	if errors.As(err, &operation) && operation.Err != nil {
		return operation.Err
	}
	return err
}

// SchoolPasswordResetRuntime binds the school portal's reset runtime: only
// accounts holding a school-portal role are served, and the link lands on
// the school host.
func (f *Factory) SchoolPasswordResetRuntime() PasswordResetRuntime {
	return PasswordResetRuntimeFor(f.AccountAuthentication(), identityaccess.PasswordResetScopeSchool)
}

// ParentPasswordResetRuntime binds the parents portal's reset runtime: only
// accounts with guardian access are served, and the link lands on the
// parents host.
func (f *Factory) ParentPasswordResetRuntime() PasswordResetRuntime {
	return PasswordResetRuntimeFor(f.AccountAuthentication(), identityaccess.PasswordResetScopeParent)
}
