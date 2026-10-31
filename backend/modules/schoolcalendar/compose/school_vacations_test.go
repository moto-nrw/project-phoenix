package compose

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The sync's calendar day: the running school year started on 2026-08-01.
const vacationToday = "2026-10-31"

const firstSyncBody = `[
 {"startDate":"2026-12-23","endDate":"2027-01-06","type":"School","name":[{"language":"DE","text":"Weihnachtsferien"}],"subdivisions":[{"code":"DE-NW"}]},
 {"startDate":"2027-03-22","endDate":"2027-04-03","type":"School","name":[{"language":"DE","text":"Osterferien"}],"subdivisions":[{"code":"DE-NW"}]},
 {"startDate":"2026-12-21","endDate":"2027-01-02","type":"School","name":[{"language":"DE","text":"Weihnachtsferien"}],"subdivisions":[{"code":"DE-BY"}]}
]`

// The second answer moves the NRW Easter vacation and names Bavaria not at
// all: NRW is replaced, Bavaria keeps its rows.
const secondSyncBody = `[
 {"startDate":"2026-12-23","endDate":"2027-01-06","type":"School","name":[{"language":"DE","text":"Weihnachtsferien"}],"subdivisions":[{"code":"DE-NW"}]},
 {"startDate":"2027-03-29","endDate":"2027-04-10","type":"School","name":[{"language":"DE","text":"Osterferien"}],"subdivisions":[{"code":"DE-NW"}]}
]`

// vacationSource is an OpenHolidays stand-in whose answer the test switches.
type vacationSource struct {
	server *httptest.Server
	body   atomic.Value
	status atomic.Int32
	calls  atomic.Int32
}

func newVacationSource(t *testing.T, body string) *vacationSource {
	t.Helper()
	source := &vacationSource{}
	source.answer(http.StatusOK, body)
	source.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		source.calls.Add(1)
		w.WriteHeader(int(source.status.Load()))
		_, _ = w.Write([]byte(source.body.Load().(string)))
	}))
	t.Cleanup(source.server.Close)
	return source
}

func (s *vacationSource) answer(status int, body string) {
	s.status.Store(int32(status))
	s.body.Store(body)
}

type vacationSchool struct {
	ctx    context.Context
	region string
}

func vacationRegion(schools map[int64]string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) { return schools[tenant.FromContext(ctx)], nil }
}

// vacationFixture builds the school vacations over an isolated database: the
// vacations table has no school, so no other test may see its rows.
func vacationFixture(t *testing.T, body string, regions ...string) (schoolcalendar.SchoolVacationOwner, *vacationSource, context.Context, []vacationSchool, *bun.DB) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	runtime := testpkg.WithTestTenantRuntime(t, context.Background())
	byTenant := map[int64]string{}
	schools := make([]vacationSchool, 0, len(regions))
	for _, region := range regions {
		id := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, id)
		byTenant[id] = region
		schools = append(schools, vacationSchool{ctx: tenant.WithTenantID(runtime, id), region: region})
	}
	source := newVacationSource(t, body)
	vacations, err := NewSchoolVacations(vacationRegion(byTenant), source.server.URL, func() string { return vacationToday })
	require.NoError(t, err)
	return vacations, source, runtime, schools, db
}

func listVacations(t *testing.T, vacations schoolcalendar.SchoolVacations, school vacationSchool) schoolcalendar.SchoolVacationList {
	t.Helper()
	var list schoolcalendar.SchoolVacationList
	require.NoError(t, tenant.WithinCurrentTenant(school.ctx, func(txCtx context.Context) error {
		var err error
		list, err = vacations.TenantSchoolVacations(txCtx, "2026-08-01", "2027-07-31")
		return err
	}))
	return list
}

func acknowledge(school vacationSchool, vacations schoolcalendar.SchoolVacations, start, end string) error {
	return tenant.WithinCurrentTenant(school.ctx, func(txCtx context.Context) error {
		return vacations.AcknowledgeSchoolVacation(txCtx, schoolcalendar.SchoolVacationAcknowledgement{StartDate: start, EndDate: end})
	})
}

func vacationDates(list schoolcalendar.SchoolVacationList) []string {
	dates := make([]string, 0, len(list.Vacations))
	for _, vacation := range list.Vacations {
		dates = append(dates, vacation.Name+" "+vacation.StartDate+" "+vacation.EndDate)
	}
	return dates
}

func TestSchoolVacationSync_StoresReplacesAndKeepsOnFailure(t *testing.T) {
	t.Parallel()
	vacations, source, runtime, schools, _ := vacationFixture(t, firstSyncBody, "DE-NW", "DE-BY")
	nrw, bavaria := schools[0], schools[1]

	before := listVacations(t, vacations, nrw)
	assert.Nil(t, before.SyncedAt, "before the first sync the state's vacations are not read yet")
	assert.Empty(t, before.Vacations)
	assert.Equal(t, schoolcalendar.HolidayRegion{Code: "DE-NW", Name: "Nordrhein-Westfalen"}, before.Region)

	result, err := vacations.SyncSchoolVacations(runtime)
	require.NoError(t, err)
	assert.Equal(t, schoolcalendar.SchoolVacationSyncResult{Regions: 2, Vacations: 3}, result)
	first := listVacations(t, vacations, nrw)
	require.NotNil(t, first.SyncedAt)
	assert.Equal(t, []string{"Weihnachtsferien 2026-12-23 2027-01-06", "Osterferien 2027-03-22 2027-04-03"}, vacationDates(first))
	assert.Equal(t, []string{"Weihnachtsferien 2026-12-21 2027-01-02"}, vacationDates(listVacations(t, vacations, bavaria)))

	source.answer(http.StatusOK, secondSyncBody)
	_, err = vacations.SyncSchoolVacations(runtime)
	require.NoError(t, err)
	assert.Equal(t, []string{"Weihnachtsferien 2026-12-23 2027-01-06", "Osterferien 2027-03-29 2027-04-10"}, vacationDates(listVacations(t, vacations, nrw)),
		"a repeated sync replaces the state's windows instead of adding to them")
	assert.Equal(t, []string{"Weihnachtsferien 2026-12-21 2027-01-02"}, vacationDates(listVacations(t, vacations, bavaria)),
		"a state the answer does not name keeps its vacations")

	source.answer(http.StatusInternalServerError, `{"title":"down"}`)
	_, err = vacations.SyncSchoolVacations(runtime)
	require.Error(t, err)
	assert.Equal(t, []string{"Weihnachtsferien 2026-12-23 2027-01-06", "Osterferien 2027-03-29 2027-04-10"}, vacationDates(listVacations(t, vacations, nrw)),
		"a failed request keeps the stored vacations")
	assert.Equal(t, int32(3), source.calls.Load())
}

