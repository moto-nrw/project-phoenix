package operator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	jwtPkg "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// Mock OperatorAuthService
type mockOperatorAuthService struct {
	loginFn                          func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error)
	refreshTokenFn                   func(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error)
	getOperatorFn                    func(ctx context.Context, id int64) (*platform.Operator, error)
	updateProfileFn                  func(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error)
	changePasswordFn                 func(ctx context.Context, operatorID int64, currentPassword, newPassword string) error
	initiateEmailChangeFn            func(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error
	confirmEmailChangeFn             func(ctx context.Context, token string, clientIP net.IP) (string, error)
	cleanupExpiredEmailChangeTokenFn func(ctx context.Context) (int, error)
	loginWithMFAGateFn               func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platformSvc.OperatorLoginResult, error)
}

func (m *mockOperatorAuthService) Login(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, email, password, clientIP)
	}
	return "", "", nil, nil
}

// LoginWithMFAGate / SetMFAService — no-op stubs added so
// *mockOperatorAuthService still satisfies OperatorAuthService after the
// MFA additions in issue #1308 phase 7b-3. The legacy auth_test cases
// don't exercise these paths; they're called by the new login handler
// which has its own MFA-aware test in mfa_login_test.go.
func (m *mockOperatorAuthService) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platformSvc.OperatorLoginResult, error) {
	if m.loginWithMFAGateFn != nil {
		return m.loginWithMFAGateFn(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
	}
	return nil, nil
}
func (m *mockOperatorAuthService) SetMFAService(_ platformSvc.OperatorMFAService) {}

// IssueTokensForAuthenticatedOperator no-op stub. The legacy auth_test cases
// exercise the password-only Login flow; the new MFA verify flow has its own
// tests in mfa_internal_test.go that wire a dedicated stub.
func (m *mockOperatorAuthService) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error) {
	return "", "", nil
}

func (m *mockOperatorAuthService) RefreshToken(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
	if m.refreshTokenFn != nil {
		return m.refreshTokenFn(ctx, operatorID, refreshTokenValue)
	}
	return "", "", nil
}

func (m *mockOperatorAuthService) ValidateOperator(ctx context.Context, email, password string) (*platform.Operator, error) {
	return nil, nil
}

func (m *mockOperatorAuthService) GetOperator(ctx context.Context, id int64) (*platform.Operator, error) {
	if m.getOperatorFn != nil {
		return m.getOperatorFn(ctx, id)
	}
	return nil, nil
}

func (m *mockOperatorAuthService) UpdateProfile(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error) {
	if m.updateProfileFn != nil {
		return m.updateProfileFn(ctx, operatorID, displayName)
	}
	return nil, nil
}

func (m *mockOperatorAuthService) ChangePassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	if m.changePasswordFn != nil {
		return m.changePasswordFn(ctx, operatorID, currentPassword, newPassword)
	}
	return nil
}

func (m *mockOperatorAuthService) InitiateEmailChange(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
	if m.initiateEmailChangeFn != nil {
		return m.initiateEmailChangeFn(ctx, operatorID, newEmail, currentPassword, clientIP)
	}
	return nil
}

func (m *mockOperatorAuthService) ConfirmEmailChange(ctx context.Context, token string, clientIP net.IP) (string, error) {
	if m.confirmEmailChangeFn != nil {
		return m.confirmEmailChangeFn(ctx, token, clientIP)
	}
	return "", nil
}

func (m *mockOperatorAuthService) CleanupExpiredEmailChangeTokens(ctx context.Context) (int, error) {
	if m.cleanupExpiredEmailChangeTokenFn != nil {
		return m.cleanupExpiredEmailChangeTokenFn(ctx)
	}
	return 0, nil
}

