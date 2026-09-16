package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// OperatorAuthentication runs operator login, the MFA-proven token issue,
// refresh with rotation recovery, profile and password changes and the
// session revocation they entail (#3252) over the owner's operator service
// and the consumer-owned ports. Operators are platform-wide: the flows open
// an administrative transaction where rotation or revocation and its audit
// evidence must commit together and otherwise run on the root connection.
type OperatorAuthentication struct {
	operators   *Service
	passwords   ports.PasswordVerifier
	hasher      ports.PasswordHasher
	codec       ports.TokenCodec
	mfa         ports.OperatorMFAGate
	audit       ports.OperatorAudit
	credentials ports.OperatorCredentialCleanup
	runtime     ports.Runtime
	rotation    ports.Rotation
	logger      *slog.Logger
}

// OperatorAuthenticationDependencies are the ports the operator flows
// consume.
type OperatorAuthenticationDependencies struct {
	Passwords   ports.PasswordVerifier
	Hasher      ports.PasswordHasher
	Codec       ports.TokenCodec
	MFA         ports.OperatorMFAGate
	Audit       ports.OperatorAudit
	Credentials ports.OperatorCredentialCleanup
	Runtime     ports.Runtime
	Rotation    ports.Rotation
	Logger      *slog.Logger
}

// NewOperatorAuthentication composes the flows over the operator service
// that owns platform.operators and platform.operator_refresh_tokens.
func NewOperatorAuthentication(operators *Service, deps OperatorAuthenticationDependencies) (*OperatorAuthentication, error) {
	switch {
	case operators == nil, deps.Passwords == nil, deps.Hasher == nil, deps.Codec == nil, deps.MFA == nil,
		deps.Audit == nil, deps.Credentials == nil, deps.Runtime == nil, deps.Rotation == nil:
		return nil, errors.New("identity access operator authentication: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &OperatorAuthentication{
		operators: operators, passwords: deps.Passwords, hasher: deps.Hasher, codec: deps.Codec, mfa: deps.MFA,
		audit: deps.Audit, credentials: deps.Credentials, runtime: deps.Runtime, rotation: deps.Rotation,
		logger: logger.With("component", "operator-authentication"),
	}, nil
}

// LoginWithMFAGate checks the credentials and then consults the MFA gate.
// Operator MFA is mandatory: an enrolled operator without a verifiable
// trusted-device cookie receives a challenge token, an operator not yet
// enrolled receives an enrollment-scoped token that only authorizes the
// enrollment surface, and an unconfigured gate issues the token pair
// directly.
func (s *OperatorAuthentication) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, _, trustedDeviceCookie string) (*domain.OperatorLoginResult, error) {
	operator, err := s.validateCredentials(ctx, email, password)
	if err != nil {
		return nil, err
	}
	if !s.mfa.Configured() {
		return s.authenticatedResult(ctx, operator, ipAddress)
	}
	// An infrastructure error refuses this login instead of treating the
	// operator as not enrolled, which would silently downgrade an enrolled
	// operator to the enrollment flow.
	enrolled, err := s.mfa.HasEnrollment(ctx, operator.ID)
	if err != nil {
		return nil, err
	}
	if !enrolled {
		token, err := s.codec.IssueMFAEnrollmentToken(operator.ID, 0, domain.OperatorMFAEnrollmentScope, domain.MFAEnrollmentTokenTTL)
		if err != nil {
			return nil, fmt.Errorf("issue operator mfa enrollment token: %w", err)
		}
		return &domain.OperatorLoginResult{
			Status:                domain.LoginStatusMFAEnrollmentRequired,
			AccessToken:           token,
			Operator:              &operator,
			MaskedEmail:           domain.MaskEmail(operator.Email),
			MFAEnrollmentRequired: true,
		}, nil
	}
	trusted := false
	if trustedDeviceCookie != "" {
		ok, _ := s.mfa.VerifyTrustedDevice(ctx, operator.ID, trustedDeviceCookie)
		trusted = ok
	}
	if !trusted {
		challenge, err := s.mfa.StartChallenge(ctx, operator.ID, ipAddress)
		if err != nil {
			return nil, fmt.Errorf("start operator mfa challenge: %w", err)
		}
		return &domain.OperatorLoginResult{
			Status:               domain.LoginStatusMFARequired,
			ChallengeToken:       challenge,
			MaskedEmail:          domain.MaskEmail(operator.Email),
			TrustedDeviceEnabled: true,
			TrustedDeviceDays:    s.mfa.TrustedDeviceDays(),
		}, nil
	}
	return s.authenticatedResult(ctx, operator, ipAddress)
}

