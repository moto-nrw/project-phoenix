package identityaccess

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"
)

// The passkey ceremonies (#3331). A registration is gated by an e-mail code,
// a login is discoverable, and both are pinned to the portal's own origin.
// The browser's WebAuthn payloads travel opaquely: the module hands back the
// options the relying party built and reads back the response the
// authenticator signed.
var (
	// ErrPasskeyFlowsUnavailable reports a module composed without the
	// passkey ceremony dependencies.
	ErrPasskeyFlowsUnavailable = errors.New("passkey ceremonies are not composed")
	// ErrPasskeyOriginInvalid refuses a ceremony started from a host that is
	// not the portal's own.
	ErrPasskeyOriginInvalid = errors.New("passkey origin is invalid")
	// ErrPasskeySessionInvalid refuses a ceremony whose stored state does
	// not match the request completing it.
	ErrPasskeySessionInvalid = errors.New("passkey session is invalid")
	// ErrPasskeyNotFound reports a revocation that matched no active
	// credential.
	ErrPasskeyNotFound = errors.New("passkey not found")
)

// PasskeyEnrollmentChallenge is what a registration start hands the portal.
// The tags are the wire contract both portals render it under.
type PasskeyEnrollmentChallenge struct {
	ChallengeToken string `json:"challenge_token"`
	MaskedEmail    string `json:"masked_email"`
}

// PasskeyCeremonyOptions names one started ceremony and carries the options
// the browser passes to the authenticator.
type PasskeyCeremonyOptions struct {
	SessionID string `json:"session_id"`
	Options   any    `json:"options"`
}

// PasskeyLoginResult is the token pair a completed login ceremony minted.
type PasskeyLoginResult struct {
	AccessToken  string
	RefreshToken string
}

// PasskeyCredentialSummary is one registered credential as the portals list
// it.
type PasskeyCredentialSummary struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// AccountPasskeyRegistrationStart starts a school-portal registration.
type AccountPasskeyRegistrationStart struct {
	AccountID       int64
	TenantID        int64
	TenantSubdomain string
	ExpectedOrigin  string
	Code            string
	Name            string
}

// AccountPasskeyRegistrationFinish completes a school-portal registration.
type AccountPasskeyRegistrationFinish struct {
	AccountID          int64
	SessionID          string
	CredentialResponse json.RawMessage
	Name               string
}

// AccountPasskeyLoginStart starts a school-portal discoverable login.
type AccountPasskeyLoginStart struct {
	TenantID        int64
	TenantSubdomain string
	ExpectedOrigin  string
}

// AccountPasskeyLoginFinish completes a school-portal login.
type AccountPasskeyLoginFinish struct {
	SessionID          string
	CredentialResponse json.RawMessage
	IPAddress          string
	UserAgent          string
}

// OperatorPasskeyRegistrationStart starts an operator registration.
type OperatorPasskeyRegistrationStart struct {
	OperatorID     int64
	ExpectedOrigin string
	Code           string
	Name           string
}

// OperatorPasskeyRegistrationFinish completes an operator registration.
type OperatorPasskeyRegistrationFinish struct {
	OperatorID         int64
	SessionID          string
	CredentialResponse json.RawMessage
	Name               string
}

// OperatorPasskeyLoginFinish completes an operator login.
type OperatorPasskeyLoginFinish struct {
	SessionID          string
	CredentialResponse json.RawMessage
	IPAddress          string
	UserAgent          string
}

// AccountPasskeyFlows is the school-portal ceremony capability. A credential
// belongs to the account and carries no school; the login verifies the
// account's membership in the school whose portal started the ceremony
// before it mints a session.
type AccountPasskeyFlows interface {
	StartAccountPasskeyEnrollment(ctx context.Context, accountID, tenantID int64, ip net.IP) (PasskeyEnrollmentChallenge, error)
	BeginAccountPasskeyRegistration(ctx context.Context, request AccountPasskeyRegistrationStart) (PasskeyCeremonyOptions, error)
	FinishAccountPasskeyRegistration(ctx context.Context, request AccountPasskeyRegistrationFinish) (PasskeyCredentialSummary, error)
	BeginAccountPasskeyLogin(ctx context.Context, request AccountPasskeyLoginStart) (PasskeyCeremonyOptions, error)
	FinishAccountPasskeyLogin(ctx context.Context, request AccountPasskeyLoginFinish) (PasskeyLoginResult, error)
	ListAccountPasskeyCredentials(ctx context.Context, accountID int64) ([]PasskeyCredentialSummary, error)
	RevokeAccountPasskeyCredential(ctx context.Context, accountID, credentialID int64) error
}

