package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	emailpkg "github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// OperatorAuthService handles operator authentication
type OperatorAuthService interface {
	// Login authenticates an operator and returns JWT tokens
	Login(ctx context.Context, email, password string, clientIP net.IP) (accessToken, refreshToken string, operator *platform.Operator, err error)

	// RefreshToken validates the operator is still active and issues a new token pair
	RefreshToken(ctx context.Context, operatorID int64) (accessToken, refreshToken string, err error)

	// ValidateOperator validates an operator's credentials without generating tokens
	ValidateOperator(ctx context.Context, email, password string) (*platform.Operator, error)

	// GetOperator retrieves an operator by ID
	GetOperator(ctx context.Context, id int64) (*platform.Operator, error)

	// ListOperators retrieves all operators
	ListOperators(ctx context.Context) ([]*platform.Operator, error)

	// UpdateProfile updates an operator's display name
	UpdateProfile(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error)

	// ChangePassword changes an operator's password after verifying the current one
	ChangePassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error

	// InitiateEmailChange starts the email change verification flow.
	// clientIP is recorded in the audit log for incident investigation.
	InitiateEmailChange(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error

	// ConfirmEmailChange completes the email change using a verification token.
	// Returns the new email address on success. clientIP is recorded in the audit log.
	ConfirmEmailChange(ctx context.Context, token string, clientIP net.IP) (string, error)

	// CleanupExpiredEmailChangeTokens removes expired and used email change tokens
	CleanupExpiredEmailChangeTokens(ctx context.Context) (int, error)
}

type operatorAuthService struct {
	operatorRepo         platform.OperatorRepository
	auditLogRepo         platform.OperatorAuditLogRepository
	emailChangeTokenRepo platform.OperatorEmailChangeTokenRepository
	tokenAuth            *jwt.TokenAuth
	db                   *bun.DB
	logger               *slog.Logger
	dispatcher           *emailpkg.Dispatcher
	defaultFrom          emailpkg.Email
	frontendURL          string
	emailChangeExpiry    time.Duration
}

// OperatorAuthServiceConfig holds configuration for the operator auth service
type OperatorAuthServiceConfig struct {
	OperatorRepo         platform.OperatorRepository
	AuditLogRepo         platform.OperatorAuditLogRepository
	EmailChangeTokenRepo platform.OperatorEmailChangeTokenRepository
	DB                   *bun.DB
	Logger               *slog.Logger
	Dispatcher           *emailpkg.Dispatcher
	DefaultFrom          emailpkg.Email
	FrontendURL          string
	EmailChangeExpiry    time.Duration
}

// NewOperatorAuthService creates a new operator auth service
func NewOperatorAuthService(cfg OperatorAuthServiceConfig) (OperatorAuthService, error) {
	tokenAuth, err := jwt.NewTokenAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to create token auth: %w", err)
	}

	return &operatorAuthService{
		operatorRepo:         cfg.OperatorRepo,
		auditLogRepo:         cfg.AuditLogRepo,
		emailChangeTokenRepo: cfg.EmailChangeTokenRepo,
		tokenAuth:            tokenAuth,
		db:                   cfg.DB,
		logger:               cfg.Logger,
		dispatcher:           cfg.Dispatcher,
		defaultFrom:          cfg.DefaultFrom,
		frontendURL:          cfg.FrontendURL,
		emailChangeExpiry:    cfg.EmailChangeExpiry,
	}, nil
}

func (s *operatorAuthService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// Login authenticates an operator and returns JWT tokens
func (s *operatorAuthService) Login(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Find operator by email
	operator, err := s.operatorRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", "", nil, err
	}
	if operator == nil {
		return "", "", nil, &InvalidCredentialsError{}
	}

	// Check if operator is active
	if !operator.Active {
		return "", "", nil, &OperatorInactiveError{OperatorID: operator.ID}
	}

	// Verify password using userpass package
	match, err := userpass.VerifyPassword(password, operator.PasswordHash)
	if err != nil || !match {
		return "", "", nil, &InvalidCredentialsError{}
	}

	// Generate JWT tokens with platform scope
	accessClaims := jwt.AppClaims{
		ID:          int(operator.ID),
		Sub:         fmt.Sprintf("operator:%d", operator.ID),
		Username:    operator.Email,
		FirstName:   operator.DisplayName,
		LastName:    "",
		Roles:       []string{"operator"},
		Permissions: []string{}, // Operators don't have tenant permissions
		IsAdmin:     false,
		Scope:       "platform", // Key differentiation from tenant tokens
	}

	refreshClaims := jwt.RefreshClaims{
		ID:    int(operator.ID),
		Token: fmt.Sprintf("operator-refresh-%d", operator.ID),
	}

	accessToken, refreshToken, err := s.tokenAuth.GenTokenPair(accessClaims, refreshClaims)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Update last login
	if err := s.operatorRepo.UpdateLastLogin(ctx, operator.ID); err != nil {
		s.getLogger().Error("failed to update last login",
			"operator_id", operator.ID,
			"error", err,
		)
	}

	// Audit log
	auditEntry := &platform.OperatorAuditLog{
		OperatorID:   operator.ID,
		Action:       platform.ActionLogin,
		ResourceType: platform.ResourceOperator,
		ResourceID:   &operator.ID,
		RequestIP:    clientIP,
	}
	if err := s.auditLogRepo.Create(ctx, auditEntry); err != nil {
		s.getLogger().Error("failed to create audit log",
			"operator_id", operator.ID,
			"action", platform.ActionLogin,
			"error", err,
		)
	}

	return accessToken, refreshToken, operator, nil
}

