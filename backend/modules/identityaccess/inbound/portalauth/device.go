package portalauth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

// PrepareDevice freezes the trusted-device policy of one exchange before the
// mint, or answers nil when the device is not to be remembered.
type PrepareDevice func(context.Context) (identityaccess.TrustedDeviceIssue, error)

// Mint issues the session pair of a proven second factor.
type Mint func(context.Context) (accessToken, refreshToken string, err error)

// Exchange completes a second factor in the one order every portal keeps:
// prepare the device, mint the session, then store the device and set its
// cookie. Preparing first reads the policy and the MFA reset generation
// before the mint can evict a session; storing last keeps a refused mint
// from leaving a device behind. Remembering a device is optional, so its
// failures are logged and the session still answers.
func Exchange(w http.ResponseWriter, r *http.Request, accountID int64, prepare PrepareDevice, mint Mint) (TokenResponse, error) {
	var issue identityaccess.TrustedDeviceIssue
	if prepare != nil {
		prepared, err := prepare(r.Context())
		if err != nil {
			slog.Default().Warn("trusted-device policy unavailable; skipping cookie",
				slog.Int64("account_id", accountID),
				slog.String("error", err.Error()),
			)
		}
		issue = prepared
	}
	access, refresh, err := mint(r.Context())
	if err != nil {
		return TokenResponse{}, err
	}
	if issue != nil {
		if err := setIssuedDevice(w, r, issue); err != nil {
			slog.Default().Warn("failed to issue trusted-device cookie",
				slog.Int64("account_id", accountID),
				slog.String("error", err.Error()),
			)
		}
	}
	return TokenResponse{AccessToken: access, RefreshToken: refresh}, nil
}

// setIssuedDevice stores the device and sets its cookie.
func setIssuedDevice(w http.ResponseWriter, r *http.Request, issue identityaccess.TrustedDeviceIssue) error {
	value, expiresAt, err := issue(r.Context())
	if err != nil {
		return err
	}
	common.SetTrustedDevice(w, value, expiresAt)
	return nil
}

// RememberDevice prepares the device only when the client asked for it.
func RememberDevice(remember bool, prepare PrepareDevice) PrepareDevice {
	if !remember {
		return nil
	}
	return prepare
}

// Client is what a sign-in records about its caller.
type Client struct {
	IP        string
	UserAgent string
}

// ClientOf reads the router-selected address and the user agent.
func ClientOf(r *http.Request) Client {
	return Client{IP: common.GetClientIPString(r), UserAgent: r.Header.Get("User-Agent")}
}
