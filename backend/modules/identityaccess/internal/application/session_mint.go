package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
)

// Every account session of every portal is minted by mintAccountSession and
// renewed by the refresh rotation; both run the same steps under the same
// locks. Inside the write transaction, in this order:
//
//  1. the school's MFA policy lock, for portals that follow it;
//  2. the account row FOR UPDATE (lockLiveAccount), and for a password
//     sign-in the credential generation it checked (recheckPassword);
//  3. the proof re-check: a redeemed challenge, a remembered device or the
//     Träger session an Absprung starts from must still hold;
//  4. the portal's eligibility, which locks mapping and role rows FOR SHARE
//     and returns the claims the tokens are built from;
//  5. the live-account stamp, the source family and lineage, and the MFA
//     floor, which locks the permission sources last.
//
// The order is the one MFA reset, revocation and the settings writer walk:
// policy -> account -> mapping/roles -> permission sources. Checks before the
// transaction only choose a response (challenge, enrollment, wrong portal);
// the transaction is the authority.

// eligibility re-checks under the account lock that the account may hold a
// session of its portal and returns the claims the tokens are built from.
// It reads through the store on the caller's administrative transaction.
// The claims and the session commit together or not at all: a refresh that
// assembled its claims after the commit would have rotated the caller's
// token with nothing to hand back.
type eligibility func(ctx context.Context, account domain.LoginAccount) (*domain.AccountClaimsPayload, error)

// mintGuardError marks a refusal of the mint's authorization steps so the
// retry loop passes the caller's sentinel through.
type mintGuardError struct{ err error }

func (e *mintGuardError) Error() string { return e.err.Error() }
func (e *mintGuardError) Unwrap() error { return e.err }

func refused(err error) error {
	if err == nil {
		return nil
	}
	return &mintGuardError{err: err}
}

var (
	errMissingClaimsPayload              = errors.New("session claims payload missing after eligibility")
	errTraegerSchoolSwitchWithoutAddress = errors.New("identity access: a Träger school switch needs the client address for its audit entry")
)

// proofKind names what the holder proved beyond a password. The session's
// mfa_verified follows from it, never from a flag the caller sets.
type proofKind int

const (
	// proofNone: a password, or a switch whose source family supplies the
	// proof (adoptSourceFamily). The MFA floor decides whether that suffices.
	proofNone proofKind = iota
	// proofChallenge: a redeemed e-mail code of this portal.
	proofChallenge
	// proofPasskey: a WebAuthn assertion with user verification.
	proofPasskey
	// proofTrustedDevice: a device the account remembered after a challenge.
	proofTrustedDevice
	// proofDemoLink: the demo access link leaves by mail only (#3465), so
	// redeeming it proves the mailbox like an e-mail code. The demo school's
	// administrator holds privileged rights, which the floor never lets pass
	// without proof.
	proofDemoLink
	// proofTraegerSession: the Träger session an Absprung starts from;
	// verified carries its proof as read before the transaction.
	proofTraegerSession
)

type mintProof struct {
	kind                proofKind
	challenge           *identityaccess.VerifiedMFAChallenge
	trustedDeviceCookie string
	// traegerFamilyID and traegerVerified describe the source of an Absprung.
	traegerFamilyID string
	traegerVerified bool
}

func (p mintProof) verified() bool {
	switch p.kind {
	case proofChallenge, proofPasskey, proofTrustedDevice, proofDemoLink:
		return true
	case proofTraegerSession:
		return p.traegerVerified
	default:
		return false
	}
}

func challengeProof(challenge *identityaccess.VerifiedMFAChallenge) mintProof {
	if challenge == nil {
		return mintProof{}
	}
	return mintProof{kind: proofChallenge, challenge: challenge}
}

// mintAudit files the authentication event after the commit, at schoolID
// (zero: the account's first school).
type mintAudit struct {
	event     string
	ipAddress string
	userAgent string
	schoolID  int64
}

