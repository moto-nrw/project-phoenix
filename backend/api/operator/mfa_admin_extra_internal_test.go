package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// newStubTenantMFAService returns an MFAServiceMock pre-seeded
// with the "safe default" behaviors the operator admin endpoint tests
// (HasEnrollment, GetTenantMFAOverride, OperatorAdminDisable,
// OperatorSetMFAOverride, etc.) have always relied on when they don't
// override a specific Fn — e.g. AccountBelongsToTenant defaulting to true
// and the MFA overrides defaulting to identityoperator.MFAAdminOverrideNone rather
// than an empty string. Tests mutate the returned Fn fields directly.
func newStubTenantMFAService() *MFAServiceMock {
	return &MFAServiceMock{
		AccountBelongsToSchoolFn: func(context.Context, int64, int64) (bool, error) { return true, nil },
		IsTrustedDeviceEnabledFn: func(context.Context, int64) bool { return true },
		TrustedDeviceDaysFn:      func(context.Context, int64) int { return 90 },
		GetTenantMFAOverrideFn: func(context.Context, int64, int64) (string, error) {
			return identityoperator.MFAAdminOverrideNone, nil
		},
		GetGlobalMFAOverrideFn: func(context.Context, int64) (string, error) {
			return identityoperator.MFAAdminOverrideNone, nil
		},
	}
}

// --- helpers -----------------------------------------------------------

func reqWithSchoolAccount(t *testing.T, method, schoolID, accountID string, body any, operatorID int) *http.Request {
	t.Helper()
	var rdr *bytes.Reader
	if body == nil {
		rdr = bytes.NewReader([]byte("{}"))
	} else {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, "/operator/schools/x/accounts/y/mfa", rdr)
	r.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", schoolID)
	rctx.URLParams.Add("accountId", accountID)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	if operatorID > 0 {
		ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: operatorID})
	}
	return r.WithContext(ctx)
}

func mfaAdminResourceFor(mfa identityoperator.AccountMFA) *SchoolAccountMFAResource {
	return &SchoolAccountMFAResource{
		TenantMFAService: mfa,
	}
}

// --- tests -------------------------------------------------------------

func TestGetSchoolAccountMFAState_HappyPath(t *testing.T) {
	t.Parallel()

	mfa := newStubTenantMFAService()
	mfa.HasMFAEnrollmentFn = func(context.Context, int64) (bool, error) { return true, nil }
	mfa.GetTenantMFAOverrideFn = func(context.Context, int64, int64) (string, error) {
		return identityoperator.MFAAdminOverrideForceOn, nil
	}
	mfa.AccountBelongsToSchoolFn = func(context.Context, int64, int64) (bool, error) { return true, nil }
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodGet, "10", "200", nil, 7)
	rr := httptest.NewRecorder()
	rs.GetSchoolAccountMFAState(rr, r)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestGetSchoolAccountMFAState_AccountNotInSchool_Returns404(t *testing.T) {
	t.Parallel()

	mfa := newStubTenantMFAService()
	mfa.AccountBelongsToSchoolFn = func(context.Context, int64, int64) (bool, error) { return false, nil }
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodGet, "10", "200", nil, 7)
	rr := httptest.NewRecorder()
	rs.GetSchoolAccountMFAState(rr, r)

	assert.Equal(t, http.StatusNotFound, rr.Code,
		"the cross-tenant guard must reject lookups for accounts that aren't members of the school")
}

func TestGetSchoolAccountMFAState_MembershipLookupErrorMapsTo500(t *testing.T) {
	t.Parallel()

	mfa := newStubTenantMFAService()
	mfa.AccountBelongsToSchoolFn = func(context.Context, int64, int64) (bool, error) { return false, errors.New("db down") }
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodGet, "10", "200", nil, 7)
	rr := httptest.NewRecorder()
	rs.GetSchoolAccountMFAState(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestResetSchoolAccountMFA_HappyPath(t *testing.T) {
	t.Parallel()

	var capturedReason string
	mfa := newStubTenantMFAService()
	mfa.OperatorDisableMFAFn = func(_ context.Context, _, _, _ int64, reason string) error {
		capturedReason = reason
		return nil
	}
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodDelete, "10", "200",
		MFAAdminResetRequest{Reason: "User left organization"}, 7)
	rr := httptest.NewRecorder()
	rs.ResetSchoolAccountMFA(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "User left organization", capturedReason)
}

