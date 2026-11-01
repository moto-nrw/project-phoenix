package application

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// AccountPasskeyFlows runs the school-portal WebAuthn ceremonies (#3331):
// the MFA-gated registration and the discoverable login.
//
// A credential belongs to the account and carries no school. The ceremony
// records the school whose portal started it, and the login verifies the
// account's membership in that school before it mints a session, so a
// passkey registered at one school never opens another.
type AccountPasskeyFlows struct {
	accounts ports.AccountDirectory
	records  *AccountPasskey
	mfa      accountPasskeyGate
	sessions accountPasskeySessions
	runtime  ports.Runtime
	attach   func(context.Context) context.Context
	rpID     string
	rpName   string
	// tenantDomain is the base domain the school subdomains live under.
	tenantDomain string
	// catererHost is the caterer portal's host (#4497); empty leaves the
	// caterer's ceremonies unavailable.
	catererHost string
	now         func() time.Time
}

// AccountPasskeyFlowDependencies are what the school-portal ceremonies need
// beyond the module's own records.
type AccountPasskeyFlowDependencies struct {
	Accounts ports.AccountDirectory
	Records  *AccountPasskey
	MFA      accountPasskeyGate
	Sessions accountPasskeySessions
	Runtime  ports.Runtime
	// Attach binds the composed unit of work ahead of every public ceremony.
	Attach       func(context.Context) context.Context
	RPID         string
	RPName       string
	TenantDomain string
	// CatererFrontendURL is the caterer portal the caterer's ceremonies are
	// pinned to (#4497).
	CatererFrontendURL string
}

// accountPasskeyGate is the second factor a registration is gated by: the
// enrollment code the flow mails and the check that redeems it.
type accountPasskeyGate interface {
	StartChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error)
	VerifyCodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error
}

// accountPasskeySessions mints the pair a completed login returns and proves
// the account's membership in the school whose portal started the ceremony.
type accountPasskeySessions interface {
	IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error)
	VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error)
}

// NewAccountPasskeyFlows composes the ceremonies. A missing relying party or
// tenant domain is a wiring error and surfaces at startup.
func NewAccountPasskeyFlows(deps AccountPasskeyFlowDependencies) (*AccountPasskeyFlows, error) {
	switch {
	case deps.Accounts == nil, deps.Records == nil, deps.MFA == nil, deps.Sessions == nil, deps.Runtime == nil:
		return nil, errors.New("identity access account passkey: all dependencies are required")
	}
	rpID := strings.TrimSpace(deps.RPID)
	if rpID == "" {
		return nil, errors.New("identity access account passkey: rp id is required")
	}
	tenantDomain := strings.TrimSpace(deps.TenantDomain)
	if tenantDomain == "" {
		return nil, errors.New("identity access account passkey: tenant domain is required")
	}
	rpName := strings.TrimSpace(deps.RPName)
	if rpName == "" {
		rpName = "moto"
	}
	return &AccountPasskeyFlows{
		accounts: deps.Accounts, records: deps.Records, mfa: deps.MFA, sessions: deps.Sessions,
		runtime: deps.Runtime, attach: deps.Attach,
		rpID: domain.HostWithoutPort(rpID), rpName: rpName,
		tenantDomain: domain.HostWithoutPort(tenantDomain), catererHost: catererHost(deps.CatererFrontendURL), now: time.Now,
	}, nil
}

// StartEnrollmentChallenge mails the code a registration must echo back.
func (f *AccountPasskeyFlows) StartEnrollmentChallenge(ctx context.Context, accountID, tenantID int64, ip net.IP) (domain.PasskeyEnrollmentChallenge, error) {
	account, found, err := f.accounts.FindAccountIdentity(ctx, accountID)
	if err != nil || !found {
		return domain.PasskeyEnrollmentChallenge{}, domain.ErrAccountNotFound
	}
	token, err := f.mfa.StartChallenge(ctx, accountID, tenantID, domain.MFAChallengeScopeTenant, ip)
	if err != nil {
		return domain.PasskeyEnrollmentChallenge{}, err
	}
	return domain.PasskeyEnrollmentChallenge{ChallengeToken: token, MaskedEmail: domain.MaskEmail(account.Email)}, nil
}

