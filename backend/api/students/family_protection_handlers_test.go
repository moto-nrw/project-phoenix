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

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

type familyProtectionManagerStub struct {
	input   userService.SetFamilyProtectionInput
	current map[int64]*userModels.FamilyProtectionEvent
	// setErr is what Set returns alongside the event, so the unchanged
	// sentinel can be exercised.
	setErr error
}

func (s *familyProtectionManagerStub) Current(context.Context, []int64) (map[int64]*userModels.FamilyProtectionEvent, error) {
	return s.current, nil
}

func TestGetFamilyProtectionReturnsCurrentState(t *testing.T) {
	t.Parallel()
	svc := &familyProtectionManagerStub{current: map[int64]*userModels.FamilyProtectionEvent{
		42: {StudentID: 42, Enabled: true, Reason: "Schutz nötig"},
	}}
	rs := &Resource{ResourceConfig: ResourceConfig{FamilyProtectionService: svc}}
	req := httptest.NewRequest(http.MethodGet, "/students/42/family-protection", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.getFamilyProtection(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"success","data":{"student_id":"42","enabled":true},"message":"Family protection retrieved"}`, w.Body.String())
}

func (s *familyProtectionManagerStub) Set(_ context.Context, input userService.SetFamilyProtectionInput) (*userModels.FamilyProtectionEvent, error) {
	s.input = input
	return &userModels.FamilyProtectionEvent{StudentID: input.StudentID, Enabled: input.Enabled, Reason: input.Reason}, s.setErr
}

func TestSetFamilyProtectionForwardsActorAndReason(t *testing.T) {
	t.Parallel()
	svc := &familyProtectionManagerStub{}
	rs := &Resource{ResourceConfig: ResourceConfig{FamilyProtectionService: svc}}
	req := staffRequest(http.MethodPut, "/students/42/family-protection", `{"enabled":true,"reason":"Schutz nötig"}`, "")
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 55}))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.setFamilyProtection(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userService.SetFamilyProtectionInput{StudentID: 42, Enabled: true, Reason: "Schutz nötig", ActorAccountID: 55}, svc.input)
}

func TestSetFamilyProtectionRejectsMissingReason(t *testing.T) {
	t.Parallel()
	svc := &familyProtectionManagerStub{}
	rs := &Resource{ResourceConfig: ResourceConfig{FamilyProtectionService: svc}}
	req := httptest.NewRequest(http.MethodPut, "/students/42/family-protection", strings.NewReader(`{"enabled":true,"reason":" "}`))
	w := httptest.NewRecorder()

	rs.setFamilyProtection(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Switching a protection to the state it already has is not a failure the
// staff member has to fix: the request succeeds, the answer states the current
// state and says that nothing changed (#2267).
func TestSetFamilyProtectionUnchangedAnswersOk(t *testing.T) {
	t.Parallel()
	svc := &familyProtectionManagerStub{setErr: userService.ErrFamilyProtectionUnchanged}
	rs := &Resource{ResourceConfig: ResourceConfig{FamilyProtectionService: svc}}
	req := staffRequest(http.MethodPut, "/students/42/family-protection", `{"enabled":true,"reason":"Schutz nötig"}`, "")
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 55}))
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
