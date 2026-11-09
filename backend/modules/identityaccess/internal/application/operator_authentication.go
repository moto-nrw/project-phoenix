package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// OperatorAuthentication runs operator login, the MFA-proven token issue,
// refresh with rotation recovery, profile and password changes and the
// session revocation they entail (#3252) over the owner's operator service
// and the consumer-owned ports. Operators are platform-wide: the flows open
// an administrative transaction where rotation or revocation and its audit
// evidence must commit together and otherwise run on the root connection.
type OperatorAuthentication struct {
	operators      *Service
	passwordPolicy func() (int, time.Duration, error)
	passwords      ports.PasswordVerifier
	hasher         ports.PasswordHasher
	codec          ports.TokenCodec
	mfa            ports.OperatorMFAGate
	audit          ports.OperatorAudit
	credentials    ports.OperatorCredentialCleanup
	runtime        ports.Runtime
	// sessionUnit is the runtime's WithAdminTx, the one unit in which an
	// operator session is minted (login) or rotated (refresh).
	sessionUnit func(context.Context, func(context.Context) error) error
	attach      func(context.Context) context.Context
	rotation    ports.Rotation
	logger      *slog.Logger
}

// OperatorAuthenticationDependencies are the ports the operator flows
// consume.
type OperatorAuthenticationDependencies struct {
	PasswordPolicy func() (int, time.Duration, error)
	Passwords      ports.PasswordVerifier
	Hasher         ports.PasswordHasher
	Codec          ports.TokenCodec
	MFA            ports.OperatorMFAGate
	Audit          ports.OperatorAudit
	Credentials    ports.OperatorCredentialCleanup
	Runtime        ports.Runtime
	// Attach binds the composed unit of work ahead of every public flow.
	Attach   func(context.Context) context.Context
	Rotation ports.Rotation
	Logger   *slog.Logger
}

// NewOperatorAuthentication composes the flows over the operator service
// that owns platform.operators and platform.operator_refresh_tokens.
func NewOperatorAuthentication(operators *Service, deps OperatorAuthenticationDependencies) (*OperatorAuthentication, error) {
	switch {
	case operators == nil, deps.PasswordPolicy == nil, deps.Passwords == nil, deps.Hasher == nil, deps.Codec == nil, deps.MFA == nil,
		deps.Audit == nil, deps.Credentials == nil, deps.Runtime == nil, deps.Rotation == nil:
		return nil, errors.New("identity access operator authentication: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &OperatorAuthentication{
		operators: operators, passwords: deps.Passwords, passwordPolicy: deps.PasswordPolicy, hasher: deps.Hasher, codec: deps.Codec, mfa: deps.MFA,
		audit: deps.Audit, credentials: deps.Credentials, runtime: deps.Runtime, sessionUnit: deps.Runtime.WithAdminTx, attach: deps.Attach, rotation: deps.Rotation,
		logger: logger.With("component", "operator-authentication"),
	}, nil
}

// LoginWithMFAGate checks the credentials and then consults the MFA gate.
// Operator MFA is mandatory: an enrolled operator without a verifiable
// trusted-device cookie receives a challenge token, an operator not yet
// enrolled receives an enrollment-scoped token that only authorizes the
// enrollment surface, and an unconfigured gate issues the token pair
// directly.
func (s *OperatorAuthentication) loginWithMFAGate(ctx context.Context, email, password, ipAddress, _, trustedDeviceCookie string) (*domain.OperatorLoginResult, error) {
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
	enrolled, err := s.mfa.HasOperatorMFAEnrollment(ctx, operator.ID)
	if err != nil {
		return nil, err
	}
	if !enrolled {
		token, err := s.codec.IssueMFAEnrollmentToken(operator.ID, 0, domain.PortalPlatform.Name, domain.MFAEnrollmentTokenTTL)
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
		ok, _ := s.mfa.VerifyOperatorTrustedDevice(ctx, operator.ID, trustedDeviceCookie)
		trusted = ok
	}
	if !trusted {
		challenge, err := s.mfa.StartOperatorMFAChallenge(ctx, operator.ID, net.ParseIP(strings.TrimSpace(ipAddress)))
		if err != nil {
			return nil, fmt.Errorf("start operator mfa challenge: %w", err)
		}
		return &domain.OperatorLoginResult{
			Status:               domain.LoginStatusMFARequired,
			ChallengeToken:       challenge,
			MaskedEmail:          domain.MaskEmail(operator.Email),
			TrustedDeviceEnabled: true,
			TrustedDeviceDays:    s.mfa.OperatorTrustedDeviceDays(),
		}, nil
	}
	return s.authenticatedResult(ctx, operator, ipAddress)
}

func (s *OperatorAuthentication) validateCredentials(ctx context.Context, email, password string) (domain.Operator, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var operator domain.Operator
	var accepted bool
	err := globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		var found bool
		var err error
		operator, found, _, err = s.operators.operators.FindOperatorByEmail(txCtx, email)
		if err != nil {
			return err
		}
		if found {
			operator, found, _, err = s.operators.operators.FindOperator(txCtx, operator.ID, true)
			if err != nil {
				return err
			}
		}
		if !found {
			operator = domain.Operator{}
		}
		accepted, err = checkPasswordLogin(txCtx, s.operators.operators, s.passwords, s.passwordPolicy, operator.ID, operator.PasswordHash, password, true)
		return err
	})
	if err != nil {
		return domain.Operator{}, err
	}
	if !accepted {
		return domain.Operator{}, identityaccess.ErrOperatorInvalidCredentials
	}
	if !operator.Active {
		return domain.Operator{}, identityaccess.ErrOperatorInactive
	}
	return operator, nil
}

