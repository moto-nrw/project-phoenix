package sessiontoken

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
)

type CtxKey int

const (
	CtxClaims CtxKey = iota
	CtxRefreshToken
	CtxPermissions
	CtxEnrollmentClaims
)

// ClaimsFromCtx retrieves the parsed AppClaims from request context.
// Returns zero-value AppClaims if not present or wrong type.
func ClaimsFromCtx(ctx context.Context) AppClaims {
	claims, ok := ctx.Value(CtxClaims).(AppClaims)
	if !ok {
		return AppClaims{}
	}
	return claims
}

// ActorAccountIDFromCtx returns the acting account ID for optional audit
// fields. Missing claims and the zero ID map to nil so callers store NULL
// instead of fabricating an actor.
func ActorAccountIDFromCtx(ctx context.Context) *int64 {
	claims := ClaimsFromCtx(ctx)
	if claims.ID == 0 {
		return nil
	}
	id := int64(claims.ID)
	return &id
}

// PermissionsFromCtx retrieves the permissions array from request context.
func PermissionsFromCtx(ctx context.Context) []string {
	perms, ok := ctx.Value(CtxPermissions).([]string)
	if !ok {
		return []string{}
	}
	return perms
}

// RefreshTokenFromCtx retrieves the parsed refresh token from context.
// Returns empty string if not present or wrong type.
func RefreshTokenFromCtx(ctx context.Context) string {
	token, ok := ctx.Value(CtxRefreshToken).(string)
	if !ok {
		return ""
	}
	return token
}

// EnrollmentClaimsFromCtx retrieves the parsed MFAEnrollmentClaims from
// request context. Returns the zero value plus false when the request
// did not flow through MFAEnrollmentAuthenticator. Handlers for the
// /auth/mfa/enroll/* endpoints use this instead of ClaimsFromCtx because
// the enrollment token deliberately omits id/sub/roles.
func EnrollmentClaimsFromCtx(ctx context.Context) (MFAEnrollmentClaims, bool) {
	claims, ok := ctx.Value(CtxEnrollmentClaims).(MFAEnrollmentClaims)
	if !ok {
		return MFAEnrollmentClaims{}, false
	}
	return claims, true
}

// Authenticator admits a request whose verified access token parses as a
// session token and puts its claims and permissions on the context. Every
// other request gets 401.
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoded, err := decodedFromCtx(r.Context())
		if err != nil {
			renderVerificationError(w, r, err)
			return
		}
		if decoded.Expired {
			renderUnauthorized(w, r, ErrTokenExpired)
			return
		}
		if decoded.AccessErr != nil {
			slog.Error("failed to parse claims", slog.String("error", decoded.AccessErr.Error()))
			renderUnauthorized(w, r, ErrInvalidAccessToken)
			return
		}
		ctx := context.WithValue(r.Context(), CtxClaims, decoded.access)
		ctx = context.WithValue(ctx, CtxPermissions, decoded.access.Permissions)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MFAEnrollmentAuthenticator is the inverse of Authenticator for the
// /auth/mfa/enroll/* surface: it requires an enrollment token and refuses
// every other token. Handlers read the claims with EnrollmentClaimsFromCtx.
func MFAEnrollmentAuthenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoded, err := decodedFromCtx(r.Context())
		if err != nil {
			slog.Warn("MFA enrollment JWT error", slog.String("error", err.Error()))
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}
		if decoded.Expired {
			renderUnauthorized(w, r, ErrTokenExpired)
			return
		}
		if decoded.EnrollmentErr != nil {
			slog.Warn("enrollment token claims rejected", slog.String("error", decoded.EnrollmentErr.Error()))
			renderUnauthorized(w, r, ErrInvalidAccessToken)
			return
		}
		ctx := context.WithValue(r.Context(), CtxEnrollmentClaims, enrollmentClaimsOf(decoded.Enrollment))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// renderUnauthorized renders an unauthorized response.
func renderUnauthorized(w http.ResponseWriter, r *http.Request, err error) {
	slog.WarnContext(r.Context(), "unauthorized request",
		slog.String("error", err.Error()),
		slog.String("path", r.URL.Path),
	)
	if renderErr := render.Render(w, r, ErrUnauthorized(err)); renderErr != nil {
		slog.ErrorContext(r.Context(), "failed to render unauthorized response", slog.String("error", renderErr.Error()))
	}
}

// AuthenticateRefreshJWT admits a request whose verified token parses as a
// refresh token and puts the raw token on the context for the session
// lookup. It guards the refresh and logout routes.
func AuthenticateRefreshJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoded, err := decodedFromCtx(r.Context())
		if err != nil {
			slog.Warn("refresh token error", slog.String("error", err.Error()))
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}
		if decoded.Expired {
			renderUnauthorized(w, r, ErrTokenExpired)
			return
		}
		if decoded.RefreshErr != nil {
			slog.Error("failed to parse refresh token claims", slog.String("error", decoded.RefreshErr.Error()))
			renderUnauthorized(w, r, ErrInvalidAccessToken)
			return
		}
		tokenString := extractBearerToken(r.Header.Get("Authorization"))
		ctx := context.WithValue(r.Context(), CtxRefreshToken, tokenString)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractBearerToken(authHeader string) string {
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return ""
}

// renderVerificationError refuses a token the Verifier or RequireLiveSession
// rejected: 503 when the session family could not be read, 401 otherwise.
func renderVerificationError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Warn("JWT error", slog.String("error", err.Error()))
	response := ErrUnauthorized(ErrTokenUnauthorized)
	switch {
	case errors.Is(err, ErrSessionCheckUnavailable):
		response = ErrServiceUnavailable(ErrSessionCheckUnavailable)
	case errors.Is(err, ErrSessionRevoked):
		response = ErrUnauthorized(ErrSessionRevoked)
	}
	if err := render.Render(w, r, response); err != nil {
		slog.Error("failed to render unauthorized response", slog.String("error", err.Error()))
	}
}