// BeginRegistration verifies the emailed code and starts the attestation
// ceremony. The code is pinned to this school's tenant portal, exactly as
// StartEnrollmentChallenge issued it, so a code from another portal cannot
// authorise a registration here.
func (f *AccountPasskeyFlows) BeginRegistration(ctx context.Context, request domain.AccountPasskeyRegistrationStart) (domain.PasskeyCeremonyOptions, error) {
	if err := domain.ValidateTenantPasskeyOrigin(request.ExpectedOrigin, request.TenantSubdomain, f.tenantDomain); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	if err := f.mfa.VerifyCodeForAccount(ctx, request.AccountID, request.TenantID, request.Code, domain.MFAChallengeScopeTenant); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	account, found, err := f.accounts.FindAccountIdentity(ctx, request.AccountID)
	if err != nil || !found {
		return domain.PasskeyCeremonyOptions{}, domain.ErrAccountNotFound
	}
	tenantID := request.TenantID
	return f.beginRegistrationCeremony(ctx, account, request.ExpectedOrigin, &tenantID)
}

// beginRegistrationCeremony starts the attestation once the caller verified
// the mailed code. The ceremony records the school whose portal started it,
// or none for the caterer portal (#4497).
func (f *AccountPasskeyFlows) beginRegistrationCeremony(ctx context.Context, account domain.AccountIdentity, expectedOrigin string, tenantID *int64) (domain.PasskeyCeremonyOptions, error) {
	accountID := account.ID
	if !account.Active {
		return domain.PasskeyCeremonyOptions{}, identityaccess.ErrAccountInactive
	}
	user, err := f.passkeyUser(ctx, account)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	rpID, err := domain.ResolvePasskeyRPIDForOrigin(f.rpID, expectedOrigin)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	relyingParty, err := newWebAuthnForOrigin(rpID, f.rpName, expectedOrigin)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	creation, sessionData, err := relyingParty.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		}),
	)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	sessionID, sessionJSON, err := newCeremonySession(sessionData)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	if _, err := f.records.CreateSession(ctx, domain.AccountPasskeySession{
		ID: sessionID, AccountID: &accountID, TenantID: tenantID,
		Purpose: domain.PasskeySessionPurposeRegistration, RPID: rpID,
		ExpectedOrigin: expectedOrigin, SessionJSON: sessionJSON, ExpiresAt: sessionData.Expires,
	}); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	return domain.PasskeyCeremonyOptions{SessionID: sessionID, Options: creation}, nil
}

// FinishRegistration consumes the ceremony and stores the credential in one
// administrative transaction.
func (f *AccountPasskeyFlows) FinishRegistration(ctx context.Context, request domain.AccountPasskeyRegistrationFinish) (domain.PasskeyCredentialSummary, error) {
	var summary domain.PasskeyCredentialSummary
	err := completeCeremony(ctx, f.runtime, func(txCtx context.Context) error {
		credential, err := f.verifyRegistration(txCtx, request)
		if err != nil {
			return err
		}
		stored, err := f.records.CreateCredential(txCtx, credential)
		if err != nil {
			return err
		}
		summary = domain.SummarizeAccountPasskey(stored)
		return nil
	})
	if err != nil {
		return domain.PasskeyCredentialSummary{}, err
	}
	return summary, nil
}

