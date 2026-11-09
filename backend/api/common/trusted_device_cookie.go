package common

import (
	"net/http"
	"strings"
	"time"
)

// TrustedDeviceCookie carries the signed remember-device token. Every portal
// uses the same name: the device row is keyed by the account (and the school
// where it has one), and each portal host keeps its own cookie jar.
const TrustedDeviceCookie = "mfa_trust_device"

// TrustedDevice returns the remember-device token the browser sent, or "".
func TrustedDevice(r *http.Request) string {
	cookie, err := r.Cookie(TrustedDeviceCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// SetTrustedDevice writes the remember-device cookie; an empty value (the
// school switched remembered devices off) writes none. Path "/" sends it with
// every sign-in request; Lax keeps it on the sign-in's redirects while
// blocking cross-site posts. Browsers treat localhost as a secure context,
// so Secure holds in development too.
func SetTrustedDevice(w http.ResponseWriter, value string, expiresAt time.Time) {
	if value == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     TrustedDeviceCookie,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// FrontendOriginHeader names the browser origin the BFF saw. A passkey
// ceremony binds to it, because the API host never sees the browser's own
// Origin.
const FrontendOriginHeader = "X-Moto-Frontend-Origin"

// PasskeyOrigin is the origin a passkey ceremony must match: the BFF's
// header, else the request's Origin.
func PasskeyOrigin(r *http.Request) string {
	if origin := strings.TrimSpace(r.Header.Get(FrontendOriginHeader)); origin != "" {
		return origin
	}
	return strings.TrimSpace(r.Header.Get("Origin"))
}
