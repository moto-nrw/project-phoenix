package parent

import (
	"context"
	"errors"

	"github.com/go-chi/chi/v5"
)

// The parents sign-in and its second factor are owned by Identity & Access
// (#3251, #7051). The composition root binds the runtime below; the routes
// answer through the owner's sign-in rules.

// SignInRoutes are the owner's parents-portal routes the portal mounts: the
// public sign-in under /auth (password, code, resend), the session's reads
// of its factor and the session's credential checks (switch on and off).
type SignInRoutes interface {
	MountSignIn(r chi.Router, after func(ctx context.Context, email string))
	MountSession(r chi.Router)
	MountCredentialChecks(r chi.Router)
}

// LoginRuntime is the parents-portal sign-in port.
type LoginRuntime struct {
	// Routes serve the sign-in and the second factor; nil answers 503 on
	// the login.
	Routes SignInRoutes
	// ConfirmPassword re-checks the signed-in guardian's password before a
	// binding Erklärung (#3430).
	ConfirmPassword func(ctx context.Context, accountID int64, password string) error
}

var errLoginUnavailable = errors.New("account sessions are not composed")
