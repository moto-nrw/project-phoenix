package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// invitationsStub is a func-field double for the Invitations capability the
// routes consume. It embeds the interface so a route that reaches for a
// method the test did not configure nil-derefs and fails loudly, which is
// the guardrail these tests want.
type invitationsStub struct {
	Invitations
	createSchool   func(ctx context.Context, request identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error)
	validateSchool func(ctx context.Context, token string) (identityaccess.InvitationPreview, error)
	acceptSchool   func(ctx context.Context, token string, registration identityaccess.InvitationRegistration) (identityaccess.Account, error)
	listPending    func(ctx context.Context) ([]identityaccess.SchoolInvitation, error)
}

func (s *invitationsStub) CreateSchoolInvitation(ctx context.Context, request identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error) {
	return s.createSchool(ctx, request)
}

func (s *invitationsStub) ValidateSchoolInvitation(ctx context.Context, token string) (identityaccess.InvitationPreview, error) {
	return s.validateSchool(ctx, token)
}

func (s *invitationsStub) AcceptSchoolInvitation(ctx context.Context, token string, registration identityaccess.InvitationRegistration) (identityaccess.Account, error) {
	return s.acceptSchool(ctx, token, registration)
}

func (s *invitationsStub) ListPendingSchoolInvitations(ctx context.Context) ([]identityaccess.SchoolInvitation, error) {
	return s.listPending(ctx)
}

func (s *invitationsStub) SchoolInvitationSubdomain(context.Context, string) string { return "" }

func decodeJSONBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body
}

func TestInvitationHandlers_CreateInvitationAndListPending(t *testing.T) {
	t.Parallel()

	first := "Ada"
	last := "Lovelace"
	position := "Principal"
	roleName := "admin"
	creator := "creator@example.com"
	tokenErr := "smtp failed"
	now := time.Now().UTC()

	service := &invitationsStub{
		createSchool: func(_ context.Context, request identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error) {
			assert.Equal(t, "invitee@example.com", request.Email)
			assert.Equal(t, int64(9007199254740993), request.RoleID)
			assert.Equal(t, int64(44), request.CreatedBy)
			return identityaccess.SchoolInvitation{
				ID:           1,
				Email:        request.Email,
				RoleID:       request.RoleID,
				RoleName:     roleName,
				Token:        "tok-1",
				ExpiresAt:    now,
				FirstName:    &first,
				LastName:     &last,
				Position:     &position,
				CreatedBy:    nil,
				CreatorEmail: creator,
				Delivery:     identityaccess.TokenDelivery{Error: &tokenErr, RetryCount: 2},
			}, nil
		},
		listPending: func(context.Context) ([]identityaccess.SchoolInvitation, error) {
			return []identityaccess.SchoolInvitation{{
				ID:        2,
				Email:     "pending@example.com",
				RoleID:    9,
				RoleName:  "teacher",
				Token:     "tok-2",
				ExpiresAt: now,
				Delivery:  identityaccess.TokenDelivery{SentAt: &now, RetryCount: 1},
			}}, nil
		},
	}

	resource := NewResource(nil, service, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/invitations", bytes.NewBufferString(`{"email":" INVITEE@EXAMPLE.COM ","role_id":"9007199254740993","first_name":" Ada ","last_name":" Lovelace ","position":" Principal "}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 44}))
	rr := httptest.NewRecorder()
	resource.createInvitation(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	body := decodeJSONBody(t, rr)
	data := body["data"].(map[string]any)
	assert.Equal(t, "9007199254740993", data["role_id"])
	assert.Equal(t, float64(0), data["created_by"])
	assert.Equal(t, "failed", data["delivery_status"])
	assert.Equal(t, creator, data["creator"])

	listReq := httptest.NewRequest(http.MethodGet, "/auth/invitations", nil)
	listRR := httptest.NewRecorder()
	resource.listPendingInvitations(listRR, listReq)
	require.Equal(t, http.StatusOK, listRR.Code, listRR.Body.String())
	listBody := decodeJSONBody(t, listRR)
	items := listBody["data"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	assert.Equal(t, "sent", item["delivery_status"])
	assert.Equal(t, float64(0), item["created_by"])
}

func TestInvitationHandlers_CreateInvitation_AccountAlreadyHasTenantAccess(t *testing.T) {
	t.Parallel()

	service := &invitationsStub{
		createSchool: func(context.Context, identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error) {
			return identityaccess.SchoolInvitation{}, &identityaccess.AuthenticationError{
				Op: "create invitation", Err: identityaccess.ErrAccountAlreadyHasTenantAccess,
			}
		},
	}

	resource := NewResource(nil, service, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/invitations", bytes.NewBufferString(`{"email":"existing@example.com","role_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1}))
	rr := httptest.NewRecorder()
	resource.createInvitation(rr, req)

	require.Equal(t, http.StatusConflict, rr.Code)
	body := decodeJSONBody(t, rr)
	assert.Equal(t, "error", body["status"])
	assert.Equal(t, "ACCOUNT_ALREADY_HAS_TENANT_ACCESS", body["code"])
}

func TestInvitationHandlers_ValidateAndAccept(t *testing.T) {
	t.Parallel()

	service := &invitationsStub{
		validateSchool: func(_ context.Context, token string) (identityaccess.InvitationPreview, error) {
			assert.Equal(t, "abc123", token)
			return identityaccess.InvitationPreview{
				Email: "invitee@example.com", RoleName: "admin", ExpiresAt: time.Now().Add(time.Hour),
			}, nil
		},
		acceptSchool: func(_ context.Context, token string, registration identityaccess.InvitationRegistration) (identityaccess.Account, error) {
			assert.Equal(t, "abc123", token)
			assert.Equal(t, "Grace", registration.FirstName)
			return identityaccess.Account{ID: 77, Email: "invitee@example.com"}, nil
		},
	}
	resource := NewResource(nil, service, nil, nil, nil)

	validateReq := httptest.NewRequest(http.MethodGet, "/auth/invitations/abc123", nil)
	validateCtx := chi.NewRouteContext()
	validateCtx.URLParams.Add("token", "abc123")
	validateReq = validateReq.WithContext(context.WithValue(validateReq.Context(), chi.RouteCtxKey, validateCtx))
	validateRR := httptest.NewRecorder()
	resource.validateInvitation(validateRR, validateReq)
	assert.Equal(t, http.StatusOK, validateRR.Code)

	acceptReq := httptest.NewRequest(http.MethodPost, "/auth/invitations/abc123/accept", bytes.NewBufferString(`{"first_name":"Grace","last_name":"Hopper","password":"Password123!","confirm_password":"Password123!"}`))
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptCtx := chi.NewRouteContext()
	acceptCtx.URLParams.Add("token", "abc123")
	acceptReq = acceptReq.WithContext(context.WithValue(acceptReq.Context(), chi.RouteCtxKey, acceptCtx))
	acceptRR := httptest.NewRecorder()
	resource.acceptInvitation(acceptRR, acceptReq)
	assert.Equal(t, http.StatusCreated, acceptRR.Code)

	// Verify response includes account_id and email but no tenant_subdomain (no SchoolRepo)
	var acceptResp AcceptInvitationResponse
	err := json.Unmarshal([]byte(extractDataJSON(t, acceptRR.Body.Bytes())), &acceptResp)
	assert.NoError(t, err)
	assert.Equal(t, int64(77), acceptResp.AccountID)
	assert.Equal(t, "invitee@example.com", acceptResp.Email)
	assert.Empty(t, acceptResp.TenantSubdomain, "tenant_subdomain should be empty when SchoolRepo is nil")
}

// extractDataJSON extracts the "data" field from the standard API response envelope.
func extractDataJSON(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		// If no envelope, return raw body
		return string(body)
	}
	return string(envelope.Data)
}

