package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// adminAuthStub satisfies authService.AuthService via interface embed —
// only the methods the admin handlers reach are implemented (GetAccountByID).
// Everything else panics on nil-interface dereference, signalling a drift
// in the handler.
type adminAuthStub struct {
	authService.AuthService
	getAccountByIDFn func(ctx context.Context, id int) (*authModels.Account, error)
}

func (s *adminAuthStub) GetAccountByID(ctx context.Context, id int) (*authModels.Account, error) {
	if s.getAccountByIDFn != nil {
		return s.getAccountByIDFn(ctx, id)
	}
	return nil, nil
}

// adminMFAStub is a focused MFAService stub for the admin handlers.
// Methods used by the handlers under test (HasEnrollment, SetMFAOverride,
// AdminDisable) accept per-test overrides; the rest default to no-op.
//
// SetMFAOverride / AdminDisable carry an actorTenantID since the #1430
// Item #2 cross-tenant guard — tests can capture it via the per-method
// override and assert the handler is forwarding the JWT tenant claim.
type adminMFAStub struct {
	stubMFAService  // reuse the broader stub from mfa_handlers_extra_internal_test.go
	hasEnrollmentFn func(ctx context.Context, accountID int64) (bool, error)
	setOverrideFn   func(ctx context.Context, actorID, actorTenantID, targetID int64, override, reason string, perms []string) error
	adminDisableFn  func(ctx context.Context, actorID, actorTenantID, targetID int64, reason string, perms []string) error
}

func (s *adminMFAStub) HasEnrollment(ctx context.Context, accountID int64) (bool, error) {
	if s.hasEnrollmentFn != nil {
		return s.hasEnrollmentFn(ctx, accountID)
	}
	return false, nil
}

func (s *adminMFAStub) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetID int64, override, reason string, perms []string) error {
	if s.setOverrideFn != nil {
		return s.setOverrideFn(ctx, actorID, actorTenantID, targetID, override, reason, perms)
	}
	return nil
}

func (s *adminMFAStub) AdminDisable(ctx context.Context, actorID, actorTenantID, targetID int64, reason string, perms []string) error {
	if s.adminDisableFn != nil {
		return s.adminDisableFn(ctx, actorID, actorTenantID, targetID, reason, perms)
	}
	return nil
}

// withActorTenant overrides the TenantID on the AppClaims stored on the
// context. Used by Item-#2 tests that need to assert the handler forwards
// the actor's tenant claim to the service-layer cross-tenant guard.
func withActorTenant(r *http.Request, tenantID int64) *http.Request {
	claims, _ := r.Context().Value(jwt.CtxClaims).(jwt.AppClaims)
	claims.TenantID = tenantID
	return r.WithContext(context.WithValue(r.Context(), jwt.CtxClaims, claims))
}

// reqWithAccountID constructs a request with the {accountId} URL param
// pre-populated in chi's route context plus optional claims, so the
// admin handlers can extract everything they need without a real router.
func reqWithAccountID(t *testing.T, method, accountIDParam string, body any, claimID int, perms []string) *http.Request {
	t.Helper()
	var rdr *bytes.Reader
	if body == nil {
		rdr = bytes.NewReader([]byte("{}"))
	} else {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, "/admin/mfa", rdr)
	r.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountId", accountIDParam)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	if claimID > 0 {
		ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{
			ID:          claimID,
			Permissions: perms,
		})
	}
	return r.WithContext(ctx)
}

func TestMFAAdminGetState_HappyPath(t *testing.T) {
	rs := &Resource{
		MFAService: &adminMFAStub{
			hasEnrollmentFn: func(context.Context, int64) (bool, error) { return true, nil },
		},
		AuthService: &adminAuthStub{
			getAccountByIDFn: func(context.Context, int) (*authModels.Account, error) {
				return &authModels.Account{MFAAdminOverride: authService.MFAAdminOverrideForceOff}, nil
			},
		},
	}

	r := reqWithAccountID(t, http.MethodGet, "100", nil, 0, nil)
	rr := httptest.NewRecorder()
	rs.mfaAdminGetState(rr, r)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp MFAAdminStateResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.True(t, resp.Enrolled)
	assert.Equal(t, authService.MFAAdminOverrideForceOff, resp.Override)
}

