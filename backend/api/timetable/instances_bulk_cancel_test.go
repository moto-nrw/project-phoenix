package timetable

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
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// bulkCancelStub records the bulk cancellation the handler asks for (#3594).
type bulkCancelStub struct {
	*mockInstanceService
	calls  int
	from   timezone.Date
	to     timezone.Date
	dryRun bool
	result *timetableModule.BulkCancelResult
	err    error
}

func (s *bulkCancelStub) BulkCancelPlanned(_ context.Context, from, to timezone.Date, dryRun bool, _ *int64) (*timetableModule.BulkCancelResult, error) {
	s.calls++
	s.from, s.to, s.dryRun = from, to, dryRun
	return s.result, s.err
}

func bulkCancelRouter(t *testing.T, stub *bulkCancelStub) chi.Router {
	t.Helper()
	db, _ := testutil.SetupTimetableModule(t)
	router := chi.NewRouter()
	router.Mount("/timetable", NewResource(Dependencies{DB: db, InstanceService: stub}).Router())
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
	assert.Equal(t, timezone.NewDate(2026, 10, 12), stub.from)
	assert.Equal(t, timezone.NewDate(2026, 10, 16), stub.to)
	assert.True(t, stub.dryRun)

	var envelope struct {
		Data timetableModule.BulkCancelResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	assert.Equal(t, 3, envelope.Data.Count)
	assert.True(t, envelope.Data.DryRun)
	assert.Len(t, envelope.Data.Days, 2)
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
