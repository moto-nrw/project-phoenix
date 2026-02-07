package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/uptrace/bun"
)

// OperatorAuthService handles operator authentication
type OperatorAuthService interface {
	// Login authenticates an operator and returns JWT tokens
	Login(ctx context.Context, email, password string, clientIP net.IP) (accessToken, refreshToken string, operator *platform.Operator, err error)

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
}

type operatorAuthService struct {
	operatorRepo platform.OperatorRepository
	auditLogRepo platform.OperatorAuditLogRepository
	tokenAuth    *jwt.TokenAuth
	db           *bun.DB
	logger       *slog.Logger
}

// OperatorAuthServiceConfig holds configuration for the operator auth service
type OperatorAuthServiceConfig struct {
	OperatorRepo platform.OperatorRepository
	AuditLogRepo platform.OperatorAuditLogRepository
	DB           *bun.DB
	Logger       *slog.Logger
}

// NewOperatorAuthService creates a new operator auth service
func NewOperatorAuthService(cfg OperatorAuthServiceConfig) (OperatorAuthService, error) {
	tokenAuth, err := jwt.NewTokenAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to create token auth: %w", err)
	}

	return &operatorAuthService{
		operatorRepo: cfg.OperatorRepo,
		auditLogRepo: cfg.AuditLogRepo,
		tokenAuth:    tokenAuth,
		db:           cfg.DB,
		logger:       cfg.Logger,
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
	if err := s.operatorRepo.Update(ctx, operator); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}
