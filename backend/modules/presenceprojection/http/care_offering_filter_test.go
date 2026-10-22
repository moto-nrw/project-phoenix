package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// offeringRosterFixture is a school with one care offering ("bis 16 Uhr",
// pickup 16:00 Monday to Friday) whose bookings the cases add child by child.
// studentsTestToday is a Monday.
type offeringRosterFixture struct {
	tc       *testContext
	offering *testpkg.CareOffering
	request  *enrollment.Request
	staffID  int64
	deviceID int64
}

func newOfferingRosterFixture(t *testing.T, tc *testContext) *offeringRosterFixture {
	t.Helper()
	phase := testpkg.CreateTestEnrollmentPhaseAround(t, tc.db, studentsTestToday)
	offering := testpkg.CreateTestCareOffering(t, tc.db, phase.ID, "bis 16 Uhr")
	pickup := map[string]string{"mon": "16:00", "tue": "16:00", "wed": "16:00", "thu": "16:00", "fri": "16:00"}
	_, err := tc.db.NewUpdate().TableExpr(`enrollment.care_offerings AS "care_offering"`).
		Set("pickup_times = ?", pickup).Set("days_of_week_mode = ?", "parent_choice").
		Where("id = ?", offering.ID).Exec(t.Context())
	require.NoError(t, err)

	request := &enrollment.Request{
		PhaseID: phase.ID, GuardianFirstName: "Roster", GuardianLastName: "Eltern",
		GuardianEmail: fmt.Sprintf("roster-%d@example.test", testpkg.UniqueSuffix()),
		StatusToken:   fmt.Sprintf("roster-%d", testpkg.UniqueSuffix()),
	}
	request.TenantID = testpkg.Tenant(t)
	ownerCtx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), tc.db)
	require.NoError(t, testpkg.TenantCommand1(t, ownerCtx, tc.db, api.NewEnrollmentBookingFixture().InsertRequest, request))

	staff := testpkg.CreateTestStaff(t, tc.db, "Roster", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, fmt.Sprintf("roster-device-%d", testpkg.UniqueSuffix()))
	return &offeringRosterFixture{tc: tc, offering: offering, request: request, staffID: staff.ID, deviceID: device.ID}
}

// child creates a child who is present today.
func (f *offeringRosterFixture) child(t *testing.T, name string) int64 {
	t.Helper()
	student := testpkg.CreateTestStudent(t, f.tc.db, name, "Roster", "RO1")
	checkIn := studentsTestToday.BerlinMidnight().Add(8 * time.Hour)
	testpkg.CreateTestAttendanceForDate(t, f.tc.db, student.ID, f.staffID, f.deviceID, studentsTestToday, checkIn, nil)
	return student.ID
}

// book approves a booking of the offering for the child; until is exclusive.
func (f *offeringRosterFixture) book(t *testing.T, studentID, offeringID int64, days []string, from, until *calendar.Date) {
	t.Helper()
	owner := api.NewEnrollmentBookingFixture()
	ownerCtx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), f.tc.db)
	child := &enrollment.RequestChild{
		RequestID: f.request.ID, FirstName: "Roster", LastName: fmt.Sprintf("Kind%d", studentID),
		DateOfBirth: enrollment.Date(studentsTestToday.AddDays(-2500)), Status: enrollment.ChildStatusApproved,
		CreatedStudentID: &studentID,
	}
	child.TenantID = testpkg.Tenant(t)
	require.NoError(t, testpkg.TenantCommand1(t, ownerCtx, f.tc.db, owner.InsertChild, child))
	link := &enrollment.RequestChildOffering{
		RequestChildID: child.ID, CareOfferingID: offeringID, ManualSelectedDays: days,
		ValidFrom: (*enrollment.Date)(from), ValidUntil: (*enrollment.Date)(until),
	}
	link.TenantID = testpkg.Tenant(t)
	require.NoError(t, owner.InsertRequestChildOffering(ownerCtx, link))
}

type offeringRosterRow struct {
	ID         int64   `json:"id"`
	PickupTime *string `json:"pickup_time"`
}

