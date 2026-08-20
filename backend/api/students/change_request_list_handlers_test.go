package students

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// keysetHistoryPage mimics the real services' keyset contract: rows sorted
// newest-first, page strictly before (before, beforeID), limit+1 probe for
// the next cursor. Fakes that ignored `before` would make every cursor
// assertion in this file meaningless.
func keysetHistoryPage[T any](rows []T, key func(T) (time.Time, int64), before time.Time, beforeID int64, limit int) ([]T, *userService.HistoryCursor) {
	matched := make([]T, 0, len(rows))
	for _, row := range rows {
		u, id := key(row)
		if !before.IsZero() && (u.After(before) || (u.Equal(before) && id >= beforeID)) {
			continue
		}
		matched = append(matched, row)
	}
	if len(matched) <= limit {
		return matched, nil
	}
	matched = matched[:limit]
	u, id := key(matched[len(matched)-1])
	return matched, &userService.HistoryCursor{UpdatedAt: u, ID: id}
}

// The aggregate endpoint fakes embed their service interface so only the two
// read methods the endpoint touches need implementations; anything else
// panics on a nil interface, which is exactly the failure we want in a test.

type aggMasterFake struct {
	userService.MasterDataReviewService
	pending      []*userService.MasterDataReviewItem
	rows         []*userService.MasterDataHistoryItem
	pendingCalls int
	gotBefore    []time.Time
	gotBeforeID  []int64
	gotLimit     []int
}

func (f *aggMasterFake) ListPending(context.Context) ([]*userService.MasterDataReviewItem, error) {
	f.pendingCalls++
	return f.pending, nil
}

func (f *aggMasterFake) ListHistory(_ context.Context, before time.Time, beforeID int64, limit int) ([]*userService.MasterDataHistoryItem, *userService.HistoryCursor, error) {
	f.gotBefore = append(f.gotBefore, before)
	f.gotBeforeID = append(f.gotBeforeID, beforeID)
	f.gotLimit = append(f.gotLimit, limit)
	items, next := keysetHistoryPage(f.rows, func(it *userService.MasterDataHistoryItem) (time.Time, int64) {
		return it.Request.UpdatedAt, it.Request.ID
	}, before, beforeID, limit)
	return items, next, nil
}

type aggCareFake struct {
	scheduleService.CareScheduleRequestService
	pending      []*scheduleService.CareRequestReviewItem
	rows         []*scheduleService.CareRequestHistoryItem
	pendingCalls int
	historyCalls int
}

func (f *aggCareFake) ListPending(context.Context) ([]*scheduleService.CareRequestReviewItem, error) {
	f.pendingCalls++
	return f.pending, nil
}

func (f *aggCareFake) ListHistory(_ context.Context, before time.Time, beforeID int64, limit int) ([]*scheduleService.CareRequestHistoryItem, *userService.HistoryCursor, error) {
	f.historyCalls++
	items, next := keysetHistoryPage(f.rows, func(it *scheduleService.CareRequestHistoryItem) (time.Time, int64) {
		return it.Request.UpdatedAt, it.Request.ID
	}, before, beforeID, limit)
	return items, next, nil
}

type aggOfferingFake struct {
	enrollmentService.OfferingChangeRequestService
	pending          []*enrollmentService.OfferingChangeView
	rows             []*enrollmentService.OfferingChangeHistoryItem
	corrections      []*enrollmentService.DirectCorrectionItem
	pendingCalls     int
	historyCalls     int
	correctionsCalls int
}

func (f *aggOfferingFake) ListDirectCorrections(_ context.Context, before time.Time, beforeID int64, limit int) ([]*enrollmentService.DirectCorrectionItem, *userService.HistoryCursor, error) {
	f.correctionsCalls++
	items, next := keysetHistoryPage(f.corrections, func(it *enrollmentService.DirectCorrectionItem) (time.Time, int64) {
		return it.Adjustment.ChangedAt, it.Adjustment.ID
	}, before, beforeID, limit)
	return items, next, nil
}

