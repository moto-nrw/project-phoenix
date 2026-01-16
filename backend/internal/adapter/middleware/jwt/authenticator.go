package jwt

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	adaptermiddleware "github.com/moto-nrw/project-phoenix/internal/adapter/middleware"
	"github.com/moto-nrw/project-phoenix/internal/core/port"

	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
)

type CtxKey = port.AuthContextKey

const (
	CtxClaims       = port.CtxClaims
	CtxRefreshToken = port.CtxRefreshToken
	CtxPermissions  = port.CtxPermissions
)

func ClaimsFromCtx(ctx context.Context) AppClaims {
	return port.ClaimsFromCtx(ctx)
}

func PermissionsFromCtx(ctx context.Context) []string {
	return port.PermissionsFromCtx(ctx)
}

func RefreshTokenFromCtx(ctx context.Context) string {
	return port.RefreshTokenFromCtx(ctx)
}

// Authenticator is a default authentication middleware to enforce access from the
// Verifier middleware request context values. The Authenticator sends a 401 Unauthorized
// response for any unverified tokens and passes the good ones through.
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())

		if err != nil {
			logger.Logger.Warn("JWT error:", err)
			_ = render.Render(w, r, ErrUnauthorized(ErrTokenUnauthorized))
			return
		}

		if token == nil {
			logger.Logger.Warn("No token found in context")
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		if err := jwt.Validate(token); err != nil {
			logger.Logger.Warn("Token validation failed:", err)
			renderUnauthorized(w, r, ErrTokenExpired)
			return
		}

		// Token is authenticated, parse claims
		var c AppClaims
		if err := c.ParseClaims(claims); err != nil {
			logger.Logger.Error("Failed to parse claims:", err)
			renderUnauthorized(w, r, ErrInvalidAccessToken)
			return
		}

		// Set AppClaims and permissions on context
		ctx := context.WithValue(r.Context(), CtxClaims, c)
		ctx = context.WithValue(ctx, CtxPermissions, c.Permissions)

		event := adaptermiddleware.GetWideEvent(ctx)
		if event != nil {
			accountID := strconv.FormatInt(int64(c.ID), 10)
			event.AccountID = accountID
			if c.Sub != "" {
				event.UserID = c.Sub
			} else {
				event.UserID = accountID
			}
			event.UserRole = deriveUserRole(c)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// renderUnauthorized renders an unauthorized response with fallback to http.Error
func renderUnauthorized(w http.ResponseWriter, r *http.Request, err error) {
	if render.Render(w, r, ErrUnauthorized(err)) != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

// AuthenticateRefreshJWT checks validity of refresh tokens and is only used for access token refresh and logout requests. It responds with 401 Unauthorized for invalid or expired refresh tokens.
func AuthenticateRefreshJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())
		if err != nil {
			logger.Logger.Warn(err)
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		if token == nil {
			logger.Logger.Warn("No token found in context")
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
			logger.Logger.Error("Failed to parse refresh token claims:", err)
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

func deriveUserRole(claims AppClaims) string {
	if claims.IsAdmin {
		return "admin"
	}
	if claims.IsTeacher {
		return "teacher"
	}
	for _, role := range claims.Roles {
		if strings.TrimSpace(role) != "" {
			return role
		}
	}
	return ""
}
