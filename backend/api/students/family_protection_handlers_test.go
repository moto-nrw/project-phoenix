package students

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type familyProtectionCapabilityStub struct {
	input   peopleModule.SetFamilyProtection
	current map[int64]bool
	enabled bool
	// setErr is what SetFamilyProtection returns alongside the state, so the
	// unchanged sentinel can be exercised.
	setErr error
}

func (s *familyProtectionCapabilityStub) CurrentFamilyProtection(context.Context, []int64) (map[int64]bool, error) {
	return s.current, nil
}

func (s *familyProtectionCapabilityStub) SetFamilyProtection(_ context.Context, input peopleModule.SetFamilyProtection) (bool, error) {
	s.input = input
	if s.setErr != nil {
		return s.enabled, s.setErr
	}
	return input.Enabled, nil
}

// withConfigManage puts the gate the route enforces into the request, so the
// handler's own re-check passes.
func withConfigManage(req *http.Request, accountID int) *http.Request {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: accountID})
	return req.WithContext(context.WithValue(ctx, jwt.CtxPermissions, []string{permissions.ConfigManage}))
}

func TestGetFamilyProtectionReturnsCurrentState(t *testing.T) {
	t.Parallel()
	svc := &familyProtectionCapabilityStub{current: map[int64]bool{42: true}}
	rs := &Resource{ResourceConfig: ResourceConfig{FamilyProtection: svc}}
	req := httptest.NewRequest(http.MethodGet, "/students/42/family-protection", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.getFamilyProtection(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"success","data":{"student_id":"42","enabled":true},"message":"Family protection retrieved"}`, w.Body.String())
}

func TestSetFamilyProtectionForwardsActorAndReason(t *testing.T) {
	t.Parallel()
	svc := &familyProtectionCapabilityStub{}
	rs := &Resource{ResourceConfig: ResourceConfig{FamilyProtection: svc}}
	req := staffRequest(http.MethodPut, "/students/42/family-protection", `{"enabled":true,"reason":"Schutz nötig"}`, "")
	req = withConfigManage(req, 55)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.setFamilyProtection(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, peopleModule.SetFamilyProtection{StudentID: 42, Enabled: true, Reason: "Schutz nötig", ActorAccountID: 55}, svc.input)
}

func TestSetFamilyProtectionRejectsMissingReason(t *testing.T) {
	t.Parallel()
	svc := &familyProtectionCapabilityStub{}
	rs := &Resource{ResourceConfig: ResourceConfig{FamilyProtection: svc}}
	req := httptest.NewRequest(http.MethodPut, "/students/42/family-protection", strings.NewReader(`{"enabled":true,"reason":" "}`))
	w := httptest.NewRecorder()

	rs.setFamilyProtection(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// The privacy ledger is an admin decision; without config:manage the handler
// refuses even though the body is well formed.
func TestSetFamilyProtectionRequiresConfigManage(t *testing.T) {
	t.Parallel()
	svc := &familyProtectionCapabilityStub{}
	rs := &Resource{ResourceConfig: ResourceConfig{FamilyProtection: svc}}
	req := staffRequest(http.MethodPut, "/students/42/family-protection", `{"enabled":true,"reason":"Schutz nötig"}`, "")
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 55}))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.setFamilyProtection(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Zero(t, svc.input.StudentID)
}

// Switching a protection to the state it already has is not a failure the
// staff member has to fix: the request succeeds, the answer states the current
// state and says that nothing changed (#2267).
func TestSetFamilyProtectionUnchangedAnswersOk(t *testing.T) {
	t.Parallel()
	svc := &familyProtectionCapabilityStub{enabled: true, setErr: peopleModule.ErrFamilyProtectionUnchanged}
	rs := &Resource{ResourceConfig: ResourceConfig{FamilyProtection: svc}}
	req := staffRequest(http.MethodPut, "/students/42/family-protection", `{"enabled":true,"reason":"Schutz nötig"}`, "")
	req = withConfigManage(req, 55)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.setFamilyProtection(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t,
		`{"status":"success","data":{"student_id":"42","enabled":true,"unchanged":true},"message":"Family protection updated"}`,
		w.Body.String())
}