// verifyRegistration consumes the ceremony and verifies the attestation. A
// refused ceremony is a rejection: the consumption stays committed, so the
// same challenge cannot be retried.
func (f *AccountPasskeyFlows) verifyRegistration(ctx context.Context, request domain.AccountPasskeyRegistrationFinish) (domain.AccountPasskeyCredential, error) {
	session, err := f.records.ConsumeSession(ctx, request.SessionID, domain.PasskeySessionPurposeRegistration, f.now())
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountPasskeySessionNotFound) {
			return domain.AccountPasskeyCredential{}, rejectCeremony(identityaccess.ErrPasskeySessionInvalid)
		}
		return domain.AccountPasskeyCredential{}, err
	}
	if session.AccountID == nil || *session.AccountID != request.AccountID {
		return domain.AccountPasskeyCredential{}, rejectCeremony(identityaccess.ErrPasskeySessionInvalid)
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionJSON, &sessionData); err != nil {
		return domain.AccountPasskeyCredential{}, rejectCeremony(err)
	}
	account, found, err := f.accounts.FindAccountIdentity(ctx, request.AccountID)
	if err != nil {
		return domain.AccountPasskeyCredential{}, err
	}
	if !found {
		return domain.AccountPasskeyCredential{}, rejectCeremony(domain.ErrAccountNotFound)
	}
	user, err := f.passkeyUser(ctx, account, sessionData.UserID)
	if err != nil {
		return domain.AccountPasskeyCredential{}, err
	}
	relyingParty, err := f.relyingPartyForOrigin(session.ExpectedOrigin)
	if err != nil {
		return domain.AccountPasskeyCredential{}, rejectCeremony(err)
	}
	parsed, err := parsePasskeyCreation(request.CredentialResponse)
	if err != nil {
		return domain.AccountPasskeyCredential{}, rejectCeremony(err)
	}
	credential, err := relyingParty.CreateCredential(user, sessionData, parsed)
	if err != nil {
		return domain.AccountPasskeyCredential{}, rejectCeremony(err)
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return domain.AccountPasskeyCredential{}, err
	}
	name := domain.NormalizePasskeyName(request.Name)
	if name == "" {
		name = domain.NormalizePasskeyName(domain.DefaultPasskeyName)
	}
	return domain.AccountPasskeyCredential{
		AccountID: request.AccountID, UserHandle: user.WebAuthnID(), CredentialID: credential.ID,
		CredentialJSON: credentialJSON, Name: name,
	}, nil
}

// BeginLogin starts a discoverable login ceremony on one school's portal.
func (f *AccountPasskeyFlows) BeginLogin(ctx context.Context, request domain.AccountPasskeyLoginStart) (domain.PasskeyCeremonyOptions, error) {
	if err := domain.ValidateTenantPasskeyOrigin(request.ExpectedOrigin, request.TenantSubdomain, f.tenantDomain); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	tenantID := request.TenantID
	return f.beginLoginCeremony(ctx, request.ExpectedOrigin, &tenantID)
}

// beginLoginCeremony starts a discoverable login. The ceremony records the
// school whose portal started it, or none for the caterer portal (#4497).
func (f *AccountPasskeyFlows) beginLoginCeremony(ctx context.Context, expectedOrigin string, tenantID *int64) (domain.PasskeyCeremonyOptions, error) {
	rpID, err := domain.ResolvePasskeyRPIDForOrigin(f.rpID, expectedOrigin)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	relyingParty, err := newWebAuthnForOrigin(rpID, f.rpName, expectedOrigin)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	assertion, sessionData, err := relyingParty.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	sessionID, sessionJSON, err := newCeremonySession(sessionData)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	if _, err := f.records.CreateSession(ctx, domain.AccountPasskeySession{
		ID: sessionID, TenantID: tenantID, Purpose: domain.PasskeySessionPurposeLogin, RPID: rpID,
		ExpectedOrigin: expectedOrigin, SessionJSON: sessionJSON, ExpiresAt: sessionData.Expires,
	}); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	return domain.PasskeyCeremonyOptions{SessionID: sessionID, Options: assertion}, nil
}