type mintRequest struct {
	portal domain.Portal
	// account is the identity the caller checked before the transaction; the
	// transaction re-reads it under its lock.
	account domain.LoginAccount
	// email is the address the access token names; empty means the account's.
	email    string
	tenantID int64
	orgID    int64
	proof    mintProof
	// op names the sign-in in refusals; booked and recheckMFAGate are the
	// reads of a parent and a school sign-in the portal's guard takes over.
	op             string
	booked         []int64
	recheckMFAGate bool
	// precondition runs under the account lock before the guard.
	precondition func(ctx context.Context, account domain.LoginAccount) error
	// sourceFamilyID is the family a switch or an Absprung starts from;
	// retireFamilyID the one it replaces (#2952). An Absprung leaves its
	// Träger family live.
	sourceFamilyID string
	retireFamilyID string
	// lifetime overrides the refresh lifetime (the Absprung hand-off).
	lifetime time.Duration
	// traegerSchoolAudit files traeger_school_switch at the school in the
	// mint transaction: no Träger school session exists without its entry.
	traegerSchoolAudit *mintAudit
	audit              *mintAudit
}

type mintedSession struct {
	accessToken  string
	refreshToken string
	claims       *domain.AccountClaimsPayload
}

// mintAccountSession persists, signs and audits one account session.
func (s *AccountAuthentication) mintAccountSession(ctx context.Context, req mintRequest) (mintedSession, error) {
	session := s.newRefreshSession(req.account.ID, req.portal.Claim)
	session.OrganizationID = req.orgID
	session.MFAVerified = req.proof.verified()
	if req.lifetime > 0 {
		session.Expiry = time.Now().Add(req.lifetime)
	}
	stored, claims, err := s.persistNewSession(ctx, req, session)
	if err != nil {
		return mintedSession{}, err
	}
	email := req.email
	if email == "" {
		email = req.account.Email
	}
	access, refresh := buildClaims(req.account, stored, claims, email)
	accessToken, refreshToken, err := s.codec.IssueTokenPair(access, refresh)
	if err != nil {
		return mintedSession{}, failed("generate tokens", err)
	}
	return mintedSession{accessToken: accessToken, refreshToken: refreshToken, claims: claims}, nil
}

// persistNewSession writes a freshly minted session, retrying with a new
// family only for token-family collisions of concurrent logins.
func (s *AccountAuthentication) persistNewSession(ctx context.Context, req mintRequest, session domain.AccountSession) (domain.AccountSession, *domain.AccountClaimsPayload, error) {
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		stored, claims, err := s.persistSessionInTransaction(ctx, req, session)
		if err == nil {
			return stored, claims, nil
		}
		var guardErr *mintGuardError
		if errors.As(err, &guardErr) {
			return domain.AccountSession{}, nil, guardErr.err
		}
		if !isTokenFamilyConflict(err) {
			return domain.AccountSession{}, nil, failed("login transaction", err)
		}
		retried := uuid.Must(uuid.NewV4()).String()
		if session.SwitchedFromFamilyID == session.FamilyID {
			session.SwitchedFromFamilyID = retried
		}
		session.FamilyID = retried
		s.logger.Warn("login race condition detected, retrying",
			slog.Int64("account_id", req.account.ID),
			slog.Int("attempt", attempt+1),
			slog.Int("max_retries", maxRetries))
	}
	return domain.AccountSession{}, nil, failed("login transaction", fmt.Errorf("max retries exceeded"))
}

