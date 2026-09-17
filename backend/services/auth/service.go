// Package auth provides authentication and user management services
package auth

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	opCreateService       = "create service"
	opHashPassword        = "hash password"
	opGetAccount          = "get account"
	opUpdateAccount       = "update account"
	opValidateToken       = "validate token"
	opCreateParentAccount = "create parent account"
)

// ServiceConfig holds configuration for the auth service
type ServiceConfig struct {
	Dispatcher          *email.Dispatcher
	DefaultFrom         email.Email
	FrontendURL         string
	ParentsURL          string
	SchoolURL           string
	PasswordResetExpiry time.Duration
	RateLimitEnabled    bool
	TokenAuth           *jwt.TokenAuth
	Settings            configSvc.SettingsService
	Audit               auditModels.Command
	// Sessions is the consumer-owned port over the Identity & Access
	// account-authentication capability (#3251): login, refresh, switching,
	// session validation, cleanup and revocation moved there. The retained
	// flows (staff preview, account management, offboarding) and the
	// AuthService session methods delegate to it.
	Sessions AccountSessions
	// Lifecycle is the consumer-owned port over the Identity & Access
	// account-lifecycle capability (#3225): staff preview, staff offboarding
	// access, the school identity chain, parent accounts and guardian
	// relative access moved there. The retained account management flows
	// apply the school-role policy through it (#3314).
	Lifecycle AccountLifecycle
}

// NewServiceConfig creates and validates a new ServiceConfig
func NewServiceConfig(
	dispatcher *email.Dispatcher,
	defaultFrom email.Email,
	frontendURL string,
	passwordResetExpiry time.Duration,
) (*ServiceConfig, error) {
	if frontendURL == "" {
		return nil, errors.New("frontendURL cannot be empty")
	}
	if passwordResetExpiry <= 0 {
		return nil, errors.New("passwordResetExpiry must be positive")
	}

	return &ServiceConfig{
		Dispatcher:          dispatcher,
		DefaultFrom:         defaultFrom,
		FrontendURL:         frontendURL,
		ParentsURL:          frontendURL,
		SchoolURL:           frontendURL,
		PasswordResetExpiry: passwordResetExpiry,
	}, nil
}

// Service provides the retained authentication and user management
// functionality: password change, account administration, staff preview and
// offboarding. Tenant, parent and school login, refresh, switching, logout,
// session validation, cleanup and revocation are served by Identity & Access
// through the Sessions port (#3251); role and permission management moved
// there with #3314, the password reset, the invitations and the account
// registration and school linking with #3332.
type Service struct {
	repos         *repositories.Factory
	tokenAuth     *jwt.TokenAuth
	txHandler     *tenant.TransactionRunner
	db            *bun.DB
	logger        *slog.Logger
	settings      configSvc.SettingsService
	audit         auditModels.Command
	tenantRuntime *tenant.UnitOfWork
	sessions      AccountSessions
	lifecycle     AccountLifecycle
	// mfaService is optional. The Identity & Access login flows read it
	// through CurrentMFAService at call time, so SetMFAService keeps its
	// meaning: nil disables the gate and login behaves as a plain
	// password login. Wired post-construction to break the
	// AuthService <-> MFAService construction-order dependency.
	mfaService MFAService
}

func (s *Service) withTenantRuntime(ctx context.Context) context.Context {
	if s.tenantRuntime == nil {
		return ctx
	}
	return tenant.WithUnitOfWork(ctx, *s.tenantRuntime)
}

// WithTenantRuntime attaches the unit of work the service was composed
// with; the Identity & Access composition opens its session transactions
// under it.
func (s *Service) WithTenantRuntime(ctx context.Context) context.Context {
	return s.withTenantRuntime(ctx)
}

func (s *Service) SetTenantRuntime(runtime tenant.UnitOfWork) {
	s.tenantRuntime = &runtime
}

