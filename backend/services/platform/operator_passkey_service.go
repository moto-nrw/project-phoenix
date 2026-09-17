package platform

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type OperatorPasskeyService interface {
	StartEnrollmentChallenge(ctx context.Context, operatorID int64, ip net.IP) (*OperatorPasskeyEnrollmentChallenge, error)
	BeginRegistration(ctx context.Context, req OperatorPasskeyRegistrationStartRequest) (*authService.PasskeyCredentialCreation, error)
	FinishRegistration(ctx context.Context, req OperatorPasskeyRegistrationFinishRequest) (*authService.PasskeyCredentialSummary, error)
	BeginLogin(ctx context.Context, expectedOrigin string) (*authService.PasskeyCredentialAssertion, error)
	FinishLogin(ctx context.Context, req OperatorPasskeyLoginFinishRequest) (*authService.PasskeyLoginResult, error)
	ListCredentials(ctx context.Context, operatorID int64) ([]authService.PasskeyCredentialSummary, error)
	RevokeCredential(ctx context.Context, operatorID, credentialID int64) error
}

type OperatorPasskeyServiceConfig struct {
	// Records is the consumer-owned port over the Identity & Access operator
	// passkey credentials and ceremony sessions.
	Records OperatorPasskeyRecords
	// Operators is the consumer-owned port over the Identity & Access
	// operator rows the passkey ceremonies resolve their user from.
	Operators           OperatorDirectory
	MFAService          OperatorMFAService
	AuthService         OperatorAuthService
	DB                  *bun.DB
	Logger              *slog.Logger
	RPID                string
	RPName              string
	OperatorFrontendURL string
}

type OperatorPasskeyEnrollmentChallenge struct {
	ChallengeToken string `json:"challenge_token"`
	MaskedEmail    string `json:"masked_email"`
}

type OperatorPasskeyRegistrationStartRequest struct {
	OperatorID     int64
	ExpectedOrigin string
	Code           string
	Name           string
}

type OperatorPasskeyRegistrationFinishRequest struct {
	OperatorID         int64
	SessionID          string
	CredentialResponse json.RawMessage
	Name               string
}

type OperatorPasskeyLoginFinishRequest struct {
	SessionID          string
	CredentialResponse json.RawMessage
	IPAddress          string
	UserAgent          string
}

type operatorPasskeyService struct {
	records            OperatorPasskeyRecords
	operators          OperatorDirectory
	mfaService         OperatorMFAService
	authService        OperatorAuthService
	db                 *bun.DB
	logger             *slog.Logger
	rpID               string
	rpName             string
	operatorOriginHost string
}

var _ OperatorPasskeyService = (*operatorPasskeyService)(nil)

