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

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
)

// Mock OperatorAuthService
type mockOperatorAuthService struct {
	loginFn                          func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *identityaccess.Operator, error)
	refreshTokenFn                   func(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error)
	getOperatorFn                    func(ctx context.Context, id int64) (*identityaccess.Operator, error)
	updateProfileFn                  func(ctx context.Context, operatorID int64, displayName string) (*identityaccess.Operator, error)
	changePasswordFn                 func(ctx context.Context, operatorID int64, currentPassword, newPassword string) error
	initiateEmailChangeFn            func(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error
	confirmEmailChangeFn             func(ctx context.Context, token string, clientIP net.IP) (string, error)
	cleanupExpiredEmailChangeTokenFn func(ctx context.Context) (int, error)
	loginWithMFAGateFn               func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error)
}

func (m *mockOperatorAuthService) Login(ctx context.Context, email, password string, clientIP net.IP) (string, string, *identityaccess.Operator, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, email, password, clientIP)
	}
	return "", "", nil, nil
}

// The mock serves both the retained OperatorAuthService (profile read,
// e-mail change, MFA and passkey exchange) and the Identity & Access
// operator capability the login, refresh and profile-change routes call
// (#3252).
func (m *mockOperatorAuthService) LoginOperatorWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
	if m.loginWithMFAGateFn != nil {
		return m.loginWithMFAGateFn(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
	}
	return nil, nil
}
func (m *mockOperatorAuthService) SetMFAService(_ identityoperator.OperatorMFA) {}

// IssueTokensForAuthenticatedOperator no-op stub. The legacy auth_test cases
// exercise the password-only Login flow; the new MFA verify flow has its own
// tests in mfa_internal_test.go that wire a dedicated stub.
func (m *mockOperatorAuthService) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error) {
	return "", "", nil
}

func (m *mockOperatorAuthService) RefreshOperatorToken(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
	if m.refreshTokenFn != nil {
		return m.refreshTokenFn(ctx, operatorID, refreshTokenValue)
	}
	return "", "", nil
}

func (m *mockOperatorAuthService) ValidateOperator(ctx context.Context, email, password string) (*identityaccess.Operator, error) {
	return nil, nil
}

func (m *mockOperatorAuthService) FindOperator(ctx context.Context, id int64) (identityaccess.Operator, error) {
	if m.getOperatorFn != nil {
		op, err := m.getOperatorFn(ctx, id)
		if op == nil {
			if err == nil {
				err = identityoperator.ErrOperatorNotFound
			}
			return identityaccess.Operator{}, err
		}
		return *identityOperatorOf(op), err
	}
	return identityaccess.Operator{}, identityoperator.ErrOperatorNotFound
}

func (m *mockOperatorAuthService) FindOperatorForUpdate(ctx context.Context, id int64) (identityaccess.Operator, error) {
	return m.FindOperator(ctx, id)
}

func (m *mockOperatorAuthService) FindOperatorByEmail(context.Context, string) (identityaccess.Operator, error) {
	return identityaccess.Operator{}, identityoperator.ErrOperatorNotFound
}

func (m *mockOperatorAuthService) ListOperators(context.Context) ([]identityaccess.Operator, error) {
	return nil, nil
}

func (m *mockOperatorAuthService) UpdateOperatorProfile(ctx context.Context, operatorID int64, displayName string) (identityaccess.Operator, error) {
	if m.updateProfileFn != nil {
		op, err := m.updateProfileFn(ctx, operatorID, displayName)
		if op == nil {
			return identityaccess.Operator{}, err
		}
		return *identityOperatorOf(op), err
	}
	return identityaccess.Operator{}, nil
}

func (m *mockOperatorAuthService) ChangeOperatorPassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	if m.changePasswordFn != nil {
		return m.changePasswordFn(ctx, operatorID, currentPassword, newPassword)
	}
	return nil
}

func (m *mockOperatorAuthService) InitiateOperatorEmailChange(ctx context.Context, request identityaccess.OperatorEmailChangeRequest) error {
	if m.initiateEmailChangeFn != nil {
		return m.initiateEmailChangeFn(ctx, request.OperatorID, request.NewEmail, request.CurrentPassword, net.ParseIP(request.IPAddress))
	}
	return nil
}

func (m *mockOperatorAuthService) ConfirmOperatorEmailChange(ctx context.Context, token, ipAddress string) (identityaccess.OperatorEmailChangeResult, error) {
	if m.confirmEmailChangeFn != nil {
		newEmail, err := m.confirmEmailChangeFn(ctx, token, net.ParseIP(ipAddress))
		return identityaccess.OperatorEmailChangeResult{NewEmail: newEmail}, err
	}
	return identityaccess.OperatorEmailChangeResult{}, nil
}

func (m *mockOperatorAuthService) CleanupOperatorEmailChanges(ctx context.Context) (int, error) {
	if m.cleanupExpiredEmailChangeTokenFn != nil {
		return m.cleanupExpiredEmailChangeTokenFn(ctx)
	}
	return 0, nil
}

// The invitation half of the provisioning capability is exercised by the
// invitations tests; the auth and profile cases never reach it.

func (m *mockOperatorAuthService) InviteOperator(context.Context, identityaccess.OperatorInvitationRequest) error {
	return nil
}

func (m *mockOperatorAuthService) ValidateOperatorInvitation(context.Context, string) (identityaccess.OperatorInvitationPreview, error) {
	return identityaccess.OperatorInvitationPreview{}, nil
}

