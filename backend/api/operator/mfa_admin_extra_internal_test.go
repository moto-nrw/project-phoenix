package operator

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

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
)

// stubTenantMFAService satisfies authSvc.MFAService minimally for the
// operator admin endpoints (HasEnrollment, GetTenantMFAOverride,
// OperatorAdminDisable, OperatorSetMFAOverride, etc.).
type stubTenantMFAService struct {
	accountBelongsFn       func(ctx context.Context, accountID, tenantID int64) (bool, error)
	hasEnrollmentFn        func(ctx context.Context, accountID int64) (bool, error)
	getTenantOverrideFn    func(ctx context.Context, accountID, tenantID int64) (string, error)
	getGlobalOverrideFn    func(ctx context.Context, accountID int64) (string, error)
	operatorAdminDisableFn func(ctx context.Context, operatorID, schoolID, accountID int64, reason string) error
	operatorSetOverrideFn  func(ctx context.Context, operatorID, schoolID, accountID int64, override, reason string) error
	operatorSetGlobalFn    func(ctx context.Context, operatorID, accountID int64, override, reason string) error
}

func (s *stubTenantMFAService) AccountBelongsToTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if s.accountBelongsFn != nil {
		return s.accountBelongsFn(ctx, accountID, tenantID)
	}
	return true, nil
}

func (s *stubTenantMFAService) IsRequired(context.Context, *authModels.Account, int64) (bool, error) {
	return false, nil
}
func (s *stubTenantMFAService) HasEnrollment(ctx context.Context, accountID int64) (bool, error) {
	if s.hasEnrollmentFn != nil {
		return s.hasEnrollmentFn(ctx, accountID)
	}
	return false, nil
}
func (s *stubTenantMFAService) StartChallenge(context.Context, int64, int64, string, net.IP) (string, error) {
	return "", nil
}
func (s *stubTenantMFAService) VerifyChallenge(context.Context, string, string) (*authSvc.VerifiedChallenge, error) {
	return nil, nil
}
func (s *stubTenantMFAService) ResendChallenge(context.Context, string, net.IP) (string, error) {
	return "", nil
}
func (s *stubTenantMFAService) VerifyCodeForAccount(context.Context, int64, string) error {
	return nil
}
func (s *stubTenantMFAService) Enroll(context.Context, int64) error  { return nil }
func (s *stubTenantMFAService) Disable(context.Context, int64) error { return nil }
func (s *stubTenantMFAService) IssueTrustedDevice(context.Context, int64, int64, string, net.IP) (string, time.Time, error) {
	return "", time.Time{}, nil
}
func (s *stubTenantMFAService) VerifyTrustedDevice(context.Context, int64, int64, string) (bool, error) {
	return false, nil
}
func (s *stubTenantMFAService) ListTrustedDevices(context.Context, int64, int64) ([]*authModels.MFATrustedDevice, error) {
	return nil, nil
}
func (s *stubTenantMFAService) RevokeTrustedDevice(context.Context, int64, int64, int64) error {
	return nil
}
func (s *stubTenantMFAService) IsTrustedDeviceEnabled(context.Context, int64) bool { return true }
func (s *stubTenantMFAService) TrustedDeviceDays(context.Context, int64) int       { return 90 }
func (s *stubTenantMFAService) SetMFAOverride(context.Context, int64, int64, int64, string, string, []string) error {
	return nil
}
func (s *stubTenantMFAService) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, accountID int64, override, reason string) error {
	if s.operatorSetOverrideFn != nil {
		return s.operatorSetOverrideFn(ctx, operatorID, schoolID, accountID, override, reason)
	}
	return nil
}
func (s *stubTenantMFAService) AdminDisable(context.Context, int64, int64, int64, string, []string) error {
	return nil
}
func (s *stubTenantMFAService) OperatorAdminDisable(ctx context.Context, operatorID, schoolID, accountID int64, reason string) error {
	if s.operatorAdminDisableFn != nil {
		return s.operatorAdminDisableFn(ctx, operatorID, schoolID, accountID, reason)
	}
	return nil
}
func (s *stubTenantMFAService) GetAdminState(context.Context, int64, int64, int64, []string) (authSvc.MFAAdminState, error) {
	return authSvc.MFAAdminState{}, nil
}
func (s *stubTenantMFAService) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	if s.getTenantOverrideFn != nil {
		return s.getTenantOverrideFn(ctx, accountID, tenantID)
	}
	return authSvc.MFAAdminOverrideNone, nil
}
func (s *stubTenantMFAService) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	if s.getGlobalOverrideFn != nil {
		return s.getGlobalOverrideFn(ctx, accountID)
	}
	return authSvc.MFAAdminOverrideNone, nil
}
func (s *stubTenantMFAService) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, accountID int64, override, reason string) error {
	if s.operatorSetGlobalFn != nil {
		return s.operatorSetGlobalFn(ctx, operatorID, accountID, override, reason)
	}
	return nil
}
func (s *stubTenantMFAService) MaskEmailForChallenge(string) string { return "" }
func (s *stubTenantMFAService) CleanupExpiredChallenges(context.Context, time.Duration) (int64, error) {
	return 0, nil
}