func NewOperatorPasskeyService(cfg OperatorPasskeyServiceConfig) (OperatorPasskeyService, error) {
	if cfg.Records == nil {
		return nil, errors.New("OperatorPasskeyServiceConfig.Records is required")
	}
	if cfg.Operators == nil {
		return nil, errors.New("OperatorPasskeyServiceConfig.Operators is required")
	}
	if cfg.MFAService == nil {
		return nil, errors.New("OperatorPasskeyServiceConfig.MFAService is required")
	}
	if cfg.AuthService == nil {
		return nil, errors.New("OperatorPasskeyServiceConfig.AuthService is required")
	}
	if cfg.DB == nil {
		return nil, errors.New("OperatorPasskeyServiceConfig.DB is required")
	}
	rpID := strings.TrimSpace(cfg.RPID)
	if rpID == "" {
		return nil, errors.New("OperatorPasskeyServiceConfig.RPID is required")
	}
	originHost, err := authService.OriginHostWithoutPort(cfg.OperatorFrontendURL)
	if err != nil {
		return nil, err
	}
	rpName := strings.TrimSpace(cfg.RPName)
	if rpName == "" {
		rpName = "moto"
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &operatorPasskeyService{
		records:            cfg.Records,
		operators:          cfg.Operators,
		mfaService:         cfg.MFAService,
		authService:        cfg.AuthService,
		db:                 cfg.DB,
		logger:             logger,
		rpID:               authService.HostWithoutPort(rpID),
		rpName:             rpName,
		operatorOriginHost: originHost,
	}, nil
}

func (s *operatorPasskeyService) StartEnrollmentChallenge(ctx context.Context, operatorID int64, ip net.IP) (*OperatorPasskeyEnrollmentChallenge, error) {
	operator, err := s.findOperator(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	challengeToken, err := s.mfaService.StartChallenge(ctx, operatorID, ip)
	if err != nil {
		return nil, err
	}
	return &OperatorPasskeyEnrollmentChallenge{
		ChallengeToken: challengeToken,
		MaskedEmail:    authService.MaskEmailForUX(operator.Email),
	}, nil
}

func (s *operatorPasskeyService) BeginRegistration(ctx context.Context, req OperatorPasskeyRegistrationStartRequest) (*authService.PasskeyCredentialCreation, error) {
	if err := s.validateOperatorOrigin(req.ExpectedOrigin); err != nil {
		return nil, err
	}
	if err := s.mfaService.VerifyCodeForOperator(ctx, req.OperatorID, req.Code); err != nil {
		return nil, err
	}
	operator, err := s.findOperator(ctx, req.OperatorID)
	if err != nil {
		return nil, err
	}
	if !operator.Active {
		return nil, &OperatorInactiveError{OperatorID: req.OperatorID}
	}
	user, err := s.passkeyUserForOperator(ctx, operator)
	if err != nil {
		return nil, err
	}
	rpID, err := s.rpIDForOrigin(req.ExpectedOrigin)
	if err != nil {
		return nil, err
	}
	webAuthn, err := s.webAuthnForOrigin(req.ExpectedOrigin)
	if err != nil {
		return nil, err
	}
	creation, sessionData, err := webAuthn.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		}),
	)
	if err != nil {
		return nil, err
	}
	sessionID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return nil, err
	}
	operatorID := req.OperatorID
	session := &platform.OperatorPasskeySession{
		OperatorID:     &operatorID,
		Purpose:        platform.OperatorPasskeySessionPurposeRegistration,
		RPID:           rpID,
		ExpectedOrigin: req.ExpectedOrigin,
		SessionJSON:    sessionJSON,
		ExpiresAt:      sessionData.Expires,
	}
	session.ID = sessionID.String()
	if err := s.records.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return &authService.PasskeyCredentialCreation{SessionID: sessionID.String(), Options: creation}, nil
}