// persistSessionInTransaction writes the session in one administrative
// transaction: the login routes carry no tenant and auth.tokens is
// RLS-guarded. The replaced family is retired before the cap so it is the
// cap's first candidate.
func (s *AccountAuthentication) persistSessionInTransaction(ctx context.Context, req mintRequest, session domain.AccountSession) (domain.AccountSession, *domain.AccountClaimsPayload, error) {
	var stored domain.AccountSession
	var claims *domain.AccountClaimsPayload
	err := s.mintUnit(ctx, func(txCtx context.Context) error {
		var loginTime time.Time
		var err error
		claims, loginTime, err = s.authorizeMint(txCtx, req, &session)
		if err != nil {
			return err
		}
		if err := s.retireReplacedFamily(txCtx, req.account.ID, req.retireFamilyID, loginTime); err != nil {
			return err
		}
		session.TenantID = req.tenantID
		if err := s.requireLiveSessionSchool(txCtx, req.tenantID); err != nil {
			return refused(err)
		}
		created, err := s.sessions.CreateAccountSession(txCtx, ports.AccountSession(session))
		if err != nil {
			return err
		}
		stored = domain.AccountSession(created)
		if err := s.sessions.DeleteExpiredRotatedAccountSessions(txCtx, req.account.ID, time.Now()); err != nil {
			s.logger.Warn("failed to clean up refresh-token handoffs",
				slog.Int64("account_id", req.account.ID),
				slog.Any("error", err))
		}
		if err := s.capSessionsUnlessDemo(txCtx, req.account.ID, stored); err != nil {
			return err
		}
		if req.audit == nil {
			return nil
		}
		return s.auditSession(txCtx, req.account.ID, *req.audit)
	})
	if err != nil {
		return domain.AccountSession{}, nil, err
	}
	return stored, claims, nil
}

// authorizeMint runs steps 1 to 5 of the pipeline.
func (s *AccountAuthentication) authorizeMint(ctx context.Context, req mintRequest, session *domain.AccountSession) (*domain.AccountClaimsPayload, time.Time, error) {
	if err := s.lockMFAPolicyFor(ctx, req.portal, req.account.ID, req.tenantID); err != nil {
		return nil, time.Time{}, refused(err)
	}
	account, err := s.lockLiveAccount(ctx, req.account.ID)
	if err != nil {
		return nil, time.Time{}, refused(err)
	}
	if err := recheckPassword(req.account, account); err != nil {
		return nil, time.Time{}, refused(err)
	}
	if err := s.recheckProof(ctx, req, account); err != nil {
		return nil, time.Time{}, refused(err)
	}
	if req.precondition != nil {
		if err := req.precondition(ctx, account); err != nil {
			return nil, time.Time{}, refused(err)
		}
	}
	policy, ok := sessionPolicies[domain.SessionKindOf(req.portal, req.orgID > 0)]
	if !ok {
		return nil, time.Time{}, refused(failed("mint session", identityaccess.ErrInvalidToken))
	}
	guard := policy.guard(s, sessionBinding{op: req.op, accountID: account.ID, tenantID: req.tenantID, orgID: req.orgID, booked: req.booked, recheckMFAGate: req.recheckMFAGate})
	claims, err := s.eligibleClaims(ctx, guard, account)
	if err != nil {
		return nil, time.Time{}, refused(err)
	}
	if audit := req.traegerSchoolAudit; audit != nil {
		if err := s.recordTraegerSchoolSwitch(ctx, account.ID, req.orgID, *audit); err != nil {
			return nil, time.Time{}, refused(err)
		}
	}
	loginTime, err := s.stampLiveAccount(ctx, account.ID)
	if err != nil {
		return nil, time.Time{}, err
	}
	if err := s.adoptSourceFamily(ctx, account.ID, req.sourceFamilyID, session, loginTime); err != nil {
		return nil, time.Time{}, refused(err)
	}
	if err := s.requireSessionFloor(ctx, account.ID, req.tenantID, *session, factReads{locked: true}); err != nil {
		return nil, time.Time{}, refused(err)
	}
	return claims, loginTime, nil
}

func (s *AccountAuthentication) eligibleClaims(ctx context.Context, eligible eligibility, account domain.LoginAccount) (*domain.AccountClaimsPayload, error) {
	claims, err := eligible(ctx, account)
	if err != nil {
		return nil, err
	}
	if claims == nil {
		return nil, errMissingClaimsPayload
	}
	return claims, nil
}

