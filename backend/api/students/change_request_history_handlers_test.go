package students

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

func historyItem(id int64, reviewedAt *time.Time, updatedAt time.Time, reviewerName string) *userService.MasterDataHistoryItem {
	req := &userModels.StudentDataChangeRequest{
		StudentID:    42,
		Target:       userModels.DataChangeTargetPerson,
		FieldKey:     "first_name",
		Status:       userModels.DataChangeStatusRejected,
		ReviewedAt:   reviewedAt,
		ReviewReason: strPtr("zu kurz"),
	}
	req.ID = id
	req.UpdatedAt = updatedAt
	return &userService.MasterDataHistoryItem{
		Request:      req,
		FirstName:    "Lara",
		LastName:     "Lehmann",
		ReviewerName: reviewerName,
	}
}

func strPtr(s string) *string { return &s }

func TestListMasterDataChangeRequestHistory_EnvelopeAndCursor(t *testing.T) {
	t.Parallel()

	updated := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	reviewed := updated.Add(-time.Minute)
	svc := &fakeMasterDataReviewService{
		history:     []*userService.MasterDataHistoryItem{historyItem(100, &reviewed, updated, "Rieke Reviewer")},
		historyNext: &userService.HistoryCursor{UpdatedAt: updated, ID: 100},
	}
	rs := &Resource{ResourceConfig: ResourceConfig{MasterDataReviewService: svc}}

	req := staffRequest(http.MethodGet, "/students/master-data-change-requests/history?limit=5", "", "")
	w := httptest.NewRecorder()
	rs.listMasterDataChangeRequestHistory(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"id":"100"`)
	assert.Contains(t, body, `"decided_by_name":"Rieke Reviewer"`)
	assert.Contains(t, body, `"review_reason":"zu kurz"`)
	assert.Contains(t, body, `"decided_at":"2026-08-18T09:59:00Z"`, "decided_at is reviewed_at when a reviewer stamped it")
	assert.Contains(t, body, `"next_cursor":"`)
	assert.Equal(t, 5, svc.gotLimit)

	// Feed the returned cursor back: the service receives the decoded keyset.
	cursor := encodeHistoryCursor(svc.historyNext)
	req = staffRequest(http.MethodGet, "/students/master-data-change-requests/history?cursor="+cursor, "", "")
	w = httptest.NewRecorder()
	rs.listMasterDataChangeRequestHistory(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, svc.gotBefore.Equal(updated), "cursor round-trip must restore the keyset instant")
	assert.Equal(t, int64(100), svc.gotBeforeID)
	assert.Equal(t, historyDefaultLimit, svc.gotLimit, "no limit param falls back to the default")
}

func TestListMasterDataChangeRequestHistory_DecidedAtFallsBackToUpdatedAt(t *testing.T) {
	t.Parallel()

	updated := time.Date(2026, 8, 17, 8, 30, 0, 0, time.UTC)
	svc := &fakeMasterDataReviewService{
		history: []*userService.MasterDataHistoryItem{historyItem(101, nil, updated, "")},
	}
	rs := &Resource{ResourceConfig: ResourceConfig{MasterDataReviewService: svc}}

	req := staffRequest(http.MethodGet, "/students/master-data-change-requests/history", "", "")
	w := httptest.NewRecorder()
	rs.listMasterDataChangeRequestHistory(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"decided_at":"2026-08-17T08:30:00Z"`, "without reviewed_at the decision instant is updated_at")
	assert.NotContains(t, body, `"decided_by_name"`, "an empty reviewer name is omitted")
	assert.NotContains(t, body, `"next_cursor"`, "the last page carries no cursor")
}

func TestListMasterDataChangeRequestHistory_InvalidQuery(t *testing.T) {
	t.Parallel()

	rs := &Resource{ResourceConfig: ResourceConfig{MasterDataReviewService: &fakeMasterDataReviewService{}}}

	for _, query := range []string{"?cursor=not-base64!", "?limit=abc", "?limit=0"} {
		req := staffRequest(http.MethodGet, "/students/master-data-change-requests/history"+query, "", "")
		w := httptest.NewRecorder()
		rs.listMasterDataChangeRequestHistory(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "query %q must be rejected", query)
	}
}

// Deliberately NOT parallel: the test reaches process-global state (env
// variables, viper keys, the settings registry, os.Stdout) that the whole
// test binary shares.
func TestChangeRequestHistoryRoutesRequireUsersUpdate(t *testing.T) {
	t.Setenv("AUTH_JWT_SECRET", "hostile-developer-secret-at-least-32-characters")
	testutil.SeedTestJWTConfig()
	router := (&Resource{ResourceConfig: ResourceConfig{MasterDataReviewService: &fakeMasterDataReviewService{}}}).Router()
	claims := testutil.DefaultTestClaims()
	claims.Permissions = []string{permissions.UsersRead}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)

	for _, path := range []string{
		"/master-data-change-requests/history",
		"/care-schedule-change-requests/history",
		"/offering-change-requests/history",
		"/excused-absence-requests/history",
	} {
		req := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, strings.NewReader(""), testutil.WithJWTBearer(token))
		rr := testutil.ExecuteRequest(router, req)
		require.Equal(t, http.StatusForbidden, rr.Code, "users:read alone must not reach %s", path)
	}
}
