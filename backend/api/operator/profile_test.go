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
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

func TestGetProfile_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		getOperatorFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			assert.Equal(t, int64(123), id)
			op := &platform.Operator{
				Email:       "operator@example.com",
				DisplayName: "Test Operator",
			}
			op.ID = 123
			return op, nil
		},
	}

	resource := operator.NewProfileResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetProfile(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	assert.Equal(t, float64(123), data["id"])
	assert.Equal(t, "operator@example.com", data["email"])
	assert.Equal(t, "Test Operator", data["display_name"])
}

func TestGetProfile_OperatorNotFound(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		getOperatorFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return nil, &platformSvc.OperatorNotFoundError{}
		},
	}

	resource := operator.NewProfileResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	claims := jwt.AppClaims{ID: 999}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetProfile(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "Operator not found")
}

func TestGetProfile_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		getOperatorFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return nil, errors.New("database error")
		},
	}

	resource := operator.NewProfileResource(mockService)

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.GetProfile(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "An error occurred")
}

func TestUpdateProfile_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		updateProfileFn: func(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error) {
			assert.Equal(t, int64(123), operatorID)
			assert.Equal(t, "New Display Name", displayName)
			op := &platform.Operator{
				Email:       "operator@example.com",
				DisplayName: "New Display Name",
			}
			op.ID = 123
			return op, nil
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"display_name": "New Display Name",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/profile", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.UpdateProfile(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	assert.Equal(t, "New Display Name", data["display_name"])
}

func TestUpdateProfile_EmptyDisplayName(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"display_name": "",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/profile", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.UpdateProfile(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "display_name is required")
}

func TestUpdateProfile_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	req := httptest.NewRequest(http.MethodPut, "/profile", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.UpdateProfile(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdateProfile_InvalidData(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		updateProfileFn: func(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error) {
			return nil, &platformSvc.InvalidDataError{Err: errors.New("display name too long")}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"display_name": "Some Name",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/profile", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.UpdateProfile(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChangePassword_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		changePasswordFn: func(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
			assert.Equal(t, int64(123), operatorID)
			assert.Equal(t, "currentpass", currentPassword)
			assert.Equal(t, "newpass123", newPassword)
			return nil
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"current_password": "currentpass",
		"new_password":     "newpass123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ChangePassword(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Password changed successfully")
}

func TestChangePassword_EmptyCurrentPassword(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"current_password": "",
		"new_password":     "newpass123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ChangePassword(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "current_password and new_password are required")
}

func TestChangePassword_EmptyNewPassword(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"current_password": "currentpass",
		"new_password":     "",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ChangePassword(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "current_password and new_password are required")
}

func TestChangePassword_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/profile/password", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ChangePassword(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChangePassword_PasswordMismatch(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		changePasswordFn: func(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
			return &platformSvc.PasswordMismatchError{}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"current_password": "wrongpass",
		"new_password":     "newpass123",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/password", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.ChangePassword(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "aktuelle Passwort ist falsch")
}

func TestUpdateProfileRequest_Bind(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPut, "/profile", nil)
	updateReq := &operator.UpdateProfileRequest{}

	err := updateReq.Bind(req)
	assert.NoError(t, err)
}

func TestChangePasswordRequest_Bind(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/profile/password", nil)
	changeReq := &operator.ChangePasswordRequest{}

	err := changeReq.Bind(req)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// InitiateEmailChange handler tests
// ---------------------------------------------------------------------------

func TestInitiateEmailChange_Success(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		initiateEmailChangeFn: func(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
			assert.Equal(t, int64(123), operatorID)
			assert.Equal(t, "new@example.com", newEmail)
			assert.Equal(t, "currentpass", currentPassword)
			return nil
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"new_email":        "new@example.com",
		"current_password": "currentpass",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Verification email will be sent shortly")
}

func TestInitiateEmailChange_EmptyNewEmail(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"new_email":        "",
		"current_password": "currentpass",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "new_email and current_password are required")
}

func TestInitiateEmailChange_EmptyPassword(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"new_email":        "new@example.com",
		"current_password": "",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "new_email and current_password are required")
}

func TestInitiateEmailChange_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestInitiateEmailChange_PasswordMismatch(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		initiateEmailChangeFn: func(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
			return &platformSvc.PasswordMismatchError{}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"new_email":        "new@example.com",
		"current_password": "wrongpass",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "aktuelle Passwort ist falsch")
}

func TestInitiateEmailChange_SameEmail(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		initiateEmailChangeFn: func(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
			return &platformSvc.EmailChangeSameEmailError{}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"new_email":        "same@example.com",
		"current_password": "currentpass",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "identisch mit der aktuellen")
}

func TestInitiateEmailChange_EmailAlreadyInUse(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		initiateEmailChangeFn: func(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
			return &platformSvc.EmailAlreadyInUseError{}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"new_email":        "taken@example.com",
		"current_password": "currentpass",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), "bereits verwendet")
}

func TestInitiateEmailChange_RateLimit(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		initiateEmailChangeFn: func(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
			return &platformSvc.EmailChangeRateLimitError{}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"new_email":        "new@example.com",
		"current_password": "currentpass",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "Zu viele Versuche")
}