func (s *OperatorAuthentication) authenticatedResult(ctx context.Context, operator domain.Operator, ipAddress string) (*domain.OperatorLoginResult, error) {
	issued, access, refresh, err := s.issueTokenPair(ctx, operator.ID, ipAddress)
	if err != nil {
		return nil, err
	}
	return &domain.OperatorLoginResult{Status: domain.LoginStatusAuthenticated, AccessToken: access, RefreshToken: refresh, Operator: &issued}, nil
}

// IssueTokensForAuthenticatedOperator mints a token pair for an operator
// whose identity was proven via a non-password channel (MFA code, recovery
// code or passkey). It skips the credential check but otherwise reuses the
// login pipeline, so the session is indistinguishable from a password
// login.
func (s *OperatorAuthentication) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, _ string) (_, _ string, err error) {
	if s == nil {
		return "", "", errOperatorAuthenticationUnavailable
	}
	defer func() { err = operatorError(err) }()
	_, access, refresh, err := s.issueTokenPair(bindRuntime(s.attach, ctx), operatorID, ipAddress)
	return access, refresh, err
}

// issueTokenPair re-reads the operator, persists the refresh session, stamps
// the login and writes the login audit row in one administrative
// transaction, and signs the pair inside it: an operator deactivated after
// the credential check gets no session, and a login that cannot be recorded
// issues none either.
func (s *OperatorAuthentication) issueTokenPair(ctx context.Context, operatorID int64, ipAddress string) (operator domain.Operator, access, refresh string, err error) {
	session := s.newRefreshSession(operatorID, "", 0)
	entry := domain.OperatorAuditEntry{
		OperatorID: operatorID, Action: domain.OperatorAuditActionLogin, ResourceType: domain.OperatorAuditResourceOperator,
		ResourceID: &operatorID, IPAddress: ipAddress,
	}
	err = s.sessionUnit(ctx, func(txCtx context.Context) error {
		var err error
		if operator, err = s.activeOperator(txCtx, operatorID); err != nil {
			return err
		}
		if _, err := s.operators.CreateOperatorSession(txCtx, ports.OperatorSession(session)); err != nil {
			return fmt.Errorf("failed to persist operator refresh token: %w", err)
		}
		if err := s.operators.RecordOperatorLogin(txCtx, operator.ID); err != nil {
			return fmt.Errorf("failed to update last login: %w", err)
		}
		if err := s.audit.RecordOperatorAction(txCtx, entry); err != nil {
			return fmt.Errorf("failed to create audit log: %w", err)
		}
		access, refresh, err = s.mintTokenPair(operator, session)
		return err
	})
	if err != nil {
		return domain.Operator{}, "", "", err
	}
	return operator, access, refresh, nil
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
		AccountID:    operator.ID,
		Email:        domain.OperatorSubject(operator.ID),
		Username:     operator.Email,
		FirstName:    operator.DisplayName,
		Roles:        []string{domain.OperatorRoleName},
		RoleIsSystem: []bool{true},
		Permissions:  []string{},
		Scope:        domain.PortalPlatform.Claim,
		FamilyID:     session.FamilyID,
	}
	refresh := domain.RefreshClaims{AccountID: operator.ID, Token: session.Token, Scope: domain.PortalPlatform.Claim, ExpiresAt: session.Expiry.Unix()}
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
//
// The signed token names the operator and the session handle. Tokens of
// other portals and the stateless operator tokens from before persisted
// sessions, which carry no platform scope, are refused before the lookup.
func (s *OperatorAuthentication) RefreshToken(ctx context.Context, refreshJWT string, proof domain.RecoveryProof) (string, string, error) {
	claims, err := s.codec.ParseRefreshToken(refreshJWT)
	if err != nil || claims.Scope != domain.PortalPlatform.Claim {
		return "", "", identityaccess.ErrOperatorRefreshTokenInvalid
	}
	operatorID, refreshTokenValue := claims.AccountID, strings.TrimSpace(claims.Token)
	if refreshTokenValue == "" {
		return "", "", identityaccess.ErrOperatorRefreshTokenInvalid
	}
	var (
		accessToken, refreshToken string
		rejectAfterCommit         bool
		recovered                 bool
	)
	err = s.sessionUnit(ctx, func(txCtx context.Context) error {
		var err error
		accessToken, refreshToken, recovered, rejectAfterCommit, err = s.rotateOperatorRefresh(txCtx, operatorID, refreshTokenValue, proof)
		return err
	})
	if err != nil {
		return "", "", err
	}
	if rejectAfterCommit {
		return "", "", identityaccess.ErrOperatorRefreshTokenInvalid
	}
	if recovered {
		s.logger.Info("operator_refresh_rotation_recovered",
			slog.Int64("operator_id", operatorID))
	}
	return accessToken, refreshToken, nil
}