// NewService creates a new auth service with reduced parameter count
// Uses repository factory pattern and config struct to avoid parameter bloat
func NewService(
	repos *repositories.Factory,
	config *ServiceConfig,
	db *bun.DB,
	logger *slog.Logger,
) (*Service, error) {
	if repos == nil {
		return nil, &AuthError{Op: opCreateService, Err: errors.New("repos factory is nil")}
	}
	if config == nil {
		return nil, &AuthError{Op: opCreateService, Err: errors.New("config is nil")}
	}
	if db == nil {
		return nil, &AuthError{Op: opCreateService, Err: errors.New("database is nil")}
	}

	tokenAuth := config.TokenAuth
	if tokenAuth == nil {
		var err error
		tokenAuth, err = jwt.NewTokenAuth()
		if err != nil {
			return nil, &AuthError{Op: "create token auth", Err: err}
		}
	}

	return &Service{
		repos:     repos,
		tokenAuth: tokenAuth,
		txHandler: tenant.NewTransactionRunner(),
		db:        db,
		logger:    logger,
		settings:  config.Settings,
		audit:     config.Audit,
		sessions:  config.Sessions,
		lifecycle: config.Lifecycle,
	}, nil
}

// getLogger returns the service's logger, falling back to slog.Default() if nil.
func (s *Service) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// SetMFAService wires the optional MFA service post-construction. Idempotent
// — calling with nil clears the gate.
func (s *Service) SetMFAService(svc MFAService) {
	s.mfaService = svc
}

// CurrentMFAService returns the MFA gate the login flows consult; nil means
// the gate is disabled.
func (s *Service) CurrentMFAService() MFAService {
	return s.mfaService
}

// AccountSessions returns the Identity & Access port the service delegates
// session work to.
func (s *Service) AccountSessions() AccountSessions {
	return s.sessions
}

// AccountLifecycle returns the Identity & Access port the service delegates
// the account lifecycle flows to.
func (s *Service) AccountLifecycle() AccountLifecycle {
	return s.lifecycle
}

func (s *Service) runInTx(
	ctx context.Context,
	fn func(txCtx context.Context) error,
) error {
	ctx = s.withTenantRuntime(ctx)
	if s.txHandler == nil {
		return fn(ctx)
	}

	if tenant.FromContext(ctx) == 0 && tenant.ScopeFromContext(ctx) != "" {
		return tenant.WithinAdmin(ctx, fn)
	}

	return s.txHandler.RunInTx(ctx, func(txCtx context.Context) error {
		return fn(txCtx)
	})
}

func hasAmbientTx(ctx context.Context) bool {
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}

func (s *Service) independentCleanupCtx(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTenant(tenant.ContextWithoutTransaction(ctx)))
}

// VerifyPassword checks a plain-text password against its Argon2id hash. It
// is the credential check the Identity & Access login flows are composed
// with, so password hashing stays in one place.
func VerifyPassword(password, hash string) (bool, error) {
	return userpass.VerifyPassword(password, hash)
}

// AuthError represents an authentication-related error
type AuthError struct {
	Op  string // Operation that failed
	Err error  // Original error
}

// Error returns the error message
func (e *AuthError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("auth error during %s", e.Op)
	}
	return fmt.Sprintf("auth error during %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *AuthError) Unwrap() error {
	return e.Err
}

// MFAGateConfiguration wires the optional MFA gate into the login flows.
type MFAGateConfiguration interface {
	// SetMFAService wires the optional MFA gate. Pass nil to disable the
	// gate (login then behaves exactly as LoginWithAudit).
	SetMFAService(svc MFAService)
}

// AuthService defines the operations for authentication and user
// management. Each subject declares its operations next to its
// implementation.
type AuthService interface {
	SessionOperations
	MFAGateConfiguration
	CredentialOperations
	AccountAdministrationOperations
	StaffPreviewOperations
	ParentAccountOperations
}