func TestInvitationHandlerHelpersAndErrors(t *testing.T) {
	t.Parallel()

	assert.Equal(t, string(email.DeliveryStatusPending), deriveDeliveryStatus(nil, nil))
	errText := "failed"
	assert.Equal(t, string(email.DeliveryStatusFailed), deriveDeliveryStatus(nil, &errText))
	now := time.Now()
	assert.Equal(t, string(email.DeliveryStatusSent), deriveDeliveryStatus(&now, &errText))
	assert.Equal(t, int64(0), invitationCreatedByValue(nil))
	assert.Equal(t, int64(4), invitationCreatedByValue(ptr64(4)))

	resource := NewResource(nil, nil, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/auth/invitations/token", nil)
	rr := httptest.NewRecorder()
	resource.validateInvitation(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	errService := &invitationsStub{
		validateSchool: func(context.Context, string) (identityaccess.InvitationPreview, error) {
			return identityaccess.InvitationPreview{}, errors.New("db fail")
		},
		acceptSchool: func(context.Context, string, identityaccess.InvitationRegistration) (identityaccess.Account, error) {
			return identityaccess.Account{}, identityaccess.ErrInvitationPasswordMismatch
		},
	}
	resource = NewResource(nil, errService, nil, nil, nil)

	validateCtx := chi.NewRouteContext()
	validateCtx.URLParams.Add("token", "boom")
	req = httptest.NewRequest(http.MethodGet, "/auth/invitations/boom", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, validateCtx))
	rr = httptest.NewRecorder()
	resource.validateInvitation(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	acceptCtx := chi.NewRouteContext()
	acceptCtx.URLParams.Add("token", "boom")
	acceptReq := httptest.NewRequest(http.MethodPost, "/auth/invitations/boom/accept", bytes.NewBufferString(`{"password":"a","confirm_password":"b"}`))
	acceptReq.Header.Set("Content-Type", "application/json")
	acceptReq = acceptReq.WithContext(context.WithValue(acceptReq.Context(), chi.RouteCtxKey, acceptCtx))
	acceptRR := httptest.NewRecorder()
	resource.acceptInvitation(acceptRR, acceptReq)
	assert.Equal(t, http.StatusBadRequest, acceptRR.Code)
}

func ptr64(v int64) *int64 { return &v }