// handoffs follows the rotation hand-offs of operator refresh sessions.
func (s *OperatorAuthentication) handoffs() domain.HandoffChain[domain.OperatorSession] {
	return domain.HandoffChain[domain.OperatorSession]{
		Link: func(session domain.OperatorSession) domain.HandoffLink {
			return domain.HandoffLink{ID: session.ID, Token: session.Token, RotatedAt: session.RotatedAt, ReplacementTokenHash: session.ReplacementTokenHash, RecoveryProofHash: session.RecoveryProofHash}
		},
		Successor: s.operators.findOperatorSessionForUpdate,
		SameLineage: func(current, next domain.OperatorSession) bool {
			return next.FamilyID == current.FamilyID && next.OperatorID == current.OperatorID && next.Generation == current.Generation+1
		},
		Grace:    s.rotation.RecoveryGrace(),
		MaxHops:  s.rotation.MaxRecoveryHops(),
		Invalid:  identityaccess.ErrOperatorRefreshTokenInvalid,
		NotFound: identityaccess.ErrOperatorSessionNotFound,
		Portal:   "operator",
	}
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
	// UpdateOperator edits the row it locked, so a status change committed
	// meanwhile is kept rather than reverted by this edit.
	updated, err := s.operators.UpdateOperator(ctx, operatorID, func(operator *identityaccess.Operator) error {
		operator.DisplayName = displayName
		return nil
	})
	if errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return domain.Operator{}, identityaccess.ErrOperatorNotFound
	}
	if err != nil {
		return domain.Operator{}, fmt.Errorf("failed to update operator profile: %w", err)
	}
	return domain.Operator(updated), nil
}

