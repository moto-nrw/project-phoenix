// Router-level test for the hint „Gast aus …“ on the tablet (PyrePortal#499):
// the kiosk roster GET /api/iot/students of the hosting school names the
// school a holiday guest comes from during its week, and no other school
// sees it.
package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	devicescanCompose "github.com/moto-ogs/project-phoenix/modules/devicescan/compose"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// tabletHints reads the tablet list of a school's kiosk: by child, the
// field holiday_guest_school, absent ones as nil.
func (f holidayGuestFixture) tabletHints(t *testing.T, school int64) map[int64]*string {
	t.Helper()
	persons, err := NewRFIDTestPeople(testpkg.NewDatabase(t, f.db))
	require.NoError(t, err)
	clock := func() calendar.Date { return f.today.Load().(calendar.Date) }
	directory := devicescanCompose.NewDirectory(NewDeviceScanDirectoryPeople(persons, nil), nil,
		NewDeviceScanHolidayGuestSchools(clock), nil, DeviceScanPrincipal)
	router := chi.NewRouter()
	router.Use(testpkg.TenantTxMiddleware(f.db))
	router.Mount("/api/iot", NewIoTHTTPRuntime().Data(directory, nil, nil).Router())

	device := testpkg.CreateTestDeviceForTenant(t, f.db, school, "kiosk-feriengast")
	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/iot/students", nil, testutil.WithDeviceContext(device))
	rec := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	result := map[int64]*string{}
	for _, student := range decodeSchoolPortalData[[]map[string]any](t, rec.Body.Bytes()) {
		id := int64(student["student_id"].(float64))
		if school, ok := student["holiday_guest_school"].(string); ok {
			result[id] = &school
		} else {
			result[id] = nil
		}
	}
	return result
}

func TestTabletListNamesTheSchoolAHolidayGuestComesFrom(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengäste Tablet")
	week := f.offer.Weeks[0].ID // 06.07.2099 to 10.07.2099, hosted by the OGS Nord
	sued := f.offer.Schools[1]
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)

	var tomRegistration int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM enrollment.holiday_registrations WHERE id <> ? AND organization_id =
		(SELECT organization_id FROM enrollment.holiday_registrations WHERE id = ?)`, f.siblings, f.siblings).Scan(t.Context(), &tomRegistration))
	f.confirm(t, f.siblings, "confirm")
	f.confirm(t, tomRegistration, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)

	f.today.Store(calendar.NewDate(2099, 7, 6))
	f.reconcile(t)
	children := f.roster(t, calendar.NewDate(2099, 7, 6))
	ben, tom := children["Ben Muster"], children["Tom Beispiel"]
	require.NotZero(t, ben)
	require.NotZero(t, tom)
	// The tablet list filters on the real calendar day; the copies start in
	// 2099, so they move their start into the past to stand on it today.
	// The hint follows the week the guest copy carries, on the fixture clock.
	arrived, weekEnds := calendar.NewDate(2020, 7, 6), calendar.NewDate(2099, 7, 10)
	for _, guest := range []int64{ben, tom} {
		testpkg.SetStudentLifecycle(t, f.db, guest, testpkg.StudentStatusActive, &arrived, &weekEnds)
	}
	own := testpkg.CreateTestStudentForTenant(t, f.db, f.nord, "Eigen", "Kind", "2b").ID
	melle, suedName := "Grundschule Melle", sued.Name

	hints := f.tabletHints(t, f.nord)
	assert.Equal(t, &melle, hints[ben], "a child without a school of the Träger names the school its parents named")
	assert.Equal(t, &suedName, hints[tom], "a child of another school of the Träger names it")
	require.Contains(t, hints, own)
	assert.Nil(t, hints[own], "an own child carries no field, as on the established wire")

	// Other schools see no hint, neither of the same Träger nor of another.
	testpkg.CreateTestStudentForTenant(t, f.db, sued.ID, "Kind", "Sued", "3a")
	suedHints := f.tabletHints(t, sued.ID)
	require.NotEmpty(t, suedHints)
	for id, hint := range suedHints {
		assert.Nil(t, hint, "the OGS Süd sees no hint, child %d", id)
	}
	foreign := testpkg.CreateTestOrganization(t, f.db, &testpkg.Organization{
		Name: "Fremder Träger Tablet", Slug: fmt.Sprintf("fremder-traeger-tablet-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, f.db, foreign, "OGS eines fremden Trägers")
	testpkg.CreateTestStudentForTenant(t, f.db, foreignSchool, "Kind", "Fremd", "1a")
	foreignHints := f.tabletHints(t, foreignSchool)
	require.NotEmpty(t, foreignHints)
	for id, hint := range foreignHints {
		assert.Nil(t, hint, "a school of another Träger sees no hint, child %d", id)
	}

	f.today.Store(calendar.NewDate(2099, 7, 11))
	hints = f.tabletHints(t, f.nord)
	assert.Nil(t, hints[ben], "from the day after the week no guest has a hint")
	assert.Nil(t, hints[tom])
}
