package jwt

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
)

type CtxKey int

const (
	CtxClaims CtxKey = iota
	CtxRefreshToken
	CtxPermissions      // Context key for permissions
	CtxEnrollmentClaims // Context key for MFAEnrollmentClaims (enrollment-only token)
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

// Authenticator is a default authentication middleware to enforce access from the
// Verifier middleware request context values. The Authenticator sends a 401 Unauthorized
// response for any unverified tokens and passes the good ones through.
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())

		if err != nil {
			slog.Warn("JWT error", slog.String("error", err.Error()))
			if err := render.Render(w, r, ErrUnauthorized(ErrTokenUnauthorized)); err != nil {
				slog.Error("failed to render unauthorized response", slog.String("error", err.Error()))
			}
			return
		}

		if token == nil {
			slog.Warn("no token found in context")
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		if err := jwt.Validate(token); err != nil {
			slog.Warn("token validation failed", slog.String("error", err.Error()))
			renderUnauthorized(w, r, ErrTokenExpired)
			return
		}

		// Token is authenticated, parse claims
		var c AppClaims
		if err := c.ParseClaims(claims); err != nil {
			slog.Error("failed to parse claims", slog.String("error", err.Error()))
			renderUnauthorized(w, r, ErrInvalidAccessToken)
			return
		}

		// Set AppClaims and permissions on context
		ctx := context.WithValue(r.Context(), CtxClaims, c)
		ctx = context.WithValue(ctx, CtxPermissions, c.Permissions)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MFAEnrollmentAuthenticator authenticates requests that present an
// MFA-enrollment-scoped JWT. It is the inverse of the regular Authenticator:
// while Authenticator REJECTS tokens that carry `mfa_enrollment_pending=true`,
// this middleware REQUIRES the flag and rejects anything else. Used to
// protect the narrow /auth/mfa/enroll/* surface that a freshly-logged-in
// account must traverse before getting a full session.
//
// On success the parsed MFAEnrollmentClaims are placed on the request
// context under CtxEnrollmentClaims; handlers retrieve them with
// EnrollmentClaimsFromCtx.
func MFAEnrollmentAuthenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())
		if err != nil {
			slog.Warn("MFA enrollment JWT error", slog.String("error", err.Error()))
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}
		if token == nil {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}
		if err := jwt.Validate(token); err != nil {
			slog.Warn("enrollment token validation failed", slog.String("error", err.Error()))
			renderUnauthorized(w, r, ErrTokenExpired)
			return
		}

		var ec MFAEnrollmentClaims
		if err := ec.ParseClaims(claims); err != nil {
			slog.Warn("enrollment token claims rejected", slog.String("error", err.Error()))
			renderUnauthorized(w, r, ErrInvalidAccessToken)
			return
		}

		ctx := context.WithValue(r.Context(), CtxEnrollmentClaims, ec)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// renderUnauthorized renders an unauthorized response with fallback to http.Error
func renderUnauthorized(w http.ResponseWriter, r *http.Request, err error) {
	slog.WarnContext(r.Context(), "unauthorized request",
		slog.String("error", err.Error()),
		slog.String("path", r.URL.Path),
	)
	if render.Render(w, r, ErrUnauthorized(err)) != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

// AuthenticateRefreshJWT checks validity of refresh tokens and is only used for access token refresh and logout requests. It responds with 401 Unauthorized for invalid or expired refresh tokens.
func AuthenticateRefreshJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())
		if err != nil {
			slog.Warn("refresh token error", slog.String("error", err.Error()))
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		if token == nil {
			slog.Warn("no token found in context")
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		if jwt.Validate(token) != nil {
			renderUnauthorized(w, r, ErrTokenExpired)
			return
		}

		// Parse and validate claims to ensure token integrity
		var c RefreshClaims
		if err := c.ParseClaims(claims); err != nil {
			slog.Error("failed to parse refresh token claims", slog.String("error", err.Error()))
			renderUnauthorized(w, r, ErrInvalidAccessToken)
			return
		}

		// Extract token string from Authorization header for database lookup
		tokenString := extractBearerToken(r.Header.Get("Authorization"))

		ctx := context.WithValue(r.Context(), CtxRefreshToken, tokenString)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractBearerToken extracts the token from a Bearer authorization header
func extractBearerToken(authHeader string) string {
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return ""
}
