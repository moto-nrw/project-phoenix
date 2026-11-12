package portalauth

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

// TokenResponse is the session pair every portal answers with.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// LoginResponse is a password sign-in's answer, discriminated by status:
// authenticated carries the pair; mfa_required the challenge and the
// trusted-device label the frontend shows; mfa_enrollment_required a
// short-lived enrollment token, which only the enroll routes admit.
// mfa_enrollment_required as a field stays for clients that read the flag
// instead of the status.
type LoginResponse struct {
	Status                string `json:"status"`
	AccessToken           string `json:"access_token,omitempty"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	ChallengeToken        string `json:"challenge_token,omitempty"`
	MaskedEmail           string `json:"masked_email,omitempty"`
	MFAEnrollmentRequired bool   `json:"mfa_enrollment_required,omitempty"`
	TrustedDeviceEnabled  *bool  `json:"trusted_device_enabled,omitempty"`
	TrustedDeviceDays     *int   `json:"trusted_device_days,omitempty"`
}

// LoginBody builds the answer of a login result.
func LoginBody(result identityaccess.LoginResult) LoginResponse {
	switch result.Status {
	case identityaccess.LoginStatusMFARequired:
		enabled, days := result.TrustedDeviceEnabled, result.TrustedDeviceDays
		return LoginResponse{
			Status: string(result.Status), ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
			TrustedDeviceEnabled: &enabled, TrustedDeviceDays: &days,
		}
	case identityaccess.LoginStatusMFAEnrollmentRequired:
		return LoginResponse{
			Status: string(result.Status), AccessToken: result.AccessToken, MaskedEmail: result.MaskedEmail,
			MFAEnrollmentRequired: true,
		}
	default:
		return LoginResponse{
			Status: string(identityaccess.LoginStatusAuthenticated), AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		}
	}
}

// RenderLogin answers a login result as plain JSON.
func RenderLogin(w http.ResponseWriter, r *http.Request, result *identityaccess.LoginResult) {
	render.JSON(w, r, LoginBody(*result))
}

// EnrollStartResponse names the challenge the enrollment code belongs to;
// the confirm step redeems exactly that challenge.
type EnrollStartResponse struct {
	ChallengeToken string `json:"challenge_token"`
}
