package timetablehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// bulkCancelStub records the bulk cancellation the handler asks for (#3594).
type bulkCancelStub struct {
	*mockInstanceService
	calls  int
	from   calendar.Date
	to     calendar.Date
	opts   timetableModule.BulkCancelOptions
	result *timetableModule.BulkCancelResult
	err    error
}

func (s *bulkCancelStub) BulkCancelPlanned(_ context.Context, from, to calendar.Date, opts timetableModule.BulkCancelOptions, _ *int64) (*timetableModule.BulkCancelResult, error) {
	s.calls++
	s.from, s.to, s.opts = from, to, opts
	return s.result, s.err
}

func bulkCancelRouter(t *testing.T, stub *bulkCancelStub) chi.Router {
	t.Helper()
	_, _ = testutil.SetupTimetableModule(t)
	router := chi.NewRouter()
	router.Mount("/timetable", NewResource(Dependencies{InstanceService: stub}).Router())
	return router
}

func postBulkCancel(t *testing.T, router chi.Router, body string, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/timetable/instances/bulk-cancel", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	return testutil.ExecuteWithAuthPermissions(t, router, request, testutil.AdminTestClaims(999999), perms)
}

func TestBulkCancelRoute_RequiresSchedulesManage(t *testing.T) {
	t.Parallel()

	stub := &bulkCancelStub{mockInstanceService: &mockInstanceService{}}
	router := bulkCancelRouter(t, stub)

	response := postBulkCancel(t, router, `{"from":"2026-10-12","to":"2026-10-16","dry_run":true}`,
		[]string{permissions.SchedulesRead})

	assert.Equal(t, http.StatusForbidden, response.Code, "body=%s", response.Body.String())
	assert.Zero(t, stub.calls, "a read-only account never reaches the service")
}

func TestBulkCancelRoute_DryRunReturnsCounts(t *testing.T) {
	t.Parallel()

	stub := &bulkCancelStub{
		mockInstanceService: &mockInstanceService{},
		result: &timetableModule.BulkCancelResult{
			From: "2026-10-12", To: "2026-10-16", DryRun: true, Count: 3,
			Days: []timetableModule.BulkCancelDay{{Date: "2026-10-12", Count: 2}, {Date: "2026-10-13", Count: 1}},
		},
	}
	router := bulkCancelRouter(t, stub)

	response := postBulkCancel(t, router, `{"from":"2026-10-12","to":"2026-10-16","dry_run":true}`,
		[]string{permissions.SchedulesManage})

	require.Equal(t, http.StatusOK, response.Code, "body=%s", response.Body.String())
	assert.Equal(t, 1, stub.calls)
	assert.Equal(t, calendar.NewDate(2026, 10, 12), stub.from)
	assert.Equal(t, calendar.NewDate(2026, 10, 16), stub.to)
	assert.True(t, stub.opts.DryRun)
	assert.False(t, stub.opts.IncludeClosingDaySeries, "series planned on closing days stay by default")

	var envelope struct {
		Data timetableModule.BulkCancelResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	assert.Equal(t, 3, envelope.Data.Count)
	assert.True(t, envelope.Data.DryRun)
	assert.Len(t, envelope.Data.Days, 2)
}

func TestBulkCancelRoute_IncludesClosingDaySeriesOnRequest(t *testing.T) {
	t.Parallel()

	stub := &bulkCancelStub{
		mockInstanceService: &mockInstanceService{},
		result: &timetableModule.BulkCancelResult{
			From: "2026-10-12", To: "2026-10-16", DryRun: true, Count: 0, Days: []timetableModule.BulkCancelDay{},
			Kept: 9, KeptSeries: []timetableModule.BulkCancelKeptSeries{{Name: "Ferienbetreuung", Count: 5}, {Name: "Ferienspiele", Count: 4}},
		},
	}
	router := bulkCancelRouter(t, stub)

	response := postBulkCancel(t, router,
		`{"from":"2026-10-12","to":"2026-10-16","dry_run":true,"include_closing_day_series":true}`,
		[]string{permissions.SchedulesManage})

	require.Equal(t, http.StatusOK, response.Code, "body=%s", response.Body.String())
	assert.Equal(t, timetableModule.BulkCancelOptions{DryRun: true, IncludeClosingDaySeries: true}, stub.opts)

	var envelope struct {
		Data struct {
			Kept       int `json:"kept"`
			KeptSeries []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"kept_series"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	assert.Equal(t, 9, envelope.Data.Kept)
	require.Len(t, envelope.Data.KeptSeries, 2)
	assert.Equal(t, "Ferienbetreuung", envelope.Data.KeptSeries[0].Name)
	assert.Equal(t, 5, envelope.Data.KeptSeries[0].Count)
}

func TestBulkCancelRoute_RejectsInvalidRanges(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body       string
		serviceErr error
		wantCalls  int
	}{
		"missing to":      {body: `{"from":"2026-10-12"}`},
		"malformed date":  {body: `{"from":"12.10.2026","to":"2026-10-16"}`},
		"range too long":  {body: `{"from":"2026-01-01","to":"2027-06-01"}`, serviceErr: fmt.Errorf("%w: too long", timetableModule.ErrInvalidBulkCancelRange), wantCalls: 1},
		"reversed bounds": {body: `{"from":"2026-10-16","to":"2026-10-12"}`, serviceErr: timetableModule.ErrInvalidBulkCancelRange, wantCalls: 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stub := &bulkCancelStub{mockInstanceService: &mockInstanceService{}, err: tc.serviceErr}
			router := bulkCancelRouter(t, stub)

			response := postBulkCancel(t, router, tc.body, []string{permissions.SchedulesManage})

			assert.Equal(t, http.StatusBadRequest, response.Code, "body=%s", response.Body.String())
			assert.Equal(t, tc.wantCalls, stub.calls)
		})
	}
}
