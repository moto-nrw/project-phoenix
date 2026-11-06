package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// traegerOccupancyRequest reads path with the session's token; a counter,
// when given, counts the statements of the request.
func traegerOccupancyRequest(t *testing.T, router http.Handler, path, token string, counter *testpkg.QueryCounter) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if counter != nil {
		req = req.WithContext(counter.Context(req.Context()))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// occupancyBookings books the students of school on a care offering through
// approved enrollment children; validFrom and validUntil bound every booking.
type occupancyBookings struct {
	t      *testing.T
	db     *bun.DB
	school int64
	phase  int64
}

func newOccupancyBookings(t *testing.T, db *bun.DB, school int64) occupancyBookings {
	t.Helper()
	today := calendar.TodayDate()
	var phase int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.phases
		(tenant_id, name, kind, service_start_date, service_end_date, care_overflow_mode, care_offering_selection_mode, is_active)
		VALUES (?, ?, 'school_year', ?, ?, 'waitlist', 'optional', true) RETURNING id`,
		school, fmt.Sprintf("Phase %d", testpkg.UniqueTestTenantID(t)), today.AddDays(-30), today.AddDays(300)).Scan(context.Background(), &phase))
	return occupancyBookings{t: t, db: db, school: school, phase: phase}
}

// offering adds a care offering; capacity 0 means none.
func (b occupancyBookings) offering(name string, countsAsCare bool, capacity int) int64 {
	b.t.Helper()
	var limit *int
	if capacity > 0 {
		limit = &capacity
	}
	var id int64
	require.NoError(b.t, b.db.NewRaw(`INSERT INTO enrollment.care_offerings (tenant_id, phase_id, name, capacity, counts_as_care, available_days)
		VALUES (?, ?, ?, ?, ?, '["mon","tue","wed","thu","fri"]'::jsonb) RETURNING id`, b.school, b.phase, name, limit, countsAsCare).Scan(context.Background(), &id))
	return id
}

// book books student on offering from validFrom (inclusive) to validUntil
// (exclusive) through a request child with the given status.
func (b occupancyBookings) book(student, offering int64, status string, validFrom, validUntil *calendar.Date) {
	b.t.Helper()
	ctx := context.Background()
	suffix := testpkg.UniqueTestTenantID(b.t)
	var request int64
	require.NoError(b.t, b.db.NewRaw(`INSERT INTO enrollment.requests
		(tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token, submitted_at, submission_source)
		VALUES (?, ?, 'Gerda', 'Gast', ?, ?, NOW(), 'public') RETURNING id`,
		b.school, b.phase, fmt.Sprintf("gerda-%d@example.test", suffix), fmt.Sprintf("token-%d", suffix)).Scan(ctx, &request))
	var child int64
	require.NoError(b.t, b.db.NewRaw(`INSERT INTO enrollment.request_children
		(tenant_id, request_id, first_name, last_name, date_of_birth, status, sort_order, created_student_id)
		VALUES (?, ?, 'Kind', 'Gebucht', '2020-01-01', ?, 0, ?) RETURNING id`, b.school, request, status, student).Scan(ctx, &child))
	_, err := b.db.ExecContext(ctx, `INSERT INTO enrollment.care_offering_bookings
		(tenant_id, request_child_id, care_offering_id, valid_from, valid_until) VALUES (?, ?, ?, ?, ?)`,
		b.school, child, offering, validFrom, validUntil)
	require.NoError(b.t, err)
}

func occupancyOf(t *testing.T, got traegerview.Occupancy, school int64) traegerview.SchoolOccupancy {
	t.Helper()
	for _, row := range got.Schools {
		if row.ID == school {
			return row
		}
	}
	t.Fatalf("school %d is missing from the occupancy", school)
	return traegerview.SchoolOccupancy{}
}

func TestTraegerOccupancyCountsTheRosterByGroupAndCareForm(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Belegung", true)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Am Bach")
	empty := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Lindenweg")
	foreignOrganization := traegerOrganization(t, db, "Träger Fremd Belegung", true)
	foreign := testpkg.CreateTestSchoolInOrganization(t, db, foreignOrganization, "OGS Fremd Belegung")
	today := calendar.TodayDate()
	yesterday, tomorrow := today.AddDays(-1), today.AddDays(1)

	bears := testpkg.CreateTestEducationGroupForTenant(t, db, school, "Bären")
	foxes := testpkg.CreateTestEducationGroupForTenant(t, db, school, "Füchse")
	anna := testpkg.CreateTestStudentForTenant(t, db, school, "Anna", "Gruppe", "1a")
	ben := testpkg.CreateTestStudentForTenant(t, db, school, "Ben", "Gruppe", "1a")
	cem := testpkg.CreateTestStudentForTenant(t, db, school, "Cem", "Ohne", "2b")
	testpkg.AssignStudentToGroup(t, db, anna.ID, bears.ID)
	testpkg.AssignStudentToGroup(t, db, ben.ID, bears.ID)
	foreignChild := testpkg.CreateTestStudentForTenant(t, db, foreign, "Fritz", "Fremd", "1a")

	bookings := newOccupancyBookings(t, db, school)
	ogs := bookings.offering("OGS bis 16 Uhr", true, 25)
	early := bookings.offering("Frühbetreuung", true, 0)
	lunch := bookings.offering("Mittagessen", false, 0)
	bookings.book(anna.ID, ogs, "approved", &yesterday, nil)
	bookings.book(ben.ID, ogs, "approved", nil, &tomorrow)
	bookings.book(anna.ID, early, "approved", nil, nil)
	bookings.book(cem.ID, lunch, "approved", nil, nil)
	bookings.book(cem.ID, early, "approved", &tomorrow, nil)
	bookings.book(cem.ID, ogs, "submitted", nil, nil)
	bookings.book(cem.ID, ogs, "approved", nil, &today)
	foreignBookings := newOccupancyBookings(t, db, foreign)
	foreignBookings.book(foreignChild.ID, foreignBookings.offering("Fremdes Angebot", true, 0), "approved", nil, nil)

	office := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, office.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, router, office.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	rec := traegerOccupancyRequest(t, router, "/traeger/schools/occupancy", answer.AccessToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Anna", "the occupancy carries counts, no names of children")
	assert.NotContains(t, rec.Body.String(), "OGS Fremd Belegung", "no school of another Träger")
	assert.NotContains(t, rec.Body.String(), "Fremdes Angebot", "no offering of another Träger")
	got := decodeSchoolPortalData[traegerview.Occupancy](t, rec.Body.Bytes())
	require.NotEmpty(t, got.Day, "the answer names the day it counts")
	require.Len(t, got.Schools, 2)
	assert.Equal(t, "OGS Am Bach", got.Schools[0].Name, "ordered by name")

	bach := occupancyOf(t, got, school)
	assert.Equal(t, 3, bach.Children, "the roster of the Stichtagszahl rule")
	require.Len(t, bach.Groups, 2, "every group, also one without children")
	assert.Equal(t, 2, bach.Groups[0].Children)
	assert.Equal(t, 0, bach.Groups[1].Children)
	assert.Equal(t, 1, bach.WithoutGroup)
	capacity := 25
	assert.Equal(t, []traegerview.OccupancyCareForm{
		{ID: early, Name: "Frühbetreuung", Children: 1},
		{ID: ogs, Name: "OGS bis 16 Uhr", Children: 2, Capacity: &capacity},
	}, bach.CareForms, "only offerings that count as care with a booking valid today; an undecided child, an ended and a future booking do not count")
	assert.Equal(t, 1, bach.WithoutCareForm, "Cem has only lunch and bookings that are not valid today")

	linden := occupancyOf(t, got, empty)
	assert.Zero(t, linden.Children)
	assert.Empty(t, linden.Groups)
	assert.Empty(t, linden.CareForms)

	export := traegerOccupancyRequest(t, router, "/traeger/schools/occupancy/export", answer.AccessToken, nil)
	require.Equal(t, http.StatusOK, export.Code, export.Body.String())
	assert.Equal(t, "text/csv; charset=utf-8", export.Header().Get("Content-Type"))
	assert.Contains(t, export.Header().Get("Content-Disposition"), fmt.Sprintf("belegung-%s.csv", got.Day))
	day := got.Day.Format("02.01.2006")
	csv := strings.TrimPrefix(export.Body.String(), "\xEF\xBB\xBF")
	for _, line := range []string{
		"Datum;Schule;Ort;Art;Name;Kinder;Plätze",
		day + ";OGS Am Bach;;Alle Kinder;;3;",
		day + ";OGS Am Bach;;Gruppe;" + bears.Name + ";2;",
		day + ";OGS Am Bach;;Ohne Gruppe;;1;",
		day + ";OGS Am Bach;;Betreuungsform;OGS bis 16 Uhr;2;25",
		day + ";OGS Am Bach;;Ohne Betreuungsform;;1;",
		day + ";OGS Lindenweg;;Alle Kinder;;0;",
	} {
		assert.Contains(t, csv, line+"\n")
	}
	assert.NotContains(t, csv, "Fremd")
	assert.Contains(t, csv, day+";OGS Am Bach;;Gruppe;"+foxes.Name+";0;\n", "an empty group is exported too")
}

func TestTraegerOccupancyNeedsTheOverviewRight(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Belegung Ohne Recht", true)
	testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Belegung Ohne Recht")
	withoutRole := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, withoutRole.ID)
	code, answer := traegerSignIn(t, router, withoutRole.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	for _, path := range []string{"/traeger/schools/occupancy", "/traeger/schools/occupancy/export"} {
		rec := traegerOccupancyRequest(t, router, path, answer.AccessToken, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, path+": "+rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "OGS Belegung Ohne Recht")
	}
}

func TestTraegerOccupancyQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Belegung Budget", true)
	addSchool := func(name string) {
		school := testpkg.CreateTestSchoolInOrganization(t, db, organization, name)
		group := testpkg.CreateTestEducationGroupForTenant(t, db, school, "Gruppe")
		child := testpkg.CreateTestStudentForTenant(t, db, school, "Kind", name, "1a")
		testpkg.AssignStudentToGroup(t, db, child.ID, group.ID)
		bookings := newOccupancyBookings(t, db, school)
		bookings.book(child.ID, bookings.offering("OGS", true, 10), "approved", nil, nil)
	}
	for _, name := range []string{"OGS Eins", "OGS Zwei", "OGS Drei"} {
		addSchool(name)
	}
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	read := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		rec := traegerOccupancyRequest(t, router, "/traeger/schools/occupancy", answer.AccessToken, counter)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	three := read()
	for _, name := range []string{"OGS Vier", "OGS Fünf"} {
		addSchool(name)
	}
	assert.Len(t, read(), len(three), "the occupancy reads the same statements for any number of schools")
	querybudgets.AssertQueryBudget(t, "api.traeger.schools_occupancy", three)
}

func TestTraegerOccupancyReadsRefuseOutsideTheAdministrativeTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school, _ := testpkg.CreateTestTenant(t, db)
	other, _ := testpkg.CreateTestTenant(t, db)
	group := testpkg.CreateTestEducationGroupForTenant(t, db, school, "Eulen")
	child := testpkg.CreateTestStudentForTenant(t, db, school, "Ida", "Eule", "1a")
	testpkg.AssignStudentToGroup(t, db, child.ID, group.ID)
	testpkg.CreateTestEducationGroupForTenant(t, db, other, "Fremd")
	bookings := newOccupancyBookings(t, db, school)
	bookings.book(child.ID, bookings.offering("OGS", true, 0), "approved", nil, nil)

	ports := traegerOccupancyPorts()
	day := calendar.TodayDate()
	reads := func(ctx context.Context) []error {
		_, groups := ports.Groups.MembershipGroupIDsByTenant(ctx, []int64{school})
		_, names := ports.GroupNames.GroupsByTenant(ctx, []int64{school})
		_, booked := ports.Bookings.CareOfferingBookingsByTenant(ctx, []int64{school}, day)
		_, children := ports.Children.ApprovedChildStudentIDsByTenant(ctx, []int64{school})
		return []error{groups, names, booked, children}
	}
	for index, err := range reads(testpkg.TenantContext(school)) {
		assert.Error(t, err, "read %d refuses a tenant transaction", index)
	}
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		names, err := ports.GroupNames.GroupsByTenant(adminCtx, []int64{school})
		require.NoError(t, err)
		assert.Equal(t, map[int64][]traegerview.NamedGroup{school: {{ID: group.ID, Name: group.Name}}}, names, "only the given schools")
		return nil
	}))
}
