package domain

import "encoding/json"

// The requests and results of the passkey ceremonies (#3331). The browser's
// WebAuthn payloads travel opaquely: the module produces the options the
// relying party built and reads back the credential response the
// authenticator signed.

// PasskeyEnrollmentChallenge is what a registration start hands the portal:
// the challenge token that authorises the registration and the mailbox the
// code went to.
type PasskeyEnrollmentChallenge struct {
	ChallengeToken string
	MaskedEmail    string
}

// PasskeyCeremonyOptions names one started ceremony and carries the options
// the browser passes to the authenticator.
type PasskeyCeremonyOptions struct {
	SessionID string
	Options   any
}

// PasskeyLoginResult is the token pair a completed login ceremony minted.
type PasskeyLoginResult struct {
	AccessToken  string
	RefreshToken string
}

// AccountPasskeyRegistrationStart starts a school-portal registration.
// ExpectedOrigin is the browser origin the ceremony runs on, and
// TenantSubdomain the school whose portal it must be.
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
