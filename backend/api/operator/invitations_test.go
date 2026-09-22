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
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
)

// invitationMockService implements OperatorAccess —
// the narrow interface InvitationsResource depends on. It only stubs the
// eight invitation-management methods the handler actually calls.
type invitationMockService struct {
	inviteOperatorFn             func(ctx context.Context, email string, displayName *string, createdByID int64, clientIP net.IP) error
	validateOperatorInvitationFn func(ctx context.Context, token string) (*identityaccess.OperatorInvitation, error)
	acceptOperatorInvitationFn   func(ctx context.Context, token, displayName, password string, clientIP net.IP) (*identityaccess.Operator, error)
	listPendingInvitationsFn     func(ctx context.Context) ([]*identityaccess.OperatorInvitation, error)
	revokeOperatorInvitationFn   func(ctx context.Context, invitationID int64, actorID int64, clientIP net.IP) error
	resendOperatorInvitationFn   func(ctx context.Context, invitationID int64, actorID int64, clientIP net.IP) error
	listOperatorsFn              func(ctx context.Context) ([]*identityaccess.Operator, error)
	cleanupExpiredInvitationsFn  func(ctx context.Context) (int, error)
}

func (m *invitationMockService) ListOperators(ctx context.Context) ([]identityaccess.Operator, error) {
	if m.listOperatorsFn != nil {
		rows, err := m.listOperatorsFn(ctx)
		return identityOperators(rows), err
	}
	return []identityaccess.Operator{}, nil
}

func (m *invitationMockService) FindOperator(context.Context, int64) (identityaccess.Operator, error) {
	return identityaccess.Operator{}, identityoperator.ErrOperatorNotFound
}

func (m *invitationMockService) FindOperatorForUpdate(context.Context, int64) (identityaccess.Operator, error) {
	return identityaccess.Operator{}, identityoperator.ErrOperatorNotFound
}

func (m *invitationMockService) FindOperatorByEmail(context.Context, string) (identityaccess.Operator, error) {
	return identityaccess.Operator{}, identityoperator.ErrOperatorNotFound
}

func (m *invitationMockService) IssueTokensForAuthenticatedOperator(context.Context, int64, string, string) (string, string, error) {
	return "", "", nil
}

func (m *invitationMockService) InviteOperator(ctx context.Context, request identityaccess.OperatorInvitationRequest) error {
	if m.inviteOperatorFn != nil {
		return m.inviteOperatorFn(ctx, request.Email, request.DisplayName, request.CreatedBy, net.ParseIP(request.IPAddress))
	}
	return nil
}

func (m *invitationMockService) ValidateOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitationPreview, error) {
	if m.validateOperatorInvitationFn != nil {
		row, err := m.validateOperatorInvitationFn(ctx, token)
		if row == nil {
			return identityaccess.OperatorInvitationPreview{}, err
		}
		return identityaccess.OperatorInvitationPreview{
			Email: row.Email, DisplayName: row.DisplayName, ExpiresAt: row.ExpiresAt,
		}, err
	}
	return identityaccess.OperatorInvitationPreview{}, nil
}

func (m *invitationMockService) AcceptOperatorInvitation(ctx context.Context, acceptance identityaccess.OperatorInvitationAcceptance) (identityaccess.Operator, error) {
	if m.acceptOperatorInvitationFn != nil {
		row, err := m.acceptOperatorInvitationFn(ctx, acceptance.Token, acceptance.DisplayName, acceptance.Password, net.ParseIP(acceptance.IPAddress))
		if row == nil {
			return identityaccess.Operator{}, err
		}
		return identityOperator(row), err
	}
	return identityaccess.Operator{}, nil
}

func (m *invitationMockService) ListPendingOperatorInvitations(ctx context.Context) ([]identityaccess.OperatorInvitation, error) {
	if m.listPendingInvitationsFn != nil {
		rows, err := m.listPendingInvitationsFn(ctx)
		if err != nil {
			return nil, err
		}
		invitations := make([]identityaccess.OperatorInvitation, 0, len(rows))
		for _, row := range rows {
			if row == nil {
				continue
			}
			invitations = append(invitations, *row)
		}
		return invitations, nil
	}
	return nil, nil
}

