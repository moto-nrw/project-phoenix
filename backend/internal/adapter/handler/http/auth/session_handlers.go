package auth

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	authService "github.com/moto-nrw/project-phoenix/internal/core/service/auth"
)

// login handles user login
func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	accessToken, refreshToken, err := rs.AuthService.LoginWithAudit(r.Context(), req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidCredentials):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrInvalidCredentials)) // Mask the specific error
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrAccountInactive))
			default:
				common.RenderError(w, r, ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Special case for login endpoint - frontend expects direct token response
	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// refreshToken handles token refresh
func (rs *Resource) refreshToken(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from context
	refreshToken := jwt.RefreshTokenFromCtx(r.Context())

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	accessToken, newRefreshToken, err := rs.AuthService.RefreshTokenWithAudit(r.Context(), refreshToken, ipAddress, userAgent)
	if err != nil {
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			switch {
			case errors.Is(authErr.Err, authService.ErrInvalidToken):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrInvalidToken))
			case errors.Is(authErr.Err, authService.ErrTokenExpired):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrTokenExpired))
			case errors.Is(authErr.Err, authService.ErrTokenNotFound):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrTokenNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountNotFound):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrAccountNotFound))
			case errors.Is(authErr.Err, authService.ErrAccountInactive):
				common.RenderError(w, r, ErrorUnauthorized(authService.ErrAccountInactive))
			default:
				common.RenderError(w, r, ErrorInternalServer(err))
			}
			return
		}
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Special case for token refresh endpoint - frontend expects direct token response
	render.JSON(w, r, TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	})
}

// logout handles user logout
func (rs *Resource) logout(w http.ResponseWriter, r *http.Request) {
	// Get refresh token from context
	refreshToken := jwt.RefreshTokenFromCtx(r.Context())

	// Get IP address and user agent for audit logging
	ipAddress := getClientIP(r)
	userAgent := r.Header.Get(headerUserAgent)

	err := rs.AuthService.LogoutWithAudit(r.Context(), refreshToken, ipAddress, userAgent)
	if err != nil {
		// Even if there's an error, we want to consider the logout successful from the client's perspective
		// Log the error on the server side for debugging
		logger.Logger.WithFields(map[string]interface{}{
			"ip":    ipAddress,
			"error": err.Error(),
		}).Warn("Logout audit logging failed (client logout still successful)")
	}

	common.RespondNoContent(w, r)
}

// getClientIP extracts the real client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Real-IP header first (set by reverse proxy)
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Split the header value on commas and trim each entry
		ips := strings.Split(xff, ",")
		for i, ip := range ips {
			ips[i] = strings.TrimSpace(ip)
		}
		// Return the first IP in the list
		if len(ips) > 0 {
			return ips[0]
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}
