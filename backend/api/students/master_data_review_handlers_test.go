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

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

type fakeMasterDataReviewService struct {
	items     []*userService.MasterDataReviewItem
	listErr   error
	decided   *usersModels.StudentDataChangeRequest
	decideErr error
	gotInput  userService.MasterDataReviewDecideInput
}

func (f *fakeMasterDataReviewService) ListPending(context.Context) ([]*userService.MasterDataReviewItem, error) {
	return f.items, f.listErr
}

func (f *fakeMasterDataReviewService) Decide(_ context.Context, input userService.MasterDataReviewDecideInput) (*usersModels.StudentDataChangeRequest, error) {
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

func TestListMasterDataChangeRequests_ReturnsPendingItems(t *testing.T) {
	svc := &fakeMasterDataReviewService{
		items: []*userService.MasterDataReviewItem{{
			Request:   reviewRow(usersModels.DataChangeStatusPending),
			FirstName: "Lara",
			LastName:  "Beispiel",
		}},
	}
	rs := &Resource{MasterDataReviewService: svc}
	req := staffRequest(http.MethodGet, "/students/master-data-change-requests", "", "")
	w := httptest.NewRecorder()

	rs.listMasterDataChangeRequests(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"id":"100"`)
	assert.Contains(t, body, `"student_id":"42"`)
	assert.Contains(t, body, `"first_name":"Lara"`)
	assert.Contains(t, body, `"field_key":"first_name"`)
}

func TestListMasterDataChangeRequests_RequiresConfiguredService(t *testing.T) {
	rs := &Resource{}
	req := staffRequest(http.MethodGet, "/students/master-data-change-requests", "", "")
	w := httptest.NewRecorder()

	rs.listMasterDataChangeRequests(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDecideMasterDataChangeRequest_ForwardsDecisionAndReviewer(t *testing.T) {
	svc := &fakeMasterDataReviewService{decided: reviewRow(usersModels.DataChangeStatusApproved)}
	rs := &Resource{MasterDataReviewService: svc}
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
}

func TestDecideMasterDataChangeRequest_RejectsBadRequest(t *testing.T) {
	rs := &Resource{MasterDataReviewService: &fakeMasterDataReviewService{}}

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
			rs := &Resource{MasterDataReviewService: &fakeMasterDataReviewService{decideErr: tt.err}}
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