// ChangePassword rotates the password after verifying the current one and
// atomically invalidates the outstanding bearer-style controls: a surviving
// e-mail change link or refresh session after a password rotation would let
// an attacker re-take or keep the account.
func (s *OperatorAuthentication) ChangePassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	operator, err := s.operators.findOperatorByID(ctx, operatorID)
	if errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return identityaccess.ErrOperatorNotFound
	}
	if err != nil {
		return err
	}
	match, err := s.passwords.VerifyPassword(currentPassword, operator.PasswordHash)
	if err != nil || !match {
		return identityaccess.ErrOperatorPasswordMismatch
	}
	if err := s.hasher.ValidatePasswordStrength(newPassword); err != nil {
		return &domain.InvalidInputError{Err: domain.ErrOperatorPasswordTooWeak}
	}
	hash, err := s.hasher.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	return s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		return s.storeRotatedPassword(txCtx, operatorID, operator.PasswordHash, hash)
	})
}

// storeRotatedPassword writes the new hash on the caller's transaction and
// invalidates the e-mail change links and refresh sessions with it. The
// verification ran without a lock and the hash is slow, so the hash is set on
// the row UpdateOperator locked: a deactivation committed meanwhile stays in
// force, and a password rotated meanwhile (verifiedHash no longer stored)
// invalidates the proof.
func (s *OperatorAuthentication) storeRotatedPassword(ctx context.Context, operatorID int64, verifiedHash, hash string) error {
	_, err := s.operators.UpdateOperator(ctx, operatorID, func(current *identityaccess.Operator) error {
		if current.PasswordHash != verifiedHash {
			return identityaccess.ErrOperatorPasswordMismatch
		}
		current.PasswordHash = hash
		return nil
	})
	if errors.Is(err, identityaccess.ErrOperatorPasswordMismatch) || errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return err
	}
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	if err := s.credentials.RevokeEmailChanges(ctx, operatorID); err != nil {
		return fmt.Errorf("failed to invalidate email change tokens after password change: %w", err)
	}
	if err := s.revokeAllWithAudit(ctx, operatorID, "password_change"); err != nil {
		return fmt.Errorf("failed to revoke refresh tokens after password change: %w", err)
	}
	return nil
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

type OperatorAudit struct{ Source identityPorts.OperatorAudit }

func (a OperatorAudit) RecordOperatorAction(ctx context.Context, entry domain.OperatorAuditEntry) error {
	return a.Source.RecordOperatorAction(ctx, publicOperatorAuditEntry(entry))
}

type OrganizationDirectory struct {
	Source identityPorts.OrganizationDirectory
}

func (d OrganizationDirectory) ListSchools(ctx context.Context, ids []int64) ([]domain.School, error) {
	schools, err := d.Source.ListSchools(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]domain.School, 0, len(schools))
	for _, school := range schools {
		result = append(result, domain.School(school))
	}
	return result, nil
}

func (d OrganizationDirectory) ListOrganizationNames(ctx context.Context, ids []int64) ([]domain.OrganizationName, error) {
	organizations, err := d.Source.ListOrganizationNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]domain.OrganizationName, 0, len(organizations))
	for _, organization := range organizations {
		result = append(result, domain.OrganizationName(organization))
	}
	return result, nil
}

type SchoolIdentityProvisioner struct {
	identityPorts.SchoolIdentityProvisioner
	Lifecycle *AccountLifecycle
}

func (p SchoolIdentityProvisioner) ListAccountPersons(ctx context.Context, accountID int64) ([]domain.AccountPersonIdentity, error) {
	persons, err := p.SchoolIdentityProvisioner.ListAccountPersons(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AccountPersonIdentity, 0, len(persons))
	for _, person := range persons {
		result = append(result, domain.AccountPersonIdentity(person))
	}
	return result, nil
}