func (f *aggOfferingFake) ListPending(context.Context) ([]*enrollmentService.OfferingChangeView, error) {
	f.pendingCalls++
	return f.pending, nil
}

func (f *aggOfferingFake) ListHistory(_ context.Context, before time.Time, beforeID int64, limit int) ([]*enrollmentService.OfferingChangeHistoryItem, *userService.HistoryCursor, error) {
	f.historyCalls++
	items, next := keysetHistoryPage(f.rows, func(it *enrollmentService.OfferingChangeHistoryItem) (time.Time, int64) {
		return it.Request.UpdatedAt, it.Request.ID
	}, before, beforeID, limit)
	return items, next, nil
}

type aggExcusedFake struct {
	absenceService.ExcusedAbsenceRequestService
	pending      []*absenceService.ExcusedRequestReviewItem
	rows         []*absenceService.ExcusedRequestHistoryItem
	pendingCalls int
	historyCalls int
}

func (f *aggExcusedFake) ListPending(context.Context) ([]*absenceService.ExcusedRequestReviewItem, error) {
	f.pendingCalls++
	return f.pending, nil
}

func (f *aggExcusedFake) ListHistory(_ context.Context, before time.Time, beforeID int64, limit int) ([]*absenceService.ExcusedRequestHistoryItem, *userService.HistoryCursor, error) {
	f.historyCalls++
	items, next := keysetHistoryPage(f.rows, func(it *absenceService.ExcusedRequestHistoryItem) (time.Time, int64) {
		return it.Request.UpdatedAt, it.Request.ID
	}, before, beforeID, limit)
	return items, next, nil
}

type aggFakes struct {
	master   *aggMasterFake
	care     *aggCareFake
	offering *aggOfferingFake
	excused  *aggExcusedFake
}

func newAggResource() (*Resource, *aggFakes) {
	fakes := &aggFakes{
		master:   &aggMasterFake{},
		care:     &aggCareFake{},
		offering: &aggOfferingFake{},
		excused:  &aggExcusedFake{},
	}
	rs := NewResource(ResourceConfig{
		MasterDataReviewService: fakes.master,
		CareRequestService:      fakes.care,
		OfferingChangeService:   fakes.offering,
		ExcusedRequestService:   fakes.excused,
	})
	return rs, fakes
}

var aggBase = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

func aggMasterPending(id int64, first, last string, createdAt time.Time) *userService.MasterDataReviewItem {
	req := &usersModels.StudentDataChangeRequest{
		Model:     modelBase.Model{ID: id, CreatedAt: createdAt, UpdatedAt: createdAt},
		StudentID: 100 + id,
		FieldKey:  "first_name",
		NewValue:  json.RawMessage(`"Neu"`),
		Status:    "pending",
	}
	return &userService.MasterDataReviewItem{Request: req, FirstName: first, LastName: last}
}

func aggCarePending(id int64, first, last string, createdAt time.Time) *scheduleService.CareRequestReviewItem {
	req := &scheduleModels.CareScheduleChangeRequest{
		Model:       modelBase.Model{ID: id, CreatedAt: createdAt, UpdatedAt: createdAt},
		StudentID:   200 + id,
		RequestKind: "weekly_schedule",
		Status:      "pending",
	}
	return &scheduleService.CareRequestReviewItem{Request: req, FirstName: first, LastName: last}
}

func aggOfferingPending(id int64, name string, createdAt time.Time) *enrollmentService.OfferingChangeView {
	req := &enrollmentModels.OfferingChangeRequest{
		Model:     modelBase.Model{ID: id, CreatedAt: createdAt, UpdatedAt: createdAt},
		StudentID: 300 + id,
		Status:    "pending",
	}
	return &enrollmentService.OfferingChangeView{Request: req, StudentName: name}
}