var _ authSvc.MFAService = (*stubTenantMFAService)(nil)

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

func provisioningResourceFor(mfa authSvc.MFAService) *ProvisioningResource {
	return &ProvisioningResource{
		TenantMFAService: mfa,
	}
}

// --- tests -------------------------------------------------------------

func TestGetSchoolAccountMFAState_HappyPath(t *testing.T) {
	mfa := &stubTenantMFAService{
		hasEnrollmentFn:     func(context.Context, int64) (bool, error) { return true, nil },
		getTenantOverrideFn: func(context.Context, int64, int64) (string, error) { return authSvc.MFAAdminOverrideForceOn, nil },
	}
	mfa.accountBelongsFn = func(context.Context, int64, int64) (bool, error) { return true, nil }
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodGet, "10", "200", nil, 7)
	rr := httptest.NewRecorder()
	rs.GetSchoolAccountMFAState(rr, r)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestGetSchoolAccountMFAState_AccountNotInSchool_Returns404(t *testing.T) {
	mfa := &stubTenantMFAService{}
	mfa.accountBelongsFn = func(context.Context, int64, int64) (bool, error) { return false, nil }
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodGet, "10", "200", nil, 7)
	rr := httptest.NewRecorder()
	rs.GetSchoolAccountMFAState(rr, r)

	assert.Equal(t, http.StatusNotFound, rr.Code,
		"the cross-tenant guard must reject lookups for accounts that aren't members of the school")
}

func TestGetSchoolAccountMFAState_MembershipLookupErrorMapsTo500(t *testing.T) {
	mfa := &stubTenantMFAService{}
	mfa.accountBelongsFn = func(context.Context, int64, int64) (bool, error) { return false, errors.New("db down") }
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodGet, "10", "200", nil, 7)
	rr := httptest.NewRecorder()
	rs.GetSchoolAccountMFAState(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestResetSchoolAccountMFA_HappyPath(t *testing.T) {
	var capturedReason string
	mfa := &stubTenantMFAService{
		operatorAdminDisableFn: func(_ context.Context, _, _, _ int64, reason string) error {
			capturedReason = reason
			return nil
		},
	}
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodDelete, "10", "200",
		MFAAdminResetRequest{Reason: "User left organization"}, 7)
	rr := httptest.NewRecorder()
	rs.ResetSchoolAccountMFA(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "User left organization", capturedReason)
}