func (s *OperatorAuthentication) validateCredentials(ctx context.Context, email, password string) (domain.Operator, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	operator, err := s.operators.FindOperatorByEmail(ctx, email)
	if errors.Is(err, domain.ErrOperatorNotFound) {
		return domain.Operator{}, domain.ErrOperatorInvalidCredentials
	}
	if err != nil {
		return domain.Operator{}, err
	}
	if !operator.Active {
		return domain.Operator{}, domain.ErrOperatorInactive
	}
	match, err := s.passwords.VerifyPassword(password, operator.PasswordHash)
	if err != nil || !match {
		return domain.Operator{}, domain.ErrOperatorInvalidCredentials
	}
	return operator, nil
}

func (s *OperatorAuthentication) authenticatedResult(ctx context.Context, operator domain.Operator, ipAddress string) (*domain.OperatorLoginResult, error) {
	access, refresh, err := s.issueTokenPair(ctx, operator, ipAddress)
	if err != nil {
		return nil, err
	}
	return &domain.OperatorLoginResult{Status: domain.LoginStatusAuthenticated, AccessToken: access, RefreshToken: refresh, Operator: &operator}, nil
}

// IssueTokensForAuthenticatedOperator mints a token pair for an operator
// whose identity was proven via a non-password channel (MFA code, recovery
// code or passkey). It skips the credential check but otherwise reuses the
// login pipeline, so the session is indistinguishable from a password
// login.
func (s *OperatorAuthentication) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, _ string) (string, string, error) {
	operator, err := s.operators.FindOperator(ctx, operatorID)
	if errors.Is(err, domain.ErrOperatorNotFound) {
		return "", "", domain.ErrOperatorNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to find operator: %w", err)
	}
	if !operator.Active {
		return "", "", domain.ErrOperatorInactive
	}
	return s.issueTokenPair(ctx, operator, ipAddress)
}

// issueTokenPair mints the access and refresh JWT for a validated operator,
// persists the refresh session, stamps the login and writes the login audit
// row. A failed login stamp or audit row is logged, never fatal: the
// session exists and the tokens are valid.
func (s *OperatorAuthentication) issueTokenPair(ctx context.Context, operator domain.Operator, ipAddress string) (string, string, error) {
	session := s.newRefreshSession(operator.ID, "", 0)
	access, refresh, err := s.mintTokenPair(operator, session)
	if err != nil {
		return "", "", err
	}
	if _, err := s.operators.CreateOperatorSession(ctx, session); err != nil {
		return "", "", fmt.Errorf("failed to persist operator refresh token: %w", err)
	}
	if err := s.operators.RecordOperatorLogin(ctx, operator.ID); err != nil {
		s.logger.Error("failed to update last login",
			slog.Int64("operator_id", operator.ID),
			slog.Any("error", err))
	}
	entry := domain.OperatorAuditEntry{
		OperatorID: operator.ID, Action: domain.OperatorAuditActionLogin, ResourceType: domain.OperatorAuditResourceOperator,
		ResourceID: &operator.ID, IPAddress: ipAddress,
	}
	if err := s.audit.RecordOperatorAction(ctx, entry); err != nil {
		s.logger.Error("failed to create audit log",
			slog.Int64("operator_id", operator.ID),
			slog.String("action", domain.OperatorAuditActionLogin),
			slog.Any("error", err))
	}
	return access, refresh, nil
}