func TestMFAAdminGetState_BadIDReturns400(t *testing.T) {
	rs := &Resource{
		MFAService:  &adminMFAStub{},
		AuthService: &adminAuthStub{},
	}

	r := reqWithAccountID(t, http.MethodGet, "not-a-number", nil, 0, nil)
	rr := httptest.NewRecorder()
	rs.mfaAdminGetState(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAAdminGetState_DefaultsOverrideToNoneWhenAccountMissing(t *testing.T) {
	rs := &Resource{
		MFAService: &adminMFAStub{
			hasEnrollmentFn: func(context.Context, int64) (bool, error) { return false, nil },
		},
		AuthService: &adminAuthStub{
			getAccountByIDFn: func(context.Context, int) (*authModels.Account, error) {
				return nil, errors.New("not found")
			},
		},
	}

	r := reqWithAccountID(t, http.MethodGet, "100", nil, 0, nil)
	rr := httptest.NewRecorder()
	rs.mfaAdminGetState(rr, r)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp MFAAdminStateResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, authService.MFAAdminOverrideNone, resp.Override,
		"missing account row falls back to 'none' override so the modal still renders sanely")
}

func TestMFAAdminSetOverride_HappyPath(t *testing.T) {
	var capturedOverride string
	var capturedTenantID int64
	rs := &Resource{
		MFAService: &adminMFAStub{
			setOverrideFn: func(_ context.Context, _, actorTenantID, _ int64, override, _ string, _ []string) error {
				capturedOverride = override
				capturedTenantID = actorTenantID
				return nil
			},
		},
		AuthService: &adminAuthStub{},
	}

	r := reqWithAccountID(t, http.MethodPut, "200",
		MFAAdminOverrideSetRequest{Override: authService.MFAAdminOverrideForceOff, Reason: "Account compromised"},
		7, []string{"users:manage"})
	// The handler must forward the JWT tenant claim so the service-layer
	// cross-tenant guard (#1430 Item #2) sees the actor's tenant.
	r = withActorTenant(r, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaAdminSetOverride(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, authService.MFAAdminOverrideForceOff, capturedOverride)
	assert.Equal(t, int64(70010001), capturedTenantID,
		"handler must propagate claims.TenantID to the service for the cross-tenant guard")
}

func TestMFAAdminSetOverride_RequiresClaim(t *testing.T) {
	rs := &Resource{
		MFAService:  &adminMFAStub{},
		AuthService: &adminAuthStub{},
	}

	r := reqWithAccountID(t, http.MethodPut, "200",
		MFAAdminOverrideSetRequest{Override: authService.MFAAdminOverrideForceOff, Reason: "ok ok"},
		0, nil) // no claims
	rr := httptest.NewRecorder()
	rs.mfaAdminSetOverride(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAAdminSetOverride_RejectsBadOverride(t *testing.T) {
	rs := &Resource{
		MFAService:  &adminMFAStub{},
		AuthService: &adminAuthStub{},
	}

	r := reqWithAccountID(t, http.MethodPut, "200",
		MFAAdminOverrideSetRequest{Override: "delete-everything", Reason: "ok ok"},
		7, []string{"users:manage"})
	rr := httptest.NewRecorder()
	rs.mfaAdminSetOverride(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAAdminSetOverride_PermissionDeniedMapsTo403(t *testing.T) {
	rs := &Resource{
		MFAService: &adminMFAStub{
			setOverrideFn: func(context.Context, int64, int64, int64, string, string, []string) error {
				return authService.ErrMFAPermissionDenied
			},
		},
		AuthService: &adminAuthStub{},
	}

	r := reqWithAccountID(t, http.MethodPut, "200",
		MFAAdminOverrideSetRequest{Override: authService.MFAAdminOverrideForceOff, Reason: "ok ok"},
		7, nil) // no permission claim
	rr := httptest.NewRecorder()
	rs.mfaAdminSetOverride(rr, r)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMFAAdminDisable_HappyPath(t *testing.T) {
	var capturedReason string
	var capturedTenantID int64
	rs := &Resource{
		MFAService: &adminMFAStub{
			adminDisableFn: func(_ context.Context, _, actorTenantID, _ int64, reason string, _ []string) error {
				capturedReason = reason
				capturedTenantID = actorTenantID
				return nil
			},
		},
	}

	r := reqWithAccountID(t, http.MethodPost, "300",
		MFAAdminOverrideRequest{Reason: "User lost phone"},
		7, []string{"users:manage"})
	r = withActorTenant(r, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaAdminDisable(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "User lost phone", capturedReason)
	assert.Equal(t, int64(70010001), capturedTenantID,
		"handler must propagate claims.TenantID to the service for the cross-tenant guard")
}

func TestMFAAdminDisable_BadID_Returns400(t *testing.T) {
	rs := &Resource{MFAService: &adminMFAStub{}}

	r := reqWithAccountID(t, http.MethodPost, "0",
		MFAAdminOverrideRequest{Reason: "ok ok"},
		7, []string{"users:manage"})
	rr := httptest.NewRecorder()
	rs.mfaAdminDisable(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAAdminDisable_RejectsEmptyReason(t *testing.T) {
	rs := &Resource{MFAService: &adminMFAStub{}}

	r := reqWithAccountID(t, http.MethodPost, "300",
		MFAAdminOverrideRequest{Reason: "   "}, // whitespace only
		7, []string{"users:manage"})
	rr := httptest.NewRecorder()
	rs.mfaAdminDisable(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAAdminDisable_ServiceErrorMapsTo500(t *testing.T) {
	rs := &Resource{MFAService: &adminMFAStub{
		adminDisableFn: func(context.Context, int64, int64, int64, string, []string) error {
			return errors.New("db down")
		},
	}}

	r := reqWithAccountID(t, http.MethodPost, "300",
		MFAAdminOverrideRequest{Reason: "User lost phone"},
		7, []string{"users:manage"})
	rr := httptest.NewRecorder()
	rs.mfaAdminDisable(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// Verify reason-trimming happens before validation so leading/trailing
// whitespace doesn't sneak through as valid input.
func TestMFAAdminOverrideRequest_BindTrimsReason(t *testing.T) {
	req := &MFAAdminOverrideRequest{Reason: "  spaced reason text   "}
	require.NoError(t, req.Bind(nil))
	assert.Equal(t, "spaced reason text", req.Reason)
	assert.False(t, strings.HasPrefix(req.Reason, " "))
}