func (m *invitationMockService) RevokeOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	if m.revokeOperatorInvitationFn != nil {
		return m.revokeOperatorInvitationFn(ctx, invitationID, actorID, net.ParseIP(ipAddress))
	}
	return nil
}

func (m *invitationMockService) ResendOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	if m.resendOperatorInvitationFn != nil {
		return m.resendOperatorInvitationFn(ctx, invitationID, actorID, net.ParseIP(ipAddress))
	}
	return nil
}

func (m *invitationMockService) InitiateOperatorEmailChange(context.Context, identityaccess.OperatorEmailChangeRequest) error {
	return nil
}

func (m *invitationMockService) ConfirmOperatorEmailChange(context.Context, string, string) (identityaccess.OperatorEmailChangeResult, error) {
	return identityaccess.OperatorEmailChangeResult{}, nil
}

func (m *invitationMockService) CleanupOperatorEmailChanges(ctx context.Context) (int, error) {
	if m.cleanupExpiredInvitationsFn != nil {
		return m.cleanupExpiredInvitationsFn(ctx)
	}
	return 0, nil
}

func identityOperator(row *identityaccess.Operator) identityaccess.Operator {
	return identityaccess.Operator{
		ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Active: row.Active,
		LastLogin: row.LastLogin, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func identityOperators(rows []*identityaccess.Operator) []identityaccess.Operator {
	operators := make([]identityaccess.Operator, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		operators = append(operators, identityOperator(row))
	}
	return operators
}

// --- Helper to build request with JWT claims context ---

func invitationReqWithClaims(method, path string, body []byte, operatorID int) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	claims := testutil.Claims{ID: operatorID, Scope: "platform"}
	ctx := testutil.WithAuthenticatedContext(req.Context(), claims, nil)
	return req.WithContext(ctx)
}

func invitationReqWithClaimsAndID(method, path string, body []byte, operatorID int, urlParamID string) *http.Request {
	req := invitationReqWithClaims(method, path, body, operatorID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", urlParamID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

// =====================================================================
// CreateInvitation Tests
// =====================================================================

func TestCreateInvitation_Success(t *testing.T) {
	t.Parallel()

	var capturedEmail string
	mockService := &invitationMockService{
		inviteOperatorFn: func(_ context.Context, email string, _ *string, createdByID int64, _ net.IP) error {
			capturedEmail = email
			assert.Equal(t, int64(42), createdByID)
			return nil
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{"email": "new@example.com"})
	req := invitationReqWithClaims(http.MethodPost, "/invitations", body, 42)
	rr := httptest.NewRecorder()

	resource.CreateInvitation(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "new@example.com", capturedEmail)
}

func TestCreateInvitation_WithDisplayName(t *testing.T) {
	t.Parallel()

	var capturedDisplayName *string
	mockService := &invitationMockService{
		inviteOperatorFn: func(_ context.Context, _ string, displayName *string, _ int64, _ net.IP) error {
			capturedDisplayName = displayName
			return nil
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{"email": "new@example.com", "display_name": "New Operator"})
	req := invitationReqWithClaims(http.MethodPost, "/invitations", body, 42)
	rr := httptest.NewRecorder()

	resource.CreateInvitation(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	require.NotNil(t, capturedDisplayName)
	assert.Equal(t, "New Operator", *capturedDisplayName)
}

func TestCreateInvitation_EmptyEmail(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	body, _ := json.Marshal(map[string]string{"email": ""})
	req := invitationReqWithClaims(http.MethodPost, "/invitations", body, 42)
	rr := httptest.NewRecorder()

	resource.CreateInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "email is required")
}

func TestCreateInvitation_WhitespaceOnlyEmail(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	body, _ := json.Marshal(map[string]string{"email": "   "})
	req := invitationReqWithClaims(http.MethodPost, "/invitations", body, 42)
	rr := httptest.NewRecorder()

	resource.CreateInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateInvitation_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	req := invitationReqWithClaims(http.MethodPost, "/invitations", []byte("not-json"), 42)
	rr := httptest.NewRecorder()

	resource.CreateInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateInvitation_EmailAlreadyExists(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		inviteOperatorFn: func(_ context.Context, _ string, _ *string, _ int64, _ net.IP) error {
			return identityoperator.ErrOperatorEmailExists
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{"email": "existing@example.com"})
	req := invitationReqWithClaims(http.MethodPost, "/invitations", body, 42)
	rr := httptest.NewRecorder()

	resource.CreateInvitation(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestCreateInvitation_RateLimit(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		inviteOperatorFn: func(_ context.Context, _ string, _ *string, _ int64, _ net.IP) error {
			return identityoperator.ErrOperatorInvitationRateLimited
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{"email": "spam@example.com"})
	req := invitationReqWithClaims(http.MethodPost, "/invitations", body, 42)
	rr := httptest.NewRecorder()

	resource.CreateInvitation(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "Zu viele Einladungen")
}

func TestCreateInvitation_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		inviteOperatorFn: func(_ context.Context, _ string, _ *string, _ int64, _ net.IP) error {
			return errors.New("database connection error")
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{"email": "new@example.com"})
	req := invitationReqWithClaims(http.MethodPost, "/invitations", body, 42)
	rr := httptest.NewRecorder()

	resource.CreateInvitation(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestCreateInvitation_InvalidData(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		inviteOperatorFn: func(_ context.Context, _ string, _ *string, _ int64, _ net.IP) error {
			return &identityoperator.InvalidInputError{Err: errors.New("invalid email format")}
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{"email": "bad-email"})
	req := invitationReqWithClaims(http.MethodPost, "/invitations", body, 42)
	rr := httptest.NewRecorder()

	resource.CreateInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// =====================================================================
// ListInvitations Tests
// =====================================================================

func TestListInvitations_Success(t *testing.T) {
	t.Parallel()

	now := time.Now()
	mockService := &invitationMockService{
		listPendingInvitationsFn: func(_ context.Context) ([]*identityaccess.OperatorInvitation, error) {
			token := &identityaccess.OperatorInvitation{
				Email:     "pending@example.com",
				CreatedBy: 42,
				ExpiresAt: now.Add(48 * time.Hour),
			}
			token.ID = 100
			token.CreatedAt = now
			return []*identityaccess.OperatorInvitation{token}, nil
		},
		listOperatorsFn: func(_ context.Context) ([]*identityaccess.Operator, error) {
			op := &identityaccess.Operator{
				Email:       "admin@example.com",
				DisplayName: "Admin Op",
				Active:      true,
			}
			op.ID = 42
			op.CreatedAt = now
			return []*identityaccess.Operator{op}, nil
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaims(http.MethodGet, "/invitations", nil, 42)
	rr := httptest.NewRecorder()

	resource.ListInvitations(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	invitations := data["invitations"].([]interface{})
	operators := data["operators"].([]interface{})

	assert.Len(t, invitations, 1)
	assert.Len(t, operators, 1)

	inv := invitations[0].(map[string]interface{})
	assert.Equal(t, "pending@example.com", inv["email"])
	assert.Equal(t, "Admin Op", inv["creator_name"])
}

func TestListInvitations_EmptyLists(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		listPendingInvitationsFn: func(_ context.Context) ([]*identityaccess.OperatorInvitation, error) {
			return []*identityaccess.OperatorInvitation{}, nil
		},
		listOperatorsFn: func(_ context.Context) ([]*identityaccess.Operator, error) {
			return []*identityaccess.Operator{}, nil
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaims(http.MethodGet, "/invitations", nil, 42)
	rr := httptest.NewRecorder()

	resource.ListInvitations(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListInvitations_PendingListError(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		listPendingInvitationsFn: func(_ context.Context) ([]*identityaccess.OperatorInvitation, error) {
			return nil, errors.New("db error")
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaims(http.MethodGet, "/invitations", nil, 42)
	rr := httptest.NewRecorder()

	resource.ListInvitations(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestListInvitations_OperatorsListError(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		listPendingInvitationsFn: func(_ context.Context) ([]*identityaccess.OperatorInvitation, error) {
			return []*identityaccess.OperatorInvitation{}, nil
		},
		listOperatorsFn: func(_ context.Context) ([]*identityaccess.Operator, error) {
			return nil, errors.New("db error")
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaims(http.MethodGet, "/invitations", nil, 42)
	rr := httptest.NewRecorder()

	resource.ListInvitations(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =====================================================================
// ResendInvitation Tests
// =====================================================================

func TestResendInvitation_Success(t *testing.T) {
	t.Parallel()

	var capturedInvitationID int64
	mockService := &invitationMockService{
		resendOperatorInvitationFn: func(_ context.Context, invitationID int64, actorID int64, _ net.IP) error {
			capturedInvitationID = invitationID
			assert.Equal(t, int64(42), actorID)
			return nil
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaimsAndID(http.MethodPost, "/invitations/100/resend", nil, 42, "100")
	rr := httptest.NewRecorder()

	resource.ResendInvitation(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(100), capturedInvitationID)
}

func TestResendInvitation_InvalidID(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	req := invitationReqWithClaimsAndID(http.MethodPost, "/invitations/abc/resend", nil, 42, "abc")
	rr := httptest.NewRecorder()

	resource.ResendInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid invitation ID")
}

func TestResendInvitation_NotFound(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		resendOperatorInvitationFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error {
			return identityoperator.ErrOperatorInvitationNotFound
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaimsAndID(http.MethodPost, "/invitations/999/resend", nil, 42, "999")
	rr := httptest.NewRecorder()

	resource.ResendInvitation(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestResendInvitation_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		resendOperatorInvitationFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error {
			return errors.New("unexpected error")
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaimsAndID(http.MethodPost, "/invitations/100/resend", nil, 42, "100")
	rr := httptest.NewRecorder()

	resource.ResendInvitation(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =====================================================================
// RevokeInvitation Tests
// =====================================================================

func TestRevokeInvitation_Success(t *testing.T) {
	t.Parallel()

	var capturedInvitationID int64
	mockService := &invitationMockService{
		revokeOperatorInvitationFn: func(_ context.Context, invitationID int64, actorID int64, _ net.IP) error {
			capturedInvitationID = invitationID
			assert.Equal(t, int64(42), actorID)
			return nil
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaimsAndID(http.MethodDelete, "/invitations/100", nil, 42, "100")
	rr := httptest.NewRecorder()

	resource.RevokeInvitation(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, int64(100), capturedInvitationID)
}

func TestRevokeInvitation_InvalidID(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	req := invitationReqWithClaimsAndID(http.MethodDelete, "/invitations/abc", nil, 42, "abc")
	rr := httptest.NewRecorder()

	resource.RevokeInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRevokeInvitation_NotFound(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		revokeOperatorInvitationFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error {
			return identityoperator.ErrOperatorInvitationNotFound
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaimsAndID(http.MethodDelete, "/invitations/999", nil, 42, "999")
	rr := httptest.NewRecorder()

	resource.RevokeInvitation(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRevokeInvitation_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		revokeOperatorInvitationFn: func(_ context.Context, _ int64, _ int64, _ net.IP) error {
			return errors.New("db error")
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	req := invitationReqWithClaimsAndID(http.MethodDelete, "/invitations/100", nil, 42, "100")
	rr := httptest.NewRecorder()

	resource.RevokeInvitation(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =====================================================================
// ValidateInvitation Tests (public, unauthenticated)
// =====================================================================

func TestValidateInvitation_Success(t *testing.T) {
	t.Parallel()

	displayName := "New Op"
	mockService := &invitationMockService{
		validateOperatorInvitationFn: func(_ context.Context, token string) (*identityaccess.OperatorInvitation, error) {
			assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", token)
			return &identityaccess.OperatorInvitation{
				Email:       "new@example.com",
				DisplayName: &displayName,
				ExpiresAt:   time.Now().Add(48 * time.Hour),
			}, nil
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{"token": "550e8400-e29b-41d4-a716-446655440000"})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ValidateInvitation(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	assert.Equal(t, "new@example.com", data["email"])
	assert.Equal(t, "New Op", data["display_name"])
}

func TestValidateInvitation_EmptyToken(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	body, _ := json.Marshal(map[string]string{"token": ""})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ValidateInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestValidateInvitation_InvalidTokenFormat(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	body, _ := json.Marshal(map[string]string{"token": "not-a-uuid"})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ValidateInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid token format")
}

func TestValidateInvitation_TokenNotFound(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		validateOperatorInvitationFn: func(_ context.Context, _ string) (*identityaccess.OperatorInvitation, error) {
			return nil, identityoperator.ErrOperatorInvitationNotFound
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{"token": "550e8400-e29b-41d4-a716-446655440000"})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ValidateInvitation(rr, req)

	// Public endpoint: should return 400, not 404 (anti-enumeration)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "abgelaufen oder ungültig")
}

func TestValidateInvitation_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/validate", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.ValidateInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// =====================================================================
// AcceptInvitation Tests (public, unauthenticated)
// =====================================================================

func TestAcceptInvitation_Success(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		acceptOperatorInvitationFn: func(_ context.Context, token, displayName, password string, _ net.IP) (*identityaccess.Operator, error) {
			assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", token)
			assert.Equal(t, "New Operator", displayName)
			assert.Equal(t, "SecureP@ss1", password)
			op := &identityaccess.Operator{
				Email:       "new@example.com",
				DisplayName: "New Operator",
			}
			op.ID = 100
			return op, nil
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{
		"token":            "550e8400-e29b-41d4-a716-446655440000",
		"display_name":     "New Operator",
		"password":         "SecureP@ss1",
		"confirm_password": "SecureP@ss1",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	data := response["data"].(map[string]interface{})
	assert.Equal(t, float64(100), data["id"])
	assert.Equal(t, "new@example.com", data["email"])
}

func TestAcceptInvitation_EmptyToken(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	body, _ := json.Marshal(map[string]string{
		"token":            "",
		"display_name":     "Name",
		"password":         "SecureP@ss1",
		"confirm_password": "SecureP@ss1",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAcceptInvitation_InvalidTokenFormat(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	body, _ := json.Marshal(map[string]string{
		"token":            "not-uuid",
		"display_name":     "Name",
		"password":         "SecureP@ss1",
		"confirm_password": "SecureP@ss1",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAcceptInvitation_EmptyDisplayName(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	body, _ := json.Marshal(map[string]string{
		"token":            "550e8400-e29b-41d4-a716-446655440000",
		"display_name":     "",
		"password":         "SecureP@ss1",
		"confirm_password": "SecureP@ss1",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "display_name is required")
}

func TestAcceptInvitation_EmptyPassword(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	body, _ := json.Marshal(map[string]string{
		"token":            "550e8400-e29b-41d4-a716-446655440000",
		"display_name":     "Name",
		"password":         "",
		"confirm_password": "",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "password is required")
}

func TestAcceptInvitation_PasswordMismatch(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	body, _ := json.Marshal(map[string]string{
		"token":            "550e8400-e29b-41d4-a716-446655440000",
		"display_name":     "Name",
		"password":         "SecureP@ss1",
		"confirm_password": "DifferentP@ss1",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "passwords do not match")
}

func TestAcceptInvitation_TokenNotFound(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		acceptOperatorInvitationFn: func(_ context.Context, _, _, _ string, _ net.IP) (*identityaccess.Operator, error) {
			return nil, identityoperator.ErrOperatorInvitationNotFound
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{
		"token":            "550e8400-e29b-41d4-a716-446655440000",
		"display_name":     "Name",
		"password":         "SecureP@ss1",
		"confirm_password": "SecureP@ss1",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	// Public endpoint: generic error message
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "abgelaufen oder ungültig")
}

func TestAcceptInvitation_EmailAlreadyExists(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		acceptOperatorInvitationFn: func(_ context.Context, _, _, _ string, _ net.IP) (*identityaccess.Operator, error) {
			return nil, identityoperator.ErrOperatorEmailExists
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{
		"token":            "550e8400-e29b-41d4-a716-446655440000",
		"display_name":     "Name",
		"password":         "SecureP@ss1",
		"confirm_password": "SecureP@ss1",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	// Public endpoint: same generic message for both not-found and email-exists
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "abgelaufen oder ungültig")
}

func TestAcceptInvitation_InvalidData(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		acceptOperatorInvitationFn: func(_ context.Context, _, _, _ string, _ net.IP) (*identityaccess.Operator, error) {
			return nil, &identityoperator.InvalidInputError{Err: errors.New("password too weak")}
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{
		"token":            "550e8400-e29b-41d4-a716-446655440000",
		"display_name":     "Name",
		"password":         "SecureP@ss1",
		"confirm_password": "SecureP@ss1",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAcceptInvitation_ServiceError(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{
		acceptOperatorInvitationFn: func(_ context.Context, _, _, _ string, _ net.IP) (*identityaccess.Operator, error) {
			return nil, errors.New("unexpected error")
		},
	}

	resource := identityoperator.NewInvitationsResource(mockService)
	body, _ := json.Marshal(map[string]string{
		"token":            "550e8400-e29b-41d4-a716-446655440000",
		"display_name":     "Name",
		"password":         "SecureP@ss1",
		"confirm_password": "SecureP@ss1",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestAcceptInvitation_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockService := &invitationMockService{}
	resource := identityoperator.NewInvitationsResource(mockService)

	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/accept", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// =====================================================================
// Request Bind Tests
// =====================================================================

func TestCreateInvitationRequest_Bind_TrimsWhitespace(t *testing.T) {
	t.Parallel()

	req := &identityoperator.CreateInvitationRequest{Email: "  test@example.com  "}
	err := req.Bind(httptest.NewRequest(http.MethodPost, "/", nil))

	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", req.Email)
}

func TestValidateInvitationRequest_Bind_ValidUUID(t *testing.T) {
	t.Parallel()

	req := &identityoperator.ValidateInvitationRequest{Token: "550e8400-e29b-41d4-a716-446655440000"}
	err := req.Bind(httptest.NewRequest(http.MethodPost, "/", nil))

	assert.NoError(t, err)
}

func TestValidateInvitationRequest_Bind_WhitespaceToken(t *testing.T) {
	t.Parallel()

	req := &identityoperator.ValidateInvitationRequest{Token: "  550e8400-e29b-41d4-a716-446655440000  "}
	err := req.Bind(httptest.NewRequest(http.MethodPost, "/", nil))

	assert.NoError(t, err)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", req.Token)
}

func TestAcceptInvitationRequest_Bind_Valid(t *testing.T) {
	t.Parallel()

	req := &identityoperator.AcceptInvitationRequest{
		Token:           "550e8400-e29b-41d4-a716-446655440000",
		DisplayName:     "  New Op  ",
		Password:        "SecureP@ss1",
		ConfirmPassword: "SecureP@ss1",
	}
	err := req.Bind(httptest.NewRequest(http.MethodPost, "/", nil))

	assert.NoError(t, err)
	assert.Equal(t, "New Op", req.DisplayName)
}

func TestAcceptInvitationRequest_Bind_WhitespaceDisplayName(t *testing.T) {
	t.Parallel()

	req := &identityoperator.AcceptInvitationRequest{
		Token:           "550e8400-e29b-41d4-a716-446655440000",
		DisplayName:     "   ",
		Password:        "SecureP@ss1",
		ConfirmPassword: "SecureP@ss1",
	}
	err := req.Bind(httptest.NewRequest(http.MethodPost, "/", nil))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "display_name is required")
}
