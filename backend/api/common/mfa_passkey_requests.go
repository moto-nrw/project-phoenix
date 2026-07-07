package common

import (
	"encoding/json"
	"net/http"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// MFA and passkey request bodies shared verbatim by the tenant (api/auth)
// and operator (api/operator) portals. Both portals alias these types so
// their wire formats cannot drift; portal-specific requests (e.g. the
// tenant-only passkey login-options body) stay in their portal package.

// MFAVerifyRequest is the body for the mfa/verify endpoints.
type MFAVerifyRequest struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
	RememberDevice bool   `json:"remember_device,omitempty"`
}

// Bind validates the verify request fields.
func (req *MFAVerifyRequest) Bind(_ *http.Request) error {
	req.ChallengeToken = strings.TrimSpace(req.ChallengeToken)
	req.Code = strings.TrimSpace(req.Code)
	return validation.ValidateStruct(req,
		validation.Field(&req.ChallengeToken, validation.Required),
		validation.Field(&req.Code, validation.Required, validation.Length(authService.MFAEmailCodeLength, authService.MFAEmailCodeLength)),
	)
}

// MFAResendRequest is the body for the mfa/resend endpoints.
type MFAResendRequest struct {
	ChallengeToken string `json:"challenge_token"`
}

// Bind validates the resend request.
func (req *MFAResendRequest) Bind(_ *http.Request) error {
	req.ChallengeToken = strings.TrimSpace(req.ChallengeToken)
	return validation.ValidateStruct(req,
		validation.Field(&req.ChallengeToken, validation.Required),
	)
}

// MFAResendResponse carries the renewed challenge token. The frontend
// must replace its in-flight token with this value so the next verify
// travels with a JWT whose expiry covers the freshly emailed code.
type MFAResendResponse struct {
	ChallengeToken string `json:"challenge_token"`
}

// MFAEnrollConfirmRequest is the body for the mfa/enroll/confirm endpoints.
type MFAEnrollConfirmRequest struct {
	Code string `json:"code"`
	// RememberDevice mirrors the verify-flow flag: when true, a successful
	// confirm additionally issues a trusted-device cookie so the next
	// login on this browser can skip MFA. Optional — defaults to false.
	RememberDevice bool `json:"remember_device,omitempty"`
}

// Bind validates the confirm request.
func (req *MFAEnrollConfirmRequest) Bind(_ *http.Request) error {
	req.Code = strings.TrimSpace(req.Code)
	return validation.ValidateStruct(req,
		validation.Field(&req.Code, validation.Required, validation.Length(authService.MFAEmailCodeLength, authService.MFAEmailCodeLength)),
	)
}

// PasskeyVerifyRequest is the body for the passkey login/verify endpoints.
type PasskeyVerifyRequest struct {
	SessionID string          `json:"session_id"`
	Response  json.RawMessage `json:"response"`
}

// Bind validates the passkey verify request.
func (req *PasskeyVerifyRequest) Bind(_ *http.Request) error {
	req.SessionID = strings.TrimSpace(req.SessionID)
	return validation.ValidateStruct(req,
		validation.Field(&req.SessionID, validation.Required),
		validation.Field(&req.Response, validation.Required),
	)
}

// PasskeyRegisterOptionsRequest is the body for the passkey register/options
// endpoints.
type PasskeyRegisterOptionsRequest struct {
	Code string `json:"code"`
	Name string `json:"name,omitempty"`
}

// Bind validates the passkey register-options request.
func (req *PasskeyRegisterOptionsRequest) Bind(_ *http.Request) error {
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	return validation.ValidateStruct(req, validation.Field(&req.Code, validation.Required))
}

// PasskeyRegisterVerifyRequest is the body for the passkey register/verify
// endpoints.
type PasskeyRegisterVerifyRequest struct {
	SessionID string          `json:"session_id"`
	Name      string          `json:"name,omitempty"`
	Response  json.RawMessage `json:"response"`
}

// Bind validates the passkey register-verify request.
func (req *PasskeyRegisterVerifyRequest) Bind(_ *http.Request) error {
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.Name = strings.TrimSpace(req.Name)
	return validation.ValidateStruct(req,
		validation.Field(&req.SessionID, validation.Required),
		validation.Field(&req.Response, validation.Required),
	)
}
