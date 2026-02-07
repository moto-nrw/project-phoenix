package operator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	req := httptest.NewRequest(http.MethodPut, "/profile", nil)
	updateReq := &operator.UpdateProfileRequest{}

	err := updateReq.Bind(req)
	assert.NoError(t, err)
}

func TestChangePasswordRequest_Bind(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/profile/password", nil)
	changeReq := &operator.ChangePasswordRequest{}

	err := changeReq.Bind(req)
	assert.NoError(t, err)
}
