package jwt

import (
	"context"
	"net/http"
	"strconv"
	"strings"

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
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		if token == nil {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		if err := jwt.Validate(token); err != nil {
			renderUnauthorized(w, r, ErrTokenExpired)
			return
		}

		// Token is authenticated, parse claims
		var c AppClaims
		if err := c.ParseClaims(claims); err != nil {
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
	recordJWTAuthError(r.Context(), err)
	if render.Render(w, r, ErrUnauthorized(err)) != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

// AuthenticateRefreshJWT checks validity of refresh tokens and is only used for access token refresh and logout requests. It responds with 401 Unauthorized for invalid or expired refresh tokens.
func AuthenticateRefreshJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())
		if err != nil {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		if token == nil {
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

func recordJWTAuthError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	event := adaptermiddleware.GetWideEvent(ctx)
	if event == nil || event.Timestamp.IsZero() || event.ErrorType != "" {
		return
	}
	event.ErrorType = "jwt_auth"
	if code := jwtErrorCode(err); code != "" {
		event.ErrorCode = code
	}
	event.ErrorMessage = err.Error()
}

func jwtErrorCode(err error) string {
	switch err {
	case ErrTokenUnauthorized:
		return "token_unauthorized"
	case ErrTokenExpired:
		return "token_expired"
	case ErrInvalidAccessToken:
		return "invalid_access_token"
	case ErrInvalidRefreshToken:
		return "invalid_refresh_token"
	default:
		return ""
	}
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