func TestResetSchoolAccountMFA_RequiresOperatorClaim(t *testing.T) {
	mfa := &stubTenantMFAService{}
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodDelete, "10", "200",
		MFAAdminResetRequest{Reason: "ok ok ok"}, 0) // no claim
	rr := httptest.NewRecorder()
	rs.ResetSchoolAccountMFA(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestResetSchoolAccountMFA_BadJSONRequest(t *testing.T) {
	mfa := &stubTenantMFAService{}
	rs := provisioningResourceFor(mfa)

	// Empty reason fails Bind validation
	r := reqWithSchoolAccount(t, http.MethodDelete, "10", "200",
		MFAAdminResetRequest{Reason: ""}, 7)
	rr := httptest.NewRecorder()
	rs.ResetSchoolAccountMFA(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestResetSchoolAccountMFA_PermissionDenied_Returns403(t *testing.T) {
	mfa := &stubTenantMFAService{
		operatorAdminDisableFn: func(context.Context, int64, int64, int64, string) error {
			return authSvc.ErrMFAPermissionDenied
		},
	}
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodDelete, "10", "200",
		MFAAdminResetRequest{Reason: "User left"}, 7)
	rr := httptest.NewRecorder()
	rs.ResetSchoolAccountMFA(rr, r)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestSetSchoolAccountMFAOverride_HappyPath(t *testing.T) {
	var capturedOverride string
	mfa := &stubTenantMFAService{
		operatorSetOverrideFn: func(_ context.Context, _, _, _ int64, override, _ string) error {
			capturedOverride = override
			return nil
		},
	}
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodPut, "10", "200",
		MFAAdminOverrideSetRequest{
			Override: authSvc.MFAAdminOverrideForceOff,
			Reason:   "Compromise reported",
		}, 7)
	rr := httptest.NewRecorder()
	rs.SetSchoolAccountMFAOverride(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, authSvc.MFAAdminOverrideForceOff, capturedOverride)
}

func TestSetSchoolAccountMFAOverride_RejectsBadOverride(t *testing.T) {
	mfa := &stubTenantMFAService{}
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodPut, "10", "200",
		MFAAdminOverrideSetRequest{Override: "bogus", Reason: "ok ok"}, 7)
	rr := httptest.NewRecorder()
	rs.SetSchoolAccountMFAOverride(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSetSchoolAccountMFAOverride_InvalidOverrideFromService(t *testing.T) {
	mfa := &stubTenantMFAService{
		operatorSetOverrideFn: func(context.Context, int64, int64, int64, string, string) error {
			return authSvc.ErrMFAInvalidOverride
		},
	}
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodPut, "10", "200",
		MFAAdminOverrideSetRequest{Override: authSvc.MFAAdminOverrideForceOff, Reason: "ok ok"}, 7)
	rr := httptest.NewRecorder()
	rs.SetSchoolAccountMFAOverride(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSetSchoolAccountMFAOverride_PermissionDenied(t *testing.T) {
	mfa := &stubTenantMFAService{
		operatorSetOverrideFn: func(context.Context, int64, int64, int64, string, string) error {
			return authSvc.ErrMFAPermissionDenied
		},
	}
	rs := provisioningResourceFor(mfa)

	r := reqWithSchoolAccount(t, http.MethodPut, "10", "200",
		MFAAdminOverrideSetRequest{Override: authSvc.MFAAdminOverrideForceOn, Reason: "ok ok"}, 7)
	rr := httptest.NewRecorder()
	rs.SetSchoolAccountMFAOverride(rr, r)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestMFAAdminResetRequest_BindRejectsShortReason(t *testing.T) {
	req := &MFAAdminResetRequest{Reason: "no"}
	require.Error(t, req.Bind(nil))
}

func TestMFAAdminOverrideSetRequest_Trim(t *testing.T) {
	req := &MFAAdminOverrideSetRequest{
		Override: "  force_off  ",
		Reason:   "   compromise   ",
	}
	require.NoError(t, req.Bind(nil))
	assert.Equal(t, "force_off", req.Override)
	assert.Equal(t, "compromise", req.Reason)
}
