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
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// OperatorPasskeyFlows runs the operator portal's WebAuthn ceremonies
// (#3331): the MFA-gated registration and the discoverable login. A ceremony
// only starts on the operator portal's own host, and its completion consumes
// the ceremony and writes the credential in one administrative transaction.
type OperatorPasskeyFlows struct {
	operators *Service
	records   *OperatorPasskey
	mfa       operatorPasskeyGate
	sessions  operatorPasskeySessions
	runtime   ports.Runtime
	rpID      string
	rpName    string
	// originHost is the operator portal's host; a ceremony from any other
	// origin is refused.
	originHost string
	now        func() time.Time
}

// OperatorPasskeyFlowDependencies are what the operator ceremonies need
// beyond the module's own records.
type OperatorPasskeyFlowDependencies struct {
	Records  *OperatorPasskey
	MFA      operatorPasskeyGate
	Sessions operatorPasskeySessions
	Runtime  ports.Runtime
	RPID     string
	RPName   string
	// OperatorFrontendURL is the portal the ceremonies are pinned to.
	OperatorFrontendURL string
}

// operatorPasskeyGate is the second factor an operator registration is gated
// by; operatorPasskeySessions mints the pair a completed login returns.
type operatorPasskeyGate interface {
	StartChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error)
	VerifyCodeForOperator(ctx context.Context, operatorID int64, code string) error
}

type operatorPasskeySessions interface {
	IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error)
}

// NewOperatorPasskeyFlows composes the ceremonies. A missing relying party
// or portal URL is a wiring error and surfaces at startup.
func NewOperatorPasskeyFlows(operators *Service, deps OperatorPasskeyFlowDependencies) (*OperatorPasskeyFlows, error) {
	switch {
	case operators == nil, deps.Records == nil, deps.MFA == nil, deps.Sessions == nil, deps.Runtime == nil:
		return nil, errors.New("identity access operator passkey: all dependencies are required")
	}
	rpID := strings.TrimSpace(deps.RPID)
	if rpID == "" {
		return nil, errors.New("identity access operator passkey: rp id is required")
	}
	originHost, err := domain.OriginHostWithoutPort(deps.OperatorFrontendURL)
	if err != nil {
		return nil, err
	}
	rpName := strings.TrimSpace(deps.RPName)
	if rpName == "" {
		rpName = "moto"
	}
	return &OperatorPasskeyFlows{
		operators: operators, records: deps.Records, mfa: deps.MFA, sessions: deps.Sessions,
		runtime: deps.Runtime,
		rpID:    domain.HostWithoutPort(rpID), rpName: rpName, originHost: originHost, now: time.Now,
	}, nil
}

// StartEnrollmentChallenge mails the code a registration must echo back.
func (f *OperatorPasskeyFlows) StartEnrollmentChallenge(ctx context.Context, operatorID int64, ip net.IP) (domain.PasskeyEnrollmentChallenge, error) {
	operator, err := f.operators.FindOperator(ctx, operatorID)
	if err != nil {
		return domain.PasskeyEnrollmentChallenge{}, err
	}
	token, err := f.mfa.StartChallenge(ctx, operatorID, ip)
	if err != nil {
		return domain.PasskeyEnrollmentChallenge{}, err
	}
	return domain.PasskeyEnrollmentChallenge{ChallengeToken: token, MaskedEmail: domain.MaskEmail(operator.Email)}, nil
}

// BeginRegistration verifies the emailed code and starts the attestation
// ceremony for the operator's authenticator.
func (f *OperatorPasskeyFlows) BeginRegistration(ctx context.Context, request domain.OperatorPasskeyRegistrationStart) (domain.PasskeyCeremonyOptions, error) {
	if err := domain.ValidateOperatorPasskeyOrigin(request.ExpectedOrigin, f.originHost); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	if err := f.mfa.VerifyCodeForOperator(ctx, request.OperatorID, request.Code); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	operator, err := f.operators.FindOperator(ctx, request.OperatorID)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	if !operator.Active {
		return domain.PasskeyCeremonyOptions{}, domain.ErrOperatorInactive
	}
	user, err := f.passkeyUser(ctx, operator)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	rpID, err := domain.ResolvePasskeyRPIDForOrigin(f.rpID, request.ExpectedOrigin)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	relyingParty, err := newWebAuthnForOrigin(rpID, f.rpName, request.ExpectedOrigin)
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
	operatorID := request.OperatorID
	if _, err := f.records.CreateSession(ctx, domain.OperatorPasskeySession{
		ID: sessionID, OperatorID: &operatorID, Purpose: domain.PasskeySessionPurposeRegistration,
		RPID: rpID, ExpectedOrigin: request.ExpectedOrigin, SessionJSON: sessionJSON, ExpiresAt: sessionData.Expires,
	}); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	return domain.PasskeyCeremonyOptions{SessionID: sessionID, Options: creation}, nil
}

