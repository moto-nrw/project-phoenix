package auth

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	passwordResetRateLimitThreshold = 3
	opCreateService                 = "create service"
	opHashPassword                  = "hash password"
	opGetAccount                    = "get account"
	opUpdateAccount                 = "update account"
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
	PasswordResetExpiry time.Duration
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
	passwordResetExpiry time.Duration
	jwtExpiry           time.Duration
	jwtRefreshExpiry    time.Duration
	txHandler           *base.TxHandler
	db                  *bun.DB
	logger              *slog.Logger
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
		passwordResetExpiry: config.PasswordResetExpiry,
		jwtExpiry:           tokenAuth.JwtExpiry,
		jwtRefreshExpiry:    tokenAuth.JwtRefreshExpiry,
		txHandler:           base.NewTxHandler(db),
		db:                  db,
		logger:              logger,
	}, nil
}

// getLogger returns the service's logger, falling back to slog.Default() if nil.
func (s *Service) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// WithTx returns a new service instance with transaction-aware repositories
// The factory pattern simplifies this - repositories use TxFromContext(ctx) to detect transactions
func (s *Service) WithTx(tx bun.Tx) interface{} {
	return &Service{
		repos:               s.repos, // Repositories detect transaction from context via TxFromContext(ctx)
		tokenAuth:           s.tokenAuth,
		dispatcher:          s.dispatcher,
		defaultFrom:         s.defaultFrom,
		frontendURL:         s.frontendURL,
		passwordResetExpiry: s.passwordResetExpiry,
		jwtExpiry:           s.jwtExpiry,
		jwtRefreshExpiry:    s.jwtRefreshExpiry,
		txHandler:           s.txHandler.WithTx(tx),
		db:                  s.db,
		logger:              s.logger,
	}
}
