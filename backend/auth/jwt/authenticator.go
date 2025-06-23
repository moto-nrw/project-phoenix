package jwt

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/logging"

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

// ClaimsFromCtx retrieves the parsed AppClaims from request context.
func ClaimsFromCtx(ctx context.Context) AppClaims {
	return ctx.Value(CtxClaims).(AppClaims)
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
func RefreshTokenFromCtx(ctx context.Context) string {
	return ctx.Value(CtxRefreshToken).(string)
}

// Authenticator is a default authentication middleware to enforce access from the
// Verifier middleware request context values. The Authenticator sends a 401 Unauthorized
// response for any unverified tokens and passes the good ones through.
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())

		if err != nil {
			logging.GetLogEntry(r).Warn("JWT error:", err)
			_ = render.Render(w, r, ErrUnauthorized(ErrTokenUnauthorized))
			return
		}

		if token == nil {
			logging.GetLogEntry(r).Warn("No token found in context")
			if renderErr := render.Render(w, r, ErrUnauthorized(ErrTokenUnauthorized)); renderErr != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		if err := jwt.Validate(token); err != nil {
			logging.GetLogEntry(r).Warn("Token validation failed:", err)
			if renderErr := render.Render(w, r, ErrUnauthorized(ErrTokenExpired)); renderErr != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		// Token is authenticated, parse claims
		var c AppClaims
		err = c.ParseClaims(claims)
		if err != nil {
			logging.GetLogEntry(r).Error("Failed to parse claims:", err)
			if renderErr := render.Render(w, r, ErrUnauthorized(ErrInvalidAccessToken)); renderErr != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		// Set AppClaims on context
		ctx := context.WithValue(r.Context(), CtxClaims, c)

		// Also set permissions on context for easier access
		ctx = context.WithValue(ctx, CtxPermissions, c.Permissions)

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
			if renderErr := render.Render(w, r, ErrUnauthorized(ErrTokenUnauthorized)); renderErr != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		if token == nil {
			logging.GetLogEntry(r).Warn("No token found in context")
			if renderErr := render.Render(w, r, ErrUnauthorized(ErrTokenUnauthorized)); renderErr != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		if err := jwt.Validate(token); err != nil {
			if renderErr := render.Render(w, r, ErrUnauthorized(ErrTokenExpired)); renderErr != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		// Parse and validate claims to ensure token integrity
		var c RefreshClaims
		err = c.ParseClaims(claims)
		if err != nil {
			logging.GetLogEntry(r).Error("Failed to parse refresh token claims:", err)
			if renderErr := render.Render(w, r, ErrUnauthorized(ErrInvalidAccessToken)); renderErr != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		// Get the raw token string from the Authorization header
		// This is needed for the auth service to look up the token in the database
		authHeader := r.Header.Get("Authorization")
		tokenString := ""
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		// Set the token string on context (refresh claims not needed in context)
		ctx := context.WithValue(r.Context(), CtxRefreshToken, tokenString)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