func TestLogin_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platformSvc.OperatorLoginResult, error) {
			assert.Equal(t, "test@example.com", email)
			assert.Equal(t, "password123", password)
			op := &platform.Operator{
				Email:       "test@example.com",
				DisplayName: "Test Operator",
			}
			op.ID = 1
			return &platformSvc.OperatorLoginResult{
				Status:       platformSvc.OperatorLoginStatusAuthenticated,
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				Operator:     op,
			}, nil
		},
	}

	resource := operator.NewAuthResource(mockService)

	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "success", response["status"])
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "authenticated", data["status"])
	assert.Equal(t, "access-token", data["access_token"])
	assert.Equal(t, "refresh-token", data["refresh_token"])

	operatorData := data["operator"].(map[string]interface{})
	assert.Equal(t, float64(1), operatorData["id"])
	assert.Equal(t, "test@example.com", operatorData["email"])
	assert.Equal(t, "Test Operator", operatorData["display_name"])
}

func TestLogin_EmptyEmail(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewAuthResource(mockService)

	body := map[string]string{
		"email":    "",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid email or password")
}

func TestLogin_EmptyPassword(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewAuthResource(mockService)

	body := map[string]string{
		"email":    "test@example.com",
		"password": "",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid email or password")
}

func TestLogin_InvalidCredentials(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platformSvc.OperatorLoginResult, error) {
			return nil, &platformSvc.InvalidCredentialsError{}
		},
	}

	resource := operator.NewAuthResource(mockService)

	body := map[string]string{
		"email":    "test@example.com",
		"password": "wrongpassword",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid email or password")
}

func TestLogin_OperatorInactive(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platformSvc.OperatorLoginResult, error) {
			return nil, &platformSvc.OperatorInactiveError{}
		},
	}

	resource := operator.NewAuthResource(mockService)

	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Operator account is inactive")
}

func TestLogin_OperatorNotFound(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platformSvc.OperatorLoginResult, error) {
			return nil, &platformSvc.OperatorNotFoundError{}
		},
	}

	resource := operator.NewAuthResource(mockService)

	body := map[string]string{
		"email":    "notfound@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid email or password")
}

func TestLogin_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platformSvc.OperatorLoginResult, error) {
			return nil, errors.New("database connection error")
		},
	}

	resource := operator.NewAuthResource(mockService)

	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Authentication failed")
}

func TestLogin_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewAuthResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestLogin_ClientIPExtraction_XForwardedFor(t *testing.T) {
	t.Parallel()

	var capturedIP string
	mockService := &mockOperatorAuthService{
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platformSvc.OperatorLoginResult, error) {
			capturedIP = ipAddress
			op := &platform.Operator{Email: email, DisplayName: "Test"}
			op.ID = 1
			return &platformSvc.OperatorLoginResult{
				Status:       platformSvc.OperatorLoginStatusAuthenticated,
				AccessToken:  "access",
				RefreshToken: "refresh",
				Operator:     op,
			}, nil
		},
	}

	resource := operator.NewAuthResource(mockService)

	body := map[string]string{"email": "test@example.com", "password": "password123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	rr := httptest.NewRecorder()

	serveOperatorLoginThroughXFFMiddleware(resource, rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "192.168.1.100", capturedIP)
}

func TestLogin_ClientIPExtraction_IgnoresRawXRealIP(t *testing.T) {
	t.Parallel()

	var capturedIP string
	mockService := &mockOperatorAuthService{
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platformSvc.OperatorLoginResult, error) {
			capturedIP = ipAddress
			op := &platform.Operator{Email: email, DisplayName: "Test"}
			op.ID = 1
			return &platformSvc.OperatorLoginResult{
				Status:       platformSvc.OperatorLoginStatusAuthenticated,
				AccessToken:  "access",
				RefreshToken: "refresh",
				Operator:     op,
			}, nil
		},
	}

	resource := operator.NewAuthResource(mockService)

	body := map[string]string{"email": "test@example.com", "password": "password123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", "10.0.0.50")
	req.RemoteAddr = "192.0.2.55:1234"
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "192.0.2.55", capturedIP)
}

func serveOperatorLoginThroughXFFMiddleware(resource *operator.AuthResource, rr *httptest.ResponseRecorder, req *http.Request) {
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFF())
	router.Post("/auth/login", resource.Login)
	router.ServeHTTP(rr, req)
}

func TestLoginRequest_Bind(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	loginReq := &operator.LoginRequest{}

	err := loginReq.Bind(req)
	assert.NoError(t, err)
}

