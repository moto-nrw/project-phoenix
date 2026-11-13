package careplanhttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

type fakeMasterDataReviewService struct {
	decided   *masterdatarequests.ReviewItem
	decideErr error
	gotInput  masterdatarequests.DecideInput
}

func (f *fakeMasterDataReviewService) Decide(_ context.Context, input masterdatarequests.DecideInput) (*masterdatarequests.ReviewItem, error) {
	f.gotInput = input
	return f.decided, f.decideErr
}

func (*fakeMasterDataReviewService) Correct(context.Context, int64, bool, string, string, int64) error {
	return nil
}

func staffRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	return req.WithContext(context.WithValue(req.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 55}))
}

func reviewItem(status string) *masterdatarequests.ReviewItem {
	return &masterdatarequests.ReviewItem{
		Request: &masterdatarequests.Request{
			ID: 100, CreatedAt: time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC),
			StudentID: 42,
			Target:    masterdatarequests.TargetPerson,
			FieldKey:  "first_name",
			OldValue:  json.RawMessage(`"Lara"`),
			NewValue:  json.RawMessage(`"Lea"`),
			Status:    status,
		},
		FirstName: "Lara",
		LastName:  "Beispiel",
	}
}

func TestMasterDataChangeRequestRoutesRequireUsersUpdate(t *testing.T) {
	t.Parallel()
	router := (testResource(careplanhttp.ResourceConfig{
		MasterDataReviewService: &fakeMasterDataReviewService{}})).Router(careplanhttp.ScopeTestTenantRoutes(testpkg.TenantSessionGatesWithCaches(scopeTestSettingsCache)))
	// The decide route gates on users:update (deciding a request is the same
	// child write as editing the child directly), with per-child scope enforced
	// in the service. A caller with only users:read cannot reach it.
	claims := testutil.DefaultTestClaims()
	claims.Permissions = []string{permissions.UsersRead}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)

	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/master-data-change-requests/100/decide",
		strings.NewReader(`{"approve":true}`), testutil.WithJWTBearer(token))
	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusForbidden, rr.Code)
}

func TestDecideMasterDataChangeRequest_ForwardsDecisionAndReviewer(t *testing.T) {
	t.Parallel()

	svc := &fakeMasterDataReviewService{decided: reviewItem(masterdatarequests.StatusApproved)}
	rs := testResource(careplanhttp.ResourceConfig{
		MasterDataReviewService: svc})
	req := staffRequest(
		http.MethodPost,
		"/students/master-data-change-requests/100/decide",
		`{"approve":true,"reason":"passt"}`,
	)
	w := httptest.NewRecorder()

	serveRoute(t, rs, w, req, permissions.UsersUpdate)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(100), svc.gotInput.RequestID)
	assert.True(t, svc.gotInput.Approve)
	assert.Equal(t, "passt", svc.gotInput.Reason)
	assert.Equal(t, int64(55), svc.gotInput.ReviewedBy)
	assert.Contains(t, w.Body.String(), `"status":"approved"`)
	assert.Contains(t, w.Body.String(), `"first_name":"Lara"`)
	assert.Contains(t, w.Body.String(), `"last_name":"Beispiel"`)
}

func TestDecideMasterDataChangeRequest_RejectsBadRequest(t *testing.T) {
	t.Parallel()

	rs := testResource(careplanhttp.ResourceConfig{
		MasterDataReviewService: &fakeMasterDataReviewService{}})

	req := staffRequest(http.MethodPost, "/students/master-data-change-requests/nope/decide", `{}`)
	w := httptest.NewRecorder()
	serveRoute(t, rs, w, req, permissions.UsersUpdate)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	req = staffRequest(http.MethodPost, "/students/master-data-change-requests/100/decide", `{`)
	w = httptest.NewRecorder()
	serveRoute(t, rs, w, req, permissions.UsersUpdate)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	req = staffRequest(http.MethodPost, "/students/master-data-change-requests/100/decide", `{"aproove":true}`)
	w = httptest.NewRecorder()
	serveRoute(t, rs, w, req, permissions.UsersUpdate)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideMasterDataChangeRequest_MapsServiceErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
		code string
	}{
		{name: "not found", err: masterdatarequests.ErrReviewNotFound, want: http.StatusNotFound},
		{name: "not pending", err: masterdatarequests.ErrReviewNotPending, want: http.StatusConflict, code: "students.change_request_not_pending"},
		{name: "stale", err: masterdatarequests.ErrReviewStaleValue, want: http.StatusConflict, code: "students.change_request_stale"},
		{name: "invalid target", err: masterdatarequests.ErrReviewInvalidTarget, want: http.StatusBadRequest},
		{name: "internal", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := testResource(careplanhttp.ResourceConfig{
				MasterDataReviewService: &fakeMasterDataReviewService{decideErr: tt.err}})
			req := staffRequest(http.MethodPost, "/students/master-data-change-requests/100/decide", `{"approve":false}`)
			w := httptest.NewRecorder()

			serveRoute(t, rs, w, req, permissions.UsersUpdate)

			assert.Equal(t, tt.want, w.Code)
			if tt.code != "" {
				assert.Contains(t, w.Body.String(), tt.code)
			}
		})
	}
}