// FinishRegistration consumes the ceremony and stores the credential in one
// administrative transaction.
func (f *OperatorPasskeyFlows) FinishRegistration(ctx context.Context, request domain.OperatorPasskeyRegistrationFinish) (domain.PasskeyCredentialSummary, error) {
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
		summary = domain.SummarizeOperatorPasskey(stored)
		return nil
	})
	if err != nil {
		return domain.PasskeyCredentialSummary{}, err
	}
	return summary, nil
}

// verifyRegistration consumes the ceremony and verifies the attestation. A
// refused ceremony is a rejection: the consumption stays committed.
func (f *OperatorPasskeyFlows) verifyRegistration(ctx context.Context, request domain.OperatorPasskeyRegistrationFinish) (domain.OperatorPasskeyCredential, error) {
	session, err := f.records.ConsumeSession(ctx, request.SessionID, domain.PasskeySessionPurposeRegistration, f.now())
	if err != nil {
		if errors.Is(err, domain.ErrOperatorPasskeySessionNotFound) {
			return domain.OperatorPasskeyCredential{}, rejectCeremony(domain.ErrPasskeySessionInvalid)
		}
		return domain.OperatorPasskeyCredential{}, err
	}
	if session.OperatorID == nil || *session.OperatorID != request.OperatorID {
		return domain.OperatorPasskeyCredential{}, rejectCeremony(domain.ErrPasskeySessionInvalid)
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionJSON, &sessionData); err != nil {
		return domain.OperatorPasskeyCredential{}, rejectCeremony(err)
	}
	operator, err := f.operators.FindOperator(ctx, request.OperatorID)
	if err != nil {
		return domain.OperatorPasskeyCredential{}, err
	}
	user, err := f.passkeyUser(ctx, operator, sessionData.UserID)
	if err != nil {
		return domain.OperatorPasskeyCredential{}, err
	}
	relyingParty, err := f.relyingPartyForOrigin(session.ExpectedOrigin)
	if err != nil {
		return domain.OperatorPasskeyCredential{}, rejectCeremony(err)
	}
	parsed, err := parsePasskeyCreation(request.CredentialResponse)
	if err != nil {
		return domain.OperatorPasskeyCredential{}, rejectCeremony(err)
	}
	credential, err := relyingParty.CreateCredential(user, sessionData, parsed)
	if err != nil {
		return domain.OperatorPasskeyCredential{}, rejectCeremony(err)
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return domain.OperatorPasskeyCredential{}, err
	}
	name := domain.NormalizePasskeyName(request.Name)
	if name == "" {
		name = domain.DefaultPasskeyName
	}
	return domain.OperatorPasskeyCredential{
		OperatorID: request.OperatorID, UserHandle: user.WebAuthnID(), CredentialID: credential.ID,
		CredentialJSON: credentialJSON, Name: name,
	}, nil
}

