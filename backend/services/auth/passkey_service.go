package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/uptrace/bun"
)

const passkeyUserHandleBytes = 32

const PasskeyCeremonyTimeout = 10 * time.Minute

var (
	ErrPasskeyUnavailable    = errors.New("passkey service is unavailable")
	ErrPasskeyOriginInvalid  = errors.New("passkey origin is invalid")
	ErrPasskeySessionInvalid = errors.New("passkey session is invalid")
	ErrPasskeyNotFound       = errors.New("passkey not found")
)

type PasskeyService interface {
	StartEnrollmentChallenge(ctx context.Context, accountID, tenantID int64, ip net.IP) (*PasskeyEnrollmentChallenge, error)
	BeginRegistration(ctx context.Context, req PasskeyRegistrationStartRequest) (*PasskeyCredentialCreation, error)
	FinishRegistration(ctx context.Context, req PasskeyRegistrationFinishRequest) (*PasskeyCredentialSummary, error)
	BeginLogin(ctx context.Context, req PasskeyLoginStartRequest) (*PasskeyCredentialAssertion, error)
	FinishLogin(ctx context.Context, req PasskeyLoginFinishRequest) (*PasskeyLoginResult, error)
	ListCredentials(ctx context.Context, accountID int64) ([]PasskeyCredentialSummary, error)
	RevokeCredential(ctx context.Context, accountID, credentialID int64) error
}

type PasskeyServiceConfig struct {
	Repos        *repositories.Factory
	MFAService   MFAService
	AuthService  AuthService
	DB           *bun.DB
	Logger       *slog.Logger
	RPID         string
	RPName       string
	TenantDomain string
}

type PasskeyEnrollmentChallenge struct {
	ChallengeToken string
	MaskedEmail    string
}

type PasskeyRegistrationStartRequest struct {
	AccountID       int64
	TenantID        int64
	TenantSubdomain string
	ExpectedOrigin  string
	Code            string
	Name            string
}

type PasskeyRegistrationFinishRequest struct {
	AccountID          int64
	SessionID          string
	CredentialResponse json.RawMessage
	Name               string
}

type PasskeyLoginStartRequest struct {
	TenantID        int64
	TenantSubdomain string
	ExpectedOrigin  string
}

type PasskeyLoginFinishRequest struct {
	SessionID          string
	CredentialResponse json.RawMessage
	IPAddress          string
	UserAgent          string
}

type PasskeyCredentialCreation struct {
	SessionID string `json:"session_id"`
	Options   any    `json:"options"`
}

type PasskeyCredentialAssertion struct {
	SessionID string `json:"session_id"`
	Options   any    `json:"options"`
}

type PasskeyLoginResult struct {
	AccessToken  string
	RefreshToken string
}

type PasskeyCredentialSummary struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type passkeyService struct {
	repos        *repositories.Factory
	mfaService   MFAService
	authService  AuthService
	db           *bun.DB
	logger       *slog.Logger
	rpID         string
	rpName       string
	tenantDomain string
}

var _ PasskeyService = (*passkeyService)(nil)

func NewPasskeyService(cfg PasskeyServiceConfig) (PasskeyService, error) {
	if cfg.Repos == nil {
		return nil, errors.New("PasskeyServiceConfig.Repos is required")
	}
	if cfg.MFAService == nil {
		return nil, errors.New("PasskeyServiceConfig.MFAService is required")
	}
	if cfg.AuthService == nil {
		return nil, errors.New("PasskeyServiceConfig.AuthService is required")
	}
	if cfg.DB == nil {
		return nil, errors.New("PasskeyServiceConfig.DB is required")
	}
	rpID := strings.TrimSpace(cfg.RPID)
	if rpID == "" {
		return nil, errors.New("PasskeyServiceConfig.RPID is required")
	}
	rpName := strings.TrimSpace(cfg.RPName)
	if rpName == "" {
		rpName = "moto"
	}
	tenantDomain := strings.TrimSpace(cfg.TenantDomain)
	if tenantDomain == "" {
		return nil, errors.New("PasskeyServiceConfig.TenantDomain is required")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &passkeyService{
		repos:        cfg.Repos,
		mfaService:   cfg.MFAService,
		authService:  cfg.AuthService,
		db:           cfg.DB,
		logger:       logger,
		rpID:         hostWithoutPort(rpID),
		rpName:       rpName,
		tenantDomain: hostWithoutPort(tenantDomain),
	}, nil
}

func (s *passkeyService) StartEnrollmentChallenge(ctx context.Context, accountID, tenantID int64, ip net.IP) (*PasskeyEnrollmentChallenge, error) {
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return nil, &AuthError{Op: "start passkey enrollment", Err: ErrAccountNotFound}
	}
	challengeToken, err := s.mfaService.StartChallenge(ctx, accountID, tenantID, jwt.MFAChallengeScopeTenant, ip)
	if err != nil {
		return nil, &AuthError{Op: "start passkey enrollment challenge", Err: err}
	}
	return &PasskeyEnrollmentChallenge{
		ChallengeToken: challengeToken,
		MaskedEmail:    maskEmailForUX(account.Email),
	}, nil
}