func TestRefreshToken_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		refreshTokenFn: func(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "opaque-refresh-handle", refreshTokenValue)
			assert.True(t, rotation.MatchesRecoveryProof(ctx, rotation.RecoveryProofHash(rotation.WithRecoveryProof(context.Background(), "independent-recovery-secret"))))
			return "new-access-token", "new-refresh-token", nil
		},
	}

	resource := operator.NewAuthResource(mockService)

	// Create a real token with claims
	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":    float64(42),
		"token": "opaque-refresh-handle",
		"scope": "platform",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set(rotation.RecoveryProofHeader, "independent-recovery-secret")

	// Set CtxRefreshToken in context
	ctx := context.WithValue(req.Context(), jwtPkg.CtxRefreshToken, tokenString)

	// Set jwtauth context with parsed token
	token, _ := jwtauth.VerifyToken(tokenAuth, tokenString)
	ctx = jwtauth.NewContext(ctx, token, nil)

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.RefreshToken(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "new-access-token", data["access_token"])
	assert.Equal(t, "new-refresh-token", data["refresh_token"])
}

func TestRefreshToken_MissingTokenContext(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewAuthResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	rr := httptest.NewRecorder()

	resource.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Unauthorized")
}

func TestRefreshToken_InvalidClaims(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewAuthResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := context.WithValue(req.Context(), jwtPkg.CtxRefreshToken, "some-token-string")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Unauthorized")
}

func TestRefreshToken_RejectsNonPlatformScope(t *testing.T) {
	t.Parallel()

	called := false
	mockService := &mockOperatorAuthService{
		refreshTokenFn: func(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
			called = true
			return "", "", nil
		},
	}
	resource := operator.NewAuthResource(mockService)

	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":        float64(42),
		"token":     "tenant-refresh-handle",
		"tenant_id": float64(1),
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := context.WithValue(req.Context(), jwtPkg.CtxRefreshToken, tokenString)
	token, _ := jwtauth.VerifyToken(tokenAuth, tokenString)
	ctx = jwtauth.NewContext(ctx, token, nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, called, "non-platform refresh claims must be rejected before service rotation")
}

func TestRefreshToken_RejectsLegacyDeterministicOperatorToken(t *testing.T) {
	t.Parallel()

	called := false
	mockService := &mockOperatorAuthService{
		refreshTokenFn: func(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
			called = true
			return "", "", nil
		},
	}
	resource := operator.NewAuthResource(mockService)

	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":    float64(42),
		"token": "operator-refresh-42",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := context.WithValue(req.Context(), jwtPkg.CtxRefreshToken, tokenString)
	token, _ := jwtauth.VerifyToken(tokenAuth, tokenString)
	ctx = jwtauth.NewContext(ctx, token, nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, called, "legacy deterministic operator refresh claims must not reach the service")
}

func TestRefreshToken_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		refreshTokenFn: func(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
			return "", "", &platformSvc.OperatorInactiveError{}
		},
	}

	resource := operator.NewAuthResource(mockService)

	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":    float64(42),
		"token": "opaque-refresh-handle",
		"scope": "platform",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := context.WithValue(req.Context(), jwtPkg.CtxRefreshToken, tokenString)

	token, _ := jwtauth.VerifyToken(tokenAuth, tokenString)
	ctx = jwtauth.NewContext(ctx, token, nil)

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.RefreshToken(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "Operator account is inactive")
}

func TestRefreshToken_InvalidRefreshSessionMapsToUnauthorized(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		refreshTokenFn: func(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
			return "", "", &platformSvc.OperatorRefreshTokenInvalidError{}
		},
	}
	resource := operator.NewAuthResource(mockService)

	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":    float64(42),
		"token": "stale-refresh-handle",
		"scope": "platform",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := context.WithValue(req.Context(), jwtPkg.CtxRefreshToken, tokenString)
	token, _ := jwtauth.VerifyToken(tokenAuth, tokenString)
	ctx = jwtauth.NewContext(ctx, token, nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Unauthorized")
	assert.NotContains(t, rr.Body.String(), "Invalid email or password")
}
