package platform

import (
	"context"
	"crypto/rand"
	"database/sql"
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
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
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
	Repos               *repositories.Factory
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
	repos              *repositories.Factory
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
	if cfg.Repos == nil {
		return nil, errors.New("OperatorPasskeyServiceConfig.Repos is required")
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
		repos:              cfg.Repos,
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
	operator, err := s.repos.Operator.FindByID(ctx, operatorID)
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
	operator, err := s.repos.Operator.FindByID(ctx, req.OperatorID)
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
	if err := s.repos.OperatorPasskeySession.Create(ctx, session); err != nil {
		return nil, err
	}
	return &authService.PasskeyCredentialCreation{SessionID: sessionID.String(), Options: creation}, nil
}

func (s *operatorPasskeyService) FinishRegistration(ctx context.Context, req OperatorPasskeyRegistrationFinishRequest) (*authService.PasskeyCredentialSummary, error) {
	sessionRow, err := s.repos.OperatorPasskeySession.Consume(ctx, req.SessionID, platform.OperatorPasskeySessionPurposeRegistration, time.Now())
	if err != nil {
		return nil, authService.ErrPasskeySessionInvalid
	}
	if sessionRow.OperatorID == nil || *sessionRow.OperatorID != req.OperatorID {
		return nil, authService.ErrPasskeySessionInvalid
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionRow.SessionJSON, &sessionData); err != nil {
		return nil, err
	}
	operator, err := s.repos.Operator.FindByID(ctx, req.OperatorID)
	if err != nil {
		return nil, err
	}
	user, err := s.passkeyUserForOperator(ctx, operator, sessionData.UserID)
	if err != nil {
		return nil, err
	}
	webAuthn, err := s.webAuthnForOrigin(sessionRow.ExpectedOrigin)
	if err != nil {
		return nil, err
	}
	httpReq, err := authService.PasskeyResponseRequest(ctx, req.CredentialResponse)
	if err != nil {
		return nil, err
	}
	credential, err := webAuthn.FinishRegistration(user, sessionData, httpReq)
	if err != nil {
		return nil, err
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return nil, err
	}
	name := authService.NormalizePasskeyName(req.Name)
	if name == "" {
		name = "Passkey"
	}
	row := &platform.OperatorPasskeyCredential{
		OperatorID:     req.OperatorID,
		UserHandle:     user.WebAuthnID(),
		CredentialID:   credential.ID,
		CredentialJSON: credentialJSON,
		Name:           name,
	}
	if err := s.repos.OperatorPasskeyCredential.Create(ctx, row); err != nil {
		return nil, err
	}
	return summarizeOperatorPasskeyCredential(row), nil
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
	if err := s.repos.OperatorPasskeySession.Create(ctx, session); err != nil {
		return nil, err
	}
	return &authService.PasskeyCredentialAssertion{SessionID: sessionID.String(), Options: assertion}, nil
}

func (s *operatorPasskeyService) FinishLogin(ctx context.Context, req OperatorPasskeyLoginFinishRequest) (*authService.PasskeyLoginResult, error) {
	sessionRow, err := s.repos.OperatorPasskeySession.Consume(ctx, req.SessionID, platform.OperatorPasskeySessionPurposeLogin, time.Now())
	if err != nil {
		return nil, authService.ErrPasskeySessionInvalid
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionRow.SessionJSON, &sessionData); err != nil {
		return nil, err
	}
	webAuthn, err := s.webAuthnForOrigin(sessionRow.ExpectedOrigin)
	if err != nil {
		return nil, err
	}
	httpReq, err := authService.PasskeyResponseRequest(ctx, req.CredentialResponse)
	if err != nil {
		return nil, err
	}
	var matchedCredentialID int64
	user, credential, err := webAuthn.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		row, err := s.repos.OperatorPasskeyCredential.FindActiveByCredentialIDAndUserHandle(ctx, rawID, userHandle)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, &InvalidCredentialsError{}
			}
			return nil, err
		}
		matchedCredentialID = row.ID
		operator, err := s.repos.Operator.FindByID(ctx, row.OperatorID)
		if err != nil {
			return nil, err
		}
		return s.passkeyUserForOperator(ctx, operator)
	}, sessionData, httpReq)
	if err != nil {
		return nil, &InvalidCredentialsError{}
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return nil, err
	}
	if err := s.repos.OperatorPasskeyCredential.UpdateAfterUse(ctx, matchedCredentialID, credentialJSON, time.Now()); err != nil {
		return nil, err
	}
	passkeyUser, ok := user.(*authService.WebAuthnUser)
	if !ok {
		return nil, &InvalidCredentialsError{}
	}
	accessToken, refreshToken, err := s.authService.IssueTokensForAuthenticatedOperator(ctx, passkeyUser.ID, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}
	return &authService.PasskeyLoginResult{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *operatorPasskeyService) ListCredentials(ctx context.Context, operatorID int64) ([]authService.PasskeyCredentialSummary, error) {
	rows, err := s.repos.OperatorPasskeyCredential.FindActiveByOperatorID(ctx, operatorID)
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
	if err := s.repos.OperatorPasskeyCredential.Revoke(ctx, operatorID, credentialID, time.Now()); err != nil {
		return authService.ErrPasskeyNotFound
	}
	return nil
}

func (s *operatorPasskeyService) passkeyUserForOperator(ctx context.Context, operator *platform.Operator, sessionUserHandle ...[]byte) (*authService.WebAuthnUser, error) {
	rows, err := s.repos.OperatorPasskeyCredential.FindActiveByOperatorID(ctx, operator.ID)
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
