package classdayhttp_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const (
	classTripWritePermission    = "class_day:class_trip_write"
	schoolPortalWriteScopeTrips = "class_trips"
	// A Wednesday to Friday, and the Monday of the following week, far in
	// the future.
	classTripWednesday = "2099-03-04"
	classTripFriday    = "2099-03-06"
	classTripMonday    = "2099-03-09"
)

// classTripFixture is a Lehrkraft with an assigned class of two children and
// a child of another class, on a tenant of its own.
type classTripFixture struct {
	*arrivalExceptionFixture
	first, second, foreign int64
}

func setupClassTripFixture(t *testing.T) *classTripFixture {
	t.Helper()
	f := setupArrivalExceptionFixture(t)
	first := testpkg.CreateTestStudent(t, f.db, "Anna", "Ausflug", f.class)
	second := testpkg.CreateTestStudent(t, f.db, "Ben", "Ausflug", f.class)
	foreign := testpkg.CreateTestStudent(t, f.db, "Cem", "Anderswo", fmt.Sprintf("xx%d", time.Now().UnixNano()%100000))
	return &classTripFixture{arrivalExceptionFixture: f, first: first.ID, second: second.ID, foreign: foreign.ID}
}

func (f *classTripFixture) openTrips(t *testing.T) {
	t.Helper()
	_, err := f.db.ExecContext(testpkg.Ctx(t),
		`INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?, ?, ?::jsonb)
		 ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value`,
		testpkg.Tenant(t), schoolPortalWriteScopeKey, `["`+schoolPortalWriteScopeTrips+`"]`)
	require.NoError(t, err)
}

func (f *classTripFixture) post(t *testing.T, body string, perms ...string) (int, string) {
	t.Helper()
	rec := f.do(t, http.MethodPost, "/class-trips/"+f.class, body, perms...)
	return rec.Code, rec.Body.String()
}

func (f *classTripFixture) classTripDays(t *testing.T, studentID int64) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(
		`SELECT COUNT(*) FROM active.student_status_days
		 WHERE student_id = ? AND status = 'class_trip' AND cleared_at IS NULL`, studentID,
	).Scan(testpkg.Ctx(t), &count))
	return count
}

func (f *classTripFixture) arrival(t *testing.T, studentID int64, date string) (string, string, bool) {
	t.Helper()
	var rows []struct {
		ArrivalTime string `bun:"arrival_time"`
		Reason      string `bun:"reason"`
	}
	require.NoError(t, f.db.NewRaw(
		`SELECT to_char(expected_arrival, 'HH24:MI') AS arrival_time, COALESCE(reason, '') AS reason
		 FROM schedule.student_arrival_exceptions WHERE student_id = ? AND exception_date = ?`, studentID, date,
	).Scan(testpkg.Ctx(t), &rows))
	if len(rows) == 0 {
		return "", "", false
	}
	return rows[0].ArrivalTime, rows[0].Reason, true
}

func TestSchoolClassTripsClosedByDefault(t *testing.T) {
	t.Parallel()
	f := setupClassTripFixture(t)

	rec := f.do(t, http.MethodGet, "/classes", "", classDayReadPermission, classTripWritePermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"can_report_class_trip":false`)

	body := fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[%d]}`, classTripWednesday, classTripFriday, f.first)
	code, resp := f.post(t, body, classTripWritePermission)
	require.Equal(t, http.StatusForbidden, code, resp)
	assert.Contains(t, resp, "classday.school_write_disabled")

	// The arrival-exception release does not open class trips.
	f.openWrite(t)
	code, resp = f.post(t, body, classTripWritePermission)
	require.Equal(t, http.StatusForbidden, code, resp)
	assert.Contains(t, resp, "classday.school_write_disabled")
	assert.Zero(t, f.classTripDays(t, f.first))
}

func TestSchoolClassTripsRequirePermission(t *testing.T) {
	t.Parallel()
	f := setupClassTripFixture(t)
	f.openTrips(t)

	body := fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[%d]}`, classTripWednesday, classTripFriday, f.first)
	code, resp := f.post(t, body, classDayReadPermission, classDayWritePermission)
	require.Equal(t, http.StatusForbidden, code, resp)
	assert.NotContains(t, resp, "classday.school_write_disabled", "the permission gate answers before the setting")
	assert.Zero(t, f.classTripDays(t, f.first))

	rec := f.do(t, http.MethodGet, "/classes", "", classDayReadPermission, classTripWritePermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"can_report_class_trip":true`)
}