func (m *mockOperatorAuthService) AcceptOperatorInvitation(context.Context, identityaccess.OperatorInvitationAcceptance) (identityaccess.Operator, error) {
	return identityaccess.Operator{}, nil
}

func (m *mockOperatorAuthService) ListPendingOperatorInvitations(context.Context) ([]identityaccess.OperatorInvitation, error) {
	return nil, nil
}

func (m *mockOperatorAuthService) RevokeOperatorInvitationByActor(context.Context, int64, int64, string) error {
	return nil
}

func (m *mockOperatorAuthService) ResendOperatorInvitationByActor(context.Context, int64, int64, string) error {
	return nil
}

func TestLogin_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
			assert.Equal(t, "test@example.com", email)
			assert.Equal(t, "password123", password)
			op := &identityaccess.Operator{
				Email:       "test@example.com",
				DisplayName: "Test Operator",
			}
			op.ID = 1
			return &identityaccess.OperatorLoginResult{
				Status:       identityaccess.LoginStatusAuthenticated,
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				Operator:     identityOperatorOf(op),
			}, nil
		},
	}

	resource := newIdentityResource(mockService)

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
	resource := newIdentityResource(mockService)

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
	resource := newIdentityResource(mockService)

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
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
			return nil, identityoperator.ErrOperatorInvalidCredentials
		},
	}

	resource := newIdentityResource(mockService)

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
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
			return nil, identityoperator.ErrOperatorInactive
		},
	}

	resource := newIdentityResource(mockService)

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
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
			return nil, identityoperator.ErrOperatorNotFound
		},
	}

	resource := newIdentityResource(mockService)

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
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
			return nil, errors.New("database connection error")
		},
	}

	resource := newIdentityResource(mockService)

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
	resource := newIdentityResource(mockService)

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
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
			capturedIP = ipAddress
			op := &identityaccess.Operator{Email: email, DisplayName: "Test"}
			op.ID = 1
			return &identityaccess.OperatorLoginResult{
				Status:       identityaccess.LoginStatusAuthenticated,
				AccessToken:  "access",
				RefreshToken: "refresh",
				Operator:     identityOperatorOf(op),
			}, nil
		},
	}

	resource := newIdentityResource(mockService)

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
		loginWithMFAGateFn: func(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
			capturedIP = ipAddress
			op := &identityaccess.Operator{Email: email, DisplayName: "Test"}
			op.ID = 1
			return &identityaccess.OperatorLoginResult{
				Status:       identityaccess.LoginStatusAuthenticated,
				AccessToken:  "access",
				RefreshToken: "refresh",
				Operator:     identityOperatorOf(op),
			}, nil
		},
	}

	resource := newIdentityResource(mockService)

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

func serveOperatorLoginThroughXFFMiddleware(resource *identityoperator.Resource, rr *httptest.ResponseRecorder, req *http.Request) {
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFF())
	router.Post("/auth/login", resource.Login)
	router.ServeHTTP(rr, req)
}

func TestLoginRequest_Bind(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	loginReq := &identityoperator.LoginRequest{}

	err := loginReq.Bind(req)
	assert.NoError(t, err)
}

// recoveryProofHeader is the HTTP contract of the refresh recovery proof
// (auth/rotation.RecoveryProofHeader), spelled out so the router test does not
// import the rotation policy.
const recoveryProofHeader = "X-Refresh-Recovery-Proof"

func TestRefreshToken_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		refreshTokenFn: func(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
			assert.Equal(t, int64(42), operatorID)
			assert.Equal(t, "opaque-refresh-handle", refreshTokenValue)
			// COVERAGE GAP (#2736): the header → recovery-proof context wiring is no longer asserted; it needs auth/rotation, which this package may not import.
			return "new-access-token", "new-refresh-token", nil
		},
	}

	resource := newIdentityResource(mockService)

	// Create a real token with claims
	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":    float64(42),
		"token": "opaque-refresh-handle",
		"scope": "platform",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set(recoveryProofHeader, "independent-recovery-secret")

	// Set CtxRefreshToken in context
	ctx := testutil.WithRefreshToken(req.Context(), tokenString)

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
	resource := newIdentityResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	rr := httptest.NewRecorder()

	resource.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Unauthorized")
}

func TestRefreshToken_InvalidClaims(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := newIdentityResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := testutil.WithRefreshToken(req.Context(), "some-token-string")
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
	resource := newIdentityResource(mockService)

	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":        float64(42),
		"token":     "tenant-refresh-handle",
		"tenant_id": float64(1),
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := testutil.WithRefreshToken(req.Context(), tokenString)
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
	resource := newIdentityResource(mockService)

	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":    float64(42),
		"token": "operator-refresh-42",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := testutil.WithRefreshToken(req.Context(), tokenString)
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
			return "", "", identityoperator.ErrOperatorInactive
		},
	}

	resource := newIdentityResource(mockService)

	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":    float64(42),
		"token": "opaque-refresh-handle",
		"scope": "platform",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := testutil.WithRefreshToken(req.Context(), tokenString)

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
			return "", "", identityaccess.ErrOperatorRefreshTokenInvalid
		},
	}
	resource := newIdentityResource(mockService)

	tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]interface{}{
		"id":    float64(42),
		"token": "stale-refresh-handle",
		"scope": "platform",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	ctx := testutil.WithRefreshToken(req.Context(), tokenString)
	token, _ := jwtauth.VerifyToken(tokenAuth, tokenString)
	ctx = jwtauth.NewContext(ctx, token, nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.RefreshToken(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Unauthorized")
	assert.NotContains(t, rr.Body.String(), "Invalid email or password")
}
