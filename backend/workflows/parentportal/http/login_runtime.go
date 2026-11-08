package parent

import (
	"context"
	"errors"
)

// The parents login is owned by Identity & Access (#3251). The composition
// root binds it to the runtime below; the refusals answer through the
// owner's login rules.

// LoginRuntime is the parents-portal login port. Login mints the
// parent-scope token pair.
type LoginRuntime struct {
	Login func(ctx context.Context, email, password, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	// ConfirmPassword re-checks the signed-in guardian's password before a
	// binding Erklärung (#3430).
	ConfirmPassword func(ctx context.Context, accountID int64, password string) error
}

var errLoginUnavailable = errors.New("account sessions are not composed")
