package requestreviewhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	reviewbindings "github.com/moto-nrw/project-phoenix/roots/requestreviewbinding"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keysetPage mimics the real services' contract: rows sorted newest-first,
// page strictly before (BeforeInstant, BeforeID), the child filters applied in
// the query, limit+1 probe for the next cursor. Fakes that ignored any of
// those would make the cursor assertions in this file meaningless — the SQL
// that really does it is covered by the router tests in
// change_request_list_db_test.go.
func keysetPage[T any](
	rows []T,
	filters excusedrequests.QueueFilter,
	key func(T) (time.Time, int64),
	child func(T) (studentID int64, name string),
) ([]T, *excusedrequests.Cursor) {
	matched := make([]T, 0, len(rows))
	for _, row := range rows {
		instant, id := key(row)
		if !filters.BeforeInstant.IsZero() &&
			(instant.After(filters.BeforeInstant) || (instant.Equal(filters.BeforeInstant) && id >= filters.BeforeID)) {
			continue
		}
		studentID, name := child(row)
		if filters.StudentID != 0 && studentID != filters.StudentID {
			continue
		}
		if filters.Search != "" && !containsFold(name, filters.Search) {
			continue
		}
		matched = append(matched, row)
	}
	if filters.Limit <= 0 || len(matched) <= filters.Limit {
		return matched, nil
	}
	matched = matched[:filters.Limit]
	instant, id := key(matched[len(matched)-1])
	return matched, &excusedrequests.Cursor{UpdatedAt: instant, ID: id}
}

