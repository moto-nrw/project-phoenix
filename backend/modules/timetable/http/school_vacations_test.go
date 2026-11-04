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
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// schoolVacationStub records what the vacation routes (#5345) ask for.
type schoolVacationStub struct {
	list            schoolcalendar.SchoolVacationList
	listFrom        string
	listTo          string
	acknowledged    []schoolcalendar.SchoolVacationAcknowledgement
	acknowledgeErr  error
	hints           []timetableModule.SchoolVacationHint
	hintCalls       int
	acknowledgeCall int
	seeded          []schoolcalendar.SchoolVacation
	local           bool
}

func (s *schoolVacationStub) TenantSchoolVacations(_ context.Context, from, to string) (schoolcalendar.SchoolVacationList, error) {
	s.listFrom, s.listTo = from, to
	return s.list, nil
}

func (s *schoolVacationStub) AcknowledgeSchoolVacation(_ context.Context, acknowledgement schoolcalendar.SchoolVacationAcknowledgement) error {
	s.acknowledgeCall++
	s.acknowledged = append(s.acknowledged, acknowledgement)
	return s.acknowledgeErr
}

func (s *schoolVacationStub) SeedSchoolVacations(_ context.Context, vacations []schoolcalendar.SchoolVacation) (int, error) {
	s.seeded = append(s.seeded, vacations...)
	return len(vacations) * 16, nil
}

func (s *schoolVacationStub) SchoolVacationHints(context.Context) ([]timetableModule.SchoolVacationHint, error) {
	s.hintCalls++
	return s.hints, nil
}

func schoolVacationRouter(t *testing.T, stub *schoolVacationStub) chi.Router {
	t.Helper()
	_, _ = RootCompositions.Module(t)
	router := chi.NewRouter()
	isLocal := func(*http.Request) bool { return stub.local }
	router.Mount("/timetable", newResourceForTest(Dependencies{SchoolVacations: stub, SchoolVacationHints: stub, IsLocalSeed: isLocal}).
		Router(scopeTestTenantRoutes(testpkg.TenantSessionGatesWithCaches(scopeTestSettingsCache))))
	return router
}

func callSchoolVacations(t *testing.T, router chi.Router, method, path, body string, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	return testutil.ExecuteWithAuthPermissions(t, router, request, testutil.AdminTestClaims(999999), perms)
}