// lockMFAPolicyFor pins the school's mfa_mode for the transaction in shared
// mode where the portal follows it; an unavailable lock fails closed.
func (s *AccountAuthentication) lockMFAPolicyFor(ctx context.Context, portal domain.Portal, accountID, tenantID int64) error {
	if !s.mfa.Configured() || portal.MFA != domain.MFASchoolPolicy || tenantID <= 0 {
		return nil
	}
	if err := s.mfaLock.LockMFAPolicySharedForTenant(ctx, tenantID); err != nil {
		s.logger.Warn("mfa policy lock unavailable at session mint; refusing to mint",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()))
		return identityaccess.ErrMFAStatusUnavailable
	}
	return nil
}

// lockLiveAccount locks the account row and refuses a missing or inactive
// account. Only a genuine miss is not found; a lookup failure stays
// retryable.
func (s *AccountAuthentication) lockLiveAccount(ctx context.Context, accountID int64) (domain.LoginAccount, error) {
	account, found, _, err := findLoginAccount(ctx, s.runtime, s.store, accountID, true)
	if err != nil {
		return domain.LoginAccount{}, failed("get account", fmt.Errorf("account lookup failed: %w", err))
	}
	if !found {
		return domain.LoginAccount{}, failed("get account", domain.ErrAccountNotFound)
	}
	if !account.Active {
		return domain.LoginAccount{}, failed("check account status", identityaccess.ErrAccountInactive)
	}
	return account, nil
}

// recheckPassword refuses a password sign-in whose credential generation
// moved on between its check and this mint (#6289): a password write revoked
// every session that existed, and the old password must not earn a new one.
// Only a sign-in that checked a password is held to it.
func recheckPassword(checked, locked domain.LoginAccount) error {
	if checked.PasswordChecked && checked.MFAResetGeneration != locked.MFAResetGeneration {
		return failed("login", identityaccess.ErrInvalidCredentials)
	}
	return nil
}

// recheckProof holds the proof to the state MFA reset leaves under the same
// account lock: a code proved before a reset, a device the reset revoked or
// a Träger session that lost its proof mints no verified session.
func (s *AccountAuthentication) recheckProof(ctx context.Context, req mintRequest, account domain.LoginAccount) error {
	switch req.proof.kind {
	case proofChallenge:
		proof := req.proof.challenge
		if proof.AccountID != account.ID || proof.Scope != req.portal.Name || proof.MFAResetGeneration != account.MFAResetGeneration {
			return identityaccess.ErrInvalidToken
		}
	case proofTrustedDevice:
		return s.recheckTrustedDevice(ctx, req, account.ID)
	case proofTraegerSession:
		current, err := s.traegerSessionMFAProof(ctx, account.ID, req.orgID, req.proof.traegerFamilyID)
		if err != nil {
			return err
		}
		if req.proof.traegerVerified && !current {
			return failed("Träger school session", identityaccess.ErrInvalidToken)
		}
	case proofNone, proofPasskey, proofDemoLink:
	}
	return nil
}

func (s *AccountAuthentication) recheckTrustedDevice(ctx context.Context, req mintRequest, accountID int64) error {
	deviceSchoolID := req.tenantID
	if req.portal.Schoolless() {
		// A device of a portal without a school names none, nor does its
		// ledger.
		ctx, deviceSchoolID = withoutSchoolLedgerContext(ctx), 0
	}
	enrolled, err := s.mfa.HasEnrollment(ctx, accountID)
	if err != nil {
		return identityaccess.ErrMFAStatusUnavailable
	}
	if !enrolled || !s.trustedDeviceVerified(ctx, accountID, deviceSchoolID, req.proof.trustedDeviceCookie) {
		return identityaccess.ErrInvalidToken
	}
	return nil
}

// stampLiveAccount writes last_login under the account lock. An account the
// erasure of unused accounts anonymized after the caller's checks gets no
// session (#5546); the login routes answer it like wrong credentials.
func (s *AccountAuthentication) stampLiveAccount(ctx context.Context, accountID int64) (time.Time, error) {
	loginTime := time.Now()
	recorded, _, err := s.store.RecordAccountLogin(ctx, accountID, loginTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("update last login before token issuance: %w", err)
	}
	if !recorded {
		return time.Time{}, refused(failed("issue session", domain.ErrAccountNotFound))
	}
	return loginTime, nil
}

