package staff_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type formerStaffRow struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	StartedOn string `json:"started_on"`
	EndedOn   string `json:"ended_on"`
}

// berlinNoon is a fixed instant on the given Berlin calendar day.
func berlinNoon(year int, month time.Month, day int) time.Time {
	return calendar.NewDate(year, month, day).BerlinMidnight().Add(12 * time.Hour)
}

func instant(value time.Time) *time.Time { return &value }

// formerStaffFixture covers every case the Ehemalige filter decides:
//
//   - 1 is current staff (person 11).
//   - 2 left and person 12 has no live membership: a former employment.
//   - 3 and 4 are two former employments of person 13; only the latest shows.
//   - 5 left, but person 11 is current staff again under 1: not listed twice.
//   - 6 was erased by the automatic deletion of staff (#3963).
//   - 7 was an external caregiver without an account (#3823).
func formerStaffFixture() *fakeMembership {
	return &fakeMembership{staff: map[int64]schoolmembership.Staff{
		1: {ID: 1, PersonID: 11, CreatedAt: berlinNoon(2025, 8, 1)},
		2: {ID: 2, PersonID: 12, CreatedAt: berlinNoon(2024, 9, 2), DeletedAt: instant(berlinNoon(2026, 7, 31))},
		3: {ID: 3, PersonID: 13, CreatedAt: berlinNoon(2022, 8, 1), DeletedAt: instant(berlinNoon(2023, 7, 31))},
		4: {ID: 4, PersonID: 13, CreatedAt: berlinNoon(2024, 2, 1), DeletedAt: instant(berlinNoon(2025, 1, 31))},
		5: {ID: 5, PersonID: 11, CreatedAt: berlinNoon(2021, 8, 1), DeletedAt: instant(berlinNoon(2022, 7, 31))},
		6: {ID: 6, PersonID: 14, CreatedAt: berlinNoon(2019, 8, 1), DeletedAt: instant(berlinNoon(2020, 7, 31)),
			PersonalDataErasedAt: instant(berlinNoon(2023, 8, 1))},
		7: {ID: 7, PersonID: 15, CreatedAt: berlinNoon(2025, 3, 1), DeletedAt: instant(berlinNoon(2025, 6, 1)), IsGuest: true},
	}}
}

func withFormerPersons(h *harness) *harness {
	h.persons[11] = staffHTTP.Person{ID: 11, FirstName: "Ada", LastName: "Lovelace", AccountID: accountRef(101)}
	h.persons[12] = staffHTTP.Person{ID: 12, FirstName: "Grace", LastName: "Hopper", AccountID: accountRef(102)}
	h.persons[13] = staffHTTP.Person{ID: 13, FirstName: "Katherine", LastName: "Johnson", AccountID: accountRef(103)}
	h.persons[14] = staffHTTP.Person{ID: 14, FirstName: "Getilgt", LastName: "Getilgt"}
	h.persons[15] = staffHTTP.Person{ID: 15, FirstName: "Extern", LastName: "Kraft"}
	return h
}