func urgencyRows[T any](rows []T, filters excusedrequests.QueueFilter, urgent func(T) bool) []T {
	if filters.UrgentOnly == nil {
		return rows
	}
	filtered := make([]T, 0, len(rows))
	for _, row := range rows {
		if urgent(row) == *filters.UrgentOnly {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

type aggMasterFake struct {
	pending      []*masterdatarequests.ReviewItem
	rows         []*masterdatarequests.HistoryItem
	pendingCalls int
}

func (f *aggMasterFake) ListPending(_ context.Context, filter excusedrequests.QueueFilter) ([]*masterdatarequests.ReviewItem, *excusedrequests.Cursor, error) {
	filters := legacyQueueFilters(filter)
	f.pendingCalls++
	rows := urgencyRows(f.pending, filters, func(*masterdatarequests.ReviewItem) bool { return false })
	items, next := keysetPage(rows, filters,
		func(it *masterdatarequests.ReviewItem) (time.Time, int64) {
			return it.Request.CreatedAt, it.Request.ID
		},
		func(it *masterdatarequests.ReviewItem) (int64, string) {
			return it.Request.StudentID, it.FirstName + " " + it.LastName
		})
	return items, excusedCursor(next), nil
}

func (f *aggMasterFake) ListHistory(_ context.Context, filter excusedrequests.QueueFilter) ([]*masterdatarequests.HistoryItem, *excusedrequests.Cursor, error) {
	filters := legacyQueueFilters(filter)
	items, next := keysetPage(f.rows, filters,
		func(it *masterdatarequests.HistoryItem) (time.Time, int64) {
			return it.Request.UpdatedAt, it.Request.ID
		},
		func(it *masterdatarequests.HistoryItem) (int64, string) {
			return it.Request.StudentID, it.FirstName + " " + it.LastName
		})
	return items, excusedCursor(next), nil
}

type aggCareFake struct {
	pending      []*careplan.CareScheduleReviewItem
	rows         []*careplan.CareScheduleHistoryItem
	pendingCalls int
}

func (f *aggCareFake) ListPending(_ context.Context, nativeFilter careplan.RequestQueueFilter) ([]*careplan.CareScheduleReviewItem, *careplan.RequestCursor, error) {
	filters := legacyQueueFilters(nativeFilter)
	f.pendingCalls++
	rows := urgencyRows(f.pending, filters, func(it *careplan.CareScheduleReviewItem) bool {
		return careplan.CareReviewUrgentOn(*it, careplan.Date(calendar.TodayDate()))
	})
	items, next := keysetPage(rows, filters,
		func(it *careplan.CareScheduleReviewItem) (time.Time, int64) {
			return it.Request.CreatedAt, it.Request.ID
		},
		func(it *careplan.CareScheduleReviewItem) (int64, string) {
			return it.Request.StudentID, it.FirstName + " " + it.LastName
		})
	return items, excusedCursor(next), nil
}

func (f *aggCareFake) ListHistory(_ context.Context, nativeFilter careplan.RequestQueueFilter) ([]*careplan.CareScheduleHistoryItem, *careplan.RequestCursor, error) {
	filters := legacyQueueFilters(nativeFilter)
	items, next := keysetPage(f.rows, filters,
		func(it *careplan.CareScheduleHistoryItem) (time.Time, int64) {
			return it.Request.UpdatedAt, it.Request.ID
		},
		func(it *careplan.CareScheduleHistoryItem) (int64, string) {
			return it.Request.StudentID, it.FirstName + " " + it.LastName
		})
	return items, excusedCursor(next), nil
}

type aggOfferingFake struct {
	pending          []*careplan.OfferingReviewItem
	rows             []*careplan.OfferingHistoryItem
	corrections      []requestreview.Row
	pendingCalls     int
	correctionsCalls int
}

func (f *aggOfferingFake) PendingCount(context.Context, careplan.Date) (int, error) {
	return len(f.pending), nil
}

func (f *aggOfferingFake) History(_ context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	f.correctionsCalls++
	filters := excusedrequests.QueueFilter{Limit: filter.Limit, StudentID: filter.StudentID, StudentIDs: filter.StudentIDs, Search: filter.Search}
	if filter.Before != nil {
		filters.BeforeID, filters.BeforeInstant = filter.Before.ID, filter.Before.Instant
	}
	items, next := keysetPage(f.corrections, filters, func(row requestreview.Row) (time.Time, int64) { return row.SortTime, row.ID }, func(row requestreview.Row) (int64, string) { return row.StudentID, row.StudentName })
	if next == nil {
		return items, nil, nil
	}
	return items, &requestreview.Cursor{ID: next.ID, Instant: next.UpdatedAt}, nil
}

func (f *aggOfferingFake) ListPending(_ context.Context, nativeFilter careplan.RequestQueueFilter) ([]*careplan.OfferingReviewItem, *careplan.RequestCursor, error) {
	filters := legacyQueueFilters(nativeFilter)
	f.pendingCalls++
	rows := urgencyRows(f.pending, filters, func(item *careplan.OfferingReviewItem) bool {
		return !calendar.Date(item.Request.EffectiveFrom).After(calendar.TodayDate())
	})
	items, next := keysetPage(rows, filters,
		func(it *careplan.OfferingReviewItem) (time.Time, int64) {
			return it.Request.CreatedAt, it.Request.ID
		},
		func(it *careplan.OfferingReviewItem) (int64, string) {
			return it.Request.StudentID, it.StudentName
		})
	return items, excusedCursor(next), nil
}

func (f *aggOfferingFake) ListHistory(_ context.Context, nativeFilter careplan.RequestQueueFilter) ([]*careplan.OfferingHistoryItem, *careplan.RequestCursor, error) {
	filters := legacyQueueFilters(nativeFilter)
	items, next := keysetPage(f.rows, filters,
		func(it *careplan.OfferingHistoryItem) (time.Time, int64) {
			return it.Request.UpdatedAt, it.Request.ID
		},
		func(it *careplan.OfferingHistoryItem) (int64, string) {
			return it.Request.StudentID, it.StudentName
		})
	return items, excusedCursor(next), nil
}

type aggExcusedFake struct {
	excusedrequests.Service
	pending      []*excusedrequests.ReviewItem
	rows         []*excusedrequests.HistoryItem
	pendingCalls int
}

func (f *aggExcusedFake) ListPending(_ context.Context, filter excusedrequests.QueueFilter) ([]*excusedrequests.ReviewItem, *excusedrequests.Cursor, error) {
	f.pendingCalls++
	filters := legacyQueueFilters(filter)
	rows := urgencyRows(f.pending, filters, func(item *excusedrequests.ReviewItem) bool {
		return slices.Contains(item.Request.Dates, excusedrequests.Date(calendar.TodayDate()))
	})
	items, next := keysetPage(rows, filters,
		func(it *excusedrequests.ReviewItem) (time.Time, int64) {
			return it.Request.CreatedAt, it.Request.ID
		},
		func(it *excusedrequests.ReviewItem) (int64, string) {
			return it.Request.StudentID, it.FirstName + " " + it.LastName
		})
	return items, excusedCursor(next), nil
}

func (f *aggExcusedFake) ListHistory(_ context.Context, filter excusedrequests.QueueFilter) ([]*excusedrequests.HistoryItem, *excusedrequests.Cursor, error) {
	items, next := keysetPage(f.rows, legacyQueueFilters(filter),
		func(it *excusedrequests.HistoryItem) (time.Time, int64) {
			return it.Request.UpdatedAt, it.Request.ID
		},
		func(it *excusedrequests.HistoryItem) (int64, string) {
			return it.Request.StudentID, it.FirstName + " " + it.LastName
		})
	return items, excusedCursor(next), nil
}

// legacyQueueFilters and excusedCursor bridge the Care Plan queue vocabulary
// to the shared fake paging helpers above.
func legacyQueueFilters(filter excusedrequests.QueueFilter) excusedrequests.QueueFilter {
	return filter
}

func excusedCursor(next *excusedrequests.Cursor) *excusedrequests.Cursor {
	return next
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
	// The list route reads the shared request-review projection (#2705);
	// the fakes stand in for the retained owner queues it adapts.
	access, err := reviewbindings.NewAccess(RequestReviewPrincipal, nil)
	if err != nil {
		panic(err)
	}
	careQueue, err := reviewbindings.NewCareScheduleQueue(fakes.care, func() careplan.Date { return careplan.Date(calendar.TodayDate()) })
	if err != nil {
		panic(err)
	}
	offeringQueue, err := reviewbindings.NewOfferingQueue(fakes.offering, func() careplan.Date { return careplan.Date(calendar.TodayDate()) })
	if err != nil {
		panic(err)
	}
	masterQueue, err := reviewbindings.NewMasterDataQueue(fakes.master)
	if err != nil {
		panic(err)
	}
	excusedQueue, err := reviewbindings.NewExcusedQueue(fakes.excused, func() careplan.Date { return careplan.Date(calendar.TodayDate()) })
	if err != nil {
		panic(err)
	}
	review, err := requestreview.NewChecked(requestreview.Dependencies{
		Queues: requestreview.Queues{DirectCorrections: fakes.offering, MasterData: masterQueue, CareSchedule: careQueue, Offering: offeringQueue, Excused: excusedQueue},
		Access: access,
	})
	if err != nil {
		panic(err)
	}
	rs := testResource(ResourceConfig{

		RequestReview: review,
	})
	return rs, fakes
}

var aggBase = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

func aggMasterPending(id int64, first, last string, createdAt time.Time) *masterdatarequests.ReviewItem {
	req := &masterdatarequests.Request{ID: id, CreatedAt: createdAt, UpdatedAt: createdAt,
		StudentID: 100 + id,
		FieldKey:  "first_name",
		NewValue:  json.RawMessage(`"Neu"`),
		Status:    "pending",
	}
	return &masterdatarequests.ReviewItem{Request: req, FirstName: first, LastName: last}
}

func aggCarePending(id int64, first, last string, createdAt time.Time) *careplan.CareScheduleReviewItem {
	req := &careplan.CareScheduleChangeRequest{
		ID: id, CreatedAt: createdAt, UpdatedAt: createdAt,
		StudentID:   200 + id,
		RequestKind: "weekly_schedule",
		Status:      "pending",
	}
	return &careplan.CareScheduleReviewItem{
		Request: req, FirstName: first, LastName: last,
		ImpactAvailable: true,
		AffectedBlocks: []careplan.CareReviewBlock{{
			ID:        81,
			Title:     "Nachmittags-AG",
			StartTime: time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2000, 1, 1, 16, 0, 0, 0, time.UTC),
		}},
	}
}

func aggOfferingPending(id int64, name string, createdAt time.Time) *careplan.OfferingReviewItem {
	req := &careplan.OfferingChangeRequest{
		ID: id, CreatedAt: createdAt, UpdatedAt: createdAt,
		StudentID: 300 + id,
		Status:    "pending",
	}
	return &careplan.OfferingReviewItem{Request: req, StudentName: name}
}

func aggExcusedPending(id int64, first, last string, createdAt time.Time) *excusedrequests.ReviewItem {
	req := &excusedrequests.Request{
		ID: id, CreatedAt: createdAt, UpdatedAt: createdAt,
		StudentID: 400 + id,
		Status:    "pending",
	}
	return &excusedrequests.ReviewItem{Request: req, FirstName: first, LastName: last}
}

// aggRequest builds a request whose context carries staff claims plus the
// given permissions, mirroring what the router middleware injects.
func aggRequest(t *testing.T, rawQuery string, perms []string) *http.Request {
	t.Helper()
	req := httptest.NewRequest("GET", "/change-requests", nil)
	if rawQuery != "" {
		req.URL.RawQuery = rawQuery
	}
	ctx := context.WithValue(req.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 55})
	ctx = context.WithValue(ctx, sessiontoken.CtxPermissions, perms)
	return req.WithContext(ctx)
}

type aggPage struct {
	Items []struct {
		RequestType string          `json:"request_type"`
		UrgentToday bool            `json:"urgent_today"`
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

func assertCareAffectedBlocks(t *testing.T, data json.RawMessage) {
	t.Helper()
	var care struct {
		AffectedBlocks []requestreview.AffectedCareBlock `json:"affected_blocks"`
		ImpactToken    string                            `json:"impact_token"`
	}
	require.NoError(t, json.Unmarshal(data, &care))
	require.Len(t, care.AffectedBlocks, 1)
	assert.Equal(t, requestreview.AffectedCareBlock{
		ID: "81", Title: "Nachmittags-AG", StartTime: "15:00", EndTime: "16:00",
	}, care.AffectedBlocks[0])
	assert.Equal(t, "c3fe58ec4e34cd0f441911e13e7bcc0bca6a22cfc9fa6a231afed828fe6ed1f1", care.ImpactToken)
}

func TestAggregatedChangeRequests_OpenMergesAllTypesNewestFirst(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.pending = []*masterdatarequests.ReviewItem{aggMasterPending(1, "Anna", "Alt", aggBase.Add(3*time.Hour))}
	fakes.care.pending = []*careplan.CareScheduleReviewItem{aggCarePending(2, "Ben", "Berg", aggBase.Add(1*time.Hour))}
	fakes.offering.pending = []*careplan.OfferingReviewItem{aggOfferingPending(3, "Cem Can", aggBase.Add(4*time.Hour))}
	fakes.excused.pending = []*excusedrequests.ReviewItem{aggExcusedPending(4, "Dua", "Deml", aggBase.Add(2*time.Hour))}

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

	assertCareAffectedBlocks(t, page.Items[3].Data)
}

func TestAggregatedChangeRequests_OpenPaginatesAllUrgentRowsBeforeNewerNormalRows(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.pending = []*masterdatarequests.ReviewItem{
		aggMasterPending(1, "Neu", "Normal", aggBase.Add(48*time.Hour)),
	}
	urgent := aggExcusedPending(2, "Alt", "Dringend", aggBase)
	urgent.Request.Dates = []excusedrequests.Date{excusedrequests.Date(calendar.TodayDate())}
	fakes.excused.pending = []*excusedrequests.ReviewItem{urgent}

	rr, page := execAggregated(t, rs, "limit=1", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, []string{"excused"}, aggTypes(page))
	assert.True(t, page.Items[0].UrgentToday)
	require.NotEmpty(t, page.NextCursor)

	rr, page = execAggregated(t, rs, "limit=1&cursor="+url.QueryEscape(page.NextCursor), aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"master_data"}, aggTypes(page))
}

// A source can be prefetched to compare its head with the other queues yet
// contribute no row to this page. Its cursor must preserve that explicit
// start position; otherwise the next request loses the buffered state and
// later repeats the rows already returned by the winning source.
func TestAggregatedChangeRequests_CursorPreservesUnconsumedPrefetchedSource(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.pending = []*masterdatarequests.ReviewItem{
		aggMasterPending(1, "Anna", "Alt", aggBase.Add(2*time.Hour)),
		aggMasterPending(2, "Ben", "Berg", aggBase.Add(time.Hour)),
	}
	fakes.excused.pending = []*excusedrequests.ReviewItem{
		aggExcusedPending(3, "Cem", "Can", aggBase.Add(3*time.Hour)),
	}

	rr, page := execAggregated(t, rs, "limit=1", aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, []string{"excused"}, aggTypes(page))
	require.NotEmpty(t, page.NextCursor)

	rr, page = execAggregated(t, rs, "limit=1&cursor="+url.QueryEscape(page.NextCursor), aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"master_data"}, aggTypes(page))

	rr, page = execAggregated(t, rs, "limit=1&cursor="+url.QueryEscape(page.NextCursor), aggUpdatePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"master_data"}, aggTypes(page))
	assert.Empty(t, page.NextCursor)
}

