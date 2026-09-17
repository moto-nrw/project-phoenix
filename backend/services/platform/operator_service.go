package platform

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	emailpkg "github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// OperatorAuthService is the retained operator contract the MFA and passkey
// exchanges, the profile read, the e-mail change and the scheduler consume.
// Operator login, refresh, profile and password changes are Identity &
// Access routes (#3252, modules/identityaccess/inbound/operator); the token
// issue the second factors end in delegates to the consumer-owned
// OperatorSessions port the root binds to the module.
//
// Invitation operations and ListOperators live on the narrower
// OperatorInvitationService interface (see operator_invitation_interface.go).
// The concrete operatorAuthService struct satisfies both interfaces.
type OperatorAuthService interface {
	// IssueTokensForAuthenticatedOperator mints an access + refresh token
	// pair for an operator whose identity was proven via a non-password
	// channel, typically an MFA email-code, recovery-code or passkey
	// verification.
	IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)

	// GetOperator retrieves an operator by ID
	GetOperator(ctx context.Context, id int64) (*platform.Operator, error)

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
	OperatorAuthServiceConfig
	tenantRuntime *tenant.UnitOfWork
}

// SetTenantRuntime wires the transaction runtime used by the retained
// operator flows.
func (s *operatorAuthService) SetTenantRuntime(runtime tenant.UnitOfWork) {
	s.tenantRuntime = &runtime
}

func (s *operatorAuthService) withTenantRuntime(ctx context.Context) context.Context {
	if s.tenantRuntime == nil {
		return ctx
	}
	return tenant.WithUnitOfWork(ctx, *s.tenantRuntime)
}

func detachedOperatorContext(ctx context.Context) context.Context {
	ctx = context.WithoutCancel(ctx)
	ctx = tenant.ContextWithoutTransaction(ctx)
	return tenant.ContextWithoutAfterCommitHooks(ctx)
}

// OperatorAuthServiceConfig holds configuration for the retained operator
// service. OperatorRepo and Sessions are the consumer-owned ports over the
// Identity & Access owner.
type OperatorAuthServiceConfig struct {
	OperatorRepo         OperatorDirectory
	Sessions             OperatorSessions
	AuditLogRepo         platform.OperatorAuditLogRepository
	EmailChangeTokenRepo OperatorEmailChangeTokens
	InvitationTokenRepo  OperatorInvitationTokens
	DB                   *bun.DB
	Logger               *slog.Logger
	Dispatcher           *emailpkg.Dispatcher
	DefaultFrom          emailpkg.Email
	FrontendURL          string
	OperatorFrontendURL  string
	EmailChangeExpiry    time.Duration
	InvitationExpiry     time.Duration
}

// ErrOperatorSessionsUnavailable reports a service composed without the
// Identity & Access operator authentication port.
var ErrOperatorSessionsUnavailable = errors.New("operator sessions are not composed")

// NewOperatorAuthService creates the retained operator service. Returns the
// combined interface so the factory can expose it through both the narrow
// OperatorAuthService and OperatorInvitationService at the same time.
func NewOperatorAuthService(cfg OperatorAuthServiceConfig) (OperatorAuthAndInvitationService, error) {
	if cfg.OperatorRepo == nil {
		return nil, errors.New("operator auth service: operator directory is required")
	}
	return &operatorAuthService{OperatorAuthServiceConfig: cfg}, nil
}

func (s *operatorAuthService) getLogger() *slog.Logger {
	return cmp.Or(s.Logger, slog.Default())
}

// IssueTokensForAuthenticatedOperator delegates to the Identity & Access
// operator authentication.
func (s *operatorAuthService) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error) {
	if s.Sessions == nil {
		return "", "", ErrOperatorSessionsUnavailable
	}
	return s.Sessions.IssueTokensForAuthenticatedOperator(ctx, operatorID, ipAddress, userAgent)
}

// GetOperator retrieves an operator by ID
func (s *operatorAuthService) GetOperator(ctx context.Context, id int64) (*platform.Operator, error) {
	operator, err := s.OperatorRepo.FindByID(ctx, id)
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
	return s.OperatorRepo.List(ctx)
}
