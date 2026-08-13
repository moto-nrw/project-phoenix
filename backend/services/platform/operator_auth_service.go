package platform

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/rotation"
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

	// RefreshToken validates a persisted operator refresh session and rotates it.
	RefreshToken(ctx context.Context, operatorID int64, refreshTokenValue string) (accessToken, refreshToken string, err error)

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
	OperatorAuthServiceConfig
	tokenAuth *jwt.TokenAuth
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
	RefreshTokenRepo     platform.OperatorRefreshTokenRepository
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
		OperatorAuthServiceConfig: cfg,
		tokenAuth:                 tokenAuth,
	}, nil
}

func (s *operatorAuthService) getLogger() *slog.Logger {
	return cmp.Or(s.Logger, slog.Default())
}

// OperatorLoginStatus discriminates between the two shapes the operator
// /login response can take. Mirror of authService.LoginStatus.
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
	clientIP := authSvc.ParseClientIP(ipAddress)

	// Step 1: credential check (same as Login).
	operator, err := s.OperatorRepo.FindByEmail(ctx, email)
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

	// HasEnrollment now distinguishes sql.ErrNoRows (legitimate "not
	// enrolled" — every fresh operator hits it) from infra errors. On
	// infra errors we refuse THIS login with 503 instead of treating it
	// as not-enrolled, which would silently downgrade an enrolled
	// operator straight back to the enrollment-token flow.
	enrolled, err := s.mfaService.HasEnrollment(ctx, operator.ID)
	if err != nil {
		return nil, err
	}

	// Step 3: not-enrolled branch. Operator MFA is mandatory, so a
	// not-yet-enrolled operator MUST go through the narrow enrollment
	// surface before getting a full session. Previous design issued a
	// full token pair plus an MFAEnrollmentRequired hint; the flag was
	// advisory only and a direct API client could skip enrollment entirely.
	// Issue an enrollment-scoped JWT instead — the same defense the tenant
	// path uses.
	if !enrolled {
		enrollmentToken, err := s.tokenAuth.CreateMFAEnrollmentJWT(jwt.MFAEnrollmentClaims{
			AccountID: operator.ID,
			Scope:     jwt.MFAEnrollmentScopePlatform,
		}, authSvc.MFAEnrollmentTokenTTL)
		if err != nil {
			return nil, fmt.Errorf("issue operator mfa enrollment token: %w", err)
		}
		_ = userAgent // reserved for audit parity with tenant path
		return &OperatorLoginResult{
			Status:                OperatorLoginStatusMFAEnrollmentRequired,
			AccessToken:           enrollmentToken,
			Operator:              operator,
			MaskedEmail:           authSvc.MaskEmailForUX(operator.Email),
			MFAEnrollmentRequired: true,
		}, nil
	}

	// Step 4: trusted-device cookie short-circuit. Only meaningful when
	// the operator is already enrolled — fresh enrollments are handled by
	// Step 3 above.
	trustedDeviceVerified := false
	if trustedDeviceCookie != "" {
		ok, _ := s.mfaService.VerifyTrustedDevice(ctx, operator.ID, trustedDeviceCookie)
		trustedDeviceVerified = ok
	}

	// Step 5: enrolled + no trusted device → challenge; enrolled + trusted
	// device → tokens (MFA skipped).
	if !trustedDeviceVerified {
		challenge, chErr := s.mfaService.StartChallenge(ctx, operator.ID, clientIP)
		if chErr != nil {
			return nil, fmt.Errorf("start operator mfa challenge: %w", chErr)
		}
		return &OperatorLoginResult{
			Status:               OperatorLoginStatusMFARequired,
			ChallengeToken:       challenge,
			MaskedEmail:          authSvc.MaskEmailForUX(operator.Email),
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
		Status:       OperatorLoginStatusAuthenticated,
		AccessToken:  access,
		RefreshToken: refresh,
		Operator:     operator,
	}, nil
}

