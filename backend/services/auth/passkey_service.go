package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const PasskeyUserHandleBytes = 32

const PasskeyCeremonyTimeout = 10 * time.Minute

var (
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
	Repos *repositories.Factory
	// Records is the consumer-owned port over the Identity & Access
	// school-portal passkey credentials and ceremony sessions.
	Records      PasskeyRecords
	MFAService   MFAService
	AuthService  AuthService
	DB           *bun.DB
	Logger       *slog.Logger
	RPID         string
	RPName       string
	TenantDomain string
}

type PasskeyEnrollmentChallenge struct {
	ChallengeToken string `json:"challenge_token"`
	MaskedEmail    string `json:"masked_email"`
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
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type passkeyService struct {
	repos        *repositories.Factory
	records      PasskeyRecords
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
	if cfg.Records == nil {
		return nil, errors.New("PasskeyServiceConfig.Records is required")
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
		records:      cfg.Records,
		mfaService:   cfg.MFAService,
		authService:  cfg.AuthService,
		db:           cfg.DB,
		logger:       logger,
		rpID:         HostWithoutPort(rpID),
		rpName:       rpName,
		tenantDomain: HostWithoutPort(tenantDomain),
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
		MaskedEmail:    MaskEmailForUX(account.Email),
	}, nil
}

func (s *passkeyService) BeginRegistration(ctx context.Context, req PasskeyRegistrationStartRequest) (*PasskeyCredentialCreation, error) {
	if err := s.validateTenantOrigin(req.ExpectedOrigin, req.TenantSubdomain); err != nil {
		return nil, &AuthError{Op: "begin passkey registration", Err: err}
	}
	// Same portal binding as the tenant enroll-confirm: StartEnrollmentChallenge
	// issues a tenant-scope code for this school, so only that code may be
	// redeemed here.
	if err := s.mfaService.VerifyCodeForAccount(ctx, req.AccountID, req.TenantID, req.Code, jwt.MFAChallengeScopeTenant); err != nil {
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
	session := &PasskeySession{
		AccountID:      &accountID,
		TenantID:       &tenantID,
		Purpose:        PasskeySessionPurposeRegistration,
		RPID:           rpID,
		ExpectedOrigin: req.ExpectedOrigin,
		SessionJSON:    sessionJSON,
		ExpiresAt:      sessionData.Expires,
	}
	session.ID = sessionID.String()
	if err := s.records.CreateSession(ctx, session); err != nil {
		return nil, &AuthError{Op: "store passkey session", Err: err}
	}

	return &PasskeyCredentialCreation{SessionID: sessionID.String(), Options: creation}, nil
}

// FinishRegistration consumes the ceremony and stores the credential in one
// administrative transaction (see completeCeremony).
func (s *passkeyService) FinishRegistration(ctx context.Context, req PasskeyRegistrationFinishRequest) (*PasskeyCredentialSummary, error) {
	var summary *PasskeyCredentialSummary
	err := s.completeCeremony(ctx, func(txCtx context.Context) error {
		row, err := s.verifyRegistration(txCtx, req)
		if err != nil {
			return err
		}
		if err := s.records.CreateCredential(txCtx, row); err != nil {
			return &AuthError{Op: "store passkey credential", Err: err}
		}
		summary = summarizePasskeyCredential(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// verifyRegistration consumes the ceremony and verifies the attestation. It
// returns the credential to store; a refused ceremony is a rejection.
func (s *passkeyService) verifyRegistration(ctx context.Context, req PasskeyRegistrationFinishRequest) (*PasskeyCredential, error) {
	sessionRow, err := s.records.ConsumeSession(ctx, req.SessionID, PasskeySessionPurposeRegistration, time.Now())
	if err != nil {
		return nil, &AuthError{Op: "consume passkey registration session", Err: err}
	}
	if sessionRow == nil || sessionRow.AccountID == nil || *sessionRow.AccountID != req.AccountID {
		return nil, rejectCeremony(&AuthError{Op: "finish passkey registration", Err: ErrPasskeySessionInvalid})
	}

	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionRow.SessionJSON, &sessionData); err != nil {
		return nil, rejectCeremony(&AuthError{Op: "unmarshal passkey session", Err: err})
	}
	account, err := s.repos.Account.FindByID(ctx, req.AccountID)
	if err != nil {
		return nil, rejectCeremony(&AuthError{Op: "finish passkey registration", Err: ErrAccountNotFound})
	}
	user, err := s.passkeyUserForAccount(ctx, account, sessionData.UserID)
	if err != nil {
		return nil, &AuthError{Op: "load passkey user", Err: err}
	}
	webAuthn, err := s.webAuthnForOrigin(sessionRow.ExpectedOrigin)
	if err != nil {
		return nil, rejectCeremony(&AuthError{Op: "configure passkey registration finish", Err: err})
	}
	httpReq, err := PasskeyResponseRequest(ctx, req.CredentialResponse)
	if err != nil {
		return nil, rejectCeremony(&AuthError{Op: "parse passkey registration response", Err: err})
	}
	credential, err := webAuthn.FinishRegistration(user, sessionData, httpReq)
	if err != nil {
		return nil, rejectCeremony(&AuthError{Op: "finish passkey registration", Err: err})
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return nil, &AuthError{Op: "marshal passkey credential", Err: err}
	}
	name := NormalizePasskeyName(req.Name)
	if name == "" {
		name = NormalizePasskeyName("Passkey")
	}
	return &PasskeyCredential{
		AccountID:      req.AccountID,
		UserHandle:     user.WebAuthnID(),
		CredentialID:   credential.ID,
		CredentialJSON: credentialJSON,
		Name:           name,
	}, nil
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
	session := &PasskeySession{
		TenantID:       &tenantID,
		Purpose:        PasskeySessionPurposeLogin,
		RPID:           rpID,
		ExpectedOrigin: req.ExpectedOrigin,
		SessionJSON:    sessionJSON,
		ExpiresAt:      sessionData.Expires,
	}
	session.ID = sessionID.String()
	if err := s.records.CreateSession(ctx, session); err != nil {
		return nil, &AuthError{Op: "store passkey login session", Err: err}
	}
	return &PasskeyCredentialAssertion{SessionID: sessionID.String(), Options: assertion}, nil
}

// FinishLogin consumes the ceremony and records the credential use in one
// administrative transaction (see completeCeremony). The token pair is
// issued after the commit.
func (s *passkeyService) FinishLogin(ctx context.Context, req PasskeyLoginFinishRequest) (*PasskeyLoginResult, error) {
	var verified verifiedPasskeyLogin
	err := s.completeCeremony(ctx, func(txCtx context.Context) error {
		result, err := s.verifyLogin(txCtx, req)
		if err != nil {
			return err
		}
		if err := s.records.RecordCredentialUse(txCtx, result.credentialID, result.credentialJSON, time.Now()); err != nil {
			return &AuthError{Op: "update passkey after login", Err: err}
		}
		verified = result
		return nil
	})
	if err != nil {
		return nil, err
	}
	accessToken, refreshToken, err := s.authService.IssueTokensForAuthenticatedAccount(ctx, verified.accountID, verified.tenantID, req.IPAddress, req.UserAgent)
	if err != nil {
		return nil, err
	}
	return &PasskeyLoginResult{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

type verifiedPasskeyLogin struct {
	accountID      int64
	tenantID       int64
	credentialID   int64
	credentialJSON []byte
}

// verifyLogin consumes the ceremony, verifies the assertion and checks that
// the account belongs to the school whose portal started it. A refused
// assertion or a school the account has no access to is a rejection; a
// failed read is returned as it is.
func (s *passkeyService) verifyLogin(ctx context.Context, req PasskeyLoginFinishRequest) (verifiedPasskeyLogin, error) {
	sessionRow, err := s.records.ConsumeSession(ctx, req.SessionID, PasskeySessionPurposeLogin, time.Now())
	if err != nil {
		return verifiedPasskeyLogin{}, &AuthError{Op: "consume passkey login session", Err: err}
	}
	if sessionRow == nil || sessionRow.TenantID == nil {
		return verifiedPasskeyLogin{}, rejectCeremony(&AuthError{Op: "finish passkey login", Err: ErrPasskeySessionInvalid})
	}
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionRow.SessionJSON, &sessionData); err != nil {
		return verifiedPasskeyLogin{}, rejectCeremony(&AuthError{Op: "unmarshal passkey session", Err: err})
	}
	webAuthn, err := s.webAuthnForOrigin(sessionRow.ExpectedOrigin)
	if err != nil {
		return verifiedPasskeyLogin{}, rejectCeremony(&AuthError{Op: "configure passkey login finish", Err: err})
	}
	httpReq, err := PasskeyResponseRequest(ctx, req.CredentialResponse)
	if err != nil {
		return verifiedPasskeyLogin{}, rejectCeremony(&AuthError{Op: "parse passkey login response", Err: err})
	}

	var matchedCredentialID int64
	// The WebAuthn library reports every handler error as a failed
	// assertion; a failed read is kept apart so a store outage does not read
	// as wrong credentials.
	var readErr error
	user, credential, err := webAuthn.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		row, err := s.records.FindActiveCredential(ctx, rawID, userHandle)
		if err != nil {
			readErr = &AuthError{Op: "find passkey credential", Err: err}
			return nil, err
		}
		if row == nil {
			return nil, ErrInvalidCredentials
		}
		matchedCredentialID = row.ID
		account, err := s.repos.Account.FindByID(ctx, row.AccountID)
		if err != nil {
			return nil, err
		}
		user, err := s.passkeyUserForAccount(ctx, account)
		if err != nil {
			readErr = &AuthError{Op: "load passkey user", Err: err}
			return nil, err
		}
		return user, nil
	}, sessionData, httpReq)
	if readErr != nil {
		return verifiedPasskeyLogin{}, readErr
	}
	if err != nil {
		return verifiedPasskeyLogin{}, rejectCeremony(&AuthError{Op: "finish passkey login", Err: ErrInvalidCredentials})
	}
	passkeyUser, ok := user.(*WebAuthnUser)
	if !ok {
		return verifiedPasskeyLogin{}, rejectCeremony(&AuthError{Op: "finish passkey login", Err: ErrInvalidCredentials})
	}
	tenantID := *sessionRow.TenantID
	hasAccess, err := s.authService.VerifyAccountTenantMembership(ctx, passkeyUser.ID, tenantID)
	if err != nil || !hasAccess {
		// The school binding is decided per ceremony: a passkey of another
		// school never mints a session here, and the ceremony is spent.
		return verifiedPasskeyLogin{}, rejectCeremony(&AuthError{Op: "verify passkey tenant access", Err: ErrTenantAccessDenied})
	}
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return verifiedPasskeyLogin{}, &AuthError{Op: "marshal passkey credential", Err: err}
	}
	return verifiedPasskeyLogin{
		accountID: passkeyUser.ID, tenantID: tenantID, credentialID: matchedCredentialID, credentialJSON: credentialJSON,
	}, nil
}

// passkeyRejection marks a ceremony the verification refused.
type passkeyRejection struct {
	cause error
}

func (r passkeyRejection) Error() string { return r.cause.Error() }

func (r passkeyRejection) Unwrap() error { return r.cause }

func rejectCeremony(cause error) error {
	return passkeyRejection{cause: cause}
}

// completeCeremony runs a ceremony completion in one administrative
// transaction. A rejection commits: the consumed ceremony stays spent, so
// the same challenge cannot be tried again, and the caller receives the
// rejection's cause. Any other failure rolls back, so the ceremony can be
// completed again after a failed read or write. The unit of work comes from
// the request context, which the API root attaches to every route.
func (s *passkeyService) completeCeremony(ctx context.Context, fn func(context.Context) error) error {
	var rejected error
	err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		rejected = nil
		err := fn(txCtx)
		var rejection passkeyRejection
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

func (s *passkeyService) ListCredentials(ctx context.Context, accountID int64) ([]PasskeyCredentialSummary, error) {
	rows, err := s.records.ListActiveCredentials(ctx, accountID)
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
	revoked, err := s.records.RevokeCredential(ctx, accountID, credentialID, time.Now())
	if err != nil {
		return &AuthError{Op: "revoke passkey", Err: err}
	}
	if !revoked {
		return &AuthError{Op: "revoke passkey", Err: ErrPasskeyNotFound}
	}
	return nil
}

func (s *passkeyService) passkeyUserForAccount(ctx context.Context, account *authModel.Account, sessionUserHandle ...[]byte) (*WebAuthnUser, error) {
	rows, err := s.records.ListActiveCredentials(ctx, account.ID)
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
			userHandle = make([]byte, PasskeyUserHandleBytes)
			if _, err := rand.Read(userHandle); err != nil {
				return nil, err
			}
		}
	}
	return &WebAuthnUser{
		ID:          account.ID,
		UserHandle:  userHandle,
		Name:        account.Email,
		DisplayName: account.Email,
		Credentials: credentials,
	}, nil
}

func (s *passkeyService) webAuthnForOrigin(origin string) (*webauthn.WebAuthn, error) {
	rpID, err := s.rpIDForOrigin(origin)
	if err != nil {
		return nil, err
	}
	return NewWebAuthnForOrigin(rpID, s.rpName, origin)
}

// NewWebAuthnForOrigin builds a *webauthn.WebAuthn scoped to a single origin
// with the ceremony timeouts shared by the tenant and operator passkey flows.
func NewWebAuthnForOrigin(rpID, rpName, origin string) (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpName,
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
	originHost, err := OriginHostWithoutPort(origin)
	if err != nil {
		return ErrPasskeyOriginInvalid
	}
	root := strings.ToLower(HostWithoutPort(s.tenantDomain))
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

// WebAuthnUser adapts an account or operator to the go-webauthn user
// interface. ID is the account/operator PK, read by callers outside the
// webauthn.User method set. Shared by the tenant and operator passkey flows.
type WebAuthnUser struct {
	ID          int64
	UserHandle  []byte
	Name        string
	DisplayName string
	Credentials []webauthn.Credential
}

func (u *WebAuthnUser) WebAuthnID() []byte                         { return u.UserHandle }
func (u *WebAuthnUser) WebAuthnName() string                       { return u.Name }
func (u *WebAuthnUser) WebAuthnDisplayName() string                { return u.DisplayName }
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }

func PasskeyResponseRequest(ctx context.Context, raw json.RawMessage) (*http.Request, error) {
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

func summarizePasskeyCredential(row *PasskeyCredential) *PasskeyCredentialSummary {
	return &PasskeyCredentialSummary{
		ID:         strconv.FormatInt(row.ID, 10),
		Name:       row.Name,
		CreatedAt:  row.CreatedAt,
		LastUsedAt: row.LastUsedAt,
	}
}

func NormalizePasskeyName(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}

func HostWithoutPort(value string) string {
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

func ResolvePasskeyRPIDForOrigin(configuredRPID, origin string) (string, error) {
	rpID := HostWithoutPort(configuredRPID)
	if rpID != "localhost" {
		return rpID, nil
	}
	originHost, err := OriginHostWithoutPort(origin)
	if err != nil {
		return "", err
	}
	return originHost, nil
}

func OriginHostWithoutPort(origin string) (string, error) {
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
	return HostWithoutPort(parsed.Host), nil
}