func aggExcusedPending(id int64, first, last string, createdAt time.Time) *absenceService.ExcusedRequestReviewItem {
	req := &activeModels.ExcusedAbsenceRequest{
		Model:     modelBase.Model{ID: id, CreatedAt: createdAt, UpdatedAt: createdAt},
		StudentID: 400 + id,
		Status:    "pending",
	}
	return &absenceService.ExcusedRequestReviewItem{Request: req, FirstName: first, LastName: last}
}

func aggMasterHistory(id int64, first, last, status string, decidedAt time.Time) *userService.MasterDataHistoryItem {
	reviewed := decidedAt
	item := &userService.MasterDataHistoryItem{
		Request: &usersModels.StudentDataChangeRequest{
			Model:      modelBase.Model{ID: id, CreatedAt: decidedAt.Add(-24 * time.Hour), UpdatedAt: decidedAt},
			StudentID:  100 + id,
			FieldKey:   "first_name",
			NewValue:   json.RawMessage(`"Neu"`),
			Status:     status,
			ReviewedAt: &reviewed,
		},
		FirstName:    first,
		LastName:     last,
		ReviewerName: "Revi Ewer",
	}
	if status == "withdrawn" || status == "auto_applied" {
		item.Request.ReviewedAt = nil
		item.ReviewerName = ""
	}
	return item
}

func aggExcusedHistory(id int64, first, last, status string, decidedAt time.Time) *absenceService.ExcusedRequestHistoryItem {
	reviewed := decidedAt
	item := &absenceService.ExcusedRequestHistoryItem{
		Request: &activeModels.ExcusedAbsenceRequest{
			Model:      modelBase.Model{ID: id, CreatedAt: decidedAt.Add(-24 * time.Hour), UpdatedAt: decidedAt},
			StudentID:  400 + id,
			Status:     status,
			ReviewedAt: &reviewed,
		},
		FirstName:    first,
		LastName:     last,
		ReviewerName: "Revi Ewer",
	}
	if status == "withdrawn" {
		item.Request.ReviewedAt = nil
		item.ReviewerName = ""
	}
	return item
}

// aggRequest builds a request whose context carries staff claims plus the
// given permissions, mirroring what the router middleware injects.
func aggRequest(t *testing.T, rawQuery string, perms []string) *http.Request {
	t.Helper()
	req := httptest.NewRequest("GET", "/change-requests", nil)
	if rawQuery != "" {
		req.URL.RawQuery = rawQuery
	}
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 55})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, perms)
	return req.WithContext(ctx)
}

type aggPage struct {
	Items []struct {
		RequestType string          `json:"request_type"`
		Data        json.RawMessage `json:"data"`
	} `json:"items"`
	NextCursor string `json:"next_cursor"`
}

func execAggregated(t *testing.T, rs *Resource, rawQuery string, perms []string) (*httptest.ResponseRecorder, aggPage) {
	t.Helper()
	rr := httptest.NewRecorder()
	rs.listAggregatedChangeRequests(rr, aggRequest(t, rawQuery, perms))
	var env struct {
		Data aggPage `json:"data"`
	}
	if rr.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	}
	return rr, env.Data
}

var aggUpdatePerms = []string{"users:read", "users:update"}

func aggTypes(page aggPage) []string {
	types := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		types = append(types, item.RequestType)
	}
	return types
}