// RefreshToken validates the operator is still active and issues a new JWT token pair.
// No DB token table is involved — it verifies the operator exists and is active, then generates fresh tokens.
func (s *operatorAuthService) RefreshToken(ctx context.Context, operatorID int64) (string, string, error) {
	operator, err := s.operatorRepo.FindByID(ctx, operatorID)
	if err != nil {
		return "", "", fmt.Errorf("failed to find operator: %w", err)
	}
	if operator == nil {
		return "", "", &OperatorNotFoundError{OperatorID: operatorID}
	}
	if !operator.Active {
		return "", "", &OperatorInactiveError{OperatorID: operatorID}
	}

	accessClaims := jwt.AppClaims{
		ID:          int(operator.ID),
		Sub:         fmt.Sprintf("operator:%d", operator.ID),
		Username:    operator.Email,
		FirstName:   operator.DisplayName,
		LastName:    "",
		Roles:       []string{"operator"},
		Permissions: []string{},
		IsAdmin:     false,
		Scope:       "platform",
	}

	refreshClaims := jwt.RefreshClaims{
		ID:    int(operator.ID),
		Token: fmt.Sprintf("operator-refresh-%d", operator.ID),
	}

	accessToken, refreshToken, err := s.tokenAuth.GenTokenPair(accessClaims, refreshClaims)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate tokens: %w", err)
	}

	return accessToken, refreshToken, nil
}

// ValidateOperator validates an operator's credentials without generating tokens
func (s *operatorAuthService) ValidateOperator(ctx context.Context, email, password string) (*platform.Operator, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	operator, err := s.operatorRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if operator == nil {
		return nil, &InvalidCredentialsError{}
	}

	if !operator.Active {
		return nil, &OperatorInactiveError{OperatorID: operator.ID}
	}

	// Verify password using userpass package
	match, err := userpass.VerifyPassword(password, operator.PasswordHash)
	if err != nil || !match {
		return nil, &InvalidCredentialsError{}
	}

	return operator, nil
}

// GetOperator retrieves an operator by ID
func (s *operatorAuthService) GetOperator(ctx context.Context, id int64) (*platform.Operator, error) {
	operator, err := s.operatorRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if operator == nil {
		return nil, &OperatorNotFoundError{OperatorID: id}
	}
	return operator, nil
}

// ListOperators retrieves all operators
func (s *operatorAuthService) ListOperators(ctx context.Context) ([]*platform.Operator, error) {
	return s.operatorRepo.List(ctx)
}

// UpdateProfile updates an operator's display name
func (s *operatorAuthService) UpdateProfile(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, &InvalidDataError{Err: fmt.Errorf("display name is required")}
	}
	if len(displayName) > 100 {
		return nil, &InvalidDataError{Err: fmt.Errorf("display name must not exceed 100 characters")}
	}

	operator, err := s.operatorRepo.FindByID(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	if operator == nil {
		return nil, &OperatorNotFoundError{OperatorID: operatorID}
	}

	operator.DisplayName = displayName
	if err := s.operatorRepo.Update(ctx, operator); err != nil {
		return nil, fmt.Errorf("failed to update operator profile: %w", err)
	}

	return operator, nil
}

// ChangePassword changes an operator's password after verifying the current one
func (s *operatorAuthService) ChangePassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	operator, err := s.operatorRepo.FindByID(ctx, operatorID)
	if err != nil {
		return err
	}
	if operator == nil {
		return &OperatorNotFoundError{OperatorID: operatorID}
	}

	// Verify current password
	match, err := userpass.VerifyPassword(currentPassword, operator.PasswordHash)
	if err != nil || !match {
		return &PasswordMismatchError{}
	}

	// Validate new password strength
	if err := authSvc.ValidatePasswordStrength(newPassword); err != nil {
		return &InvalidDataError{Err: fmt.Errorf("password doesn't meet complexity requirements")}
	}

	// Hash new password
	hash, err := authSvc.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	operator.PasswordHash = hash

	// Atomically update the password and invalidate any outstanding email-change
	// tokens. A surviving link after a password rotation would let an attacker
	// re-take the account.
	return tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.operatorRepo.Update(txCtx, operator); err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}
		if s.emailChangeTokenRepo != nil {
			if err := s.emailChangeTokenRepo.InvalidateByOperatorID(txCtx, operatorID); err != nil {
				return fmt.Errorf("failed to invalidate email change tokens after password change: %w", err)
			}
		}
		return nil
	})
}