// issueOperatorTokenPair mints the access + refresh JWT for an
// already-validated operator and writes the login audit row. Extracted from
// the original Login flow so LoginWithMFAGate can reuse it without changing
// the legacy code path.
func (s *operatorAuthService) issueOperatorTokenPair(ctx context.Context, operator *platform.Operator, clientIP net.IP) (string, string, error) {
	refreshSession := s.newOperatorRefreshToken(operator.ID, "", 0)
	access, refresh, err := s.mintOperatorTokenPair(operator, refreshSession)
	if err != nil {
		return "", "", err
	}
	if err := s.persistOperatorRefreshToken(ctx, refreshSession); err != nil {
		return "", "", err
	}
	if err := s.OperatorRepo.UpdateLastLogin(ctx, operator.ID); err != nil {
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
	if err := s.AuditLogRepo.Create(ctx, entry); err != nil {
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
	operator, err := s.OperatorRepo.FindByID(ctx, operatorID)
	if err != nil {
		return "", "", fmt.Errorf("failed to find operator: %w", err)
	}
	if operator == nil {
		return "", "", &OperatorNotFoundError{OperatorID: operatorID}
	}
	if !operator.Active {
		return "", "", &OperatorInactiveError{OperatorID: operatorID}
	}
	return s.issueOperatorTokenPair(ctx, operator, authSvc.ParseClientIP(ipAddress))
}

func (s *operatorAuthService) operatorAccessClaims(operator *platform.Operator) jwt.AppClaims {
	return jwt.AppClaims{
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
}

func (s *operatorAuthService) newOperatorRefreshToken(operatorID int64, familyID string, generation int) *platform.OperatorRefreshToken {
	if familyID == "" {
		familyID = uuid.Must(uuid.NewV4()).String()
	}
	return &platform.OperatorRefreshToken{
		OperatorID: operatorID,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(s.tokenAuth.GetRefreshExpiry()),
		FamilyID:   familyID,
		Generation: generation,
	}
}

func (s *operatorAuthService) mintOperatorTokenPair(operator *platform.Operator, refreshToken *platform.OperatorRefreshToken) (string, string, error) {
	if refreshToken == nil {
		return "", "", fmt.Errorf("operator refresh token is required")
	}
	refreshClaims := jwt.RefreshClaims{
		ID:    int(operator.ID),
		Token: refreshToken.Token,
		Scope: "platform",
		CommonClaims: jwt.CommonClaims{
			ExpiresAt: refreshToken.Expiry.Unix(),
		},
	}
	access, refresh, err := s.tokenAuth.GenTokenPair(s.operatorAccessClaims(operator), refreshClaims)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate tokens: %w", err)
	}
	return access, refresh, nil
}

func (s *operatorAuthService) persistOperatorRefreshToken(ctx context.Context, refreshToken *platform.OperatorRefreshToken) error {
	if s.RefreshTokenRepo == nil {
		return fmt.Errorf("operator refresh token repository is not configured")
	}
	if err := s.RefreshTokenRepo.Create(ctx, refreshToken); err != nil {
		return fmt.Errorf("failed to persist operator refresh token: %w", err)
	}
	return nil
}

// Login authenticates an operator and returns JWT tokens
func (s *operatorAuthService) Login(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Find operator by email
	operator, err := s.OperatorRepo.FindByEmail(ctx, email)
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

	accessToken, refreshToken, err := s.issueOperatorTokenPair(ctx, operator, clientIP)
	if err != nil {
		return "", "", nil, err
	}

	return accessToken, refreshToken, operator, nil
}

// RefreshToken validates the persisted operator refresh session, rotates it,
// and issues a new JWT token pair. A refresh JWT without a matching DB row is
// stale, replayed, or from the pre-revocation stateless scheme and is rejected.
func (s *operatorAuthService) RefreshToken(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
	refreshTokenValue = strings.TrimSpace(refreshTokenValue)
	if refreshTokenValue == "" {
		return "", "", &OperatorRefreshTokenInvalidError{}
	}
	if s.RefreshTokenRepo == nil {
		return "", "", fmt.Errorf("operator refresh token repository is not configured")
	}

	var operator *platform.Operator
	var accessToken string
	var refreshToken string
	var rejectAfterCommit bool
	var recovered bool
	err := tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		dbToken, err := s.RefreshTokenRepo.FindByTokenForUpdate(txCtx, refreshTokenValue)
		if err != nil {
			return fmt.Errorf("failed to find operator refresh token: %w", err)
		}
		if dbToken == nil || dbToken.OperatorID != operatorID {
			return &OperatorRefreshTokenInvalidError{}
		}

		now := time.Now()
		if now.After(dbToken.Expiry) {
			if err := s.deleteOperatorFamilyWithAudit(txCtx, dbToken, "token_expired"); err != nil {
				return fmt.Errorf("failed to delete expired operator refresh-token family: %w", err)
			}
			rejectAfterCommit = true
			s.logOperatorRefreshDecision("token_expired", operatorID, dbToken.Generation)
			return nil
		}

		dbToken, recovered, err = s.resolveOperatorRefreshHandoff(txCtx, dbToken, now)
		if err != nil {
			var invalidRefresh *OperatorRefreshTokenInvalidError
			if !errors.As(err, &invalidRefresh) {
				return err
			}
			if err := s.deleteOperatorFamilyWithAudit(txCtx, dbToken, "replay_detected"); err != nil {
				return fmt.Errorf("failed to revoke replayed operator refresh-token family: %w", err)
			}
			rejectAfterCommit = true
			s.logOperatorRefreshDecision("replay_detected", operatorID, dbToken.Generation)
			return nil
		}

		latestToken, err := s.RefreshTokenRepo.GetLatestTokenInFamily(txCtx, dbToken.FamilyID)
		if err != nil {
			return fmt.Errorf("failed to inspect operator refresh token family: %w", err)
		}
		if latestToken != nil && latestToken.Generation > dbToken.Generation {
			if err := s.deleteOperatorFamilyWithAudit(txCtx, dbToken, "lineage_mismatch"); err != nil {
				return fmt.Errorf("failed to revoke operator refresh token family: %w", err)
			}
			rejectAfterCommit = true
			s.logOperatorRefreshDecision("lineage_mismatch", operatorID, dbToken.Generation)
			return nil
		}

		operator, err = s.OperatorRepo.FindByID(txCtx, operatorID)
		if err != nil {
			return fmt.Errorf("failed to find operator: %w", err)
		}
		if operator == nil {
			return &OperatorNotFoundError{OperatorID: operatorID}
		}
		if !operator.Active {
			return &OperatorInactiveError{OperatorID: operatorID}
		}

		if recovered {
			accessToken, refreshToken, err = s.mintOperatorTokenPair(operator, dbToken)
			if err != nil {
				return err
			}
		} else {
			newDBToken := s.newOperatorRefreshToken(operator.ID, dbToken.FamilyID, dbToken.Generation+1)
			accessToken, refreshToken, err = s.mintOperatorTokenPair(operator, newDBToken)
			if err != nil {
				return err
			}
			if err := s.RefreshTokenRepo.Create(txCtx, newDBToken); err != nil {
				return fmt.Errorf("failed to persist rotated operator refresh token: %w", err)
			}
			if err := s.RefreshTokenRepo.MarkRotated(txCtx, dbToken.ID, newDBToken.Token, rotation.RecoveryProofHash(txCtx), now); err != nil {
				return fmt.Errorf("failed to persist operator refresh-token handoff: %w", err)
			}
			if err := s.RefreshTokenRepo.DeleteExpiredRotated(txCtx, dbToken.FamilyID, now); err != nil {
				return fmt.Errorf("failed to clean operator refresh-token handoffs: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	if rejectAfterCommit {
		return "", "", &OperatorRefreshTokenInvalidError{}
	}
	if recovered {
		s.getLogger().Info("operator_refresh_rotation_recovered",
			slog.Int64("operator_id", operatorID),
		)
	}

	return accessToken, refreshToken, nil
}

func (s *operatorAuthService) resolveOperatorRefreshHandoff(ctx context.Context, presented *platform.OperatorRefreshToken, now time.Time) (*platform.OperatorRefreshToken, bool, error) {
	current := presented
	// The request proves possession of the token it presented. Successor hops
	// are accepted only after validating their persisted family and generation
	// lineage; each hop may have been rotated under a different access token.
	proofValidated := false
	for hop := 0; hop < rotation.MaxRecoveryHops; hop++ {
		if current.RotatedAt == nil {
			return current, current.ID != presented.ID, nil
		}
		if current.ReplacementToken == nil || current.RotatedAt.After(now) || now.Sub(*current.RotatedAt) > rotation.RecoveryGrace {
			return current, false, &OperatorRefreshTokenInvalidError{}
		}
		if !proofValidated {
			if !rotation.MatchesRecoveryProof(ctx, current.RecoveryProofHash) {
				return current, false, &OperatorRefreshTokenInvalidError{}
			}
			proofValidated = true
		}

		next, err := s.RefreshTokenRepo.FindByTokenForUpdate(ctx, *current.ReplacementToken)
		if err != nil {
			return current, false, fmt.Errorf("failed to follow operator refresh-token handoff: %w", err)
		}
		if next == nil || next.FamilyID != current.FamilyID || next.OperatorID != current.OperatorID || next.Generation != current.Generation+1 {
			return current, false, &OperatorRefreshTokenInvalidError{}
		}
		current = next
	}
	return current, false, &OperatorRefreshTokenInvalidError{}
}

func (s *operatorAuthService) logOperatorRefreshDecision(reason string, operatorID int64, generation int) {
	s.getLogger().Warn("operator_refresh_session_rejected",
		slog.String("reason", reason),
		slog.Int64("operator_id", operatorID),
		slog.Int("generation", generation),
	)
}

// ValidateOperator validates an operator's credentials without generating tokens
func (s *operatorAuthService) ValidateOperator(ctx context.Context, email, password string) (*platform.Operator, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	operator, err := s.OperatorRepo.FindByEmail(ctx, email)
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

// UpdateProfile updates an operator's display name
func (s *operatorAuthService) UpdateProfile(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, &InvalidDataError{Err: fmt.Errorf("display name is required")}
	}
	if len(displayName) > 100 {
		return nil, &InvalidDataError{Err: fmt.Errorf("display name must not exceed 100 characters")}
	}

	operator, err := s.OperatorRepo.FindByID(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	if operator == nil {
		return nil, &OperatorNotFoundError{OperatorID: operatorID}
	}

	operator.DisplayName = displayName
	if err := s.OperatorRepo.Update(ctx, operator); err != nil {
		return nil, fmt.Errorf("failed to update operator profile: %w", err)
	}

	return operator, nil
}

// ChangePassword changes an operator's password after verifying the current one
func (s *operatorAuthService) ChangePassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	operator, err := s.OperatorRepo.FindByID(ctx, operatorID)
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

	if s.RefreshTokenRepo == nil {
		return fmt.Errorf("operator refresh token repository is not configured")
	}

	// Atomically update the password and invalidate outstanding bearer-style
	// controls. A surviving email-change link or refresh session after a
	// password rotation would let an attacker re-take or keep the account.
	return tenant.WithAdminTx(ctx, s.DB, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.OperatorRepo.Update(txCtx, operator); err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}
		if s.EmailChangeTokenRepo != nil {
			if err := s.EmailChangeTokenRepo.InvalidateByOperatorID(txCtx, operatorID); err != nil {
				return fmt.Errorf("failed to invalidate email change tokens after password change: %w", err)
			}
		}
		if err := s.deleteAllOperatorTokensWithAudit(txCtx, operatorID, "password_change"); err != nil {
			return fmt.Errorf("failed to revoke refresh tokens after password change: %w", err)
		}
		return nil
	})
}