func (s *OperatorAuthentication) newRefreshSession(operatorID int64, familyID string, generation int) domain.OperatorSession {
	if familyID == "" {
		familyID = uuid.Must(uuid.NewV4()).String()
	}
	return domain.OperatorSession{
		OperatorID: operatorID,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(s.codec.RefreshExpiry()),
		FamilyID:   familyID,
		Generation: generation,
	}
}

// mintTokenPair signs the operator claims: the subject names the operator,
// the username carries the address, the display name travels as the first
// name and the single operator role marks the platform scope.
func (s *OperatorAuthentication) mintTokenPair(operator domain.Operator, session domain.OperatorSession) (string, string, error) {
	access := domain.SessionClaims{
		AccountID:   operator.ID,
		Email:       domain.OperatorSubject(operator.ID),
		Username:    operator.Email,
		FirstName:   operator.DisplayName,
		Roles:       []string{domain.OperatorRoleName},
		Permissions: []string{},
		Scope:       domain.OperatorScope,
		FamilyID:    session.FamilyID,
	}
	refresh := domain.RefreshClaims{AccountID: operator.ID, Token: session.Token, Scope: domain.OperatorScope, ExpiresAt: session.Expiry.Unix()}
	accessToken, refreshToken, err := s.codec.IssueTokenPair(access, refresh)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate tokens: %w", err)
	}
	return accessToken, refreshToken, nil
}