func TestAggregatedChangeRequests_OpenMergesAllTypesNewestFirst(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.pending = []*userService.MasterDataReviewItem{aggMasterPending(1, "Anna", "Alt", aggBase.Add(3*time.Hour))}
	fakes.care.pending = []*scheduleService.CareRequestReviewItem{aggCarePending(2, "Ben", "Berg", aggBase.Add(1*time.Hour))}
	fakes.offering.pending = []*enrollmentService.OfferingChangeView{aggOfferingPending(3, "Cem Can", aggBase.Add(4*time.Hour))}
	fakes.excused.pending = []*absenceService.ExcusedRequestReviewItem{aggExcusedPending(4, "Dua", "Deml", aggBase.Add(2*time.Hour))}

	rr, page := execAggregated(t, rs, "", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	assert.Equal(t, []string{"offering", "master_data", "excused", "care_schedule"}, aggTypes(page))
	assert.Empty(t, page.NextCursor)

	// The payload is the unchanged per-type queue projection.
	var master struct {
		ID        string `json:"id"`
		FirstName string `json:"first_name"`
		FieldKey  string `json:"field_key"`
	}
	require.NoError(t, json.Unmarshal(page.Items[1].Data, &master))
	assert.Equal(t, "1", master.ID)
	assert.Equal(t, "Anna", master.FirstName)
	assert.Equal(t, "first_name", master.FieldKey)
}

func TestAggregatedChangeRequests_OpenSearchFiltersByChildName(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.pending = []*userService.MasterDataReviewItem{aggMasterPending(1, "Anna", "Alt", aggBase)}
	fakes.offering.pending = []*enrollmentService.OfferingChangeView{aggOfferingPending(3, "Anna Alt", aggBase.Add(time.Hour))}
	fakes.excused.pending = []*absenceService.ExcusedRequestReviewItem{aggExcusedPending(4, "Dua", "Deml", aggBase.Add(2*time.Hour))}

	rr, page := execAggregated(t, rs, "search="+url.QueryEscape("anna al"), aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"offering", "master_data"}, aggTypes(page))
}

func TestAggregatedChangeRequests_OpenTypeFilterSkipsOtherServices(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.excused.pending = []*absenceService.ExcusedRequestReviewItem{aggExcusedPending(4, "Dua", "Deml", aggBase)}
	fakes.master.pending = []*userService.MasterDataReviewItem{aggMasterPending(1, "Anna", "Alt", aggBase)}

	rr, page := execAggregated(t, rs, "types=excused", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"excused"}, aggTypes(page))
	assert.Zero(t, fakes.master.pendingCalls, "filtered-out services must not be queried")
	assert.Zero(t, fakes.care.pendingCalls)
	assert.Zero(t, fakes.offering.pendingCalls)
}

func TestAggregatedChangeRequests_OpenCursorPagination(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.pending = []*userService.MasterDataReviewItem{
		aggMasterPending(1, "Anna", "Alt", aggBase.Add(3*time.Hour)),
		aggMasterPending(2, "Ben", "Berg", aggBase.Add(1*time.Hour)),
	}
	fakes.excused.pending = []*absenceService.ExcusedRequestReviewItem{aggExcusedPending(4, "Cem", "Can", aggBase.Add(2*time.Hour))}

	rr, page := execAggregated(t, rs, "limit=2", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"master_data", "excused"}, aggTypes(page))
	require.NotEmpty(t, page.NextCursor)

	rr, page = execAggregated(t, rs, "limit=2&cursor="+url.QueryEscape(page.NextCursor), aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"master_data"}, aggTypes(page))
	assert.Empty(t, page.NextCursor)
}

func TestAggregatedChangeRequests_AbsenceOnlySeesOnlyExcused(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.pending = []*userService.MasterDataReviewItem{aggMasterPending(1, "Anna", "Alt", aggBase)}
	fakes.excused.pending = []*absenceService.ExcusedRequestReviewItem{aggExcusedPending(4, "Dua", "Deml", aggBase.Add(time.Hour))}

	rr, page := execAggregated(t, rs, "", []string{"users:read", "users:absence"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"excused"}, aggTypes(page))
	assert.Zero(t, fakes.master.pendingCalls, "users:update queues must not be queried for an absence-only caller")
	assert.Zero(t, fakes.care.pendingCalls)
	assert.Zero(t, fakes.offering.pendingCalls)
}