// FinishLogin consumes the ceremony and records the credential use in one
// administrative transaction. The token pair is issued after the commit.
func (f *AccountPasskeyFlows) FinishLogin(ctx context.Context, request domain.AccountPasskeyLoginFinish) (domain.PasskeyLoginResult, error) {
	var verified verifiedAccountPasskeyLogin
	err := completeCeremony(ctx, f.runtime, func(txCtx context.Context) error {
		result, err := f.verifyLogin(txCtx, request)
		if err != nil {
			return err
		}
		if err := f.records.RecordUse(txCtx, result.credentialID, result.credentialJSON, f.now()); err != nil {
			return err
		}
		verified = result
		return nil
	})
	if err != nil {
		return domain.PasskeyLoginResult{}, err
	}
	accessToken, refreshToken, err := f.sessions.IssueTokensForAuthenticatedAccount(ctx, verified.accountID, verified.tenantID, request.IPAddress, request.UserAgent)
	if err != nil {
		return domain.PasskeyLoginResult{}, err
	}
	return domain.PasskeyLoginResult{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

type verifiedAccountPasskeyLogin struct {
	accountID      int64
	tenantID       int64
	credentialID   int64
	credentialJSON json.RawMessage
}

// verifyLogin consumes the ceremony, verifies the assertion and checks that
// the account belongs to the school whose portal started it. A refused
// assertion, or a school the account has no access to, is a rejection; a
// failed read is returned as it is, so a store outage does not read as wrong
// credentials.
func (f *AccountPasskeyFlows) verifyLogin(ctx context.Context, request domain.AccountPasskeyLoginFinish) (verifiedAccountPasskeyLogin, error) {
	session, err := f.consumeLoginSession(ctx, request.SessionID)
	if err != nil {
		return verifiedAccountPasskeyLogin{}, err
	}
	if session.TenantID == nil {
		return verifiedAccountPasskeyLogin{}, rejectCeremony(identityaccess.ErrPasskeySessionInvalid)
	}
	verified, err := f.verifyAssertion(ctx, session, request)
	if err != nil {
		return verifiedAccountPasskeyLogin{}, err
	}
	tenantID := *session.TenantID
	hasAccess, err := f.sessions.VerifyAccountTenantMembership(ctx, verified.accountID, tenantID)
	if err != nil {
		return verifiedAccountPasskeyLogin{}, err
	}
	if !hasAccess {
		// The school binding is decided per ceremony: a passkey of another
		// school never mints a session here, and the ceremony is spent.
		return verifiedAccountPasskeyLogin{}, rejectCeremony(identityaccess.ErrTenantAccessDenied)
	}
	verified.tenantID = tenantID
	return verified, nil
}

// consumeLoginSession completes a login ceremony exactly once.
func (f *AccountPasskeyFlows) consumeLoginSession(ctx context.Context, sessionID string) (domain.AccountPasskeySession, error) {
	session, err := f.records.ConsumeSession(ctx, sessionID, domain.PasskeySessionPurposeLogin, f.now())
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountPasskeySessionNotFound) {
			return domain.AccountPasskeySession{}, rejectCeremony(identityaccess.ErrPasskeySessionInvalid)
		}
		return domain.AccountPasskeySession{}, err
	}
	return session, nil
}

// verifyAssertion verifies the assertion of a consumed login ceremony and
// names the account and credential it proved. The portal's own binding is
// the caller's.
func (f *AccountPasskeyFlows) verifyAssertion(ctx context.Context, session domain.AccountPasskeySession, request domain.AccountPasskeyLoginFinish) (verifiedAccountPasskeyLogin, error) {
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionJSON, &sessionData); err != nil {
		return verifiedAccountPasskeyLogin{}, rejectCeremony(err)
	}
	relyingParty, err := f.relyingPartyForOrigin(session.ExpectedOrigin)
	if err != nil {
		return verifiedAccountPasskeyLogin{}, rejectCeremony(err)
	}
	parsed, err := parsePasskeyAssertion(request.CredentialResponse)
	if err != nil {
		return verifiedAccountPasskeyLogin{}, rejectCeremony(err)
	}

	var matchedCredentialID int64
	// The WebAuthn library reports every handler error as a failed
	// assertion; a failed read is kept apart.
	var readErr error
	user, credential, err := relyingParty.ValidatePasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		return f.resolveLoginPasskeyUser(ctx, rawID, userHandle, &matchedCredentialID, &readErr)
	}, sessionData, parsed)
	if readErr != nil {
		return verifiedAccountPasskeyLogin{}, readErr
	}
	if err != nil {
		return verifiedAccountPasskeyLogin{}, rejectCeremony(identityaccess.ErrInvalidCredentials)
	}
	matched, ok := user.(*webAuthnUser)
	if !ok {
		return verifiedAccountPasskeyLogin{}, rejectCeremony(identityaccess.ErrInvalidCredentials)
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return verifiedAccountPasskeyLogin{}, err
	}
	return verifiedAccountPasskeyLogin{
		accountID: matched.ID, credentialID: matchedCredentialID, credentialJSON: credentialJSON,
	}, nil
}