func decodeFormer(t *testing.T, h *harness, target string) []formerStaffRow {
	t.Helper()
	recorder := h.do(t, http.MethodGet, target, nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var rows []formerStaffRow
	require.NoError(t, json.Unmarshal(decode(t, recorder).Data, &rows))
	return rows
}

func TestListStaffLeavesFormerEmploymentsOutByDefault(t *testing.T) {
	t.Parallel()
	h := withFormerPersons(newHarness(t, formerStaffFixture()))
	h.allow("users:read")

	recorder := h.do(t, http.MethodGet, "/staff", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	payload := string(decode(t, recorder).Data)
	assert.Contains(t, payload, `"person_id":"11"`)
	for _, retired := range []string{`"person_id":"12"`, `"person_id":"13"`, `"person_id":"14"`, `"person_id":"15"`} {
		assert.NotContains(t, payload, retired, "without the filter the list shows current staff only")
	}
}

func TestListStaffFormerShowsEachPersonsLatestEmploymentOnce(t *testing.T) {
	t.Parallel()
	h := withFormerPersons(newHarness(t, formerStaffFixture()))
	h.allow("users:read")

	recorder := h.do(t, http.MethodGet, "/staff?employment=former", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "Former staff members retrieved successfully", decode(t, recorder).Message)
	assert.Equal(t, 1, h.txCalls, "the read stays inside the caller's school transaction")

	rows := decodeFormer(t, h, "/staff?employment=former")
	assert.Equal(t, []formerStaffRow{
		{ID: "2", FirstName: "Grace", LastName: "Hopper", StartedOn: "2024-09-02", EndedOn: "2026-07-31"},
		{ID: "4", FirstName: "Katherine", LastName: "Johnson", StartedOn: "2024-02-01", EndedOn: "2025-01-31"},
	}, rows, "current staff, erased tombstones and external caregivers stay out; older employments give way to the latest")
}

func TestListStaffFormerKeepsBigIDsAndAppliesDirectoryFilters(t *testing.T) {
	t.Parallel()
	membership := formerStaffFixture()
	membership.staff[2] = schoolmembership.Staff{
		ID: 9223372036854775807, PersonID: 12,
		CreatedAt: berlinNoon(2024, 9, 2), DeletedAt: instant(berlinNoon(2026, 7, 31)),
	}
	h := withFormerPersons(newHarness(t, membership))
	h.allow("users:read")
	// Offboarding soft-deletes the teacher profile too; the former filter must
	// still recognize it as a teacher.
	h.membership.teachers = map[int64]schoolmembership.Teacher{}
	h.membership.teachers[9223372036854775807] = schoolmembership.Teacher{
		ID: 22, StaffID: 9223372036854775807, DeletedAt: instant(berlinNoon(2026, 7, 31)),
	}
	h.roles[102] = "teacher"

	rows := decodeFormer(t, h, "/staff?employment=former&teachers_only=true&role=teacher")
	require.Len(t, rows, 1)
	assert.Equal(t, "9223372036854775807", rows[0].ID)
	assert.Equal(t, "Grace", rows[0].FirstName)

	assert.Empty(t, decodeFormer(t, h, "/staff?employment=former&role=admin"))
}

func TestListStaffFormerAppliesTheNameFilters(t *testing.T) {
	t.Parallel()
	h := withFormerPersons(newHarness(t, formerStaffFixture()))
	h.allow("staff:stammdaten")

	rows := decodeFormer(t, h, "/staff?employment=former&last_name=JOHN")
	require.Len(t, rows, 1)
	assert.Equal(t, "Katherine", rows[0].FirstName)
	assert.Equal(t, "2025-01-31", rows[0].EndedOn, "the latest of her two employments")
}

func TestListStaffFormerSkipsPersonsTheDirectoryNoLongerHolds(t *testing.T) {
	t.Parallel()
	h := newHarness(t, formerStaffFixture())
	h.allow("users:read")

	assert.Empty(t, decodeFormer(t, h, "/staff?employment=former"), "a deleted person is no row, not an empty name")
}

func TestListStaffFormerNeedsTheListPermission(t *testing.T) {
	t.Parallel()
	h := withFormerPersons(newHarness(t, formerStaffFixture()))
	h.allow("time_tracking:manage", "staff:documents")

	recorder := h.do(t, http.MethodGet, "/staff?employment=former", nil)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Zero(t, h.txCalls)
}

func TestListStaffFormerReportsLookupFailures(t *testing.T) {
	t.Parallel()
	membership := formerStaffFixture()
	membership.listErr = assert.AnError
	h := newHarness(t, membership)
	h.allow("users:read")
	assert.Equal(t, http.StatusInternalServerError, h.do(t, http.MethodGet, "/staff?employment=former", nil).Code)

	h = newHarness(t, formerStaffFixture())
	h.allow("users:read")
	h.personsErr = assert.AnError
	assert.Equal(t, http.StatusInternalServerError, h.do(t, http.MethodGet, "/staff?employment=former", nil).Code)

	h = withFormerPersons(newHarness(t, formerStaffFixture()))
	h.allow("users:read")
	h.roleMatchesErr = assert.AnError
	assert.Equal(t, http.StatusInternalServerError, h.do(t, http.MethodGet, "/staff?employment=former&role=teacher", nil).Code)
}

func TestFormerEmploymentDetailRetainsPeriodAndPermissionGates(t *testing.T) {
	t.Parallel()
	h := withFormerPersons(newHarness(t, formerStaffFixture()))
	h.allow("users:read")
	rec := h.do(t, http.MethodGet, "/staff/2", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	payload := string(decode(t, rec).Data)
	assert.Contains(t, payload, `"started_on":"2024-09-02"`)
	assert.Contains(t, payload, `"ended_on":"2026-07-31"`)
	assert.Contains(t, payload, `"first_name":"Grace"`)
	rec = h.do(t, http.MethodGet, "/staff/6", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "erased employment must not reappear")
	h = withFormerPersons(newHarness(t, formerStaffFixture()))
	rec = h.do(t, http.MethodGet, "/staff/2", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestFormerEmploymentReadDoesNotHideLookupFailure(t *testing.T) {
	t.Parallel()
	members := formerStaffFixture()
	members.listErr = errors.New("membership unavailable")
	h := withFormerPersons(newHarness(t, members))
	h.allow("users:read")
	rec := h.do(t, http.MethodGet, "/staff/2", nil)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