// OperatorPasskeyFlows is the operator-portal ceremony capability.
type OperatorPasskeyFlows interface {
	StartOperatorPasskeyEnrollment(ctx context.Context, operatorID int64, ip net.IP) (PasskeyEnrollmentChallenge, error)
	BeginOperatorPasskeyRegistration(ctx context.Context, request OperatorPasskeyRegistrationStart) (PasskeyCeremonyOptions, error)
	FinishOperatorPasskeyRegistration(ctx context.Context, request OperatorPasskeyRegistrationFinish) (PasskeyCredentialSummary, error)
	BeginOperatorPasskeyLogin(ctx context.Context, expectedOrigin string) (PasskeyCeremonyOptions, error)
	FinishOperatorPasskeyLogin(ctx context.Context, request OperatorPasskeyLoginFinish) (PasskeyLoginResult, error)
	ListOperatorPasskeyCredentials(ctx context.Context, operatorID int64) ([]PasskeyCredentialSummary, error)
	RevokeOperatorPasskeyCredential(ctx context.Context, operatorID, credentialID int64) error
}

func (m *Module) StartAccountPasskeyEnrollment(ctx context.Context, accountID, tenantID int64, ip net.IP) (PasskeyEnrollmentChallenge, error) {
	return m.engine.StartAccountPasskeyEnrollment(ctx, accountID, tenantID, ip)
}

func (m *Module) BeginAccountPasskeyRegistration(ctx context.Context, request AccountPasskeyRegistrationStart) (PasskeyCeremonyOptions, error) {
	return m.engine.BeginAccountPasskeyRegistration(ctx, request)
}

func (m *Module) FinishAccountPasskeyRegistration(ctx context.Context, request AccountPasskeyRegistrationFinish) (PasskeyCredentialSummary, error) {
	return m.engine.FinishAccountPasskeyRegistration(ctx, request)
}

func (m *Module) BeginAccountPasskeyLogin(ctx context.Context, request AccountPasskeyLoginStart) (PasskeyCeremonyOptions, error) {
	return m.engine.BeginAccountPasskeyLogin(ctx, request)
}

func (m *Module) FinishAccountPasskeyLogin(ctx context.Context, request AccountPasskeyLoginFinish) (PasskeyLoginResult, error) {
	return m.engine.FinishAccountPasskeyLogin(ctx, request)
}

func (m *Module) ListAccountPasskeyCredentials(ctx context.Context, accountID int64) ([]PasskeyCredentialSummary, error) {
	return m.engine.ListAccountPasskeyCredentials(ctx, accountID)
}

func (m *Module) RevokeAccountPasskeyCredential(ctx context.Context, accountID, credentialID int64) error {
	return m.engine.RevokeAccountPasskeyCredential(ctx, accountID, credentialID)
}

func (m *Module) StartOperatorPasskeyEnrollment(ctx context.Context, operatorID int64, ip net.IP) (PasskeyEnrollmentChallenge, error) {
	return m.engine.StartOperatorPasskeyEnrollment(ctx, operatorID, ip)
}

func (m *Module) BeginOperatorPasskeyRegistration(ctx context.Context, request OperatorPasskeyRegistrationStart) (PasskeyCeremonyOptions, error) {
	return m.engine.BeginOperatorPasskeyRegistration(ctx, request)
}

func (m *Module) FinishOperatorPasskeyRegistration(ctx context.Context, request OperatorPasskeyRegistrationFinish) (PasskeyCredentialSummary, error) {
	return m.engine.FinishOperatorPasskeyRegistration(ctx, request)
}

func (m *Module) BeginOperatorPasskeyLogin(ctx context.Context, expectedOrigin string) (PasskeyCeremonyOptions, error) {
	return m.engine.BeginOperatorPasskeyLogin(ctx, expectedOrigin)
}

func (m *Module) FinishOperatorPasskeyLogin(ctx context.Context, request OperatorPasskeyLoginFinish) (PasskeyLoginResult, error) {
	return m.engine.FinishOperatorPasskeyLogin(ctx, request)
}

func (m *Module) ListOperatorPasskeyCredentials(ctx context.Context, operatorID int64) ([]PasskeyCredentialSummary, error) {
	return m.engine.ListOperatorPasskeyCredentials(ctx, operatorID)
}

func (m *Module) RevokeOperatorPasskeyCredential(ctx context.Context, operatorID, credentialID int64) error {
	return m.engine.RevokeOperatorPasskeyCredential(ctx, operatorID, credentialID)
}