func TestAggregatedChangeRequests_HistoryMergesAndPaginates(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.rows = []*userService.MasterDataHistoryItem{
		aggMasterHistory(1, "Anna", "Alt", "approved", aggBase.Add(4*time.Hour)),
		aggMasterHistory(2, "Ben", "Berg", "rejected", aggBase.Add(1*time.Hour)),
	}
	fakes.excused.rows = []*absenceService.ExcusedRequestHistoryItem{
		aggExcusedHistory(4, "Cem", "Can", "approved", aggBase.Add(3*time.Hour)),
		aggExcusedHistory(5, "Dua", "Deml", "rejected", aggBase.Add(2*time.Hour)),
	}

	rr, page := execAggregated(t, rs, "view=history&limit=3", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"master_data", "excused", "excused"}, aggTypes(page))
	require.NotEmpty(t, page.NextCursor, "one master-data row is still unread")

	// The history payload carries the decision facts.
	var first struct {
		DecidedAt     time.Time `json:"decided_at"`
		DecidedByName string    `json:"decided_by_name"`
	}
	require.NoError(t, json.Unmarshal(page.Items[0].Data, &first))
	assert.Equal(t, "Revi Ewer", first.DecidedByName)
	assert.True(t, first.DecidedAt.Equal(aggBase.Add(4*time.Hour)))

	rr, page = execAggregated(t, rs, "view=history&limit=3&cursor="+url.QueryEscape(page.NextCursor), aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"master_data"}, aggTypes(page))
	assert.Empty(t, page.NextCursor)

	// The second master-data fetch must resume strictly before the last
	// consumed DB row of that type (the approved row from page one).
	require.Len(t, fakes.master.gotBefore, 2)
	consumedRow := fakes.master.rows[0].Request
	assert.True(t, fakes.master.gotBefore[1].Equal(consumedRow.UpdatedAt))
	assert.Equal(t, consumedRow.ID, fakes.master.gotBeforeID[1])
}

// An auto-applied row was never stamped by a reviewer: its decision instant
// falls back to updated_at and no reviewer name reaches the wire.
func TestAggregatedChangeRequests_HistoryDecidedAtFallsBackToUpdatedAt(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	decidedAt := aggBase.Add(90 * time.Minute)
	fakes.master.rows = []*userService.MasterDataHistoryItem{
		aggMasterHistory(7, "Emil", "Ohne", "auto_applied", decidedAt),
	}
	require.Nil(t, fakes.master.rows[0].Request.ReviewedAt, "fixture must carry no reviewer stamp")

	rr, page := execAggregated(t, rs, "view=history&types=master_data", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Len(t, page.Items, 1)

	var payload struct {
		DecidedAt time.Time `json:"decided_at"`
	}
	require.NoError(t, json.Unmarshal(page.Items[0].Data, &payload))
	assert.True(t, payload.DecidedAt.Equal(decidedAt), "without reviewed_at the decision instant is updated_at")
	assert.NotContains(t, string(page.Items[0].Data), `"decided_by_name"`, "an empty reviewer name is omitted")
}

func TestAggregatedChangeRequests_HistoryStatusFilter(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.rows = []*userService.MasterDataHistoryItem{
		aggMasterHistory(1, "Anna", "Alt", "approved", aggBase.Add(4*time.Hour)),
		aggMasterHistory(2, "Ben", "Berg", "auto_applied", aggBase.Add(3*time.Hour)),
		aggMasterHistory(3, "Cem", "Can", "rejected", aggBase.Add(2*time.Hour)),
	}
	fakes.excused.rows = []*absenceService.ExcusedRequestHistoryItem{
		aggExcusedHistory(4, "Dua", "Deml", "withdrawn", aggBase.Add(1*time.Hour)),
	}

	rr, page := execAggregated(t, rs, "view=history&status=approved", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"master_data", "master_data"}, aggTypes(page), "approved includes auto_applied")

	rr, page = execAggregated(t, rs, "view=history&status=rejected,withdrawn", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"master_data", "excused"}, aggTypes(page))
}

