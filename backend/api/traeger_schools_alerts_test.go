package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// traegerSchoolsAlerts reads GET /traeger/schools/alerts with the session's
// token; a counter, when given, counts the statements of the request.
func traegerSchoolsAlerts(t *testing.T, router http.Handler, token string, counter *testpkg.QueryCounter) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/traeger/schools/alerts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if counter != nil {
		req = req.WithContext(counter.Context(req.Context()))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// insertEnrollmentChildren submits one online enrollment request at school
// with one child per status; withdrawn marks the request as withdrawn.
func insertEnrollmentChildren(t *testing.T, db *bun.DB, school int64, withdrawn bool, statuses ...string) {
	t.Helper()
	ctx := context.Background()
	today := calendar.TodayDate()
	suffix := testpkg.UniqueTestTenantID(t)
	var phaseID int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.phases
		(tenant_id, name, kind, service_start_date, service_end_date, care_overflow_mode, care_offering_selection_mode, is_active)
		VALUES (?, ?, 'school_year', ?, ?, 'waitlist', 'optional', true) RETURNING id`,
		school, fmt.Sprintf("Phase %d", suffix), today.AddDays(-30), today.AddDays(300)).Scan(ctx, &phaseID))
	var withdrawnAt *time.Time
	if withdrawn {
		at := time.Now()
		withdrawnAt = &at
	}
	var requestID int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.requests
		(tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, status_token, submitted_at, submission_source, withdrawn_at)
		VALUES (?, ?, 'Gerda', 'Gast', ?, ?, NOW(), 'public', ?) RETURNING id`,
		school, phaseID, fmt.Sprintf("gerda-%d@example.test", suffix), fmt.Sprintf("token-%d", suffix), withdrawnAt).Scan(ctx, &requestID))
	for index, status := range statuses {
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth, status, sort_order) VALUES (?, ?, 'Kind', 'Neu', '2020-01-01', ?, ?)`,
			school, requestID, status, index)
		require.NoError(t, err)
	}
}

// insertSickDay reports a child of school sick for today.
func insertSickDay(t *testing.T, db *bun.DB, school, studentID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `INSERT INTO active.student_status_days
		(tenant_id, student_id, date, status, reported_at, source) VALUES (?, ?, ?, 'sick', NOW(), 'manual')`,
		school, studentID, calendar.TodayDate())
	require.NoError(t, err)
}

func traegerAlertsOf(t *testing.T, got traegerview.SchoolsAlerts, school int64) traegerview.SchoolAlerts {
	t.Helper()
	for _, row := range got.Schools {
		if row.ID == school {
			return row
		}
	}
	t.Fatalf("school %d is missing from the alerts", school)
	return traegerview.SchoolAlerts{}
}

func TestTraegerSchoolsAlertsCheckEverySchoolAgainstTheTraegerThresholds(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Hinweise", true)
	busy := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Am Bach")
	quiet := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Lindenweg")
	foreignOrganization := traegerOrganization(t, db, "Träger Fremd Hinweise", true)
	foreign := testpkg.CreateTestSchoolInOrganization(t, db, foreignOrganization, "OGS Fremd Hinweise")
	today, at := justNow()

	// Am Bach: one staff member for three present children, one of four sick.
	staff := testpkg.CreateTestStaffForTenant(t, db, busy, "Bea", "Betreuung")
	insertFigureWorkSession(t, db, busy, staff.ID, today, "present", at, nil)
	device := testpkg.CreateTestDeviceForTenant(t, db, busy, "alerts")
	for _, name := range []string{"Anna", "Ben", "Cem"} {
		child := testpkg.CreateTestStudentForTenant(t, db, busy, name, "Hier", "1a")
		testpkg.CreateTestAttendanceForTenant(t, db, busy, child.ID, staff.ID, device.ID, today, at, nil)
	}
	sick := testpkg.CreateTestStudentForTenant(t, db, busy, "Dana", "Krank", "1a")
	insertSickDay(t, db, busy, sick.ID)

	// Lindenweg: two children wait; a decided child and a withdrawn request
	// do not.
	testpkg.CreateTestStudentForTenant(t, db, quiet, "Emil", "Daheim", "1a")
	insertEnrollmentChildren(t, db, quiet, false, "submitted", "under_review", "approved")
	insertEnrollmentChildren(t, db, quiet, true, "submitted")
	insertEnrollmentChildren(t, db, foreign, false, "submitted", "submitted")

	office := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, office.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, router, office.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	rec := traegerSchoolsAlerts(t, router, answer.AccessToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Dana", "the hints carry figures, no names")
	assert.NotContains(t, rec.Body.String(), "OGS Fremd Hinweise", "no school of another Träger")
	got := decodeSchoolPortalData[traegerview.SchoolsAlerts](t, rec.Body.Bytes())
	assert.Equal(t, traegerview.AlertThresholds{ChildrenPerStaff: 20, SickPercent: 10, OpenEnrollments: 1}, got.Thresholds, "the registry defaults without a Träger value")
	require.Len(t, got.Schools, 2)
	assert.Equal(t, "OGS Am Bach", got.Schools[0].Name, "ordered by name")
	bach := traegerAlertsOf(t, got, busy)
	assert.Equal(t, []string{traegerview.AlertSickChildren}, bach.Alerts, "one of four children sick reaches ten percent")
	assert.Equal(t, 3, bach.Figures.ChildrenPresent)
	assert.Equal(t, 1, bach.Figures.ChildrenSick)
	linden := traegerAlertsOf(t, got, quiet)
	assert.Equal(t, 2, linden.OpenEnrollments, "only the waiting children of a request still open")
	assert.Equal(t, []string{traegerview.AlertOpenEnrollments}, linden.Alerts)

	// The Träger tightens the care ratio and switches the sick hint off.
	for key, value := range map[string]int{configModels.KeyTraegerAlertChildrenPerStaff: 2, configModels.KeyTraegerAlertSickPercent: 0} {
		set := traegerJSON(t, router, answer.AccessToken, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": value})
		require.Equal(t, http.StatusOK, set.Code, set.Body.String())
	}
	rec = traegerSchoolsAlerts(t, router, answer.AccessToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got = decodeSchoolPortalData[traegerview.SchoolsAlerts](t, rec.Body.Bytes())
	assert.Equal(t, traegerview.AlertThresholds{ChildrenPerStaff: 2, SickPercent: 0, OpenEnrollments: 1}, got.Thresholds)
	assert.Equal(t, []string{traegerview.AlertStaffShortage}, traegerAlertsOf(t, got, busy).Alerts, "three children on one staff member exceed two")
	assert.Equal(t, []string{traegerview.AlertOpenEnrollments}, traegerAlertsOf(t, got, quiet).Alerts)
}

func TestTraegerSchoolsAlertsNeedTheOverviewRight(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Hinweise Ohne Recht", true)
	testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Hinweise Ohne Recht")
	withoutRole := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, withoutRole.ID)
	code, answer := traegerSignIn(t, router, withoutRole.Email, password, 0)
	require.Equal(t, http.StatusOK, code)

	rec := traegerSchoolsAlerts(t, router, answer.AccessToken, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "OGS Hinweise Ohne Recht")
}

func TestTraegerSchoolsAlertsQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	organization := traegerOrganization(t, db, "Träger Hinweise Budget", true)
	for _, name := range []string{"OGS Eins", "OGS Zwei", "OGS Drei"} {
		school := testpkg.CreateTestSchoolInOrganization(t, db, organization, name)
		testpkg.CreateTestStudentForTenant(t, db, school, "Kind", name, "1a")
		insertEnrollmentChildren(t, db, school, false, "submitted")
	}
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organization, account.ID, identityaccess.TraegerRoleReadOnly)
	code, answer := traegerSignIn(t, router, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	read := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		rec := traegerSchoolsAlerts(t, router, answer.AccessToken, counter)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	three := read()
	for _, name := range []string{"OGS Vier", "OGS Fünf"} {
		school := testpkg.CreateTestSchoolInOrganization(t, db, organization, name)
		testpkg.CreateTestStudentForTenant(t, db, school, "Kind", name, "1a")
		insertEnrollmentChildren(t, db, school, false, "submitted")
	}
	assert.Len(t, read(), len(three), "the hints read the same statements for any number of schools")
	querybudgets.AssertQueryBudget(t, "api.traeger.schools_alerts", three)
}
