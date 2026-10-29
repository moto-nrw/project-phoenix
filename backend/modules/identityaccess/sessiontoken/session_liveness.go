package sessiontoken

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/jwtauth/v5"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
)

var (
	// ErrSessionRevoked refuses a signed, unexpired access token whose
	// refresh-token family is gone: logged out, revoked, expired or never
	// recorded (#5194).
	ErrSessionRevoked = errors.New("session has been revoked")
	// ErrSessionCheckUnavailable refuses a token whose family could not be
	// read. It fails closed and answers 503, not 401, so a database hiccup
	// does not end sessions.
	ErrSessionCheckUnavailable = errors.New("session check temporarily unavailable, please retry")
)

// SessionLiveness reports whether the refresh-token family an access token
// names is still live. false with a nil error means revoked; an error means
// the check could not run. The root binds it to Identity & Access.
type SessionLiveness func(ctx context.Context, claims AppClaims) (bool, error)

// RequireLiveSession checks every verified account access token against its
// refresh-token family (#5194). It runs once per request, directly after the
// Verifier: a token whose family is gone is demoted to a verification error,
// so every Authenticator downstream refuses it and public routes ignore it.
// Without this a cookie that a late response writes back after logout would
// open the school again until the access token expires. Operator tokens are
// skipped: the operator routes check their own family per request
// (RequiresActiveOperator). Refresh, MFA and enrollment tokens are no access
// tokens and pass unchanged to their own middleware.
func RequireLiveSession(live SessionLiveness) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := verifiedAccessClaims(r.Context())
			if !ok || claims.IsPlatformScope() {
				next.ServeHTTP(w, r)
				return
			}
			refusal := sessionRefusal(r.Context(), live, claims)
			if refusal == nil {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r.WithContext(jwtauth.NewContext(r.Context(), nil, refusal)))
		})
	}
}

// sessionRefusal is nil for a live family. A token without a family is
// refused like a revoked one: every session minted since families exist
// carries one.
func sessionRefusal(ctx context.Context, live SessionLiveness, claims AppClaims) error {
	if claims.FamilyID == "" {
		return ErrSessionRevoked
	}
	if live == nil {
		return ErrSessionCheckUnavailable
	}
	alive, err := live(ctx, claims)
	if err != nil {
		slog.ErrorContext(ctx, "session family check failed",
			slog.Int("account_id", claims.ID),
			slog.String("error", err.Error()),
		)
		return ErrSessionCheckUnavailable
	}
	if !alive {
		return ErrSessionRevoked
	}
	return nil
}

// verifiedAccessClaims returns the claims of a signed, unexpired access
// token in the request context; false for every other request.
func verifiedAccessClaims(ctx context.Context) (AppClaims, bool) {
	token, raw, err := jwtauth.FromContext(ctx)
	if err != nil || token == nil || tokenprimitives.Validate(token) != nil {
		return AppClaims{}, false
	}
	var claims AppClaims
	if claims.ParseClaims(raw) != nil {
		return AppClaims{}, false
	}
	return claims, true
}