func TestInitiateEmailChange_OperatorInactive(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		initiateEmailChangeFn: func(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
			return &platformSvc.OperatorInactiveError{OperatorID: operatorID}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"new_email":        "new@example.com",
		"current_password": "currentpass",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "deaktiviert")
}

func TestInitiateEmailChange_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{
		initiateEmailChangeFn: func(ctx context.Context, operatorID int64, newEmail, currentPassword string, clientIP net.IP) error {
			return errors.New("database error")
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"new_email":        "new@example.com",
		"current_password": "currentpass",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	claims := jwt.AppClaims{ID: 123}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	resource.InitiateEmailChange(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "An error occurred")
}

// ---------------------------------------------------------------------------
// ConfirmEmailChange handler tests
// ---------------------------------------------------------------------------

func TestConfirmEmailChange_Success(t *testing.T) {
	t.Parallel()

	testUUID := "00000000-0000-4000-a000-000000000001"
	mockService := &mockOperatorAuthService{
		confirmEmailChangeFn: func(ctx context.Context, token string, clientIP net.IP) (string, error) {
			assert.Equal(t, testUUID, token)
			return "new@example.com", nil
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"token": testUUID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ConfirmEmailChange(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Email changed successfully")

	// Verify response does NOT contain new_email (P1 fix: no PII on unauthenticated endpoint)
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Nil(t, response["data"], "unauthenticated response must not contain data payload")
}

func TestConfirmEmailChange_EmptyToken(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"token": "",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ConfirmEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "dieser Link ist abgelaufen oder ungültig")
}

func TestConfirmEmailChange_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ConfirmEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestConfirmEmailChange_TokenInvalid(t *testing.T) {
	t.Parallel()

	testUUID := "00000000-0000-4000-a000-000000000001"
	mockService := &mockOperatorAuthService{
		confirmEmailChangeFn: func(ctx context.Context, token string, clientIP net.IP) (string, error) {
			return "", &platformSvc.EmailChangeTokenInvalidError{}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"token": testUUID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ConfirmEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "abgelaufen oder ungültig")
}

func TestConfirmEmailChange_EmailAlreadyInUse(t *testing.T) {
	t.Parallel()

	testUUID := "00000000-0000-4000-a000-000000000001"
	mockService := &mockOperatorAuthService{
		confirmEmailChangeFn: func(ctx context.Context, token string, clientIP net.IP) (string, error) {
			return "", &platformSvc.EmailAlreadyInUseError{}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"token": testUUID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ConfirmEmailChange(rr, req)

	// Anti-enumeration: all errors return generic 400 on this unauthenticated endpoint
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "abgelaufen oder ungültig")
}

func TestConfirmEmailChange_OperatorInactive(t *testing.T) {
	t.Parallel()

	testUUID := "00000000-0000-4000-a000-000000000001"
	mockService := &mockOperatorAuthService{
		confirmEmailChangeFn: func(ctx context.Context, token string, clientIP net.IP) (string, error) {
			return "", &platformSvc.OperatorInactiveError{OperatorID: 123}
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"token": testUUID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ConfirmEmailChange(rr, req)

	// Anti-enumeration: all errors return generic 400 on this unauthenticated endpoint
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "abgelaufen oder ungültig")
}

func TestConfirmEmailChange_ServiceError(t *testing.T) {
	t.Parallel()

	testUUID := "00000000-0000-4000-a000-000000000001"
	mockService := &mockOperatorAuthService{
		confirmEmailChangeFn: func(ctx context.Context, token string, clientIP net.IP) (string, error) {
			return "", errors.New("database error")
		},
	}

	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"token": testUUID,
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ConfirmEmailChange(rr, req)

	// Infrastructure errors return 500 so the frontend can offer retry
	// (the token remains unconsumed on rollback). Anti-enumeration only
	// flattens known application errors (token invalid, email taken, inactive).
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Serverfehler")
}

func TestConfirmEmailChange_WhitespaceToken(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"token": "   ",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ConfirmEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "dieser Link ist abgelaufen oder ungültig")
}

func TestInitiateEmailChangeRequest_Bind(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/profile/email-change", nil)
	changeReq := &operator.InitiateEmailChangeRequest{
		NewEmail:        "test@example.com",
		CurrentPassword: "password",
	}

	err := changeReq.Bind(req)
	assert.NoError(t, err)
}

func TestConfirmEmailChangeRequest_Bind(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", nil)
	confirmReq := &operator.ConfirmEmailChangeRequest{
		Token: "00000000-0000-4000-a000-000000000001",
	}

	err := confirmReq.Bind(req)
	assert.NoError(t, err)
}

func TestConfirmEmailChange_InvalidTokenFormat(t *testing.T) {
	t.Parallel()

	mockService := &mockOperatorAuthService{}
	resource := operator.NewProfileResource(mockService)

	body := map[string]string{
		"token": "not-a-valid-uuid",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/auth/email-confirm", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ConfirmEmailChange(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "dieser Link ist abgelaufen oder ungültig")
}