// requireSessionFloor is the MFA floor every mint and refresh applies to the
// session it is about to persist, and a validation to the session it reads.
func (s *AccountAuthentication) requireSessionFloor(ctx context.Context, accountID, tenantID int64, session domain.AccountSession, r factReads) error {
	if err := s.requireSessionMFA(ctx, accountID, tenantID, session.PortalScope, session.MFAVerified, r); err != nil {
		return err
	}
	return s.requireTraegerSchoolMFA(session)
}

// auditSession records the event of a session in the transaction that
// persists it: no session commits without its entry (#6844), and a failed
// entry rolls the session back.
func (s *AccountAuthentication) auditSession(txCtx context.Context, accountID int64, audit mintAudit) error {
	// The ledger stores the client address and requires one (ip_address is
	// NOT NULL); a caller without it, like an in-process tool, files no event
	// rather than an invented address.
	if audit.ipAddress == "" {
		s.logger.Warn("authenticated session not audited: no client address",
			slog.Int64("account_id", accountID),
			slog.String("event_type", audit.event))
		return nil
	}
	// The ledger is school-scoped; every caller names the school the event
	// is filed at (a schoolless session its account's default school).
	if audit.schoolID <= 0 {
		return fmt.Errorf("audit %s: school is required", audit.event)
	}
	if err := s.audit.RecordAuthEvent(txCtx, domain.AuthEvent{
		AccountID: accountID, TenantID: audit.schoolID, Type: audit.event, Success: true,
		IPAddress: audit.ipAddress, UserAgent: audit.userAgent,
	}); err != nil {
		return fmt.Errorf("audit %s: %w", audit.event, err)
	}
	return nil
}

// recordTraegerSchoolSwitch needs the client address: the entry is the
// condition of the session, and the ledger stores no invented one.
func (s *AccountAuthentication) recordTraegerSchoolSwitch(ctx context.Context, accountID, organizationID int64, audit mintAudit) error {
	if audit.ipAddress == "" {
		return errTraegerSchoolSwitchWithoutAddress
	}
	return s.audit.RecordAuthEvent(ctx, domain.AuthEvent{
		AccountID: accountID, TenantID: audit.schoolID, Type: domain.AuthEventTraegerSchoolSwitch, Success: true,
		IPAddress: audit.ipAddress, UserAgent: audit.userAgent, OrganizationID: organizationID,
	})
}

func (s *AccountAuthentication) newRefreshSession(accountID int64, scope string) domain.AccountSession {
	identifier := domain.RefreshTokenIdentifier
	now := time.Now()
	familyID := uuid.Must(uuid.NewV4()).String()
	return domain.AccountSession{
		Token:      uuid.Must(uuid.NewV4()).String(),
		AccountID:  accountID,
		Expiry:     now.Add(s.codec.RefreshExpiry()),
		Identifier: &identifier,
		FamilyID:   familyID,
		// A new family is its own lineage origin until a switch or an
		// Absprung names another; an empty origin marks a family issued
		// before #5422 (capLegacyLineage).
		SwitchedFromFamilyID: familyID,
		PortalScope:          domain.PersistedPortalScope(scope),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func isTokenFamilyConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "uk_tokens_family_generation")
}

func (s *AccountAuthentication) retireReplacedFamily(txCtx context.Context, accountID int64, retireFamilyID string, loginTime time.Time) error {
	if retireFamilyID != "" {
		if err := s.sessions.RetireAccountSessionFamily(txCtx, accountID, retireFamilyID, loginTime.Add(s.rotation.RecoveryGrace())); err != nil {
			return fmt.Errorf("retire replaced refresh-token family: %w", err)
		}
	}
	return nil
}
