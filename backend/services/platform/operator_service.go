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

// OperatorAuthService is the retained operator contract the operator
// routes, the MFA and passkey exchanges and the scheduler consume. Operator
// login, refresh, profile and password changes are owned by Identity &
// Access (#3252); the operator routes have no target rule to import its
// public package yet, so this contract delegates those methods to the
// consumer-owned OperatorSessions port the root binds to the module. The
// operator lookup and the e-mail change stay here.
//
// Invitation operations and ListOperators live on the narrower
// OperatorInvitationService interface (see operator_invitation_interface.go).
// The concrete operatorAuthService struct satisfies both interfaces.
type OperatorAuthService interface {
	// LoginWithMFAGate checks the credentials and consults the mandatory
	// operator MFA gate. It returns either a full token pair, a short-lived
	// challenge token the caller must redeem at /operator/auth/mfa/verify,
	// or an enrollment-scoped token. trustedDeviceCookie may be empty; when
	// set and verifiable, MFA is skipped.
	LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*OperatorLoginResult, error)

	// IssueTokensForAuthenticatedOperator mints an access + refresh token
	// pair for an operator whose identity was proven via a non-password
	// channel, typically an MFA email-code, recovery-code or passkey
	// verification.
	IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)

	// RefreshToken validates a persisted operator refresh session and rotates it.
	RefreshToken(ctx context.Context, operatorID int64, refreshTokenValue string) (accessToken, refreshToken string, err error)

	// GetOperator retrieves an operator by ID
	GetOperator(ctx context.Context, id int64) (*platform.Operator, error)

	// UpdateProfile updates an operator's display name
	UpdateProfile(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error)

	// ChangePassword changes an operator's password after verifying the
	// current one and revokes the operator's sessions.
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

// OperatorLoginStatus discriminates between the shapes the operator login
// can take.
type OperatorLoginStatus string

const (
	OperatorLoginStatusAuthenticated OperatorLoginStatus = "authenticated"
	OperatorLoginStatusMFARequired   OperatorLoginStatus = "mfa_required"
	// OperatorLoginStatusMFAEnrollmentRequired mirrors the tenant-side
	// LoginStatusMFAEnrollmentRequired: credentials are valid but the
	// operator has not enrolled in MFA yet. Response carries a narrow
	// enrollment-scoped JWT (no refresh) that only authorizes
	// /operator/auth/mfa/enroll/*.
	OperatorLoginStatusMFAEnrollmentRequired OperatorLoginStatus = "mfa_enrollment_required"
)

// OperatorLoginResult is the discriminated response shape for
// LoginWithMFAGate.
type OperatorLoginResult struct {
	Status                OperatorLoginStatus
	AccessToken           string
	RefreshToken          string
	Operator              *platform.Operator
	ChallengeToken        string
	MaskedEmail           string
	MFAEnrollmentRequired bool
	// TrustedDeviceEnabled is populated on the MFA-required branch only.
	// Operator MFA has no per-tenant toggle; the feature is always on, but
	// the field is kept for symmetry with the tenant response shape.
	TrustedDeviceEnabled bool
	// TrustedDeviceDays derives from the hardcoded
	// OperatorMFATrustedDeviceDuration constant.
	TrustedDeviceDays int
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
	EmailChangeTokenRepo platform.OperatorEmailChangeTokenRepository
	InvitationTokenRepo  platform.OperatorInvitationTokenRepository
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

// LoginWithMFAGate delegates to the Identity & Access operator
// authentication.
func (s *operatorAuthService) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*OperatorLoginResult, error) {
	if s.Sessions == nil {
		return nil, ErrOperatorSessionsUnavailable
	}
	return s.Sessions.LoginWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
}

// IssueTokensForAuthenticatedOperator delegates to the Identity & Access
// operator authentication.
func (s *operatorAuthService) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error) {
	if s.Sessions == nil {
		return "", "", ErrOperatorSessionsUnavailable
	}
	return s.Sessions.IssueTokensForAuthenticatedOperator(ctx, operatorID, ipAddress, userAgent)
}

// RefreshToken delegates to the Identity & Access operator authentication.
func (s *operatorAuthService) RefreshToken(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
	if s.Sessions == nil {
		return "", "", ErrOperatorSessionsUnavailable
	}
	return s.Sessions.RefreshToken(ctx, operatorID, refreshTokenValue)
}

// UpdateProfile delegates to the Identity & Access operator authentication.
func (s *operatorAuthService) UpdateProfile(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error) {
	if s.Sessions == nil {
		return nil, ErrOperatorSessionsUnavailable
	}
	return s.Sessions.UpdateProfile(ctx, operatorID, displayName)
}

// ChangePassword delegates to the Identity & Access operator
// authentication.
func (s *operatorAuthService) ChangePassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	if s.Sessions == nil {
		return ErrOperatorSessionsUnavailable
	}
	return s.Sessions.ChangePassword(ctx, operatorID, currentPassword, newPassword)
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