func TestAggregatedChangeRequests_HistoryDateRangeFilter(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.rows = []*userService.MasterDataHistoryItem{
		aggMasterHistory(1, "Anna", "Alt", "approved", time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)),
		aggMasterHistory(2, "Ben", "Berg", "approved", time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)),
		aggMasterHistory(3, "Cem", "Can", "approved", time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)),
	}

	rr, page := execAggregated(t, rs, "view=history&types=master_data&from=2026-08-06&to=2026-08-10", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Len(t, page.Items, 1)
	var got struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(page.Items[0].Data, &got))
	assert.Equal(t, "2", got.ID)
}

// A search that matches nothing must still terminate cleanly: the fill loop
// keeps scanning pages within its budget and reports no cursor once every
// row of the type has been scanned.
func TestAggregatedChangeRequests_HistorySearchScansPastFilteredRows(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	rows := make([]*userService.MasterDataHistoryItem, 0, 60)
	for i := range 60 {
		rows = append(rows, aggMasterHistory(int64(i+1), "Anna", "Alt", "approved", aggBase.Add(-time.Duration(i)*time.Minute)))
	}
	fakes.master.rows = rows

	rr, page := execAggregated(t, rs, "view=history&types=master_data&search=Zebra", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Empty(t, page.Items)
	assert.Empty(t, page.NextCursor, "all rows were scanned and nothing matched — no more rows exist")
	assert.Len(t, fakes.master.gotBefore, 2, "the fill loop keeps scanning within its budget")
}

func TestAggregatedChangeRequests_InvalidQuery(t *testing.T) {
	t.Parallel()

	rs, _ := newAggResource()
	for _, query := range []string{
		"cursor=not-base64!",
		"cursor=bm9wZQ",
		"limit=0",
		"limit=abc",
		"view=nope",
		"types=unknown",
		"status=maybe&view=history",
		"status=approved",              // status is a history-only filter
		"from=2026-08-10",              // date range is history-only
		"view=history&from=10.08.2026", // ISO dates only
		"view=history&from=2026-08-10&to=2026-08-01",
	} {
		rr, _ := execAggregated(t, rs, query, aggUpdatePerms)
		assert.Equal(t, http.StatusBadRequest, rr.Code, fmt.Sprintf("query %q must be rejected", query))
	}
}

// --- Direkt-Korrekturen in der zentralen Historie (#2436) -------------------

func aggOfferingHistory(id int64, name, status string, decidedAt time.Time) *enrollmentService.OfferingChangeHistoryItem {
	reviewed := decidedAt
	return &enrollmentService.OfferingChangeHistoryItem{
		Request: &enrollmentModels.OfferingChangeRequest{
			Model:      modelBase.Model{ID: id, CreatedAt: decidedAt.Add(-24 * time.Hour), UpdatedAt: decidedAt},
			StudentID:  300 + id,
			Status:     status,
			ReviewedAt: &reviewed,
		},
		StudentName:  name,
		ReviewerName: "Revi Ewer",
	}
}

func aggDirectCorrection(id int64, name string, changedAt time.Time) *enrollmentService.DirectCorrectionItem {
	actor := "Olga Office"
	return &enrollmentService.DirectCorrectionItem{
		Adjustment: &auditModels.EnrollmentOfferingAdjustment{
			ID:                id,
			StudentID:         500 + id,
			ChangedAt:         changedAt,
			Reason:            "Telefonisch gemeldet",
			ActorNameSnapshot: &actor,
			Source:            auditModels.OfferingAdjustmentSourceDirect,
		},
		StudentName: name,
		ActorName:   actor,
		Diff: []enrollmentService.OfferingChangeDiffEntry{{
			OfferingID: 9,
			Label:      "Mittagessen",
			OldState:   "booked",
			OldDays:    []string{"mon"},
			NewState:   "removed",
		}},
	}
}

func TestAggregatedChangeRequests_HistoryShowsDirectCorrections(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.offering.rows = []*enrollmentService.OfferingChangeHistoryItem{
		aggOfferingHistory(1, "Cem Can", "approved", aggBase.Add(2*time.Hour)),
	}
	fakes.offering.corrections = []*enrollmentService.DirectCorrectionItem{
		aggDirectCorrection(7, "Anna Alt", aggBase.Add(3*time.Hour)),
	}

	rr, page := execAggregated(t, rs, "view=history", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, []string{"direct_correction", "offering"}, aggTypes(page))

	var row DirectCorrectionResponse
	require.NoError(t, json.Unmarshal(page.Items[0].Data, &row))
	assert.Equal(t, "Anna Alt", row.StudentName)
	assert.Equal(t, "Olga Office", row.ChangedByName)
	assert.Equal(t, "Telefonisch gemeldet", row.Reason)
	assert.Equal(t, aggBase.Add(3*time.Hour), row.ChangedAt.UTC())
	require.Len(t, row.Diff, 1)
	assert.Equal(t, "Mittagessen", row.Diff[0].Label)
	assert.Equal(t, "Mo", row.Diff[0].Old)
	assert.Equal(t, "abgemeldet", row.Diff[0].New)
	// A correction is not a request: nothing decided, no status on the wire.
	assert.NotContains(t, string(page.Items[0].Data), `"status"`)
}

func TestAggregatedChangeRequests_OpenNeverShowsDirectCorrections(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.offering.corrections = []*enrollmentService.DirectCorrectionItem{
		aggDirectCorrection(7, "Anna Alt", aggBase.Add(3*time.Hour)),
	}

	for _, query := range []string{"", "view=open", "view=open&types=direct_correction"} {
		rr, page := execAggregated(t, rs, query, aggUpdatePerms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Empty(t, aggTypes(page), "query %q", query)
	}
	assert.Zero(t, fakes.offering.correctionsCalls)
}

func TestAggregatedChangeRequests_DirectCorrectionsHonourSearchAndDateRange(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.offering.corrections = []*enrollmentService.DirectCorrectionItem{
		aggDirectCorrection(7, "Anna Alt", aggBase.Add(3*time.Hour)),
		aggDirectCorrection(8, "Ben Berg", aggBase.Add(2*time.Hour)),
		aggDirectCorrection(9, "Anna Alt", aggBase.AddDate(0, 0, -40)),
	}

	_, byName := execAggregated(t, rs, "view=history&search=anna", aggUpdatePerms)
	require.Len(t, byName.Items, 2)

	from := aggBase.AddDate(0, 0, -1).Format("2006-01-02")
	_, byRange := execAggregated(t, rs, "view=history&search=anna&from="+from, aggUpdatePerms)
	require.Len(t, byRange.Items, 1)
	var row DirectCorrectionResponse
	require.NoError(t, json.Unmarshal(byRange.Items[0].Data, &row))
	assert.Equal(t, "7", row.ID)
}

func TestAggregatedChangeRequests_StatusFilterExcludesDirectCorrections(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.offering.rows = []*enrollmentService.OfferingChangeHistoryItem{
		aggOfferingHistory(1, "Cem Can", "approved", aggBase.Add(2*time.Hour)),
	}
	fakes.offering.corrections = []*enrollmentService.DirectCorrectionItem{
		aggDirectCorrection(7, "Anna Alt", aggBase.Add(3*time.Hour)),
	}

	// Corrections carry no status, so a status filter is about requests only.
	rr, page := execAggregated(t, rs, "view=history&status=approved", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"offering"}, aggTypes(page))
}

func TestAggregatedChangeRequests_AbsenceOnlyCallerSeesNoDirectCorrections(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.offering.corrections = []*enrollmentService.DirectCorrectionItem{
		aggDirectCorrection(7, "Anna Alt", aggBase.Add(3*time.Hour)),
	}

	rr, page := execAggregated(t, rs, "view=history", []string{"users:absence"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Empty(t, aggTypes(page))
	assert.Zero(t, fakes.offering.correctionsCalls)
}