// FinishRegistration consumes the ceremony and stores the credential in one
// administrative transaction (see completeCeremony).
func (s *operatorPasskeyService) FinishRegistration(ctx context.Context, req OperatorPasskeyRegistrationFinishRequest) (*authService.PasskeyCredentialSummary, error) {
	var summary *authService.PasskeyCredentialSummary
	err := s.completeCeremony(ctx, func(txCtx context.Context) error {
		row, err := s.verifyRegistration(txCtx, req)
		if err != nil {
			return err
		}
		if err := s.records.CreateCredential(txCtx, row); err != nil {
			return err
		}
		summary = summarizeOperatorPasskeyCredential(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// verifyRegistration consumes the ceremony and verifies the attestation. It
// returns the credential to store; a refused ceremony is a rejection.
func (s *operatorPasskeyService) verifyRegistration(ctx context.Context, req OperatorPasskeyRegistrationFinishRequest) (*platform.OperatorPasskeyCredential, error) {
	sessionRow, err := s.records.ConsumeSession(ctx, req.SessionID, platform.OperatorPasskeySessionPurposeRegistration, time.Now())
	if err != nil {
		return nil, err
	}
	if sessionRow == nil || sessionRow.OperatorID == nil || *sessionRow.OperatorID != req.OperatorID {
		return nil, rejectCeremony(authService.ErrPasskeySessionInvalid)
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionRow.SessionJSON, &sessionData); err != nil {
		return nil, rejectCeremony(err)
	}
	operator, err := s.findOperator(ctx, req.OperatorID)
	if err != nil {
		return nil, err
	}
	user, err := s.passkeyUserForOperator(ctx, operator, sessionData.UserID)
	if err != nil {
		return nil, err
	}
	webAuthn, err := s.webAuthnForOrigin(sessionRow.ExpectedOrigin)
	if err != nil {
		return nil, rejectCeremony(err)
	}
	httpReq, err := authService.PasskeyResponseRequest(ctx, req.CredentialResponse)
	if err != nil {
		return nil, rejectCeremony(err)
	}
	credential, err := webAuthn.FinishRegistration(user, sessionData, httpReq)
	if err != nil {
		return nil, rejectCeremony(err)
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return nil, err
	}
	name := authService.NormalizePasskeyName(req.Name)
	if name == "" {
		name = "Passkey"
	}
	return &platform.OperatorPasskeyCredential{
		OperatorID:     req.OperatorID,
		UserHandle:     user.WebAuthnID(),
		CredentialID:   credential.ID,
		CredentialJSON: credentialJSON,
		Name:           name,
	}, nil
}

func (s *operatorPasskeyService) BeginLogin(ctx context.Context, expectedOrigin string) (*authService.PasskeyCredentialAssertion, error) {
	if err := s.validateOperatorOrigin(expectedOrigin); err != nil {
		return nil, err
	}
	rpID, err := s.rpIDForOrigin(expectedOrigin)
	if err != nil {
		return nil, err
	}
	webAuthn, err := s.webAuthnForOrigin(expectedOrigin)
	if err != nil {
		return nil, err
	}
	assertion, sessionData, err := webAuthn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, err
	}
	sessionID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return nil, err
	}
	session := &platform.OperatorPasskeySession{
		Purpose:        platform.OperatorPasskeySessionPurposeLogin,
		RPID:           rpID,
		ExpectedOrigin: expectedOrigin,
		SessionJSON:    sessionJSON,
		ExpiresAt:      sessionData.Expires,
	}
	session.ID = sessionID.String()
	if err := s.records.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return &authService.PasskeyCredentialAssertion{SessionID: sessionID.String(), Options: assertion}, nil
}

// FinishLogin consumes the ceremony and records the credential use in one
// administrative transaction (see completeCeremony). The token pair is
// issued after the commit.
func (s *operatorPasskeyService) FinishLogin(ctx context.Context, req OperatorPasskeyLoginFinishRequest) (*authService.PasskeyLoginResult, error) {
	var operatorID int64
	err := s.completeCeremony(ctx, func(txCtx context.Context) error {
		verified, err := s.verifyLogin(txCtx, req)
		if err != nil {
			return err
		}
		if err := s.records.RecordCredentialUse(txCtx, verified.credentialID, verified.credentialJSON, time.Now()); err != nil {
			return err
		}
		operatorID = verified.operatorID
		return nil
	})
	if err != nil {
		return nil, err
	}
	accessToken, refreshToken, err := s.authService.IssueTokensForAuthenticatedOperator(ctx, operatorID, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}
	return &authService.PasskeyLoginResult{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

type verifiedOperatorPasskeyLogin struct {
	operatorID     int64
	credentialID   int64
	credentialJSON []byte
}

// verifyLogin consumes the ceremony and verifies the assertion. A refused
// assertion is a rejection; a failed read is returned as it is.
func (s *operatorPasskeyService) verifyLogin(ctx context.Context, req OperatorPasskeyLoginFinishRequest) (verifiedOperatorPasskeyLogin, error) {
	sessionRow, err := s.records.ConsumeSession(ctx, req.SessionID, platform.OperatorPasskeySessionPurposeLogin, time.Now())
	if err != nil {
		return verifiedOperatorPasskeyLogin{}, err
	}
	if sessionRow == nil {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(authService.ErrPasskeySessionInvalid)
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionRow.SessionJSON, &sessionData); err != nil {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(err)
	}
	webAuthn, err := s.webAuthnForOrigin(sessionRow.ExpectedOrigin)
	if err != nil {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(err)
	}
	httpReq, err := authService.PasskeyResponseRequest(ctx, req.CredentialResponse)
	if err != nil {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(err)
	}
	var matchedCredentialID int64
	// The WebAuthn library reports every handler error as a failed
	// assertion; a failed read is kept apart so a store outage does not read
	// as wrong credentials.
	var readErr error
	user, credential, err := webAuthn.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		row, err := s.records.FindActiveCredential(ctx, rawID, userHandle)
		if err != nil {
			readErr = err
			return nil, err
		}
		if row == nil {
			return nil, &InvalidCredentialsError{}
		}
		matchedCredentialID = row.ID
		operator, err := s.findOperator(ctx, row.OperatorID)
		if err != nil {
			var notFound *OperatorNotFoundError
			if !errors.As(err, &notFound) {
				readErr = err
			}
			return nil, err
		}
		user, err := s.passkeyUserForOperator(ctx, operator)
		if err != nil {
			readErr = err
			return nil, err
		}
		return user, nil
	}, sessionData, httpReq)
	if readErr != nil {
		return verifiedOperatorPasskeyLogin{}, readErr
	}
	if err != nil {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(&InvalidCredentialsError{})
	}
	passkeyUser, ok := user.(*authService.WebAuthnUser)
	if !ok {
		return verifiedOperatorPasskeyLogin{}, rejectCeremony(&InvalidCredentialsError{})
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return verifiedOperatorPasskeyLogin{}, err
	}
	return verifiedOperatorPasskeyLogin{operatorID: passkeyUser.ID, credentialID: matchedCredentialID, credentialJSON: credentialJSON}, nil
}

// operatorPasskeyRejection marks a ceremony the verification refused.
type operatorPasskeyRejection struct {
	cause error
}

func (r operatorPasskeyRejection) Error() string { return r.cause.Error() }

func (r operatorPasskeyRejection) Unwrap() error { return r.cause }

func rejectCeremony(cause error) error {
	return operatorPasskeyRejection{cause: cause}
}

// completeCeremony runs a ceremony completion in one administrative
// transaction. A rejection commits: the consumed ceremony stays spent, so
// the same challenge cannot be tried again, and the caller receives the
// rejection's cause. Any other failure rolls back, so the ceremony can be
// completed again after a failed read or write. The unit of work comes from
// the request context, which the API root attaches to every route.
func (s *operatorPasskeyService) completeCeremony(ctx context.Context, fn func(context.Context) error) error {
	var rejected error
	err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		rejected = nil
		err := fn(txCtx)
		var rejection operatorPasskeyRejection
		if errors.As(err, &rejection) {
			rejected = rejection.cause
			return nil
		}
		return err
	})
	if err != nil {
		return err
	}
	return rejected
}

