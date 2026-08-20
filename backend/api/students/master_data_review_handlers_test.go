package students

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

type fakeMasterDataReviewService struct {
	items       []*userService.MasterDataReviewItem
	listErr     error
	decided     *userService.MasterDataReviewItem
	decideErr   error
	gotInput    userService.MasterDataReviewDecideInput
	history     []*userService.MasterDataHistoryItem
	historyNext *userService.HistoryCursor
	historyErr  error
	gotBefore   time.Time
	gotBeforeID int64
	gotLimit    int
}

func (f *fakeMasterDataReviewService) ListHistory(_ context.Context, filters modelBase.RequestQueueFilters) ([]*userService.MasterDataHistoryItem, *userService.HistoryCursor, error) {
	f.gotBefore = filters.BeforeInstant
	f.gotBeforeID = filters.BeforeID
	f.gotLimit = filters.Limit
	return f.history, f.historyNext, f.historyErr
}

func (f *fakeMasterDataReviewService) ListPending(context.Context, modelBase.RequestQueueFilters) ([]*userService.MasterDataReviewItem, *userService.HistoryCursor, error) {
	return f.items, nil, f.listErr
}

func (f *fakeMasterDataReviewService) Decide(_ context.Context, input userService.MasterDataReviewDecideInput) (*userService.MasterDataReviewItem, error) {
	f.gotInput = input
	return f.decided, f.decideErr
}

func staffRequest(method, path, body string, requestID string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 55}))
	rctx := chi.NewRouteContext()
	if requestID != "" {
		rctx.URLParams.Add("requestId", requestID)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func reviewItem(status string) *userService.MasterDataReviewItem {
	return &userService.MasterDataReviewItem{
		Request:   reviewRow(status),
		FirstName: "Lara",
		LastName:  "Beispiel",
	}
}

func reviewRow(status string) *usersModels.StudentDataChangeRequest {
	return &usersModels.StudentDataChangeRequest{
		Model:     modelBase.Model{ID: 100, CreatedAt: time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)},
		StudentID: 42,
		Target:    usersModels.DataChangeTargetPerson,
		FieldKey:  "first_name",
		OldValue:  json.RawMessage(`"Lara"`),
		NewValue:  json.RawMessage(`"Lea"`),
		Status:    status,
	}
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestMasterDataChangeRequestRoutesRequireUsersUpdate(t *testing.T) {
	testutil.SeedTestJWTConfig()
	router := (&Resource{ResourceConfig: ResourceConfig{MasterDataReviewService: &fakeMasterDataReviewService{}}}).Router()
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

	svc := &fakeMasterDataReviewService{decided: reviewItem(usersModels.DataChangeStatusApproved)}
	rs := &Resource{ResourceConfig: ResourceConfig{MasterDataReviewService: svc}}
	req := staffRequest(
		http.MethodPost,
		"/students/master-data-change-requests/100/decide",
		`{"approve":true,"reason":"passt"}`,
		"100",
	)
	w := httptest.NewRecorder()

	rs.decideMasterDataChangeRequest(w, req)

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

	rs := &Resource{ResourceConfig: ResourceConfig{MasterDataReviewService: &fakeMasterDataReviewService{}}}

	req := staffRequest(http.MethodPost, "/students/master-data-change-requests/nope/decide", `{}`, "nope")
	w := httptest.NewRecorder()
	rs.decideMasterDataChangeRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	req = staffRequest(http.MethodPost, "/students/master-data-change-requests/100/decide", `{`, "100")
	w = httptest.NewRecorder()
	rs.decideMasterDataChangeRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	req = staffRequest(http.MethodPost, "/students/master-data-change-requests/100/decide", `{"aproove":true}`, "100")
	w = httptest.NewRecorder()
	rs.decideMasterDataChangeRequest(w, req)
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
		{name: "not found", err: userService.ErrReviewNotFound, want: http.StatusNotFound},
		{name: "not pending", err: userService.ErrReviewNotPending, want: http.StatusConflict, code: "change_request_not_pending"},
		{name: "stale", err: userService.ErrReviewStaleValue, want: http.StatusConflict, code: "change_request_stale"},
		{name: "invalid target", err: userService.ErrReviewInvalidTarget, want: http.StatusBadRequest},
		{name: "internal", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := &Resource{ResourceConfig: ResourceConfig{MasterDataReviewService: &fakeMasterDataReviewService{decideErr: tt.err}}}
			req := staffRequest(http.MethodPost, "/students/master-data-change-requests/100/decide", `{"approve":false}`, "100")
			w := httptest.NewRecorder()

			rs.decideMasterDataChangeRequest(w, req)

			assert.Equal(t, tt.want, w.Code)
			if tt.code != "" {
				assert.Contains(t, w.Body.String(), tt.code)
			}
		})
	}
}
