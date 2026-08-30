package students

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type conflictServiceStub struct {
	input  userService.ResolveConflictInput
	called bool
	err    error
}

func (s *conflictServiceStub) ResolveConflict(_ context.Context, input userService.ResolveConflictInput) error {
	s.called = true
	s.input = input
	return s.err
}

func resolveConflictRequest(t *testing.T, body string) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	ctx := tenant.WithRollbackMarker(context.WithValue(t.Context(), jwt.CtxClaims, jwt.AppClaims{
		ID: 55, Roles: []string{"ogs_admin"},
	}))
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{"users:update"})
	req := httptest.NewRequest(
		http.MethodPost, "/students/change-requests/conflicts/resolve", strings.NewReader(body),
	).WithContext(ctx)
	return httptest.NewRecorder(), req
}

// responseCode reads the wire error code the client branches on.
func responseCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body.Code
}

func TestResolveRequestConflictForwardsTheWholeGroup(t *testing.T) {
	t.Parallel()
	svc := &conflictServiceStub{}
	rs := &Resource{ResourceConfig: ResourceConfig{ParentRequestConflictService: svc}}
	w, req := resolveConflictRequest(t, `{"kind":"excused","request_ids":["12","13"],`+
		`"expected_versions":["v1","v2"],"chosen_request_id":"13",`+
		`"conflict_key":"absence:2026-09-01","reason":"Mit den Eltern geklärt"}`)

	rs.resolveRequestConflict(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, userService.ParentRequestKindExcused, svc.input.Kind)
	assert.Equal(t, []int64{12, 13}, svc.input.RequestIDs)
	assert.Equal(t, []string{"v1", "v2"}, svc.input.ExpectedVersions)
	assert.Equal(t, int64(13), svc.input.ChosenRequestID)
	assert.Equal(t, "absence:2026-09-01", svc.input.ConflictKey)
	assert.Equal(t, "Mit den Eltern geklärt", svc.input.Reason)
	assert.Equal(t, int64(55), svc.input.ReviewerID)
	assert.Equal(t, "ogs_admin", svc.input.ActorRole)
}

func TestResolveRequestConflictForwardsAStaffValueUnread(t *testing.T) {
	t.Parallel()
	svc := &conflictServiceStub{}
	rs := &Resource{ResourceConfig: ResourceConfig{ParentRequestConflictService: svc}}
	w, req := resolveConflictRequest(t, `{"kind":"pickup_change","request_ids":["12","13"],`+
		`"expected_versions":["v1","v2"],"staff_value":{"value":"15:30"},"reason":"Kompromiss"}`)

	rs.resolveRequestConflict(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, map[string]any{"value": "15:30"}, svc.input.StaffValue,
		"the handler must not interpret a domain payload")
	assert.Zero(t, svc.input.ChosenRequestID)
}

func TestResolveRequestConflictRejectsMalformedIDs(t *testing.T) {
	t.Parallel()
	svc := &conflictServiceStub{err: errors.New("must not be called")}
	rs := &Resource{ResourceConfig: ResourceConfig{ParentRequestConflictService: svc}}
	w, req := resolveConflictRequest(t, `{"kind":"excused","request_ids":["12","no"],`+
		`"expected_versions":["v1","v2"],"none":true,"reason":"Geklärt"}`)

	rs.resolveRequestConflict(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, svc.called)
}

func TestResolveRequestConflictAnswersWithoutAWiredService(t *testing.T) {
	t.Parallel()
	rs := &Resource{}
	w, req := resolveConflictRequest(t, `{"kind":"excused","request_ids":["12","13"],`+
		`"expected_versions":["v1","v2"],"none":true,"reason":"Geklärt"}`)

	rs.resolveRequestConflict(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code,
		"deciding the group one by one is the bug this route exists to prevent")
}

// The wire contract: the client branches on the code, never on the message.
func TestResolveRequestConflictErrorCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
		code string
	}{
		{name: "stale group", err: userService.ErrParentRequestStale, want: http.StatusConflict, code: codeChangeRequestStale},
		{name: "missing reason", err: userService.ErrParentRequestReasonRequired, want: http.StatusBadRequest, code: codeReasonRequired},
		{name: "malformed command", err: userService.ErrInvalidConflictResolution, want: http.StatusBadRequest},
		{name: "kind without a domain", err: userService.ErrConflictKindUnsupported, want: http.StatusBadRequest, code: codeConflictKindUnsupported},
		{name: "domain takes no typed value", err: userService.ErrStaffValueUnsupported, want: http.StatusBadRequest, code: codeStaffValueUnsupported},
		{name: "absence value invalid", err: absenceService.ErrAbsenceRequestInvalidStatus, want: http.StatusBadRequest, code: codeStaffValueInvalid},
		{name: "care value invalid", err: scheduleService.ErrInvalidCareRequestPayload, want: http.StatusBadRequest, code: codeStaffValueInvalid},
		{name: "offering value invalid", err: enrollmentService.ErrOfferingChangeInvalid, want: http.StatusBadRequest, code: codeStaffValueInvalid},
		{name: "Stammdaten value invalid", err: userService.ErrReviewInvalidValue, want: http.StatusBadRequest, code: codeStaffValueInvalid},
		{name: "request gone", err: userService.ErrParentRequestNotFound, want: http.StatusNotFound},
		{name: "kind not permitted", err: userService.ErrParentRequestForbidden, want: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rs := &Resource{ResourceConfig: ResourceConfig{
				ParentRequestConflictService: &conflictServiceStub{err: tt.err},
			}}
			w, req := resolveConflictRequest(t, `{"kind":"excused","request_ids":["12","13"],`+
				`"expected_versions":["v1","v2"],"none":true,"reason":"Geklärt"}`)

			rs.resolveRequestConflict(w, req)

			require.Equal(t, tt.want, w.Code, w.Body.String())
			if tt.code != "" {
				assert.Equal(t, tt.code, responseCode(t, w))
			}
			assert.True(t, tenant.RollbackRequested(req.Context()),
				"a failed resolve must never leave a half-decided group behind")
		})
	}
}
