package students

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type bulkServiceStub struct {
	input userService.BulkApproveParentRequestsInput
	err   error
}

func (s *bulkServiceStub) BulkApprove(_ context.Context, input userService.BulkApproveParentRequestsInput) error {
	s.input = input
	return s.err
}

func TestBulkApproveParentRequestsForwardsCompleteSelection(t *testing.T) {
	t.Parallel()
	svc := &bulkServiceStub{}
	rs := &Resource{ResourceConfig: ResourceConfig{ParentRequestBulkService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/students/change-requests/bulk-approve", strings.NewReader(`{"requests":[{"kind":"master_data","id":"12","expected_version":"v1"},{"kind":"excused","id":"13","expected_version":"v2"}],"reason":"Geprüft"}`))
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 55})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{"users:update"})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	rs.bulkApproveParentRequests(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(55), svc.input.ReviewerID)
	assert.Equal(t, "Geprüft", svc.input.Reason)
	assert.Equal(t, []userService.ParentRequestRef{
		{Kind: userService.ParentRequestKindMasterData, ID: 12, ExpectedVersion: "v1"},
		{Kind: userService.ParentRequestKindExcused, ID: 13, ExpectedVersion: "v2"},
	}, svc.input.Requests)
}

func TestBulkApproveParentRequestsReturnsConflictAndMarksRollback(t *testing.T) {
	t.Parallel()
	svc := &bulkServiceStub{err: userService.ErrParentRequestStale}
	rs := &Resource{ResourceConfig: ResourceConfig{ParentRequestBulkService: svc}}
	ctx := tenant.WithRollbackMarker(context.WithValue(t.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 55}))
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{"users:update"})
	req := httptest.NewRequest(http.MethodPost, "/students/change-requests/bulk-approve", strings.NewReader(`{"requests":[{"kind":"master_data","id":"12","expected_version":"v1"},{"kind":"excused","id":"13","expected_version":"v2"}],"reason":"Geprüft"}`)).WithContext(ctx)
	w := httptest.NewRecorder()

	rs.bulkApproveParentRequests(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestBulkApproveParentRequestsRejectsMalformedID(t *testing.T) {
	t.Parallel()
	svc := &bulkServiceStub{err: errors.New("must not be called")}
	rs := &Resource{ResourceConfig: ResourceConfig{ParentRequestBulkService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/students/change-requests/bulk-approve", strings.NewReader(`{"requests":[{"kind":"master_data","id":"no","expected_version":"v1"}],"reason":"Geprüft"}`))
	w := httptest.NewRecorder()

	rs.bulkApproveParentRequests(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, svc.input.Requests)
}