func (s *passkeyService) BeginRegistration(ctx context.Context, req PasskeyRegistrationStartRequest) (*PasskeyCredentialCreation, error) {
	if err := s.validateTenantOrigin(req.ExpectedOrigin, req.TenantSubdomain); err != nil {
		return nil, &AuthError{Op: "begin passkey registration", Err: err}
	}
	if err := s.mfaService.VerifyCodeForAccount(ctx, req.AccountID, req.Code); err != nil {
		return nil, &AuthError{Op: "verify passkey enrollment code", Err: err}
	}

	account, err := s.repos.Account.FindByID(ctx, req.AccountID)
	if err != nil {
		return nil, &AuthError{Op: "begin passkey registration", Err: ErrAccountNotFound}
	}
	if !account.Active {
		return nil, &AuthError{Op: "begin passkey registration", Err: ErrAccountInactive}
	}

	user, err := s.passkeyUserForAccount(ctx, account)
	if err != nil {
		return nil, &AuthError{Op: "load passkey user", Err: err}
	}

	rpID, err := s.rpIDForOrigin(req.ExpectedOrigin)
	if err != nil {
		return nil, &AuthError{Op: "resolve passkey rp id", Err: err}
	}
	webAuthn, err := s.webAuthnForOrigin(req.ExpectedOrigin)
	if err != nil {
		return nil, &AuthError{Op: "configure passkey registration", Err: err}
	}
	creation, sessionData, err := webAuthn.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		}),
	)
	if err != nil {
		return nil, &AuthError{Op: "begin passkey registration", Err: err}
	}

	sessionID, err := uuid.NewV4()
	if err != nil {
		return nil, &AuthError{Op: "create passkey session id", Err: err}
	}
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return nil, &AuthError{Op: "marshal passkey session", Err: err}
	}
	accountID := req.AccountID
	tenantID := req.TenantID
	if err := s.repos.PasskeySession.Create(ctx, &authModel.PasskeySession{
		ID:             sessionID.String(),
		AccountID:      &accountID,
		TenantID:       &tenantID,
		Purpose:        authModel.PasskeySessionPurposeRegistration,
		RPID:           rpID,
		ExpectedOrigin: req.ExpectedOrigin,
		SessionJSON:    sessionJSON,
		ExpiresAt:      sessionData.Expires,
	}); err != nil {
		return nil, &AuthError{Op: "store passkey session", Err: err}
	}

	return &PasskeyCredentialCreation{SessionID: sessionID.String(), Options: creation}, nil
}

func (s *passkeyService) FinishRegistration(ctx context.Context, req PasskeyRegistrationFinishRequest) (*PasskeyCredentialSummary, error) {
	sessionRow, err := s.repos.PasskeySession.Consume(ctx, req.SessionID, authModel.PasskeySessionPurposeRegistration, time.Now())
	if err != nil {
		return nil, &AuthError{Op: "consume passkey registration session", Err: ErrPasskeySessionInvalid}
	}
	if sessionRow.AccountID == nil || *sessionRow.AccountID != req.AccountID {
		return nil, &AuthError{Op: "finish passkey registration", Err: ErrPasskeySessionInvalid}
	}

	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionRow.SessionJSON, &sessionData); err != nil {
		return nil, &AuthError{Op: "unmarshal passkey session", Err: err}
	}
	account, err := s.repos.Account.FindByID(ctx, req.AccountID)
	if err != nil {
		return nil, &AuthError{Op: "finish passkey registration", Err: ErrAccountNotFound}
	}
	user, err := s.passkeyUserForAccount(ctx, account, sessionData.UserID)
	if err != nil {
		return nil, &AuthError{Op: "load passkey user", Err: err}
	}
	webAuthn, err := s.webAuthnForOrigin(sessionRow.ExpectedOrigin)
	if err != nil {
		return nil, &AuthError{Op: "configure passkey registration finish", Err: err}
	}
	httpReq, err := passkeyResponseRequest(ctx, req.CredentialResponse)
	if err != nil {
		return nil, &AuthError{Op: "parse passkey registration response", Err: err}
	}
	credential, err := webAuthn.FinishRegistration(user, sessionData, httpReq)
	if err != nil {
		return nil, &AuthError{Op: "finish passkey registration", Err: err}
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return nil, &AuthError{Op: "marshal passkey credential", Err: err}
	}
	name := normalizePasskeyName(req.Name)
	if name == "" {
		name = normalizePasskeyName("Passkey")
	}
	row := &authModel.PasskeyCredential{
		AccountID:      req.AccountID,
		UserHandle:     user.WebAuthnID(),
		CredentialID:   credential.ID,
		CredentialJSON: credentialJSON,
		Name:           name,
	}
	if err := s.repos.PasskeyCredential.Create(ctx, row); err != nil {
		return nil, &AuthError{Op: "store passkey credential", Err: err}
	}
	return summarizePasskeyCredential(row), nil
}

