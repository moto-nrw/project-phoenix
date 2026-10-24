package account

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
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	findFormer     func(ctx context.Context, email string) (identityaccess.FormerEmployment, bool, error)
}

func (s *invitationsStub) FindFormerEmployment(ctx context.Context, email string) (identityaccess.FormerEmployment, bool, error) {
	return s.findFormer(ctx, email)
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

	resource := NewResource(nil, service, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

	req := httptest.NewRequest(http.MethodPost, "/auth/invitations", bytes.NewBufferString(`{"email":" INVITEE@EXAMPLE.COM ","role_id":"9007199254740993","first_name":" Ada ","last_name":" Lovelace ","position":" Principal "}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(testutil.WithAuthenticatedContext(req.Context(), testutil.Claims{ID: 44}, nil))
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

	resource := NewResource(nil, service, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

	req := httptest.NewRequest(http.MethodPost, "/auth/invitations", bytes.NewBufferString(`{"email":"existing@example.com","role_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(testutil.WithAuthenticatedContext(req.Context(), testutil.Claims{ID: 1}, nil))
	rr := httptest.NewRecorder()
	resource.createInvitation(rr, req)

	require.Equal(t, http.StatusConflict, rr.Code)
	body := decodeJSONBody(t, rr)
	assert.Equal(t, float64(rr.Code), body["status"])
	assert.Equal(t, "identity.account_already_has_tenant_access", body["code"])
}

func TestInvitationHandlers_CreateInvitation_EmailAlreadyExistsNamesTheField(t *testing.T) {
	t.Parallel()

	service := &invitationsStub{
		createSchool: func(context.Context, identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error) {
			return identityaccess.SchoolInvitation{}, &identityaccess.AuthenticationError{
				Op: "create invitation", Err: identityaccess.ErrEmailAlreadyExists,
			}
		},
	}
	resource := NewResource(nil, service, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

	req := httptest.NewRequest(http.MethodPost, "/auth/invitations", bytes.NewBufferString(`{"email":"existing@example.com","role_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(testutil.WithAuthenticatedContext(req.Context(), testutil.Claims{ID: 1}, nil))
	rr := httptest.NewRecorder()
	resource.createInvitation(rr, req)

	require.Equal(t, http.StatusConflict, rr.Code)
	body := decodeJSONBody(t, rr)
	assert.Equal(t, "identity.email_already_exists", body["code"])
	errs := body["errors"].([]any)
	require.Len(t, errs, 1)
	assert.Equal(t, "email", errs[0].(map[string]any)["field"])
}

// Every blank required field comes back as its own entry, so the form marks
// all of them in one attempt (#2511).
func TestInvitationHandlers_CreateInvitation_BlankFieldsComeBackAsFieldErrors(t *testing.T) {
	t.Parallel()

	resource := NewResource(nil, &invitationsStub{}, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

	req := httptest.NewRequest(http.MethodPost, "/auth/invitations", bytes.NewBufferString(`{"email":"","role_id":null}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(testutil.WithAuthenticatedContext(req.Context(), testutil.Claims{ID: 1}, nil))
	rr := httptest.NewRecorder()
	resource.createInvitation(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	body := decodeJSONBody(t, rr)
	assert.Equal(t, "general.input", body["code"])
	errs := body["errors"].([]any)
	fields := make([]string, 0, len(errs))
	for _, entry := range errs {
		fields = append(fields, entry.(map[string]any)["field"].(string))
	}
	assert.Equal(t, []string{"email", "role_id"}, fields)
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
	resource := NewResource(nil, service, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

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

	assert.Equal(t, "pending", deriveDeliveryStatus(nil, nil))
	errText := "failed"
	assert.Equal(t, "failed", deriveDeliveryStatus(nil, &errText))
	now := time.Now()
	assert.Equal(t, "sent", deriveDeliveryStatus(&now, &errText))
	assert.Equal(t, int64(0), invitationCreatedByValue(nil))
	assert.Equal(t, int64(4), invitationCreatedByValue(ptr64(4)))

	resource := NewResource(nil, nil, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

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
	resource = NewResource(nil, errService, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

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

func TestInvitationHandlers_FindFormerEmployment(t *testing.T) {
	t.Parallel()

	var asked string
	service := &invitationsStub{
		findFormer: func(_ context.Context, email string) (identityaccess.FormerEmployment, bool, error) {
			asked = email
			return identityaccess.FormerEmployment{
				StartedOn:    calendar.NewDate(2024, time.August, 1),
				EndedOn:      calendar.NewDate(2026, time.July, 31),
				Resumes:      []identityaccess.FormerEmploymentPart{identityaccess.FormerEmploymentMasterData},
				NotReturning: []identityaccess.FormerEmploymentPart{identityaccess.FormerEmploymentStaffDocuments},
			}, true, nil
		},
	}
	resource := NewResource(nil, service, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

	rr := postFormerEmployment(resource, `{"email":" Former@Example.com "}`)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "former@example.com", asked)
	data := decodeJSONBody(t, rr)["data"].(map[string]any)
	assert.Equal(t, map[string]any{
		"started_on":    "2024-08-01",
		"ended_on":      "2026-07-31",
		"resumes":       []any{"master_data"},
		"not_returning": []any{"staff_documents"},
	}, data["former_employment"])
}

// Without a former employment the answer carries nothing, whether or not an
// account exists for the address.
func TestInvitationHandlers_FindFormerEmployment_NoneIsNull(t *testing.T) {
	t.Parallel()

	service := &invitationsStub{
		findFormer: func(context.Context, string) (identityaccess.FormerEmployment, bool, error) {
			return identityaccess.FormerEmployment{}, false, nil
		},
	}
	resource := NewResource(nil, service, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

	rr := postFormerEmployment(resource, `{"email":"new@example.com"}`)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	data := decodeJSONBody(t, rr)["data"].(map[string]any)
	assert.Contains(t, data, "former_employment")
	assert.Nil(t, data["former_employment"])
}

func TestInvitationHandlers_FindFormerEmployment_InvalidEmail(t *testing.T) {
	t.Parallel()

	resource := NewResource(nil, &invitationsStub{}, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{})

	rr := postFormerEmployment(resource, `{"email":"not-an-address"}`)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	errs := decodeJSONBody(t, rr)["errors"].([]any)
	require.Len(t, errs, 1)
	assert.Equal(t, "email", errs[0].(map[string]any)["field"])
}

func postFormerEmployment(resource *Resource, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/former-employment", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(testutil.WithAuthenticatedContext(req.Context(), testutil.Claims{ID: 1}, nil))
	rr := httptest.NewRecorder()
	resource.findFormerEmployment(rr, req)
	return rr
}

func TestInvitationEmploymentActionWireContract(t *testing.T) {
	t.Parallel()
	for _, action := range []identityaccess.EmploymentAction{"", identityaccess.EmploymentActionResume, identityaccess.EmploymentActionNew, "unknown"} {
		req := CreateInvitationRequest{Email: "invitee@example.com", RoleID: 42, EmploymentAction: action}
		err := req.Bind(httptest.NewRequest(http.MethodPost, "/auth/invitations", nil))
		if action == "unknown" {
			require.Error(t, err)
			require.Contains(t, err.Error(), "employment_action")
			continue
		}
		require.NoError(t, err)
		resource := &Resource{}
		request := httptest.NewRequest(http.MethodPost, "/auth/invitations", nil)
		mapped := resource.buildInvitationRequest(request, &req, testutil.DefaultTestClaims())
		require.Equal(t, action, mapped.EmploymentAction)
	}
	// The public validation response must not expose the school's decision.
	body, err := json.Marshal(toInvitationValidationResponse(identityaccess.InvitationPreview{}))
	require.NoError(t, err)
	require.NotContains(t, string(body), "employment_action")
	body, err = json.Marshal(toInvitationResponse(identityaccess.SchoolInvitation{EmploymentAction: identityaccess.EmploymentActionNew}))
	require.NoError(t, err)
	require.Contains(t, string(body), `"employment_action":"new"`)
}

func TestInvitationResetWireContract(t *testing.T) {
	t.Parallel()
	for _, action := range []identityaccess.EmploymentAction{"", identityaccess.EmploymentActionResume, identityaccess.EmploymentActionNew} {
		req := CreateInvitationRequest{Email: "return@example.test", RoleID: 42, EmploymentAction: action, ResetMasterData: true, ResetQualifications: true, ResetWorkTime: true}
		err := req.Bind(httptest.NewRequest(http.MethodPost, "/auth/invitations", nil))
		if action == identityaccess.EmploymentActionNew {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		mapped := (&Resource{}).buildInvitationRequest(httptest.NewRequest(http.MethodPost, "/auth/invitations", nil), &req, testutil.DefaultTestClaims())
		require.True(t, mapped.ResetMasterData)
		require.True(t, mapped.ResetQualifications)
		require.True(t, mapped.ResetWorkTime)
	}
	body, err := json.Marshal(toInvitationValidationResponse(identityaccess.InvitationPreview{}))
	require.NoError(t, err)
	require.NotContains(t, string(body), "reset_")
}

// Unexpected shell reads panic in invitation-only tests.
type unusedTenantSettings struct{ settings.TenantReader }
