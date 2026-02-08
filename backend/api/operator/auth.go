package operator

import (
	"net"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// AuthResource handles operator authentication endpoints
type AuthResource struct {
	authService platformSvc.OperatorAuthService
}

// NewAuthResource creates a new auth resource
func NewAuthResource(authService platformSvc.OperatorAuthService) *AuthResource {
	return &AuthResource{
		authService: authService,
	}
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Bind validates the login request
func (req *LoginRequest) Bind(r *http.Request) error {
	return nil
}

// LoginResponse represents the login response
type LoginResponse struct {
	AccessToken  string            `json:"access_token"`
	RefreshToken string            `json:"refresh_token"`
	Operator     *OperatorResponse `json:"operator"`
}

// OperatorResponse represents an operator in the response
type OperatorResponse struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// Login handles operator login
func (rs *AuthResource) Login(w http.ResponseWriter, r *http.Request) {
	req := &LoginRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	if req.Email == "" || req.Password == "" {
		common.RenderError(w, r, ErrInvalidCredentials())
		return
	}

	// Get client IP
	clientIP := getClientIP(r)

	accessToken, refreshToken, operator, err := rs.authService.Login(r.Context(), req.Email, req.Password, clientIP)
	if err != nil {
		common.RenderError(w, r, AuthErrorRenderer(err))
		return
	}

	response := &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Operator: &OperatorResponse{
			ID:          operator.ID,
			Email:       operator.Email,
			DisplayName: operator.DisplayName,
		},
	}

	common.Respond(w, r, http.StatusOK, response, "Login successful")
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) net.IP {
	// Check X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		return net.ParseIP(xff)
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return net.ParseIP(xri)
	}

	// Fall back to RemoteAddr
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return net.ParseIP(host)
}
