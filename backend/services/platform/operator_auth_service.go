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

// OperatorAuthService handles operator authentication, session management,
// profile updates, password changes, and email-change verification.
//
// Invitation operations (invite, validate, accept, list, resend, revoke,
// cleanup) and ListOperators live on the narrower OperatorInvitationService
// interface (see operator_invitation_interface.go). The concrete
// operatorAuthService struct satisfies both interfaces.
type OperatorAuthService interface {
	// Login authenticates an operator and returns JWT tokens
	Login(ctx context.Context, email, password string, clientIP net.IP) (accessToken, refreshToken string, operator *platform.Operator, err error)

	// LoginWithMFAGate is the MFA-aware sibling of Login. After a valid
	// credential check it consults the optional MFAService and returns
	// either a full token pair or a short-lived challenge token the caller
	// must redeem at /operator/auth/mfa/verify. trustedDeviceCookie may be
	// empty; when set and verifiable, MFA is skipped.
	LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*OperatorLoginResult, error)

	// SetMFAService wires the optional MFA gate post-construction.
	SetMFAService(svc OperatorMFAService)

	// IssueTokensForAuthenticatedOperator mints an access + refresh token pair
	// for an operator whose identity was proven via a non-password channel —
	// typically an MFA email-code or recovery-code verification. Skips
	// password validation but otherwise reuses the same token-pair pipeline
	// as a regular login, so the resulting session is indistinguishable.
	// Mirrors authService.IssueTokensForAuthenticatedAccount.
	IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)

	// RefreshToken validates the operator is still active and issues a new token pair
	RefreshToken(ctx context.Context, operatorID int64) (accessToken, refreshToken string, err error)

	// ValidateOperator validates an operator's credentials without generating tokens
	ValidateOperator(ctx context.Context, email, password string) (*platform.Operator, error)

	// GetOperator retrieves an operator by ID
	GetOperator(ctx context.Context, id int64) (*platform.Operator, error)

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
	invitationTokenRepo  platform.OperatorInvitationTokenRepository
	tokenAuth            *jwt.TokenAuth
	db                   *bun.DB
	logger               *slog.Logger
	dispatcher           *emailpkg.Dispatcher
	defaultFrom          emailpkg.Email
	frontendURL          string
	operatorFrontendURL  string
	emailChangeExpiry    time.Duration
	invitationExpiry     time.Duration
	// mfaService is optional. When non-nil the LoginWithMFAGate path uses it
	// to gate token issuance behind a second factor. Wired post-construction
	// via SetMFAService to break the OperatorAuthService ↔ OperatorMFAService
	// cycle in the services factory.
	mfaService OperatorMFAService
}

// SetMFAService wires the optional MFA gate. Idempotent — calling with nil
// clears the gate. Mirrors the auth-side AuthService.SetMFAService.
func (s *operatorAuthService) SetMFAService(svc OperatorMFAService) {
	s.mfaService = svc
}

// OperatorAuthServiceConfig holds configuration for the operator auth service
type OperatorAuthServiceConfig struct {
	OperatorRepo         platform.OperatorRepository
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

// NewOperatorAuthService creates a new operator auth service. Returns the
// combined interface so the factory can expose it through both the narrow
// OperatorAuthService and OperatorInvitationService at the same time.
func NewOperatorAuthService(cfg OperatorAuthServiceConfig) (OperatorAuthAndInvitationService, error) {
	tokenAuth, err := jwt.NewTokenAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to create token auth: %w", err)
	}

	return &operatorAuthService{
		operatorRepo:         cfg.OperatorRepo,
		auditLogRepo:         cfg.AuditLogRepo,
		emailChangeTokenRepo: cfg.EmailChangeTokenRepo,
		invitationTokenRepo:  cfg.InvitationTokenRepo,
		tokenAuth:            tokenAuth,
		db:                   cfg.DB,
		logger:               cfg.Logger,
		dispatcher:           cfg.Dispatcher,
		defaultFrom:          cfg.DefaultFrom,
		frontendURL:          cfg.FrontendURL,
		operatorFrontendURL:  cfg.OperatorFrontendURL,
		emailChangeExpiry:    cfg.EmailChangeExpiry,
		invitationExpiry:     cfg.InvitationExpiry,
	}, nil
}