func (s *passkeyService) BeginLogin(ctx context.Context, req PasskeyLoginStartRequest) (*PasskeyCredentialAssertion, error) {
	if err := s.validateTenantOrigin(req.ExpectedOrigin, req.TenantSubdomain); err != nil {
		return nil, &AuthError{Op: "begin passkey login", Err: err}
	}
	rpID, err := s.rpIDForOrigin(req.ExpectedOrigin)
	if err != nil {
		return nil, &AuthError{Op: "resolve passkey rp id", Err: err}
	}
	webAuthn, err := s.webAuthnForOrigin(req.ExpectedOrigin)
	if err != nil {
		return nil, &AuthError{Op: "configure passkey login", Err: err}
	}
	assertion, sessionData, err := webAuthn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, &AuthError{Op: "begin passkey login", Err: err}
	}
	sessionID, err := uuid.NewV4()
	if err != nil {
		return nil, &AuthError{Op: "create passkey session id", Err: err}
	}
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return nil, &AuthError{Op: "marshal passkey session", Err: err}
	}
	tenantID := req.TenantID
	if err := s.repos.PasskeySession.Create(ctx, &authModel.PasskeySession{
		ID:             sessionID.String(),
		TenantID:       &tenantID,
		Purpose:        authModel.PasskeySessionPurposeLogin,
		RPID:           rpID,
		ExpectedOrigin: req.ExpectedOrigin,
		SessionJSON:    sessionJSON,
		ExpiresAt:      sessionData.Expires,
	}); err != nil {
		return nil, &AuthError{Op: "store passkey login session", Err: err}
	}
	return &PasskeyCredentialAssertion{SessionID: sessionID.String(), Options: assertion}, nil
}

func (s *passkeyService) FinishLogin(ctx context.Context, req PasskeyLoginFinishRequest) (*PasskeyLoginResult, error) {
	sessionRow, err := s.repos.PasskeySession.Consume(ctx, req.SessionID, authModel.PasskeySessionPurposeLogin, time.Now())
	if err != nil {
		return nil, &AuthError{Op: "consume passkey login session", Err: ErrPasskeySessionInvalid}
	}
	if sessionRow.TenantID == nil {
		return nil, &AuthError{Op: "finish passkey login", Err: ErrPasskeySessionInvalid}
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionRow.SessionJSON, &sessionData); err != nil {
		return nil, &AuthError{Op: "unmarshal passkey session", Err: err}
	}
	webAuthn, err := s.webAuthnForOrigin(sessionRow.ExpectedOrigin)
	if err != nil {
		return nil, &AuthError{Op: "configure passkey login finish", Err: err}
	}
	httpReq, err := passkeyResponseRequest(ctx, req.CredentialResponse)
	if err != nil {
		return nil, &AuthError{Op: "parse passkey login response", Err: err}
	}

	var matchedCredentialID int64
	user, credential, err := webAuthn.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		row, err := s.repos.PasskeyCredential.FindActiveByCredentialIDAndUserHandle(ctx, rawID, userHandle)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrInvalidCredentials
			}
			return nil, err
		}
		matchedCredentialID = row.ID
		account, err := s.repos.Account.FindByID(ctx, row.AccountID)
		if err != nil {
			return nil, err
		}
		return s.passkeyUserForAccount(ctx, account)
	}, sessionData, httpReq)
	if err != nil {
		return nil, &AuthError{Op: "finish passkey login", Err: ErrInvalidCredentials}
	}

	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return nil, &AuthError{Op: "marshal passkey credential", Err: err}
	}
	if err := s.repos.PasskeyCredential.UpdateAfterUse(ctx, matchedCredentialID, credentialJSON, time.Now()); err != nil {
		return nil, &AuthError{Op: "update passkey after login", Err: err}
	}
	passkeyUser, ok := user.(*tenantPasskeyUser)
	if !ok {
		return nil, &AuthError{Op: "finish passkey login", Err: ErrInvalidCredentials}
	}
	accessToken, refreshToken, err := s.authService.IssueTokensForAuthenticatedAccount(ctx, passkeyUser.accountID, *sessionRow.TenantID, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}
	return &PasskeyLoginResult{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (s *passkeyService) ListCredentials(ctx context.Context, accountID int64) ([]PasskeyCredentialSummary, error) {
	rows, err := s.repos.PasskeyCredential.FindActiveByAccountID(ctx, accountID)
	if err != nil {
		return nil, &AuthError{Op: "list passkeys", Err: err}
	}
	result := make([]PasskeyCredentialSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, *summarizePasskeyCredential(row))
	}
	return result, nil
}