func TestSchoolVacationRoutes_ListReturnsTheStoredVacations(t *testing.T) {
	t.Parallel()
	stub := &schoolVacationStub{list: schoolcalendar.SchoolVacationList{
		Region:    schoolcalendar.HolidayRegion{Code: "DE-NW", Name: "Nordrhein-Westfalen"},
		Vacations: []schoolcalendar.SchoolVacation{{Name: "Weihnachtsferien", StartDate: "2026-12-23", EndDate: "2027-01-06"}},
	}}
	router := schoolVacationRouter(t, stub)

	response := callSchoolVacations(t, router, http.MethodGet, "/timetable/school-vacations?from=2026-08-01&to=2027-07-31", "", []string{permissions.SchedulesRead})

	require.Equal(t, http.StatusOK, response.Code, "body=%s", response.Body.String())
	assert.Equal(t, "2026-08-01", stub.listFrom)
	assert.Equal(t, "2027-07-31", stub.listTo)
	var envelope struct {
		Data SchoolVacationsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	assert.Equal(t, "Nordrhein-Westfalen", envelope.Data.FederalStateName)
	assert.Nil(t, envelope.Data.SyncedAt, "not read yet stays visible as null")
	require.Len(t, envelope.Data.Vacations, 1)
	assert.Equal(t, "Weihnachtsferien", envelope.Data.Vacations[0].Name)
}

func TestSchoolVacationRoutes_HintsAndConfirmationNeedSchedulesManage(t *testing.T) {
	t.Parallel()
	stub := &schoolVacationStub{}
	router := schoolVacationRouter(t, stub)

	hints := callSchoolVacations(t, router, http.MethodGet, "/timetable/school-vacation-hints", "", []string{permissions.SchedulesRead})
	confirm := callSchoolVacations(t, router, http.MethodPost, "/timetable/school-vacation-hints/acknowledgements",
		`{"start_date":"2026-12-23","end_date":"2027-01-06"}`, []string{permissions.SchedulesRead})

	assert.Equal(t, http.StatusForbidden, hints.Code)
	assert.Equal(t, http.StatusForbidden, confirm.Code)
	assert.Zero(t, stub.hintCalls)
	assert.Zero(t, stub.acknowledgeCall)
}

func TestSchoolVacationRoutes_HintsListTheCountedVacations(t *testing.T) {
	t.Parallel()
	stub := &schoolVacationStub{hints: []timetableModule.SchoolVacationHint{
		{Name: "Weihnachtsferien", StartDate: "2026-12-23", EndDate: "2027-01-06", CancelFrom: "2026-12-23", Count: 12},
	}}
	router := schoolVacationRouter(t, stub)

	response := callSchoolVacations(t, router, http.MethodGet, "/timetable/school-vacation-hints", "", []string{permissions.SchedulesManage})

	require.Equal(t, http.StatusOK, response.Code, "body=%s", response.Body.String())
	var envelope struct {
		Data SchoolVacationHintsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	assert.Equal(t, stub.hints, envelope.Data.Hints)
}

func TestSchoolVacationRoutes_ConfirmationOutcomes(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		body   string
		err    error
		status int
		code   string
	}{
		"confirmed":      {body: `{"start_date":"2026-12-23","end_date":"2027-01-06"}`, status: http.StatusOK},
		"missing dates":  {body: `{"start_date":"2026-12-23"}`, status: http.StatusBadRequest},
		"malformed":      {body: `{"start_date":"x","end_date":"y"}`, err: fmt.Errorf("%w: bad", schoolcalendar.ErrInvalidSchoolVacationWindow), status: http.StatusBadRequest},
		"unknown window": {body: `{"start_date":"2026-12-21","end_date":"2027-01-02"}`, err: fmt.Errorf("%w: other state", schoolcalendar.ErrSchoolVacationNotFound), status: http.StatusNotFound, code: "timetable.school_vacation_not_found"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stub := &schoolVacationStub{acknowledgeErr: tc.err}
			router := schoolVacationRouter(t, stub)

			response := callSchoolVacations(t, router, http.MethodPost, "/timetable/school-vacation-hints/acknowledgements", tc.body, []string{permissions.SchedulesManage})

			require.Equal(t, tc.status, response.Code, "body=%s", response.Body.String())
			if tc.code != "" {
				var problem struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
				assert.Equal(t, tc.code, problem.Code)
			}
			if name == "confirmed" {
				require.Len(t, stub.acknowledged, 1)
				assert.Equal(t, "2026-12-23", stub.acknowledged[0].StartDate)
				assert.Equal(t, "2027-01-06", stub.acknowledged[0].EndDate)
			}
		})
	}
}

func TestSchoolVacationRoutes_SeedOnlyForTheLocalSeeder(t *testing.T) {
	t.Parallel()
	body := `{"vacations":[{"name":"Herbstferien","start_date":"2026-10-19","end_date":"2026-10-30"}]}`

	remote := &schoolVacationStub{}
	response := callSchoolVacations(t, schoolVacationRouter(t, remote), http.MethodPost, "/timetable/school-vacations/seed", body, []string{permissions.SchedulesManage})
	assert.Equal(t, http.StatusNotFound, response.Code, "body=%s", response.Body.String())
	assert.Empty(t, remote.seeded, "outside the local seeder nothing is stored")

	local := &schoolVacationStub{local: true}
	response = callSchoolVacations(t, schoolVacationRouter(t, local), http.MethodPost, "/timetable/school-vacations/seed", body, []string{permissions.SchedulesManage})
	require.Equal(t, http.StatusCreated, response.Code, "body=%s", response.Body.String())
	require.Len(t, local.seeded, 1)
	assert.Equal(t, "Herbstferien", local.seeded[0].Name)
}