func listOfferingRoster(t *testing.T, tc *testContext, offeringID int64, perms ...string) []offeringRosterRow {
	t.Helper()
	if len(perms) == 0 {
		perms = []string{"users:read"}
	}
	path := fmt.Sprintf("/?location_state=present&include_pickup_times=true&care_offering_id=%d&page_size=100", offeringID)
	rr := authExec(t, tc, testutil.NewRequest("GET", path, nil), testutil.AdminTestClaims(1), perms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	var resp struct {
		Data []offeringRosterRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp.Data
}

func rosterIDs(rows []offeringRosterRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestListStudentsCareOfferingFilter(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	f := newOfferingRosterFixture(t, tc)
	today := studentsTestToday
	monday := []string{"mon", "tue", "wed", "thu", "fri"}

	booked := f.child(t, "Gebucht")
	f.book(t, booked, f.offering.ID, monday, nil, nil)
	lateException := f.child(t, "Spaeter")
	f.book(t, lateException, f.offering.ID, monday, nil, nil)
	testpkg.CreateTestPickupException(t, tc.db, lateException, today, f.staffID, "17:00", "Oma holt später")
	earlyRegular := f.child(t, "RegelFrueh")
	f.book(t, earlyRegular, f.offering.ID, monday, nil, nil)
	testpkg.CreateTestPickupSchedule(t, tc.db, earlyRegular, 1, f.staffID, "14:00")

	notThisWeekday := f.child(t, "Dienstag")
	f.book(t, notThisWeekday, f.offering.ID, []string{"tue"}, nil, nil)
	endsToday := f.child(t, "EndetHeute")
	start := today.AddDays(-14)
	f.book(t, endsToday, f.offering.ID, monday, &start, &today)
	earlyException := f.child(t, "Frueher")
	f.book(t, earlyException, f.offering.ID, monday, nil, nil)
	testpkg.CreateTestPickupException(t, tc.db, earlyException, today, f.staffID, "14:30", "Arzttermin")
	notComing := f.child(t, "KommtNicht")
	f.book(t, notComing, f.offering.ID, monday, nil, nil)
	testpkg.CreateTestPickupException(t, tc.db, notComing, today, f.staffID, "", "Kommt heute nicht")
	notArriving := f.child(t, "KommtNichtAnkunft")
	f.book(t, notArriving, f.offering.ID, monday, nil, nil)
	testpkg.CreateTestArrivalException(t, tc.db, notArriving, today, f.staffID, "", "Kommt heute nicht")
	sick := f.child(t, "Krank")
	f.book(t, sick, f.offering.ID, monday, nil, nil)
	testpkg.CreateTestStudentStatusDay(t, tc.db, sick, today, "sick")
	unbooked := f.child(t, "OhneBuchung")

	// Full access, so the rows carry their Gehzeit; a redacted row has none.
	rows := listOfferingRoster(t, tc, f.offering.ID, "admin:*")

	assert.ElementsMatch(t, []int64{booked, lateException, earlyRegular}, rosterIDs(rows))
	assert.NotContains(t, rosterIDs(rows), unbooked)
	for _, row := range rows {
		if row.ID == lateException {
			require.NotNil(t, row.PickupTime)
			assert.Equal(t, "17:00", *row.PickupTime, "the row keeps showing the later Gehzeit")
		}
	}
}

func TestListStudentsCareOfferingFilterRejectsUnusableValues(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	for _, query := range []string{
		"location_state=present&care_offering_id=abc",
		"location_state=present&care_offering_id=0",
		"care_offering_id=5",
		"location_state=transit&care_offering_id=5",
	} {
		rr := authExec(t, tc, testutil.NewRequest("GET", "/?"+query, nil), testutil.AdminTestClaims(1), []string{"users:read"})
		assert.Equal(t, http.StatusBadRequest, rr.Code, "query %q, body: %s", query, rr.Body.String())
	}
}

func TestListStudentsCareOfferingFilterStaysInsideTheSchool(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	own := newOfferingRosterFixture(t, tc)
	ownChild := own.child(t, "Eigen")
	own.book(t, ownChild, own.offering.ID, []string{"mon"}, nil, nil)

	var foreignOfferingID, foreignChild int64
	t.Run("other school books its own child", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreign := newOfferingRosterFixture(t, tc)
		foreignChild = foreign.child(t, "Fremd")
		foreign.book(t, foreignChild, foreign.offering.ID, []string{"mon"}, nil, nil)
		foreignOfferingID = foreign.offering.ID
		require.Equal(t, []int64{foreignChild}, rosterIDs(listOfferingRoster(t, tc, foreignOfferingID)))
	})

	assert.Empty(t, listOfferingRoster(t, tc, foreignOfferingID), "another school's offering id holds no child here")
	assert.Equal(t, []int64{ownChild}, rosterIDs(listOfferingRoster(t, tc, own.offering.ID)))
	options := listCareOfferingOptions(t, tc, []string{"users:read"})
	assert.NotContains(t, optionIDs(options), foreignOfferingID)
}

type careOfferingOptionRow struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func listCareOfferingOptions(t *testing.T, tc *testContext, perms []string) []careOfferingOptionRow {
	t.Helper()
	rr := authExec(t, tc, testutil.NewRequest("GET", "/care-offerings", nil), testutil.AdminTestClaims(1), perms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	var resp struct {
		Data []careOfferingOptionRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return resp.Data
}

func optionIDs(rows []careOfferingOptionRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestCareOfferingOptionsNeedOnlyUsersRead(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	phase := testpkg.CreateTestEnrollmentPhaseAround(t, tc.db, studentsTestToday)
	active := testpkg.CreateTestCareOffering(t, tc.db, phase.ID, "bis 16 Uhr")
	inactive := testpkg.CreateTestCareOffering(t, tc.db, phase.ID, "Alt")
	notCare := testpkg.CreateTestCareOffering(t, tc.db, phase.ID, "Mittagessen")
	_, err := tc.db.NewUpdate().TableExpr(`enrollment.care_offerings AS "care_offering"`).
		Set("is_active = false").Where("id = ?", inactive.ID).Exec(t.Context())
	require.NoError(t, err)
	_, err = tc.db.NewUpdate().TableExpr(`enrollment.care_offerings AS "care_offering"`).
		Set("counts_as_care = false").Where("id = ?", notCare.ID).Exec(t.Context())
	require.NoError(t, err)

	options := listCareOfferingOptions(t, tc, []string{"users:read"})

	assert.Equal(t, []careOfferingOptionRow{{ID: active.ID, Name: active.Name}}, options)
	rr := authExec(t, tc, testutil.NewRequest("GET", "/care-offerings", nil), testutil.AdminTestClaims(1), []string{"groups:read"})
	assert.Equal(t, http.StatusForbidden, rr.Code, "body: %s", rr.Body.String())
}

// TestListStudentsCareOfferingFilterQueryBudget keeps the filtered list free
// of a per-child query: the offering filter reads its facts in batches.
func TestListStudentsCareOfferingFilterQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	f := newOfferingRosterFixture(t, tc)
	addChildren := func(n int) {
		for range n {
			id := f.child(t, fmt.Sprintf("Budget%d", testpkg.UniqueSuffix()))
			f.book(t, id, f.offering.ID, []string{"mon"}, nil, nil)
			testpkg.CreateTestPickupException(t, tc.db, id, studentsTestToday, f.staffID, "17:00", "später")
		}
	}
	counter := testpkg.CaptureQueries(t, tc.db)
	run := func() int {
		counter.Reset()
		require.NotEmpty(t, listOfferingRoster(t, tc, f.offering.ID))
		return counter.Total()
	}

	addChildren(3)
	small := run()
	addChildren(7)
	large := run()

	assert.Equal(t, small, large, "query count must not grow with the number of children")
	testpkg.AssertQueryBudget(t, "api.students.list.care_offering", counter.Queries())
}
