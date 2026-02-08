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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// Mock OperatorAuthService
type mockOperatorAuthService struct {
	loginFn          func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error)
	getOperatorFn    func(ctx context.Context, id int64) (*platform.Operator, error)
	updateProfileFn  func(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error)
	changePasswordFn func(ctx context.Context, operatorID int64, currentPassword, newPassword string) error
}

func (m *mockOperatorAuthService) Login(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, email, password, clientIP)
	}
	return "", "", nil, nil
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

func (m *mockOperatorAuthService) ListOperators(ctx context.Context) ([]*platform.Operator, error) {
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

func TestLogin_Success(t *testing.T) {
	mockService := &mockOperatorAuthService{
		loginFn: func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
			assert.Equal(t, "test@example.com", email)
			assert.Equal(t, "password123", password)
			op := &platform.Operator{
				Email:       "test@example.com",
				DisplayName: "Test Operator",
			}
			op.ID = 1
			return "access-token", "refresh-token", op, nil
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
	assert.Equal(t, "access-token", data["access_token"])
	assert.Equal(t, "refresh-token", data["refresh_token"])

	operatorData := data["operator"].(map[string]interface{})
	assert.Equal(t, float64(1), operatorData["id"])
	assert.Equal(t, "test@example.com", operatorData["email"])
	assert.Equal(t, "Test Operator", operatorData["display_name"])
}

func TestLogin_EmptyEmail(t *testing.T) {
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
	mockService := &mockOperatorAuthService{
		loginFn: func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
			return "", "", nil, &platformSvc.InvalidCredentialsError{}
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
	mockService := &mockOperatorAuthService{
		loginFn: func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
			return "", "", nil, &platformSvc.OperatorInactiveError{}
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
	mockService := &mockOperatorAuthService{
		loginFn: func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
			return "", "", nil, &platformSvc.OperatorNotFoundError{}
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
	mockService := &mockOperatorAuthService{
		loginFn: func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
			return "", "", nil, errors.New("database connection error")
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
	mockService := &mockOperatorAuthService{}
	resource := operator.NewAuthResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestLogin_ClientIPExtraction_XForwardedFor(t *testing.T) {
	var capturedIP net.IP
	mockService := &mockOperatorAuthService{
		loginFn: func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
			capturedIP = clientIP
			op := &platform.Operator{Email: email, DisplayName: "Test"}
			op.ID = 1
			return "access", "refresh", op, nil
		},
	}

	resource := operator.NewAuthResource(mockService)

	body := map[string]string{"email": "test@example.com", "password": "password123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "192.168.1.100", capturedIP.String())
}

func TestLogin_ClientIPExtraction_XRealIP(t *testing.T) {
	var capturedIP net.IP
	mockService := &mockOperatorAuthService{
		loginFn: func(ctx context.Context, email, password string, clientIP net.IP) (string, string, *platform.Operator, error) {
			capturedIP = clientIP
			op := &platform.Operator{Email: email, DisplayName: "Test"}
			op.ID = 1
			return "access", "refresh", op, nil
		},
	}

	resource := operator.NewAuthResource(mockService)

	body := map[string]string{"email": "test@example.com", "password": "password123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", "10.0.0.50")
	rr := httptest.NewRecorder()

	resource.Login(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "10.0.0.50", capturedIP.String())
}

func TestLoginRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	loginReq := &operator.LoginRequest{}

	err := loginReq.Bind(req)
	assert.NoError(t, err)
}