// ListCredentials returns the account's active passkeys, oldest first.
func (f *AccountPasskeyFlows) ListCredentials(ctx context.Context, accountID int64) ([]domain.PasskeyCredentialSummary, error) {
	rows, err := f.records.ListActiveCredentials(ctx, accountID)
	if err != nil {
		return nil, err
	}
	summaries := make([]domain.PasskeyCredentialSummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, domain.SummarizeAccountPasskey(row))
	}
	return summaries, nil
}

// RevokeCredential revokes one active passkey of the account.
func (f *AccountPasskeyFlows) RevokeCredential(ctx context.Context, accountID, credentialID int64) error {
	if err := f.records.RevokeCredential(ctx, accountID, credentialID, f.now()); err != nil {
		if errors.Is(err, identityaccess.ErrAccountPasskeyNotFound) {
			return identityaccess.ErrPasskeyNotFound
		}
		return err
	}
	return nil
}

func (f *AccountPasskeyFlows) relyingPartyForOrigin(origin string) (*webauthn.WebAuthn, error) {
	rpID, err := domain.ResolvePasskeyRPIDForOrigin(f.rpID, origin)
	if err != nil {
		return nil, err
	}
	return newWebAuthnForOrigin(rpID, f.rpName, origin)
}

func (f *AccountPasskeyFlows) passkeyUser(ctx context.Context, account domain.AccountIdentity, sessionHandle ...[]byte) (*webAuthnUser, error) {
	rows, err := f.records.ListActiveCredentials(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	stored := make([]json.RawMessage, 0, len(rows))
	handles := make([][]byte, 0, len(rows))
	for _, row := range rows {
		stored = append(stored, row.CredentialJSON)
		handles = append(handles, row.UserHandle)
	}
	var carried []byte
	if len(sessionHandle) > 0 {
		carried = sessionHandle[0]
	}
	credentials, handle, err := passkeyCredentials(stored, handles, carried)
	if err != nil {
		return nil, err
	}
	return &webAuthnUser{
		ID: account.ID, UserHandle: handle, Name: account.Email,
		DisplayName: account.Email, Credentials: credentials,
	}, nil
}

func (f *AccountPasskeyFlows) resolveLoginPasskeyUser(ctx context.Context, rawID, userHandle []byte, matchedCredentialID *int64, readErr *error) (webauthn.User, error) {
	row, findErr := f.records.FindActiveCredential(ctx, rawID, userHandle)
	if findErr != nil {
		if !errors.Is(findErr, identityaccess.ErrAccountPasskeyNotFound) {
			*readErr = findErr
		}
		return nil, findErr
	}
	*matchedCredentialID = row.ID
	account, found, accountErr := f.accounts.FindAccountIdentity(ctx, row.AccountID)
	if accountErr != nil {
		*readErr = accountErr
		return nil, accountErr
	}
	if !found {
		return nil, domain.ErrAccountNotFound
	}
	passkeyUser, userErr := f.passkeyUser(ctx, account)
	if userErr != nil {
		*readErr = userErr
		return nil, userErr
	}
	return passkeyUser, nil
}

func (f *AccountPasskeyFlows) StartAccountPasskeyEnrollment(ctx context.Context, accountID, tenantID int64, ip net.IP) (identityaccess.PasskeyEnrollmentChallenge, error) {
	if f == nil {
		return identityaccess.PasskeyEnrollmentChallenge{}, identityaccess.ErrPasskeyFlowsUnavailable
	}
	challenge, err := f.StartEnrollmentChallenge(bindRuntime(f.attach, ctx), accountID, tenantID, ip)
	return identityaccess.PasskeyEnrollmentChallenge(challenge), passkeyError(err)
}

func (f *AccountPasskeyFlows) BeginAccountPasskeyRegistration(ctx context.Context, request identityaccess.AccountPasskeyRegistrationStart) (identityaccess.PasskeyCeremonyOptions, error) {
	if f == nil {
		return identityaccess.PasskeyCeremonyOptions{}, identityaccess.ErrPasskeyFlowsUnavailable
	}
	options, err := f.BeginRegistration(bindRuntime(f.attach, ctx), domain.AccountPasskeyRegistrationStart(request))
	return identityaccess.PasskeyCeremonyOptions(options), passkeyError(err)
}

func (f *AccountPasskeyFlows) FinishAccountPasskeyRegistration(ctx context.Context, request identityaccess.AccountPasskeyRegistrationFinish) (identityaccess.PasskeyCredentialSummary, error) {
	if f == nil {
		return identityaccess.PasskeyCredentialSummary{}, identityaccess.ErrPasskeyFlowsUnavailable
	}
	summary, err := f.FinishRegistration(bindRuntime(f.attach, ctx), domain.AccountPasskeyRegistrationFinish(request))
	return identityaccess.PasskeyCredentialSummary(summary), passkeyError(err)
}

func (f *AccountPasskeyFlows) BeginAccountPasskeyLogin(ctx context.Context, request identityaccess.AccountPasskeyLoginStart) (identityaccess.PasskeyCeremonyOptions, error) {
	if f == nil {
		return identityaccess.PasskeyCeremonyOptions{}, identityaccess.ErrPasskeyFlowsUnavailable
	}
	options, err := f.BeginLogin(bindRuntime(f.attach, ctx), domain.AccountPasskeyLoginStart(request))
	return identityaccess.PasskeyCeremonyOptions(options), passkeyError(err)
}

func (f *AccountPasskeyFlows) FinishAccountPasskeyLogin(ctx context.Context, request identityaccess.AccountPasskeyLoginFinish) (identityaccess.PasskeyLoginResult, error) {
	if f == nil {
		return identityaccess.PasskeyLoginResult{}, identityaccess.ErrPasskeyFlowsUnavailable
	}
	result, err := f.FinishLogin(bindRuntime(f.attach, ctx), domain.AccountPasskeyLoginFinish(request))
	return identityaccess.PasskeyLoginResult(result), passkeyError(err)
}

func (f *AccountPasskeyFlows) ListAccountPasskeyCredentials(ctx context.Context, accountID int64) ([]identityaccess.PasskeyCredentialSummary, error) {
	if f == nil {
		return nil, identityaccess.ErrPasskeyFlowsUnavailable
	}
	summaries, err := f.ListCredentials(bindRuntime(f.attach, ctx), accountID)
	if err != nil {
		return nil, passkeyError(err)
	}
	return publicPasskeySummaries(summaries), nil
}

func (f *AccountPasskeyFlows) RevokeAccountPasskeyCredential(ctx context.Context, accountID, credentialID int64) error {
	if f == nil {
		return identityaccess.ErrPasskeyFlowsUnavailable
	}
	return passkeyError(f.RevokeCredential(bindRuntime(f.attach, ctx), accountID, credentialID))
}

func publicPasskeySummaries(values []domain.PasskeyCredentialSummary) []identityaccess.PasskeyCredentialSummary {
	result := make([]identityaccess.PasskeyCredentialSummary, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.PasskeyCredentialSummary(value))
	}
	return result
}

// passkeyError is MFAError: the ceremony sentinels are translated by the
// same table, and a gated registration reaches through it.
func passkeyError(err error) error { return MFAError(err) }