// A multi-day trip is the status "Klassenfahrt" for every day and every
// chosen child; the OGS's refusal of an existing status writes nothing.
func TestSchoolClassTripWritesStatusDays(t *testing.T) {
	t.Parallel()
	f := setupClassTripFixture(t)
	f.openTrips(t)

	body := fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[%d,%d],"reason":"Jugendherberge"}`, classTripWednesday, classTripFriday, f.first, f.second)
	code, resp := f.post(t, body, classTripWritePermission)
	require.Equal(t, http.StatusCreated, code, resp)
	assert.Contains(t, resp, `"kind":"class_trip"`)
	assert.Contains(t, resp, `"student_count":2`)
	assert.Contains(t, resp, `"date_count":3`)
	assert.Equal(t, 3, f.classTripDays(t, f.first))
	assert.Equal(t, 3, f.classTripDays(t, f.second))

	// Reporting the days again collides with the status just written.
	code, resp = f.post(t, body, classTripWritePermission)
	require.Equal(t, http.StatusConflict, code, resp)
	assert.Contains(t, resp, "classday.class_trip_status_conflict")
	assert.Equal(t, 3, f.classTripDays(t, f.first))

	// A child of another class is refused before anything is written.
	body = fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[%d]}`, classTripMonday, classTripMonday, f.foreign)
	code, resp = f.post(t, body, classTripWritePermission)
	require.Equal(t, http.StatusConflict, code, resp)
	assert.Contains(t, resp, "classday.class_trip_student_not_in_class")
	assert.Zero(t, f.classTripDays(t, f.foreign))
}

// A one-day outing with a return time sets each child's arrival in the OGS.
// When the OGS already entered an arrival for one child, the whole report is
// refused and the children written before it are rolled back.
func TestSchoolClassTripWithReturnTimeSetsArrivals(t *testing.T) {
	t.Parallel()
	f := setupClassTripFixture(t)
	f.openTrips(t)

	body := fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[%d],"return_time":"13:30","reason":"Zoo"}`, classTripMonday, classTripMonday, f.first)
	code, resp := f.post(t, body, classTripWritePermission)
	require.Equal(t, http.StatusCreated, code, resp)
	assert.Contains(t, resp, `"kind":"return_time"`)
	arrival, reason, ok := f.arrival(t, f.first, classTripMonday)
	require.True(t, ok)
	assert.Equal(t, "13:30", arrival)
	assert.Equal(t, "Ausflug: Zoo", reason)
	assert.Zero(t, f.classTripDays(t, f.first), "an outing with a return time writes no status day")

	body = fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[%d,%d],"return_time":"14:00"}`, classTripMonday, classTripMonday, f.second, f.first)
	code, resp = f.post(t, body, classTripWritePermission)
	require.Equal(t, http.StatusConflict, code, resp)
	assert.Contains(t, resp, "classday.class_trip_arrival_conflict")
	_, _, ok = f.arrival(t, f.second, classTripMonday)
	assert.False(t, ok, "the refused report rolls back the child written before the conflict")
	arrival, _, _ = f.arrival(t, f.first, classTripMonday)
	assert.Equal(t, "13:30", arrival, "the OGS entry stays")
}

func TestSchoolClassTripRefusesInvalidReports(t *testing.T) {
	t.Parallel()
	f := setupClassTripFixture(t)
	f.openTrips(t)

	cases := map[string]struct {
		body string
		code string
	}{
		"past start":            {fmt.Sprintf(`{"from":"2020-03-02","to":"2020-03-02","student_ids":[%d]}`, f.first), "classday.class_trip_past_date"},
		"end before start":      {fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[%d]}`, classTripFriday, classTripWednesday, f.first), "classday.class_trip_invalid_range"},
		"return time, two days": {fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[%d],"return_time":"13:30"}`, classTripWednesday, classTripFriday, f.first), "classday.class_trip_return_time_multi_day"},
		"return time, Saturday": {fmt.Sprintf(`{"from":"2099-03-07","to":"2099-03-07","student_ids":[%d],"return_time":"13:30"}`, f.first), "classday.class_trip_return_time_weekend"},
		"no children":           {fmt.Sprintf(`{"from":%q,"to":%q,"student_ids":[]}`, classTripWednesday, classTripFriday), "classday.class_trip_no_students"},
	}
	for name, tc := range cases {
		code, resp := f.post(t, tc.body, classTripWritePermission)
		require.Equal(t, http.StatusBadRequest, code, name+": "+resp)
		assert.Contains(t, resp, tc.code, name)
	}
	assert.Zero(t, f.classTripDays(t, f.first))
}
