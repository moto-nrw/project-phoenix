package auth

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/base"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/uptrace/bun"
	"golang.org/x/sync/singleflight"
)

const (
	passwordResetRateLimitThreshold = 3
	opCreateService                 = "create service"
	opHashPassword                  = "hash password"
	opGetAccount                    = "get account"
	opUpdateAccount                 = "update account"
	opValidateToken                 = "validate token"
	opAssignPermissionToRole        = "assign permission to role"
	opCreateParentAccount           = "create parent account"
)

var passwordResetEmailBackoff = []time.Duration{
	time.Second,
	5 * time.Second,
	15 * time.Second,
}

// ServiceConfig holds configuration for the auth service
type ServiceConfig struct {
	Dispatcher          *email.Dispatcher
	DefaultFrom         email.Email
	FrontendURL         string
	ParentsURL          string
	PasswordResetExpiry time.Duration
	Settings            configSvc.SettingsService
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
		PasswordResetExpiry: passwordResetExpiry,
	}, nil
}

// Service provides authentication and authorization functionality
type Service struct {
	repos               *repositories.Factory
	tokenAuth           *jwt.TokenAuth
	dispatcher          *email.Dispatcher
	defaultFrom         email.Email
	frontendURL         string
	parentsURL          string
	passwordResetExpiry time.Duration
	jwtExpiry           time.Duration
	jwtRefreshExpiry    time.Duration
	txHandler           *base.TxHandler
	db                  *bun.DB
	logger              *slog.Logger
	settings            configSvc.SettingsService
	// mfaService is optional. When non-nil and an account requires MFA the
	// login flow returns a challenge token instead of an access/refresh
	// pair; when nil the gate is bypassed and login behaves as before.
	// Wired post-construction via SetMFAService to break the
	// AuthService ↔ MFAService construction-order dependency.
	mfaService MFAService
	refreshSF  singleflight.Group // deduplicates concurrent token refresh calls
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

	tokenAuth, err := jwt.NewTokenAuth()
	if err != nil {
		return nil, &AuthError{Op: "create token auth", Err: err}
	}

	return &Service{
		repos:               repos,
		tokenAuth:           tokenAuth,
		dispatcher:          config.Dispatcher,
		defaultFrom:         config.DefaultFrom,
		frontendURL:         config.FrontendURL,
		parentsURL:          config.ParentsURL,
		passwordResetExpiry: config.PasswordResetExpiry,
		jwtExpiry:           tokenAuth.JwtExpiry,
		jwtRefreshExpiry:    tokenAuth.JwtRefreshExpiry,
		txHandler:           base.NewTxHandler(db),
		db:                  db,
		logger:              logger,
		settings:            config.Settings,
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

func (s *Service) runInTx(
	ctx context.Context,
	fn func(txCtx context.Context) error,
) error {
	if s.txHandler == nil {
		return fn(ctx)
	}

	if s.txHandler.DB == nil {
		if _, ok := base.TxFromContext(ctx); !ok {
			return fn(ctx)
		}
	}

	return s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

// VerifyAccountTenantMembership reports whether the account has a tenant
// mapping for the given school (issue #584; repository result verbatim).
func (s *Service) VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, accountID, tenantID)
}
