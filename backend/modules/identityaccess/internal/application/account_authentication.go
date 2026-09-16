package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"golang.org/x/sync/singleflight"
)

// AccountAuthentication runs tenant, parent and school login, refresh, tenant
// and school switching, logout, session validation, session cleanup and
// session revocation over the account-session service and the consumer-owned
// ports (#3251). The session writes join the transactions the runtime port
// opens; foreign facts (schools, person names, audit, push subscriptions, the
// MFA gate, the token codec) arrive through ports the composition binds.
type AccountAuthentication struct {
	sessions  *Service
	store     ports.AccountLoginStore
	schools   ports.SchoolDirectory
	persons   ports.PersonDirectory
	passwords ports.PasswordVerifier
	codec     ports.TokenCodec
	mfa       ports.MFAGate
	mfaLock   ports.MFAPolicyLock
	audit     ports.AuthAudit
	push      ports.PushSubscriptionCleanup
	runtime   ports.Runtime
	rotation  ports.Rotation
	logger    *slog.Logger
	// refreshSF deduplicates concurrent refresh calls for the same refresh
	// token and recovery proof.
	refreshSF singleflight.Group
}

// AccountAuthenticationDependencies are the ports the flows consume.
type AccountAuthenticationDependencies struct {
	Store     ports.AccountLoginStore
	Schools   ports.SchoolDirectory
	Persons   ports.PersonDirectory
	Passwords ports.PasswordVerifier
	Codec     ports.TokenCodec
	MFA       ports.MFAGate
	MFALock   ports.MFAPolicyLock
	Audit     ports.AuthAudit
	Push      ports.PushSubscriptionCleanup
	Runtime   ports.Runtime
	Rotation  ports.Rotation
	Logger    *slog.Logger
}

// NewAccountAuthentication composes the flows over the session service that
// owns auth.tokens.
func NewAccountAuthentication(sessions *Service, deps AccountAuthenticationDependencies) (*AccountAuthentication, error) {
	switch {
	case sessions == nil:
		return nil, fmt.Errorf("identity access account authentication: session service is required")
	case deps.Store == nil:
		return nil, fmt.Errorf("identity access account authentication: account store is required")
	case deps.Schools == nil:
		return nil, fmt.Errorf("identity access account authentication: school directory is required")
	case deps.Persons == nil:
		return nil, fmt.Errorf("identity access account authentication: person directory is required")
	case deps.Passwords == nil:
		return nil, fmt.Errorf("identity access account authentication: password verifier is required")
	case deps.Codec == nil:
		return nil, fmt.Errorf("identity access account authentication: token codec is required")
	case deps.MFA == nil:
		return nil, fmt.Errorf("identity access account authentication: mfa gate is required")
	case deps.MFALock == nil:
		return nil, fmt.Errorf("identity access account authentication: mfa policy lock is required")
	case deps.Audit == nil:
		return nil, fmt.Errorf("identity access account authentication: audit recorder is required")
	case deps.Push == nil:
		return nil, fmt.Errorf("identity access account authentication: push subscription cleanup is required")
	case deps.Runtime == nil:
		return nil, fmt.Errorf("identity access account authentication: tenant runtime is required")
	case deps.Rotation == nil:
		return nil, fmt.Errorf("identity access account authentication: rotation policy is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AccountAuthentication{
		sessions: sessions, store: deps.Store, schools: deps.Schools, persons: deps.Persons, passwords: deps.Passwords,
		codec: deps.Codec, mfa: deps.MFA, mfaLock: deps.MFALock, audit: deps.Audit, push: deps.Push,
		runtime: deps.Runtime, rotation: deps.Rotation, logger: logger,
	}, nil
}

// OperationError wraps a flow failure with the operation that failed, the way the
// retained auth service reported it; the sentinel stays reachable through
// errors.Is.
type OperationError struct {
	Op  string
	Err error
}

func (e *OperationError) Error() string {
	if e.Err == nil {
		return "auth error during " + e.Op
	}
	return "auth error during " + e.Op + ": " + e.Err.Error()
}

func (e *OperationError) Unwrap() error { return e.Err }

func failed(op string, err error) error { return &OperationError{Op: op, Err: err} }

// mintGuard runs inside the token-persistence transaction, immediately before
// the session row is written. It is the only place where an authorization
// fact can be re-checked atomically with the write it authorizes; the login
// flows verify everything else in already-committed transactions that may be
// stale by the time the token is minted. A guard receives the administrative
// transaction in ctx and must read through the store on it, never open a
// nested one. Returning an error aborts the mint; the error surfaces verbatim
// and is never retried. A guard is also where anything the JWT is built from
// belongs: assembling claims after the transaction committed means a failure
// there has already rotated the caller's refresh token with no successor to
// hand back.
type mintGuard func(ctx context.Context, account domain.LoginAccount) error

// mintGuardError marks an error as coming from a mintGuard so the retry loop
// passes the caller's sentinel through instead of burying it under the
// generic transaction wrapper.
type mintGuardError struct{ err error }

func (e *mintGuardError) Error() string { return e.err.Error() }
func (e *mintGuardError) Unwrap() error { return e.err }
