package jwt

import (
	"context"
	"github.com/moto-nrw/project-phoenix/logging"
	"net/http"

	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
)

type CtxKey int

const (
	CtxClaims CtxKey = iota
	CtxRefreshToken
	CtxPermissions // Context key for permissions
)

// For backward compatibility
type ctxKey = CtxKey

const (
	ctxClaims       = CtxClaims
	ctxRefreshToken = CtxRefreshToken
	ctxPermissions  = CtxPermissions
)

// ClaimsFromCtx retrieves the parsed AppClaims from request context.
func ClaimsFromCtx(ctx context.Context) AppClaims {
	return ctx.Value(ctxClaims).(AppClaims)
}

// PermissionsFromCtx retrieves the permissions array from request context.
func PermissionsFromCtx(ctx context.Context) []string {
	perms, ok := ctx.Value(ctxPermissions).([]string)
	if !ok {
		return []string{}
	}
	return perms
}

// RefreshTokenFromCtx retrieves the parsed refresh token from context.
func RefreshTokenFromCtx(ctx context.Context) string {
	return ctx.Value(ctxRefreshToken).(string)
}

// Authenticator is a default authentication middleware to enforce access from the
// Verifier middleware request context values. The Authenticator sends a 401 Unauthorized
// response for any unverified tokens and passes the good ones through.
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())

		if err != nil {
			logging.GetLogEntry(r).Warn("JWT error:", err)
			render.Render(w, r, ErrUnauthorized(ErrTokenUnauthorized))
			return
		}

		if token == nil {
			logging.GetLogEntry(r).Warn("No token found in context")
			render.Render(w, r, ErrUnauthorized(ErrTokenUnauthorized))
			return
		}

		if err := jwt.Validate(token); err != nil {
			logging.GetLogEntry(r).Warn("Token validation failed:", err)
			render.Render(w, r, ErrUnauthorized(ErrTokenExpired))
			return
		}

		// Token is authenticated, parse claims
		var c AppClaims
		err = c.ParseClaims(claims)
		if err != nil {
			logging.GetLogEntry(r).Error("Failed to parse claims:", err)
			render.Render(w, r, ErrUnauthorized(ErrInvalidAccessToken))
			return
		}

		// Set AppClaims on context
		ctx := context.WithValue(r.Context(), ctxClaims, c)

		// Also set permissions on context for easier access
		ctx = context.WithValue(ctx, ctxPermissions, c.Permissions)

		// Call the next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthenticateRefreshJWT checks validity of refresh tokens and is only used for access token refresh and logout requests. It responds with 401 Unauthorized for invalid or expired refresh tokens.
func AuthenticateRefreshJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())
		if err != nil {
			logging.GetLogEntry(r).Warn(err)
			render.Render(w, r, ErrUnauthorized(ErrTokenUnauthorized))
			return
		}

		if err := jwt.Validate(token); err != nil {
			render.Render(w, r, ErrUnauthorized(ErrTokenExpired))
			return
		}

		// Token is authenticated, parse refresh token string
		var c RefreshClaims
		err = c.ParseClaims(claims)
		if err != nil {
			logging.GetLogEntry(r).Error(err)
			render.Render(w, r, ErrUnauthorized(ErrInvalidRefreshToken))
			return
		}
		// Set refresh token string on context
		ctx := context.WithValue(r.Context(), ctxRefreshToken, c.Token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