func (s *passkeyService) RevokeCredential(ctx context.Context, accountID, credentialID int64) error {
	if err := s.repos.PasskeyCredential.Revoke(ctx, accountID, credentialID, time.Now()); err != nil {
		return &AuthError{Op: "revoke passkey", Err: ErrPasskeyNotFound}
	}
	return nil
}

func (s *passkeyService) passkeyUserForAccount(ctx context.Context, account *authModel.Account, sessionUserHandle ...[]byte) (*tenantPasskeyUser, error) {
	rows, err := s.repos.PasskeyCredential.FindActiveByAccountID(ctx, account.ID)
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
			userHandle = make([]byte, passkeyUserHandleBytes)
			if _, err := rand.Read(userHandle); err != nil {
				return nil, err
			}
		}
	}
	return &tenantPasskeyUser{
		accountID:   account.ID,
		userHandle:  userHandle,
		name:        account.Email,
		displayName: account.Email,
		credentials: credentials,
	}, nil
}

func (s *passkeyService) webAuthnForOrigin(origin string) (*webauthn.WebAuthn, error) {
	rpID, err := s.rpIDForOrigin(origin)
	if err != nil {
		return nil, err
	}
	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: s.rpName,
		RPOrigins:     []string{origin},
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    PasskeyCeremonyTimeout,
				TimeoutUVD: PasskeyCeremonyTimeout,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    PasskeyCeremonyTimeout,
				TimeoutUVD: PasskeyCeremonyTimeout,
			},
		},
	})
}

func (s *passkeyService) rpIDForOrigin(origin string) (string, error) {
	return ResolvePasskeyRPIDForOrigin(s.rpID, origin)
}

func (s *passkeyService) validateTenantOrigin(origin, subdomain string) error {
	originHost, err := originHostWithoutPort(origin)
	if err != nil {
		return ErrPasskeyOriginInvalid
	}
	root := strings.ToLower(hostWithoutPort(s.tenantDomain))
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if root == "localhost" {
		if originHost == "localhost" || originHost == subdomain+".localhost" {
			return nil
		}
		return ErrPasskeyOriginInvalid
	}
	if subdomain == "" {
		return ErrPasskeyOriginInvalid
	}
	if originHost != subdomain+"."+root {
		return ErrPasskeyOriginInvalid
	}
	return nil
}

type tenantPasskeyUser struct {
	accountID   int64
	userHandle  []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (u *tenantPasskeyUser) WebAuthnID() []byte                         { return u.userHandle }
func (u *tenantPasskeyUser) WebAuthnName() string                       { return u.name }
func (u *tenantPasskeyUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *tenantPasskeyUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func passkeyResponseRequest(ctx context.Context, raw json.RawMessage) (*http.Request, error) {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, ErrPasskeySessionInvalid
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func summarizePasskeyCredential(row *authModel.PasskeyCredential) *PasskeyCredentialSummary {
	return &PasskeyCredentialSummary{
		ID:         row.ID,
		Name:       row.Name,
		CreatedAt:  row.CreatedAt,
		LastUsedAt: row.LastUsedAt,
	}
}

func normalizePasskeyName(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}

// PasskeyResponseRequestForPlatform exposes the shared WebAuthn response
// adapter to the platform service without forcing handlers to duplicate it.
func PasskeyResponseRequestForPlatform(ctx context.Context, raw json.RawMessage) (*http.Request, error) {
	return passkeyResponseRequest(ctx, raw)
}

func NormalizePasskeyNameForPlatform(value string) string {
	return normalizePasskeyName(value)
}

func PasskeyUserHandleBytesForPlatform() int {
	return passkeyUserHandleBytes
}

func hostWithoutPort(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil {
			value = parsed.Host
		}
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.ToLower(host)
	}
	return strings.TrimSuffix(value, ".")
}

func HostWithoutPortForPlatform(value string) string {
	return hostWithoutPort(value)
}

func ResolvePasskeyRPIDForOrigin(configuredRPID, origin string) (string, error) {
	rpID := hostWithoutPort(configuredRPID)
	if rpID != "localhost" {
		return rpID, nil
	}
	originHost, err := originHostWithoutPort(origin)
	if err != nil {
		return "", err
	}
	return originHost, nil
}

func originHostWithoutPort(origin string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("origin must include scheme and host")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("unsupported origin scheme")
	}
	return hostWithoutPort(parsed.Host), nil
}

func OriginHostWithoutPortForPlatform(origin string) (string, error) {
	return originHostWithoutPort(origin)
}