func TestSchoolVacationAcknowledgement_HidesOnlyThatWindowForOnlyThatSchool(t *testing.T) {
	t.Parallel()
	vacations, _, runtime, schools, _ := vacationFixture(t, firstSyncBody, "DE-NW", "DE-NW")
	first, second := schools[0], schools[1]
	_, err := vacations.SyncSchoolVacations(runtime)
	require.NoError(t, err)

	require.NoError(t, acknowledge(first, vacations, "2026-12-23", "2027-01-06"))
	require.NoError(t, acknowledge(first, vacations, "2026-12-23", "2027-01-06"), "confirming twice keeps the first record")

	acknowledged := func(list schoolcalendar.SchoolVacationList) map[string]bool {
		result := map[string]bool{}
		for _, vacation := range list.Vacations {
			result[vacation.Name] = vacation.Acknowledged
		}
		return result
	}
	assert.Equal(t, map[string]bool{"Weihnachtsferien": true, "Osterferien": false}, acknowledged(listVacations(t, vacations, first)),
		"only the confirmed window")
	assert.Equal(t, map[string]bool{"Weihnachtsferien": false, "Osterferien": false}, acknowledged(listVacations(t, vacations, second)),
		"another school of the same state keeps its hint")
}

func TestSchoolVacationAcknowledgement_RejectsUnknownAndMalformedWindows(t *testing.T) {
	t.Parallel()
	vacations, _, runtime, schools, _ := vacationFixture(t, firstSyncBody, "DE-NW")
	_, err := vacations.SyncSchoolVacations(runtime)
	require.NoError(t, err)

	assert.ErrorIs(t, acknowledge(schools[0], vacations, "2026-12-21", "2027-01-02"), schoolcalendar.ErrSchoolVacationNotFound,
		"the Bavarian window is not a vacation of an NRW school")
	assert.ErrorIs(t, acknowledge(schools[0], vacations, "2027-01-06", "2026-12-23"), schoolcalendar.ErrInvalidSchoolVacationWindow)
	assert.ErrorIs(t, acknowledge(schools[0], vacations, "23.12.2026", "2027-01-06"), schoolcalendar.ErrInvalidSchoolVacationWindow)
}

func TestSchoolVacations_WithoutFederalStateServesOnlyTheSync(t *testing.T) {
	t.Parallel()
	_, source, runtime, schools, _ := vacationFixture(t, firstSyncBody, "DE-NW")
	syncOnly, err := NewSchoolVacations(nil, source.server.URL, func() string { return vacationToday })
	require.NoError(t, err)

	result, err := syncOnly.SyncSchoolVacations(runtime)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Vacations)
	err = tenant.WithinCurrentTenant(schools[0].ctx, func(txCtx context.Context) error {
		_, readErr := syncOnly.TenantSchoolVacations(txCtx, "2026-08-01", "2027-07-31")
		return readErr
	})
	assert.ErrorIs(t, err, schoolcalendar.ErrFederalStateUnavailable)
}

func TestSchoolVacationSeed_StoresTheWindowsForEveryStateWithoutTheSource(t *testing.T) {
	t.Parallel()
	vacations, source, runtime, schools, _ := vacationFixture(t, firstSyncBody, "DE-NW", "DE-SH")
	seed := []schoolcalendar.SchoolVacation{
		{Name: "Herbstferien", StartDate: "2026-10-19", EndDate: "2026-10-30"},
		{Name: "Weihnachtsferien", StartDate: "2026-12-23", EndDate: "2027-01-06"},
	}

	stored, err := vacations.SeedSchoolVacations(runtime, seed)
	require.NoError(t, err)
	assert.Equal(t, 32, stored, "two windows for each of the 16 states")
	_, err = vacations.SeedSchoolVacations(runtime, seed)
	require.NoError(t, err)

	for _, school := range schools {
		assert.Equal(t, []string{"Herbstferien 2026-10-19 2026-10-30", "Weihnachtsferien 2026-12-23 2027-01-06"}, vacationDates(listVacations(t, vacations, school)),
			"a repeated seed replaces its windows, %s", school.region)
	}
	assert.Zero(t, source.calls.Load(), "the seed never asks the external source")
	_, err = vacations.SeedSchoolVacations(runtime, []schoolcalendar.SchoolVacation{{Name: "X", StartDate: "2027-01-06", EndDate: "2026-12-23"}})
	assert.ErrorIs(t, err, schoolcalendar.ErrInvalidSchoolVacationWindow)
}