func (p SchoolIdentityProvisioner) EnsureSchoolIdentity(ctx context.Context, request domain.SchoolIdentityRequest) error {
	if p.Lifecycle != nil {
		role := request.Role
		_, err := p.Lifecycle.EnsureSchoolIdentityChain(ctx, domain.SchoolIdentityInput{
			AccountID: request.AccountID, TenantID: request.TenantID,
			Role: &domain.RoleFacts{
				ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole,
				SchoolPortal: role.SchoolPortal,
			},
			FirstName: request.FirstName, LastName: request.LastName, Position: request.Position, CreatePerson: request.CreatePerson,
		})
		if errors.Is(err, domain.ErrSchoolIdentityNamesRequired) || errors.Is(err, domain.ErrSchoolIdentityPersonIsStudent) {
			return &domain.InvalidInputError{Err: err}
		}
		return err
	}
	err := p.SchoolIdentityProvisioner.EnsureSchoolIdentity(ctx, identityPorts.SchoolIdentityRequest{
		AccountID: request.AccountID, TenantID: request.TenantID, Role: schoolRoleFact(request.Role),
		FirstName: request.FirstName, LastName: request.LastName, Position: request.Position, CreatePerson: request.CreatePerson,
	})
	if invalid, ok := errors.AsType[*identityaccess.InvalidInputError](err); ok {
		return &domain.InvalidInputError{Err: invalid.Err}
	}
	return err
}

func (p SchoolIdentityProvisioner) ListAccountIdentityFacts(ctx context.Context, accountID int64) ([]domain.AccountIdentityFact, error) {
	facts, err := p.SchoolIdentityProvisioner.ListAccountIdentityFacts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AccountIdentityFact, 0, len(facts))
	for _, fact := range facts {
		result = append(result, domain.AccountIdentityFact(fact))
	}
	return result, nil
}

// schoolRolePolicy binds the public school-role policy every school-access
// path shares (#3314). A role with ID zero is one that does not exist; the
// validation reports why a role may not be handed out at the school with the
// message the operator sees.
type SchoolRolePolicy struct{}

func (SchoolRolePolicy) ValidateAssignableSchoolRole(role domain.RoleFact, tenantID int64) error {
	return identityaccess.ValidateAssignableSchoolRole(operatorRoleFacts(role), tenantID)
}

func (SchoolRolePolicy) IsLehrkraftRole(role domain.RoleFact) bool {
	return identityaccess.IsLehrkraftRole(operatorRoleFacts(role))
}

// RoleNeedsStaffRecord classifies the fact as given, ID zero included, as the
// operator flows always did; only the lookup-shaped decisions treat ID zero
// as no role.
func (SchoolRolePolicy) RoleNeedsStaffRecord(role domain.RoleFact) bool {
	return identityaccess.RoleNeedsStaffRecord(&identityaccess.RoleFacts{
		ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole,
		SchoolPortal: role.SchoolPortal,
	})
}

func (SchoolRolePolicy) LehrkraftRoleImmutable() error {
	return identityaccess.ErrLehrkraftRoleImmutable
}

// publicOperatorAuditEntry renders one ledger entry across the module
// boundary, with the typed evidence each action carries.
func publicOperatorAuditEntry(entry domain.OperatorAuditEntry) identityaccess.OperatorAuditEntry {
	public := identityaccess.OperatorAuditEntry{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, IPAddress: entry.IPAddress,
	}
	if entry.RevokedSessions != nil {
		evidence := identityaccess.RevokedSessionsEvidence(*entry.RevokedSessions)
		public.RevokedSessions = &evidence
	}
	if entry.AccessChange != nil {
		change := identityaccess.OperatorAccessChange(*entry.AccessChange)
		public.AccessChange = &change
	}
	if entry.MFA != nil {
		evidence := identityaccess.OperatorMFAEvidence{Reason: entry.MFA.Reason, LockedUntil: entry.MFA.LockedUntil}
		if entry.MFA.Override != nil {
			override := identityaccess.OperatorMFAOverrideEvidence(*entry.MFA.Override)
			evidence.Override = &override
		}
		public.MFA = &evidence
	}
	return public
}