func TestAggregatedChangeRequests_AbsenceOnlySeesOnlyExcused(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.master.pending = []*masterdatarequests.ReviewItem{aggMasterPending(1, "Anna", "Alt", aggBase)}
	fakes.excused.pending = []*excusedrequests.ReviewItem{aggExcusedPending(4, "Dua", "Deml", aggBase.Add(time.Hour))}

	rr, page := execAggregated(t, rs, "", []string{"users:read", "users:absence"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, []string{"excused"}, aggTypes(page))
	assert.Zero(t, fakes.master.pendingCalls, "users:update queues must not be queried for an absence-only caller")
	assert.Zero(t, fakes.care.pendingCalls)
	assert.Zero(t, fakes.offering.pendingCalls)
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

func aggDirectCorrection(id int64, name string, changedAt time.Time) requestreview.Row {
	return requestreview.Row{Type: requestreview.TypeDirectCorrection, ID: id, StudentID: 500 + id, StudentName: name, SortTime: changedAt, DecidedAt: changedAt,
		Data: requestreview.DirectCorrectionResponse{ID: fmt.Sprint(id), StudentID: fmt.Sprint(500 + id), StudentName: name, ChangedAt: changedAt, ChangedByName: "Olga Office", Reason: "Telefonisch gemeldet", Diff: []requestreview.OfferingRequestDiffResponse{{OfferingID: "9", Label: "Mittagessen", Old: "Mo", New: "abgemeldet"}}},
	}
}

func TestAggregatedChangeRequests_AbsenceOnlyCallerSeesNoDirectCorrections(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	fakes.offering.corrections = []requestreview.Row{
		aggDirectCorrection(7, "Anna Alt", aggBase.Add(3*time.Hour)),
	}

	rr, page := execAggregated(t, rs, "view=history", []string{"users:absence"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Empty(t, aggTypes(page))
	assert.Zero(t, fakes.offering.correctionsCalls)
}

// conflictPage decodes just the conflict/current-value fields, so this test
// keeps working when the shared aggPage grows.
type conflictPage struct {
	Items []struct {
		RequestType         string            `json:"request_type"`
		ConflictKeys        []string          `json:"conflict_keys"`
		ConflictKey         string            `json:"conflict_key"`
		ConflictGroupSize   int               `json:"conflict_group_size"`
		CurrentValueChanged *bool             `json:"current_value_changed"`
		CurrentStatusByDate map[string]string `json:"current_status_by_date"`
		Past                bool              `json:"past"`
	} `json:"items"`
	ReviewAccess string `json:"review_access"`
}

func execConflictPage(t *testing.T, rs *Resource, rawQuery string, perms []string) conflictPage {
	t.Helper()
	rr := httptest.NewRecorder()
	rs.listAggregatedChangeRequests(rr, aggRequest(t, rawQuery, perms))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var env struct {
		Data conflictPage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	return env.Data
}

// #2267 A12: two absence requests for the same child on the same day
// contradict each other and must be offered as ONE decision. Deciding them in
// sequence lets the second silently overwrite the first.
func TestAggregatedChangeRequests_GroupsContradictingAbsencesOnOneDay(t *testing.T) {
	t.Parallel()

	rs, fakes := newAggResource()
	day := calendar.NewDate(2026, 8, 24).AddDays(3)
	first := aggExcusedPending(1, "Mia", "Muster", aggBase)
	first.Request.Dates = []excusedrequests.Date{excusedrequests.Date(day)}
	first.Request.AbsenceStatus = "excused"
	second := aggExcusedPending(2, "Mia", "Muster", aggBase.Add(-time.Hour))
	second.Request.StudentID = first.Request.StudentID
	second.Request.Dates = []excusedrequests.Date{excusedrequests.Date(day)}
	second.Request.AbsenceStatus = "sick"
	fakes.excused.pending = []*excusedrequests.ReviewItem{first, second}

	page := execConflictPage(t, rs, "types=excused", aggUpdatePerms)
	require.Len(t, page.Items, 2)
	for _, item := range page.Items {
		assert.Equal(t, []string{"absence:" + day.String()}, item.ConflictKeys)
		assert.Equal(t, "absence:"+day.String(), item.ConflictKey,
			"both requests must group on the same key whatever status they ask for")
		assert.Equal(t, 2, item.ConflictGroupSize)
	}
}
