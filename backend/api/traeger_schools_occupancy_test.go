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
	return newOccupancyBookingsInPhase(t, db, school, -30, 300, true)
}

// newOccupancyBookingsInPhase books in a phase whose service period runs
// from today plus startDays to today plus endDays (both inclusive). The API
// counts on the real day, so the period follows it.
func newOccupancyBookingsInPhase(t *testing.T, db *bun.DB, school int64, startDays, endDays int, active bool) occupancyBookings {
	t.Helper()
	today := calendar.TodayDate()
	start, end := today.AddDays(startDays), today.AddDays(endDays)
	var phase int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.phases
		(tenant_id, name, kind, service_start_date, service_end_date, care_overflow_mode, care_offering_selection_mode, is_active)
		VALUES (?, ?, 'school_year', ?, ?, 'waitlist', 'optional', ?) RETURNING id`,
		school, fmt.Sprintf("Phase %d", testpkg.UniqueTestTenantID(t)), start, end, active).Scan(context.Background(), &phase))
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
		{ID: early, Name: "Frühbetreuung", Children: 1, Active: true},
		{ID: ogs, Name: "OGS bis 16 Uhr", Children: 2, Capacity: &capacity, Active: true},
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

// #5977: an offering of a running phase shows up without a booking, with 0
// children and its places; offerings outside their service period, of an
// inactive phase or offering, and of another Träger do not.
func TestTraegerOccupancyListsRunningOfferingsWithoutBooking(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Belegung Frei", true)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Am Teich")
	foreignOrganization := traegerOrganization(t, db, "Träger Fremd Frei", true)
	foreign := testpkg.CreateTestSchoolInOrganization(t, db, foreignOrganization, "OGS Fremd Frei")
	child := testpkg.CreateTestStudentForTenant(t, db, school, "Mia", "Teich", "1a")

	running := newOccupancyBookings(t, db, school)
	empty := running.offering("Nachmittagsbetreuung", true, 10)
	full := running.offering("Kurzbetreuung", true, 1)
	running.book(child.ID, full, "approved", nil, nil)
	running.offering("Mittagessen ohne Betreuung", false, 30)
	inactive := running.offering("Ruhendes Angebot", true, 5)
	retired := running.offering("Auslaufendes Angebot", true, 6)
	running.book(child.ID, retired, "approved", nil, nil)
	_, err := db.ExecContext(context.Background(), `UPDATE enrollment.care_offerings SET is_active = false WHERE id IN (?, ?)`, inactive, retired)
	require.NoError(t, err)
	newOccupancyBookingsInPhase(t, db, school, -200, -1, true).offering("Beendetes Angebot", true, 8)
	newOccupancyBookingsInPhase(t, db, school, 1, 200, true).offering("Künftiges Angebot", true, 8)
	newOccupancyBookingsInPhase(t, db, school, -30, 30, false).offering("Angebot ruhender Phase", true, 8)
	newOccupancyBookingsInPhase(t, db, school, 0, 0, true).offering("Tagesangebot", true, 4)
	newOccupancyBookings(t, db, foreign).offering("Fremdes freies Angebot", true, 10)

	office := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, office.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, router, office.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	rec := traegerOccupancyRequest(t, router, "/traeger/schools/occupancy", answer.AccessToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Fremdes freies Angebot", "no offering of another Träger")
	teich := occupancyOf(t, decodeSchoolPortalData[traegerview.Occupancy](t, rec.Body.Bytes()), school)
	ten, one, four, six := 10, 1, 4, 6
	var oneDay int64
	for _, form := range teich.CareForms {
		if form.Name == "Tagesangebot" {
			oneDay = form.ID
		}
	}
	assert.Equal(t, []traegerview.OccupancyCareForm{
		{ID: retired, Name: "Auslaufendes Angebot", Children: 1, Capacity: &six, Active: false},
		{ID: full, Name: "Kurzbetreuung", Children: 1, Capacity: &one, Active: true},
		{ID: empty, Name: "Nachmittagsbetreuung", Children: 0, Capacity: &ten, Active: true},
		{ID: oneDay, Name: "Tagesangebot", Children: 0, Capacity: &four, Active: true},
	}, teich.CareForms, "running offerings without a booking count 0 children; an inactive offering shows only with a booking and says so; ended, future and lunch offerings do not appear")
	assert.Zero(t, teich.WithoutCareForm)

	export := traegerOccupancyRequest(t, router, "/traeger/schools/occupancy/export", answer.AccessToken, nil)
	require.Equal(t, http.StatusOK, export.Code, export.Body.String())
	csv := export.Body.String()
	assert.Contains(t, csv, ";OGS Am Teich;;Betreuungsform;Nachmittagsbetreuung;0;10\n", "the export lists the empty offering")
	for _, absent := range []string{"Beendetes Angebot", "Künftiges Angebot", "Angebot ruhender Phase", "Ruhendes Angebot", "Mittagessen", "Fremd"} {
		assert.NotContains(t, csv, absent)
	}
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
	read := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, db)
		rec := traegerOccupancyRequest(t, router, "/traeger/schools/occupancy", answer.AccessToken, counter)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	three := read()
	for _, name := range []string{"OGS Vier", "OGS Fünf"} {
		addSchool(name)
	}
	assert.Len(t, read().Statements, len(three.Statements), "the occupancy reads the same statements for any number of schools")
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
	ogs := bookings.offering("OGS", true, 0)
	bookings.book(child.ID, ogs, "approved", nil, nil)
	// The other school holds everything the reads look for: a running phase,
	// a booked and an unbooked offering and an approved child. The
	// administrative transaction bypasses RLS (#5977, rule 25), so only the
	// school filter of each statement keeps them out.
	foreignChild := testpkg.CreateTestStudentForTenant(t, db, other, "Fritz", "Fremd", "1a")
	foreign := newOccupancyBookings(t, db, other)
	foreign.book(foreignChild.ID, foreign.offering("Fremd gebucht", true, 0), "approved", nil, nil)
	foreign.offering("Fremd leer", true, 5)

	ports := traegerOccupancyPorts()
	day := calendar.TodayDate()
	reads := func(ctx context.Context) []error {
		_, groups := ports.Groups.MembershipGroupIDsByTenant(ctx, []int64{school})
		_, names := ports.GroupNames.GroupsByTenant(ctx, []int64{school})
		_, booked := ports.Bookings.CareOfferingBookingsByTenant(ctx, []int64{school}, day, nil)
		_, _, children := ports.Children.BookedChildrenAndRunningPhases(ctx, []int64{school}, day)
		return []error{groups, names, booked, children}
	}
	for index, err := range reads(testpkg.TenantContext(school)) {
		assert.Error(t, err, "read %d refuses a tenant transaction", index)
	}
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		names, err := ports.GroupNames.GroupsByTenant(adminCtx, []int64{school})
		require.NoError(t, err)
		assert.Equal(t, map[int64][]traegerview.NamedGroup{school: {{ID: group.ID, Name: group.Name}}}, names, "only the given schools")

		children, phases, err := ports.Children.BookedChildrenAndRunningPhases(adminCtx, []int64{school}, day)
		require.NoError(t, err)
		assert.Equal(t, []int64{bookings.phase}, phases, "only the running phases of the given schools")
		require.Len(t, children, 1, "only the children of the given schools")
		for _, student := range children[school] {
			assert.Equal(t, child.ID, student)
		}
		require.Len(t, children[school], 1)

		// Even a phase of another school in the list does not bring its
		// offerings: the statement filters on the schools itself.
		offerings, err := ports.Bookings.CareOfferingBookingsByTenant(adminCtx, []int64{school}, day, []int64{bookings.phase, foreign.phase})
		require.NoError(t, err)
		require.Len(t, offerings, 1, "only the offerings of the given schools")
		require.Len(t, offerings[school], 1)
		assert.Equal(t, ogs, offerings[school][0].ID)
		return nil
	}))
}