// operatorRoleFacts projects a resolved role; ID zero is no role at all.
func operatorRoleFacts(role domain.RoleFact) *identityaccess.RoleFacts {
	if role.ID <= 0 {
		return nil
	}
	return &identityaccess.RoleFacts{ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole, SchoolPortal: role.SchoolPortal}
}

func schoolRoleFact(role domain.RoleFact) identityaccess.SchoolRole {
	return identityaccess.SchoolRole{ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole, SchoolPortal: role.SchoolPortal}
}

func (s *OperatorAuthentication) mintRotatedOperatorSession(txCtx context.Context, operator domain.Operator, session domain.OperatorSession, recovered bool, proof domain.RecoveryProof, now time.Time) (accessToken, refreshToken string, err error) {
	if recovered {
		accessToken, refreshToken, err = s.mintTokenPair(operator, session)
		return accessToken, refreshToken, err
	}
	// The successor follows from the presented token, so a lost response
	// can be recomputed although only hashes are stored (#4902).
	successor := s.newRefreshSession(operator.ID, session.FamilyID, session.Generation+1)
	successor.Token = domain.SuccessorRefreshToken(session.Token)
	accessToken, refreshToken, err = s.mintTokenPair(operator, successor)
	if err != nil {
		return accessToken, refreshToken, err
	}
	if _, err := s.operators.CreateOperatorSession(txCtx, ports.OperatorSession(successor)); err != nil {
		return accessToken, refreshToken, fmt.Errorf("failed to persist rotated operator refresh token: %w", err)
	}
	if err := s.operators.MarkOperatorSessionRotated(txCtx, session.ID, successor.Token, proof.Hash(), now); err != nil {
		return accessToken, refreshToken, fmt.Errorf("failed to persist operator refresh-token handoff: %w", err)
	}
	if err := s.operators.DeleteExpiredRotatedOperatorSessions(txCtx, session.FamilyID, now); err != nil {
		return accessToken, refreshToken, fmt.Errorf("failed to clean operator refresh-token handoffs: %w", err)
	}
	return accessToken, refreshToken, nil
}

func (s *OperatorAuthentication) rejectOperatorLineage(txCtx context.Context, session domain.OperatorSession, operatorID int64) (bool, error) {
	latest, err := s.operators.LatestOperatorSessionInFamily(txCtx, session.FamilyID)
	if err != nil && !errors.Is(err, identityaccess.ErrOperatorSessionNotFound) {
		return false, fmt.Errorf("failed to inspect operator refresh token family: %w", err)
	}
	if err == nil && latest.Generation > session.Generation {
		if err := s.revokeFamilyWithAudit(txCtx, session, "lineage_mismatch"); err != nil {
			return false, fmt.Errorf("failed to revoke operator refresh token family: %w", err)
		}
		s.logRefreshRejected("lineage_mismatch", operatorID, session.Generation)
		return true, nil
	}
	return false, nil
}

