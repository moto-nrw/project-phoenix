package parent

import (
	"context"
	"errors"
)

// The parents login is owned by Identity & Access (#3251). The portal may
// not name the owner's contract, so it consumes the login through the
// runtime below, which the composition root binds — the same shape the
// reset runtime next to it uses (#3364). The status codes, the wire codes
// and the rendered sentences stay this package's business.

// LoginRuntime is the parents-portal login port. Login mints the
// parent-scope token pair; the classifiers say what a refused attempt was,
// so the portal keeps deciding which of them it discloses.
type LoginRuntime struct {
	Login func(ctx context.Context, email, password, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	// InvalidCredentials reports an unknown address or a wrong password.
	// The portal masks both as one answer.
	InvalidCredentials func(error) bool
	// AccountInactive reports a deactivated account, which the portal
	// discloses so the page can point at the school.
	AccountInactive func(error) bool
	// NotAGuardian reports an account without guardian access at any
	// school — a staff account at the parents login.
	NotAGuardian func(error) bool
}

func (rt *LoginRuntime) complete() bool {
	return rt != nil && rt.Login != nil && rt.InvalidCredentials != nil &&
		rt.AccountInactive != nil && rt.NotAGuardian != nil
}

var (
	// ErrLoginUnavailable reports a resource composed without the login
	// runtime; the route answers 500, as a service composed without its
	// session port did.
	ErrLoginUnavailable = errors.New("account sessions are not composed")
	// ErrInvalidCredentials is the 401 body of a masked login refusal.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrAccountInactive is the 401 body of a deactivated account.
	ErrAccountInactive = errors.New("account is inactive")
	// ErrAccountNoGuardianRole is the 403 body of an account that holds no
	// guardian access anywhere.
	ErrAccountNoGuardianRole = errors.New("account is not a guardian at any school")
)