func TestResetSchoolAccountMFA_RequiresOperatorClaim(t *testing.T) {
	t.Parallel()

	mfa := newStubTenantMFAService()
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodDelete, "10", "200",
		MFAAdminResetRequest{Reason: "ok ok ok"}, 0) // no claim
	rr := httptest.NewRecorder()
	rs.ResetSchoolAccountMFA(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestResetSchoolAccountMFA_BadJSONRequest(t *testing.T) {
	t.Parallel()

	mfa := newStubTenantMFAService()
	rs := mfaAdminResourceFor(mfa)

	// Empty reason fails Bind validation
	r := reqWithSchoolAccount(t, http.MethodDelete, "10", "200",
		MFAAdminResetRequest{Reason: ""}, 7)
	rr := httptest.NewRecorder()
	rs.ResetSchoolAccountMFA(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestResetSchoolAccountMFA_PermissionDenied_Returns403(t *testing.T) {
	t.Parallel()

	mfa := newStubTenantMFAService()
	mfa.OperatorDisableMFAFn = func(context.Context, int64, int64, int64, string) error {
		return identityoperator.ErrMFAPermissionDenied
	}
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodDelete, "10", "200",
		MFAAdminResetRequest{Reason: "User left"}, 7)
	rr := httptest.NewRecorder()
	rs.ResetSchoolAccountMFA(rr, r)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestSetSchoolAccountMFAOverride_HappyPath(t *testing.T) {
	t.Parallel()

	var capturedOverride string
	mfa := newStubTenantMFAService()
	mfa.OperatorSetMFAOverrideFn = func(_ context.Context, _, _, _ int64, override, _ string) error {
		capturedOverride = override
		return nil
	}
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodPut, "10", "200",
		MFAAdminOverrideSetRequest{
			Override: identityoperator.MFAAdminOverrideForceOff,
			Reason:   "Compromise reported",
		}, 7)
	rr := httptest.NewRecorder()
	rs.SetSchoolAccountMFAOverride(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, identityoperator.MFAAdminOverrideForceOff, capturedOverride)
}

func TestSetSchoolAccountMFAOverride_RejectsBadOverride(t *testing.T) {
	t.Parallel()

	mfa := newStubTenantMFAService()
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodPut, "10", "200",
		MFAAdminOverrideSetRequest{Override: "bogus", Reason: "ok ok"}, 7)
	rr := httptest.NewRecorder()
	rs.SetSchoolAccountMFAOverride(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSetSchoolAccountMFAOverride_InvalidOverrideFromService(t *testing.T) {
	t.Parallel()

	mfa := newStubTenantMFAService()
	mfa.OperatorSetMFAOverrideFn = func(context.Context, int64, int64, int64, string, string) error {
		return identityoperator.ErrMFAInvalidOverride
	}
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodPut, "10", "200",
		MFAAdminOverrideSetRequest{Override: identityoperator.MFAAdminOverrideForceOff, Reason: "ok ok"}, 7)
	rr := httptest.NewRecorder()
	rs.SetSchoolAccountMFAOverride(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSetSchoolAccountMFAOverride_PermissionDenied(t *testing.T) {
	t.Parallel()

	mfa := newStubTenantMFAService()
	mfa.OperatorSetMFAOverrideFn = func(context.Context, int64, int64, int64, string, string) error {
		return identityoperator.ErrMFAPermissionDenied
	}
	rs := mfaAdminResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodPut, "10", "200",
		MFAAdminOverrideSetRequest{Override: identityoperator.MFAAdminOverrideForceOn, Reason: "ok ok"}, 7)
	rr := httptest.NewRecorder()
	rs.SetSchoolAccountMFAOverride(rr, r)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMFAAdminResetRequest_BindRejectsShortReason(t *testing.T) {
	t.Parallel()

	req := &MFAAdminResetRequest{Reason: "no"}
	require.Error(t, req.Bind(nil))
}

func TestMFAAdminOverrideSetRequest_Trim(t *testing.T) {
	t.Parallel()

	req := &MFAAdminOverrideSetRequest{
		Override: "  force_off  ",
		Reason:   "   compromise   ",
	}
	require.NoError(t, req.Bind(nil))
	assert.Equal(t, "force_off", req.Override)
	assert.Equal(t, "compromise", req.Reason)
}