func (s *OperatorAuthentication) rotateOperatorRefresh(txCtx context.Context, operatorID int64, refreshTokenValue string, proof domain.RecoveryProof) (accessToken, refreshToken string, recovered, rejectAfterCommit bool, err error) {
	// The operator row first, then the session: the order every command
	// that ends the operator's sessions (deactivation, password change)
	// takes, so a rotation either commits before such a command reads the
	// sessions or reads the operator after it committed (#6289).
	if _, err := s.operators.findOperatorForUpdate(txCtx, operatorID); err != nil && !errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return accessToken, refreshToken, recovered, rejectAfterCommit, fmt.Errorf("failed to lock operator: %w", err)
	}
	session, err := s.operators.findOperatorSessionForUpdate(txCtx, refreshTokenValue)
	if errors.Is(err, identityaccess.ErrOperatorSessionNotFound) {
		return accessToken, refreshToken, recovered, rejectAfterCommit, identityaccess.ErrOperatorRefreshTokenInvalid
	}
	if err != nil {
		return accessToken, refreshToken, recovered, rejectAfterCommit, fmt.Errorf("failed to find operator refresh token: %w", err)
	}
	if session.OperatorID != operatorID {
		return accessToken, refreshToken, recovered, rejectAfterCommit, identityaccess.ErrOperatorRefreshTokenInvalid
	}
	now := time.Now()
	if now.After(session.Expiry) {
		if err := s.revokeFamilyWithAudit(txCtx, session, "token_expired"); err != nil {
			return accessToken, refreshToken, recovered, rejectAfterCommit, fmt.Errorf("failed to delete expired operator refresh-token family: %w", err)
		}
		rejectAfterCommit = true
		s.logRefreshRejected("token_expired", operatorID, session.Generation)
		return accessToken, refreshToken, recovered, rejectAfterCommit, nil
	}
	session, recovered, err = s.handoffs().Resolve(txCtx, session, proof, now)
	if err != nil {
		if !errors.Is(err, identityaccess.ErrOperatorRefreshTokenInvalid) {
			return accessToken, refreshToken, recovered, rejectAfterCommit, err
		}
		if err := s.revokeFamilyWithAudit(txCtx, session, "replay_detected"); err != nil {
			return accessToken, refreshToken, recovered, rejectAfterCommit, fmt.Errorf("failed to revoke replayed operator refresh-token family: %w", err)
		}
		rejectAfterCommit = true
		s.logRefreshRejected("replay_detected", operatorID, session.Generation)
		return accessToken, refreshToken, recovered, rejectAfterCommit, nil
	}
	rejectAfterCommit, err = s.rejectOperatorLineage(txCtx, session, operatorID)
	if err != nil || rejectAfterCommit {
		return accessToken, refreshToken, recovered, rejectAfterCommit, err
	}
	operator, err := s.activeOperator(txCtx, operatorID)
	if err != nil {
		return accessToken, refreshToken, recovered, rejectAfterCommit, err
	}
	accessToken, refreshToken, err = s.mintRotatedOperatorSession(txCtx, operator, session, recovered, proof, now)
	return accessToken, refreshToken, recovered, rejectAfterCommit, err
}

// activeOperator locks the operator inside a session transaction and
// refuses a missing or deactivated one. The lock orders a mint behind a
// deactivation that ends the operator's sessions (#6289).
func (s *OperatorAuthentication) activeOperator(txCtx context.Context, operatorID int64) (domain.Operator, error) {
	operator, err := s.operators.findOperatorForUpdate(txCtx, operatorID)
	if errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return domain.Operator{}, identityaccess.ErrOperatorNotFound
	}
	if err != nil {
		return domain.Operator{}, fmt.Errorf("failed to find operator: %w", err)
	}
	if !operator.Active {
		return domain.Operator{}, identityaccess.ErrOperatorInactive
	}
	return operator, nil
}

func (s *OperatorAuthentication) LoginOperatorWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
	return runFlow(ctx, s.flow(), func(ctx context.Context) (*identityaccess.OperatorLoginResult, error) {
		result, err := s.loginWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
		return operatorLoginResult(result), err
	})
}

func (s *OperatorAuthentication) RefreshOperatorToken(ctx context.Context, refreshToken, recoveryProof string) (string, string, error) {
	return runFlow2(ctx, s.flow(), func(ctx context.Context) (string, string, error) {
		access, refresh, err := s.RefreshToken(ctx, refreshToken, domain.NewRecoveryProof(recoveryProof))
		return access, refresh, err
	})
}

func (s *OperatorAuthentication) UpdateOperatorProfile(ctx context.Context, operatorID int64, displayName string) (identityaccess.Operator, error) {
	return runFlow(ctx, s.flow(), func(ctx context.Context) (identityaccess.Operator, error) {
		operator, err := s.UpdateProfile(ctx, operatorID, displayName)
		return identityaccess.Operator(operator), err
	})
}

func (s *OperatorAuthentication) ChangeOperatorPassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	return runFlowErr(ctx, s.flow(), func(ctx context.Context) error {
		return s.ChangePassword(ctx, operatorID, currentPassword, newPassword)
	})
}