func (s *operatorAuthService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// OperatorLoginStatus discriminates between the two shapes the operator
// /login response can take. Mirror of authService.LoginStatus.
type OperatorLoginStatus string

const (
	OperatorLoginStatusAuthenticated OperatorLoginStatus = "authenticated"
	OperatorLoginStatusMFARequired   OperatorLoginStatus = "mfa_required"
)

// OperatorLoginResult is the discriminated response shape for
// LoginWithMFAGate. Mirror of authService.LoginResult.
type OperatorLoginResult struct {
	Status                OperatorLoginStatus
	AccessToken           string
	RefreshToken          string
	Operator              *platform.Operator
	ChallengeToken        string
	MaskedEmail           string
	MFAEnrollmentRequired bool
	// TrustedDeviceEnabled is populated on the MFA-required branch only.
	// Operator MFA has no per-tenant toggle today — the feature is
	// always on — but the field is kept for symmetry with the tenant
	// response shape so the frontend code can stay identical.
	TrustedDeviceEnabled bool
	// TrustedDeviceDays mirrors the tenant field. Derived from the
	// hardcoded OperatorMFATrustedDeviceDuration constant — operators
	// don't expose this as a configurable setting today.
	TrustedDeviceDays int
}

// LoginWithMFAGate is the MFA-aware sibling of Login. The original
// password-only Login stays untouched so existing callers and tests don't
// shift. Operators face hardcoded mandatory MFA — there is no IsRequired
// branch, only "is the operator already enrolled and not on a trusted
// device?".
func (s *operatorAuthService) LoginWithMFAGate(
	ctx context.Context,
	email, password, ipAddress, userAgent, trustedDeviceCookie string,
) (*OperatorLoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	clientIP := parseClientIPForOperator(ipAddress)

	// Step 1: credential check (same as Login).
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
	match, err := userpass.VerifyPassword(password, operator.PasswordHash)
	if err != nil || !match {
		return nil, &InvalidCredentialsError{}
	}

	// Step 2: MFA branch. If the gate isn't wired in (early phases /
	// disabled deployments) fall straight through to a token pair —
	// matches the behaviour of the original Login.
	if s.mfaService == nil {
		access, refresh, issueErr := s.issueOperatorTokenPair(ctx, operator, clientIP)
		if issueErr != nil {
			return nil, issueErr
		}
		return &OperatorLoginResult{
			Status:       OperatorLoginStatusAuthenticated,
			AccessToken:  access,
			RefreshToken: refresh,
			Operator:     operator,
		}, nil
	}

	enrolled, _ := s.mfaService.HasEnrollment(ctx, operator.ID)

	// Step 3: trusted-device cookie short-circuit. Only meaningful when
	// the operator is already enrolled — fresh enrollments need to go
	// through the challenge flow.
	trustedDeviceVerified := false
	if enrolled && trustedDeviceCookie != "" {
		ok, _ := s.mfaService.VerifyTrustedDevice(ctx, operator.ID, trustedDeviceCookie)
		trustedDeviceVerified = ok
	}

	// Step 4: decide which response to return.
	//   enrolled, !trusted → challenge
	//   enrolled,  trusted → tokens (MFA skipped)
	//  !enrolled           → tokens + MFAEnrollmentRequired flag
	//                        (frontend forces enrollment screen)
	if enrolled && !trustedDeviceVerified {
		challenge, chErr := s.mfaService.StartChallenge(ctx, operator.ID, clientIP)
		if chErr != nil {
			return nil, fmt.Errorf("start operator mfa challenge: %w", chErr)
		}
		return &OperatorLoginResult{
			Status:               OperatorLoginStatusMFARequired,
			ChallengeToken:       challenge,
			MaskedEmail:          maskOperatorEmailForUX(operator.Email),
			TrustedDeviceEnabled: true,
			TrustedDeviceDays:    int(OperatorMFATrustedDeviceDuration.Hours() / 24),
		}, nil
	}

	access, refresh, issueErr := s.issueOperatorTokenPair(ctx, operator, clientIP)
	if issueErr != nil {
		return nil, issueErr
	}
	_ = userAgent // operator audit log doesn't carry UA today; reserved for future
	return &OperatorLoginResult{
		Status:                OperatorLoginStatusAuthenticated,
		AccessToken:           access,
		RefreshToken:          refresh,
		Operator:              operator,
		MFAEnrollmentRequired: !enrolled,
	}, nil
}

// issueOperatorTokenPair mints the access + refresh JWT for an
// already-validated operator and writes the login audit row. Extracted from
// the original Login flow so LoginWithMFAGate can reuse it without changing
// the legacy code path.
func (s *operatorAuthService) issueOperatorTokenPair(ctx context.Context, operator *platform.Operator, clientIP net.IP) (string, string, error) {
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
	access, refresh, err := s.tokenAuth.GenTokenPair(accessClaims, refreshClaims)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate tokens: %w", err)
	}
	if err := s.operatorRepo.UpdateLastLogin(ctx, operator.ID); err != nil {
		s.getLogger().Error("failed to update last login",
			"operator_id", operator.ID,
			"error", err,
		)
	}
	entry := &platform.OperatorAuditLog{
		OperatorID:   operator.ID,
		Action:       platform.ActionLogin,
		ResourceType: platform.ResourceOperator,
		ResourceID:   &operator.ID,
		RequestIP:    clientIP,
	}
	if err := s.auditLogRepo.Create(ctx, entry); err != nil {
		s.getLogger().Error("failed to create audit log",
			"operator_id", operator.ID,
			"action", platform.ActionLogin,
			"error", err,
		)
	}
	return access, refresh, nil
}

// IssueTokensForAuthenticatedOperator mints an access + refresh token pair
// for an operator whose identity was proven via MFA email-code / recovery
// code. Loads + active-checks the operator, then delegates to
// issueOperatorTokenPair so audit + last-login behaviour matches a regular
// password login. Mirrors authService.IssueTokensForAuthenticatedAccount.
func (s *operatorAuthService) IssueTokensForAuthenticatedOperator(
	ctx context.Context,
	operatorID int64,
	ipAddress, userAgent string,
) (string, string, error) {
	_ = userAgent // operator audit log doesn't carry UA today; kept for parity
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
	return s.issueOperatorTokenPair(ctx, operator, parseClientIPForOperator(ipAddress))
}

// maskOperatorEmailForUX mirrors auth.maskEmailForUX so the operator UX
// matches the tenant one — show users which mailbox just received a code
// without leaking the full address.
func maskOperatorEmailForUX(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return email
	}
	local := email[:at]
	domain := email[at:]
	if len(local) <= 1 {
		return local + "***" + domain
	}
	return string(local[0]) + "***" + domain
}

// parseClientIPForOperator wraps net.ParseIP with the empty-string guard
// so audit rows don't get malformed inet values.
func parseClientIPForOperator(ipAddress string) net.IP {
	if ipAddress == "" {
		return nil
	}
	return net.ParseIP(ipAddress)
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