// RefreshToken validates the persisted operator refresh session, rotates it
// and issues a new token pair. A refresh handle without a matching row is
// stale, replayed or from the pre-revocation stateless scheme and is
// rejected. An expired session, a replay outside the recovery grace and a
// lineage mismatch revoke the whole family with audit evidence and reject
// after that revocation committed; a lookup that fails for infrastructure
// reasons rolls back and stays retryable.
func (s *OperatorAuthentication) RefreshToken(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
	refreshTokenValue = strings.TrimSpace(refreshTokenValue)
	if refreshTokenValue == "" {
		return "", "", domain.ErrOperatorRefreshTokenInvalid
	}
	var (
		accessToken, refreshToken string
		rejectAfterCommit         bool
		recovered                 bool
	)
	err := s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		session, err := s.operators.FindOperatorSessionForUpdate(txCtx, refreshTokenValue)
		if errors.Is(err, domain.ErrOperatorSessionNotFound) {
			return domain.ErrOperatorRefreshTokenInvalid
		}
		if err != nil {
			return fmt.Errorf("failed to find operator refresh token: %w", err)
		}
		if session.OperatorID != operatorID {
			return domain.ErrOperatorRefreshTokenInvalid
		}
		now := time.Now()
		if now.After(session.Expiry) {
			if err := s.revokeFamilyWithAudit(txCtx, session, "token_expired"); err != nil {
				return fmt.Errorf("failed to delete expired operator refresh-token family: %w", err)
			}
			rejectAfterCommit = true
			s.logRefreshRejected("token_expired", operatorID, session.Generation)
			return nil
		}
		session, recovered, err = s.resolveRefreshHandoff(txCtx, session, now)
		if err != nil {
			if !errors.Is(err, domain.ErrOperatorRefreshTokenInvalid) {
				return err
			}
			if err := s.revokeFamilyWithAudit(txCtx, session, "replay_detected"); err != nil {
				return fmt.Errorf("failed to revoke replayed operator refresh-token family: %w", err)
			}
			rejectAfterCommit = true
			s.logRefreshRejected("replay_detected", operatorID, session.Generation)
			return nil
		}
		latest, err := s.operators.LatestOperatorSessionInFamily(txCtx, session.FamilyID)
		if err != nil && !errors.Is(err, domain.ErrOperatorSessionNotFound) {
			return fmt.Errorf("failed to inspect operator refresh token family: %w", err)
		}
		if err == nil && latest.Generation > session.Generation {
			if err := s.revokeFamilyWithAudit(txCtx, session, "lineage_mismatch"); err != nil {
				return fmt.Errorf("failed to revoke operator refresh token family: %w", err)
			}
			rejectAfterCommit = true
			s.logRefreshRejected("lineage_mismatch", operatorID, session.Generation)
			return nil
		}
		operator, err := s.operators.FindOperator(txCtx, operatorID)
		if errors.Is(err, domain.ErrOperatorNotFound) {
			return domain.ErrOperatorNotFound
		}
		if err != nil {
			return fmt.Errorf("failed to find operator: %w", err)
		}
		if !operator.Active {
			return domain.ErrOperatorInactive
		}
		if recovered {
			accessToken, refreshToken, err = s.mintTokenPair(operator, session)
			return err
		}
		successor := s.newRefreshSession(operator.ID, session.FamilyID, session.Generation+1)
		accessToken, refreshToken, err = s.mintTokenPair(operator, successor)
		if err != nil {
			return err
		}
		if _, err := s.operators.CreateOperatorSession(txCtx, successor); err != nil {
			return fmt.Errorf("failed to persist rotated operator refresh token: %w", err)
		}
		if err := s.operators.MarkOperatorSessionRotated(txCtx, session.ID, successor.Token, s.rotation.RecoveryProofHash(txCtx), now); err != nil {
			return fmt.Errorf("failed to persist operator refresh-token handoff: %w", err)
		}
		if err := s.operators.DeleteExpiredRotatedOperatorSessions(txCtx, session.FamilyID, now); err != nil {
			return fmt.Errorf("failed to clean operator refresh-token handoffs: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	if rejectAfterCommit {
		return "", "", domain.ErrOperatorRefreshTokenInvalid
	}
	if recovered {
		s.logger.Info("operator_refresh_rotation_recovered",
			slog.Int64("operator_id", operatorID))
	}
	return accessToken, refreshToken, nil
}

// resolveRefreshHandoff follows the rotation hand-offs of a presented
// session within the recovery grace. The request proves possession of the
// token it presented; successor hops are accepted only after validating
// their persisted family and generation lineage, because each hop may have
// been rotated under a different access token.
func (s *OperatorAuthentication) resolveRefreshHandoff(ctx context.Context, presented domain.OperatorSession, now time.Time) (domain.OperatorSession, bool, error) {
	current := presented
	proofValidated := false
	for hop := 0; hop < s.rotation.MaxRecoveryHops(); hop++ {
		if current.RotatedAt == nil {
			return current, current.ID != presented.ID, nil
		}
		if current.ReplacementToken == nil || current.RotatedAt.After(now) || now.Sub(*current.RotatedAt) > s.rotation.RecoveryGrace() {
			return current, false, domain.ErrOperatorRefreshTokenInvalid
		}
		if !proofValidated {
			if !s.rotation.MatchesRecoveryProof(ctx, current.RecoveryProofHash) {
				return current, false, domain.ErrOperatorRefreshTokenInvalid
			}
			proofValidated = true
		}
		next, err := s.operators.FindOperatorSessionForUpdate(ctx, *current.ReplacementToken)
		if errors.Is(err, domain.ErrOperatorSessionNotFound) {
			return current, false, domain.ErrOperatorRefreshTokenInvalid
		}
		if err != nil {
			return current, false, fmt.Errorf("failed to follow operator refresh-token handoff: %w", err)
		}
		if next.FamilyID != current.FamilyID || next.OperatorID != current.OperatorID || next.Generation != current.Generation+1 {
			return current, false, domain.ErrOperatorRefreshTokenInvalid
		}
		current = next
	}
	return current, false, domain.ErrOperatorRefreshTokenInvalid
}

func (s *OperatorAuthentication) logRefreshRejected(reason string, operatorID int64, generation int) {
	s.logger.Warn("operator_refresh_session_rejected",
		slog.String("reason", reason),
		slog.Int64("operator_id", operatorID),
		slog.Int("generation", generation))
}

// UpdateProfile changes the operator's display name.
func (s *OperatorAuthentication) UpdateProfile(ctx context.Context, operatorID int64, displayName string) (domain.Operator, error) {
	displayName, err := domain.ValidateOperatorDisplayName(displayName)
	if err != nil {
		return domain.Operator{}, err
	}
	operator, err := s.operators.FindOperator(ctx, operatorID)
	if errors.Is(err, domain.ErrOperatorNotFound) {
		return domain.Operator{}, domain.ErrOperatorNotFound
	}
	if err != nil {
		return domain.Operator{}, err
	}
	operator.DisplayName = displayName
	updated, err := s.operators.UpdateOperator(ctx, operator)
	if err != nil {
		return domain.Operator{}, fmt.Errorf("failed to update operator profile: %w", err)
	}
	return updated, nil
}

// ChangePassword rotates the password after verifying the current one and
// atomically invalidates the outstanding bearer-style controls: a surviving
// e-mail change link or refresh session after a password rotation would let
// an attacker re-take or keep the account.
func (s *OperatorAuthentication) ChangePassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	operator, err := s.operators.FindOperator(ctx, operatorID)
	if errors.Is(err, domain.ErrOperatorNotFound) {
		return domain.ErrOperatorNotFound
	}
	if err != nil {
		return err
	}
	match, err := s.passwords.VerifyPassword(currentPassword, operator.PasswordHash)
	if err != nil || !match {
		return domain.ErrOperatorPasswordMismatch
	}
	if err := s.hasher.ValidatePasswordStrength(newPassword); err != nil {
		return domain.InvalidInput("password doesn't meet complexity requirements")
	}
	hash, err := s.hasher.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	operator.PasswordHash = hash
	return s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if _, err := s.operators.UpdateOperator(txCtx, operator); err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}
		if err := s.credentials.InvalidateEmailChangeTokens(txCtx, operatorID); err != nil {
			return fmt.Errorf("failed to invalidate email change tokens after password change: %w", err)
		}
		if err := s.revokeAllWithAudit(txCtx, operatorID, "password_change"); err != nil {
			return fmt.Errorf("failed to revoke refresh tokens after password change: %w", err)
		}
		return nil
	})
}

