package auth

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/uptrace/bun"
)

const (
	opCreateService          = "create service"
	opHashPassword           = "hash password"
	opGetAccount             = "get account"
	opUpdateAccount          = "update account"
	opAssignPermissionToRole = "assign permission to role"
	opCreateParentAccount    = "create parent account"
)

// ServiceConfig holds configuration for the auth service
type ServiceConfig struct {
	Dispatcher           port.EmailDispatcher
	DefaultFrom          port.EmailAddress
	FrontendURL          string
	PasswordResetExpiry  time.Duration
	RateLimitEnabled     bool
	RateLimitMaxRequests int // Maximum password reset requests per time window
}

// NewServiceConfig creates and validates a new ServiceConfig.
// All configuration is passed explicitly following 12-Factor App principles.
func NewServiceConfig(
	dispatcher port.EmailDispatcher,
	defaultFrom port.EmailAddress,
	frontendURL string,
	passwordResetExpiry time.Duration,
	rateLimitEnabled bool,
	rateLimitMaxRequests int,
) (*ServiceConfig, error) {
	if frontendURL == "" {
		return nil, errors.New("frontendURL cannot be empty")
	}
	if passwordResetExpiry <= 0 {
		return nil, errors.New("passwordResetExpiry must be positive")
	}

	if rateLimitEnabled {
		if rateLimitMaxRequests <= 0 {
			return nil, errors.New("rateLimitMaxRequests must be a positive integer when rate limiting is enabled")
		}
		if rateLimitMaxRequests > 100 {
			return nil, errors.New("rateLimitMaxRequests must be less than or equal to 100")
		}
	}

	return &ServiceConfig{
		Dispatcher:           dispatcher,
		DefaultFrom:          defaultFrom,
		FrontendURL:          frontendURL,
		PasswordResetExpiry:  passwordResetExpiry,
		RateLimitEnabled:     rateLimitEnabled,
		RateLimitMaxRequests: rateLimitMaxRequests,
	}, nil
}

// Service provides authentication and authorization functionality
type Service struct {
	repos                Repositories
	tokenProvider        port.TokenProvider
	dispatcher           port.EmailDispatcher
	defaultFrom          port.EmailAddress
	frontendURL          string
	passwordResetExpiry  time.Duration
	rateLimitEnabled     bool
	rateLimitMaxRequests int
	jwtExpiry            time.Duration
	jwtRefreshExpiry     time.Duration
	txHandler            *base.TxHandler
	db                   *bun.DB
}

// NewService creates a new auth service with reduced parameter count
// Uses repository factory pattern and config struct to avoid parameter bloat
func NewService(
	repos Repositories,
	config *ServiceConfig,
	db *bun.DB,
	tokenProvider port.TokenProvider,
) (*Service, error) {
	if config == nil {
		return nil, &AuthError{Op: opCreateService, Err: errors.New("config is nil")}
	}
	if db == nil {
		return nil, &AuthError{Op: opCreateService, Err: errors.New("database is nil")}
	}
	if tokenProvider == nil {
		return nil, &AuthError{Op: opCreateService, Err: errors.New("token provider is nil")}
	}

	return &Service{
		repos:                repos,
		tokenProvider:        tokenProvider,
		dispatcher:           config.Dispatcher,
		defaultFrom:          config.DefaultFrom,
		frontendURL:          config.FrontendURL,
		passwordResetExpiry:  config.PasswordResetExpiry,
		rateLimitEnabled:     config.RateLimitEnabled,
		rateLimitMaxRequests: config.RateLimitMaxRequests,
		jwtExpiry:            tokenProvider.AccessExpiry(),
		jwtRefreshExpiry:     tokenProvider.RefreshExpiry(),
		txHandler:            base.NewTxHandler(db),
		db:                   db,
	}, nil
}

// WithTx returns a new service instance with transaction-aware repositories
// The factory pattern simplifies this - repositories use TxFromContext(ctx) to detect transactions
func (s *Service) WithTx(tx bun.Tx) any {
	return &Service{
		repos:                s.repos, // Repositories detect transaction from context via TxFromContext(ctx)
		tokenProvider:        s.tokenProvider,
		dispatcher:           s.dispatcher,
		defaultFrom:          s.defaultFrom,
		frontendURL:          s.frontendURL,
		passwordResetExpiry:  s.passwordResetExpiry,
		rateLimitEnabled:     s.rateLimitEnabled,
		rateLimitMaxRequests: s.rateLimitMaxRequests,
		jwtExpiry:            s.jwtExpiry,
		jwtRefreshExpiry:     s.jwtRefreshExpiry,
		txHandler:            s.txHandler.WithTx(tx),
		db:                   s.db,
	}
}