// BeginLogin starts a discoverable login ceremony on the operator portal.
func (f *OperatorPasskeyFlows) BeginLogin(ctx context.Context, expectedOrigin string) (domain.PasskeyCeremonyOptions, error) {
	if err := domain.ValidateOperatorPasskeyOrigin(expectedOrigin, f.originHost); err != nil {
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
	assertion, sessionData, err := relyingParty.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	sessionID, sessionJSON, err := newCeremonySession(sessionData)
	if err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	if _, err := f.records.CreateSession(ctx, domain.OperatorPasskeySession{
		ID: sessionID, Purpose: domain.PasskeySessionPurposeLogin, RPID: rpID,
		ExpectedOrigin: expectedOrigin, SessionJSON: sessionJSON, ExpiresAt: sessionData.Expires,
	}); err != nil {
		return domain.PasskeyCeremonyOptions{}, err
	}
	return domain.PasskeyCeremonyOptions{SessionID: sessionID, Options: assertion}, nil
}

// FinishLogin consumes the ceremony and records the credential use in one
// administrative transaction. The token pair is issued after the commit.
func (f *OperatorPasskeyFlows) FinishLogin(ctx context.Context, request domain.OperatorPasskeyLoginFinish) (domain.PasskeyLoginResult, error) {
	var operatorID int64
	err := completeCeremony(ctx, f.runtime, func(txCtx context.Context) error {
		verified, err := f.verifyLogin(txCtx, request)
		if err != nil {
			return err
		}
		if err := f.records.RecordUse(txCtx, verified.credentialID, verified.credentialJSON, f.now()); err != nil {
			return err
		}
		operatorID = verified.operatorID
		return nil
	})
	if err != nil {
		return domain.PasskeyLoginResult{}, err
	}
	accessToken, refreshToken, err := f.sessions.IssueTokensForAuthenticatedOperator(ctx, operatorID, request.IPAddress, request.UserAgent)
	if err != nil {
		return domain.PasskeyLoginResult{}, err
	}
	return domain.PasskeyLoginResult{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

type verifiedOperatorPasskeyLogin struct {
	operatorID     int64
	credentialID   int64
	credentialJSON json.RawMessage
}

// verifyLogin consumes the ceremony and verifies the assertion. A refused
// assertion is a rejection; a failed read is returned as it is, so a store
// outage does not read as wrong credentials.
func (f *OperatorPasskeyFlows) verifyLogin(ctx context.Context, request domain.OperatorPasskeyLoginFinish) (verifiedOperatorPasskeyLogin, error) {
	session, err := f.records.ConsumeSession(ctx, request.SessionID, domain.PasskeySessionPurposeLogin, f.now())
	if err != nil {
		if errors.Is(err, domain.ErrOperatorPasskeySessionNotFound) {
			return verifiedOperatorPasskeyLogin{}, rejectCeremony(domain.ErrPasskeySessionInvalid)
		}
		return verifiedOperatorPasskeyLogin{}, err
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionJSON, &sessionData); err != nil {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(err)
	}
	relyingParty, err := f.relyingPartyForOrigin(session.ExpectedOrigin)
	if err != nil {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(err)
	}
	parsed, err := parsePasskeyAssertion(request.CredentialResponse)
	if err != nil {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(err)
	}
	var matchedCredentialID int64
	// The WebAuthn library reports every handler error as a failed
	// assertion; a failed read is kept apart.
	var readErr error
	user, credential, err := relyingParty.ValidatePasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		row, findErr := f.records.FindActiveCredential(ctx, rawID, userHandle)
		if findErr != nil {
			if !errors.Is(findErr, domain.ErrOperatorPasskeyNotFound) {
				readErr = findErr
			}
			return nil, findErr
		}
		matchedCredentialID = row.ID
		operator, findErr := f.operators.FindOperator(ctx, row.OperatorID)
		if findErr != nil {
			if !errors.Is(findErr, domain.ErrOperatorNotFound) {
				readErr = findErr
			}
			return nil, findErr
		}
		passkeyUser, userErr := f.passkeyUser(ctx, operator)
		if userErr != nil {
			readErr = userErr
			return nil, userErr
		}
		return passkeyUser, nil
	}, sessionData, parsed)
	if readErr != nil {
		return verifiedOperatorPasskeyLogin{}, readErr
	}
	if err != nil {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(domain.ErrInvalidCredentials)
	}
	matched, ok := user.(*webAuthnUser)
	if !ok {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(domain.ErrInvalidCredentials)
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return verifiedOperatorPasskeyLogin{}, err
	}
	return verifiedOperatorPasskeyLogin{operatorID: matched.ID, credentialID: matchedCredentialID, credentialJSON: credentialJSON}, nil
}

// ListCredentials returns the operator's active passkeys, oldest first.
func (f *OperatorPasskeyFlows) ListCredentials(ctx context.Context, operatorID int64) ([]domain.PasskeyCredentialSummary, error) {
	rows, err := f.records.ListActiveCredentials(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	summaries := make([]domain.PasskeyCredentialSummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, domain.SummarizeOperatorPasskey(row))
	}
	return summaries, nil
}

// RevokeCredential revokes one active passkey of the operator.
func (f *OperatorPasskeyFlows) RevokeCredential(ctx context.Context, operatorID, credentialID int64) error {
	if err := f.records.RevokeCredential(ctx, operatorID, credentialID, f.now()); err != nil {
		if errors.Is(err, domain.ErrOperatorPasskeyNotFound) {
			return domain.ErrPasskeyNotFound
		}
		return err
	}
	return nil
}

func (f *OperatorPasskeyFlows) relyingPartyForOrigin(origin string) (*webauthn.WebAuthn, error) {
	rpID, err := domain.ResolvePasskeyRPIDForOrigin(f.rpID, origin)
	if err != nil {
		return nil, err
	}
	return newWebAuthnForOrigin(rpID, f.rpName, origin)
}

func (f *OperatorPasskeyFlows) passkeyUser(ctx context.Context, operator domain.Operator, sessionHandle ...[]byte) (*webAuthnUser, error) {
	rows, err := f.records.ListActiveCredentials(ctx, operator.ID)
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
		ID: operator.ID, UserHandle: handle, Name: operator.Email,
		DisplayName: operator.DisplayName, Credentials: credentials,
	}, nil
}

// newCeremonySession mints the ceremony id and serialises its WebAuthn
// state.
func newCeremonySession(sessionData *webauthn.SessionData) (string, json.RawMessage, error) {
	sessionID, err := uuid.NewV4()
	if err != nil {
		return "", nil, err
	}
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return "", nil, err
	}
	return sessionID.String(), sessionJSON, nil
}