// revokeFamilyWithAudit deletes the session's family and records the
// revocation on the caller's transaction.
func (s *OperatorAuthentication) revokeFamilyWithAudit(ctx context.Context, session domain.OperatorSession, reason string) error {
	deleted, err := s.operators.RevokeOperatorSessionFamily(ctx, session.FamilyID)
	if err != nil {
		return err
	}
	return s.auditRevocation(ctx, session.OperatorID, session.FamilyID, reason, len(deleted))
}

// revokeAllWithAudit deletes every session of the operator and records one
// revocation per family, in family order.
func (s *OperatorAuthentication) revokeAllWithAudit(ctx context.Context, operatorID int64, reason string) error {
	deleted, err := s.operators.RevokeOperatorSessions(ctx, operatorID)
	if err != nil {
		return err
	}
	counts := make(map[string]int)
	for _, session := range deleted {
		counts[session.FamilyID]++
	}
	families := make([]string, 0, len(counts))
	for familyID := range counts {
		families = append(families, familyID)
	}
	sort.Strings(families)
	for _, familyID := range families {
		if err := s.auditRevocation(ctx, operatorID, familyID, reason, counts[familyID]); err != nil {
			return err
		}
	}
	return nil
}

func (s *OperatorAuthentication) auditRevocation(ctx context.Context, operatorID int64, familyID, reason string, count int) error {
	entry := domain.OperatorAuditEntry{
		OperatorID: operatorID, Action: domain.OperatorAuditActionTokenRevoked, ResourceType: domain.OperatorAuditResourceOperator,
		ResourceID: &operatorID,
		RevokedSessions: &domain.RevokedSessionsEvidence{
			PortalScope:       "operator",
			FamilyFingerprint: s.rotation.FamilyFingerprint(familyID),
			Reason:            reason,
			Count:             count,
		},
	}
	if err := s.audit.RecordOperatorAction(ctx, entry); err != nil {
		return fmt.Errorf("audit operator token revocation: %w", err)
	}
	return nil
}