// findOperator resolves the operator a ceremony belongs to; a missing row
// is OperatorNotFoundError, never a nil operator.
func (s *operatorPasskeyService) findOperator(ctx context.Context, operatorID int64) (*platform.Operator, error) {
	operator, err := s.operators.FindByID(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	if operator == nil {
		return nil, &OperatorNotFoundError{OperatorID: operatorID}
	}
	return operator, nil
}

func (s *operatorPasskeyService) ListCredentials(ctx context.Context, operatorID int64) ([]authService.PasskeyCredentialSummary, error) {
	rows, err := s.records.ListActiveCredentials(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	result := make([]authService.PasskeyCredentialSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, *summarizeOperatorPasskeyCredential(row))
	}
	return result, nil
}

func (s *operatorPasskeyService) RevokeCredential(ctx context.Context, operatorID, credentialID int64) error {
	revoked, err := s.records.RevokeCredential(ctx, operatorID, credentialID, time.Now())
	if err != nil {
		return err
	}
	if !revoked {
		return authService.ErrPasskeyNotFound
	}
	return nil
}

func (s *operatorPasskeyService) passkeyUserForOperator(ctx context.Context, operator *platform.Operator, sessionUserHandle ...[]byte) (*authService.WebAuthnUser, error) {
	rows, err := s.records.ListActiveCredentials(ctx, operator.ID)
	if err != nil {
		return nil, err
	}
	credentials := make([]webauthn.Credential, 0, len(rows))
	var userHandle []byte
	for _, row := range rows {
		if len(userHandle) == 0 {
			userHandle = row.UserHandle
		}
		var credential webauthn.Credential
		if err := json.Unmarshal(row.CredentialJSON, &credential); err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	if len(userHandle) == 0 {
		if len(sessionUserHandle) > 0 && len(sessionUserHandle[0]) > 0 {
			userHandle = sessionUserHandle[0]
		} else {
			userHandle = make([]byte, authService.PasskeyUserHandleBytes)
			if _, err := rand.Read(userHandle); err != nil {
				return nil, err
			}
		}
	}
	return &authService.WebAuthnUser{
		ID:          operator.ID,
		UserHandle:  userHandle,
		Name:        operator.Email,
		DisplayName: operator.DisplayName,
		Credentials: credentials,
	}, nil
}

func (s *operatorPasskeyService) webAuthnForOrigin(origin string) (*webauthn.WebAuthn, error) {
	rpID, err := s.rpIDForOrigin(origin)
	if err != nil {
		return nil, err
	}
	return authService.NewWebAuthnForOrigin(rpID, s.rpName, origin)
}

func (s *operatorPasskeyService) rpIDForOrigin(origin string) (string, error) {
	return authService.ResolvePasskeyRPIDForOrigin(s.rpID, origin)
}

func (s *operatorPasskeyService) validateOperatorOrigin(origin string) error {
	host, err := authService.OriginHostWithoutPort(origin)
	if err != nil {
		return authService.ErrPasskeyOriginInvalid
	}
	if host != s.operatorOriginHost {
		return authService.ErrPasskeyOriginInvalid
	}
	return nil
}

func summarizeOperatorPasskeyCredential(row *platform.OperatorPasskeyCredential) *authService.PasskeyCredentialSummary {
	return &authService.PasskeyCredentialSummary{
		ID:         strconv.FormatInt(row.ID, 10),
		Name:       row.Name,
		CreatedAt:  row.CreatedAt,
		LastUsedAt: row.LastUsedAt,
	}
}
