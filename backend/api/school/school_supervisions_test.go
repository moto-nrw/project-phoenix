package school_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/classday"
	"github.com/moto-nrw/project-phoenix/api/school"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// supervisionFixture is one Lehrkraft with a school token and the rows the
// assignment-bound surface reads.
type supervisionFixture struct {
	db       *bun.DB
	factory  *services.Factory
	tenantID int64
	router   chi.Router
	claims   jwt.AppClaims
	staffID  int64
}

const supervisionPermission = "supervision:own"

func setupSupervisionFixture(t *testing.T) *supervisionFixture {
	t.Helper()

	db, factory := testutil.SetupAPITest(t)
	tenantID := testpkg.Tenant(t)

	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Lehr", fmt.Sprintf("Kraft-%d", time.Now().UnixNano()))
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, tenantID)
	// Web check-ins attribute themselves to the virtual WEB-MANUAL device the
	// provisioning service gives every real school.
	testpkg.EnsureWebManualDevice(t, db)

	classDayResource := classday.NewResource(factory.EnrollmentReport, factory.UserContext, db, nil)
	router := school.NewResource(factory.Auth, factory.MFA, classDayResource, newSchoolTimetableResource(db, factory)).Router()

	return &supervisionFixture{
		db:       db,
		factory:  factory,
		tenantID: tenantID,
		router:   router,
		staffID:  staff.ID,
		claims: jwt.AppClaims{
			ID: int(account.ID), Sub: account.Email,
			Roles: []string{"lehrkraft"}, TenantID: tenantID,
			Scope: tenant.ScopeSchool,
		},
	}
}

// todayInstance creates a planned block for today in its own room. The window
// brackets "now" so the start lifecycle is available whenever the suite runs.
func (f *supervisionFixture) todayInstance(t *testing.T, title string) *scheduleModel.ActivityInstance {
	t.Helper()

	room := testpkg.CreateTestRoomForTenant(t, f.db, f.tenantID, fmt.Sprintf("Raum-%s-%d", title, time.Now().UnixNano()))
	now := timezone.Now()
	return testpkg.CreateTestActivityInstance(t, f.db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Title:     title,
		StartHHMM: now.Add(-30 * time.Minute).Format("15:04"),
		EndHHMM:   now.Add(90 * time.Minute).Format("15:04"),
	})
}

// request fires one authenticated school-portal call carrying exactly the
// permission the supervision surface requires.
func (f *supervisionFixture) request(t *testing.T, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	if body == nil && (method == http.MethodPost || method == http.MethodPatch) {
		// The complete handler insists on a JSON body; an empty object is the
		// "nothing confirmed" case every other caller sends too.
		body = strings.NewReader(`{}`)
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	return testutil.ExecuteWithAuthPermissions(t, f.router, req, f.claims, []string{supervisionPermission})
}

func (f *supervisionFixture) chi() chi.Router { return f.router }

// instanceIDsInDayList decodes the day list into the instance IDs it names.
func instanceIDsInDayList(t *testing.T, body []byte) []int64 {
	t.Helper()
	var envelope struct {
		Data struct {
			Instances []struct {
				ID int64 `json:"id"`
			} `json:"instances"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope), string(body))
	ids := make([]int64, 0, len(envelope.Data.Instances))
	for _, inst := range envelope.Data.Instances {
		ids = append(ids, inst.ID)
	}
	return ids
}

// TestSchoolSupervisionsFollowTheAssignment is the acceptance matrix of #2527:
// the Betreuungsplan assignment — and nothing else — decides what a Lehrkraft
// sees and may operate in "moto schule".
func TestSchoolSupervisionsFollowTheAssignment(t *testing.T) {
	t.Parallel()
	f := setupSupervisionFixture(t)

	mine := f.todayInstance(t, "Lernzeit")
	foreign := f.todayInstance(t, "Fremde Aufsicht")

	otherStaff, _ := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, f.tenantID, "Andere", fmt.Sprintf("Kraft-%d", time.Now().UnixNano()))
	testpkg.CreateTestInstanceStaff(t, f.db, foreign.ID, otherStaff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})

	// Without an assignment of her own, the day list is empty and the foreign
	// block's roster is closed — the "keine Einteilung" criterion.
	rec := f.request(t, http.MethodGet, "/supervisions/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, instanceIDsInDayList(t, rec.Body.Bytes()))

	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", foreign.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", mine.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// Assigned: her own block appears, the foreign one still does not.
	assignment := testpkg.CreateTestInstanceStaff(t, f.db, mine.ID, f.staffID, testpkg.InstanceStaffOpts{IsPrimary: true})

	rec = f.request(t, http.MethodGet, "/supervisions/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []int64{mine.ID}, instanceIDsInDayList(t, rec.Body.Bytes()))

	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", mine.ID), nil)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", foreign.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = f.request(t, http.MethodPost, fmt.Sprintf("/supervisions/%d/start", foreign.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	// Starting creates an active-group supervisor row. It must not preserve
	// school-portal access after the actual timetable assignment is revoked.
	rec = f.request(t, http.MethodPost, fmt.Sprintf("/supervisions/%d/start", mine.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Removing the assignment withdraws both, server-side.
	_, err := f.db.NewDelete().
		TableExpr("schedule.instance_staff").
		Where("id = ?", assignment.ID).
		Exec(testpkg.TenantContext(f.tenantID))
	require.NoError(t, err)

	rec = f.request(t, http.MethodGet, "/supervisions/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, instanceIDsInDayList(t, rec.Body.Bytes()))

	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", mine.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestSchoolSupervisionsAreBoundToToday pins the second half of the boundary:
// the assignment says WHICH block, today says WHEN. The day list answers only
// for today, but the detail routes take an instance id — and an id is guessable,
// so the clamp has to live behind them, not in front of them.
//
// Without it a Lehrkraft could pull the roster, and with it a child's pickup and
// emergency contacts, for every block she is planned into next week or was
// planned into months ago.
func TestSchoolSupervisionsAreBoundToToday(t *testing.T) {
	t.Parallel()
	f := setupSupervisionFixture(t)

	student := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Andere", "Woche", "2a")

	for _, tc := range []struct {
		name string
		date timezone.Date
	}{
		{name: "nächste Woche", date: timezone.TodayDate().AddDays(7)},
		{name: "vergangener Block", date: timezone.TodayDate().AddDays(-14)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			room := testpkg.CreateTestRoomForTenant(t, f.db, f.tenantID, fmt.Sprintf("Raum-%s-%d", tc.name, time.Now().UnixNano()))
			other := testpkg.CreateTestActivityInstance(t, f.db, tc.date, room.ID, testpkg.ActivityInstanceOpts{
				Title:     "Lernzeit",
				StartHHMM: "10:00",
				EndHHMM:   "11:00",
			})
			testpkg.CreateTestInstanceStaff(t, f.db, other.ID, f.staffID, testpkg.InstanceStaffOpts{IsPrimary: true})
			testpkg.CreateTestInstanceStudent(t, f.db, other.ID, student.ID, scheduleModel.AttendanceStatusExpected)

			// Assigned, same school, correct permission — and still closed,
			// because the day is part of the boundary.
			rec := f.request(t, http.MethodGet, "/supervisions/", nil)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.NotContains(t, instanceIDsInDayList(t, rec.Body.Bytes()), other.ID)

			for _, path := range []string{
				fmt.Sprintf("/supervisions/%d/roster", other.ID),
				fmt.Sprintf("/supervisions/%d/students/%d/sheet", other.ID, student.ID),
			} {
				rec = f.request(t, http.MethodGet, path, nil)
				assert.Equal(t, http.StatusForbidden, rec.Code, path+": "+rec.Body.String())
			}
			rec = f.request(t, http.MethodPost, fmt.Sprintf("/supervisions/%d/start", other.ID), nil)
			assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			rec = f.request(t, http.MethodPost, fmt.Sprintf("/supervisions/%d/students/%d/check-in", other.ID, student.ID), nil)
			assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		})
	}
}

// TestSchoolSupervisionsIgnoreOperationalOverview pins the #2380 boundary:
// switching a school to "Alle Räume für alle Mitarbeitenden" opens every
// running module to OGS staff — and must open nothing at all to a Lehrkraft,
// who holds a users.staff row just like they do.
func TestSchoolSupervisionsIgnoreOperationalOverview(t *testing.T) {
	t.Parallel()
	f := setupSupervisionFixture(t)

	foreign := f.todayInstance(t, "Fremde Aufsicht")
	otherStaff, _ := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, f.tenantID, "Andere", fmt.Sprintf("Kraft-%d", time.Now().UnixNano()))
	testpkg.CreateTestInstanceStaff(t, f.db, foreign.ID, otherStaff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})

	require.NoError(t, f.factory.Settings.SetValue(
		testpkg.TenantContext(f.tenantID),
		configModel.KeyOperationalOverviewScope,
		configModel.OverviewScopeAllStaff,
		nil, nil,
	))

	rec := f.request(t, http.MethodGet, "/supervisions/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, instanceIDsInDayList(t, rec.Body.Bytes()), "all_staff must not widen a school token")

	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", foreign.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// The same holds for an account that ALSO carries an admin claim set: the
	// portal decides, not the role.
	f.claims.IsAdmin = true
	f.claims.Roles = []string{"lehrkraft", "admin"}
	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", foreign.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestSchoolSupervisionStudentBoundary covers the per-child half of the
// promise: a Lehrkraft may only touch — and only read the contacts of —
// children the block she runs actually holds.
func TestSchoolSupervisionStudentBoundary(t *testing.T) {
	t.Parallel()
	f := setupSupervisionFixture(t)

	mine := f.todayInstance(t, "Lernzeit")
	testpkg.CreateTestInstanceStaff(t, f.db, mine.ID, f.staffID, testpkg.InstanceStaffOpts{IsPrimary: true})

	onRoster := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Auf", "Liste", "2a")
	offRoster := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Nicht", "Liste", "3b")
	testpkg.CreateTestInstanceStudent(t, f.db, mine.ID, onRoster.ID, scheduleModel.AttendanceStatusExpected)

	// Start the block so the check-in path is reachable at all.
	rec := f.request(t, http.MethodPost, fmt.Sprintf("/supervisions/%d/start", mine.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// A child of the school that is NOT on this roster stays untouchable, even
	// though the caller legitimately operates the block.
	rec = f.request(t, http.MethodPost, fmt.Sprintf("/supervisions/%d/students/%d/check-in", mine.ID, offRoster.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/students/%d/sheet", mine.ID, offRoster.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// The roster child can be checked in and its sheet read.
	rec = f.request(t, http.MethodPost, fmt.Sprintf("/supervisions/%d/students/%d/check-in", mine.ID, onRoster.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/students/%d/sheet", mine.ID, onRoster.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "pickup_contacts")
	assert.Contains(t, rec.Body.String(), "emergency_contacts")

	// The stored roster is retained as history, but an alumnus must leave the
	// effective current roster and cannot be checked out through this route.
	_, err := f.db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", "alumnus").
		Where("id = ?", onRoster.ID).
		Exec(testpkg.TenantContext(f.tenantID))
	require.NoError(t, err)
	rec = f.request(t, http.MethodPost, fmt.Sprintf("/supervisions/%d/students/%d/check-out", mine.ID, onRoster.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	careEnded := testpkg.CreateTestStudentForTenant(t, f.db, f.tenantID, "Betreuung", "Beendet", "2a")
	testpkg.CreateTestInstanceStudent(t, f.db, mine.ID, careEnded.ID, scheduleModel.AttendanceStatusExpected)
	_, err = f.db.NewUpdate().
		TableExpr("users.students").
		Set("enrolled_until = ?", timezone.TodayDate().AddDays(-1)).
		Where("id = ?", careEnded.ID).
		Exec(testpkg.TenantContext(f.tenantID))
	require.NoError(t, err)
	rec = f.request(t, http.MethodPost, fmt.Sprintf("/supervisions/%d/students/%d/check-in", mine.ID, careEnded.ID), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestSchoolSupervisionsCrossTenant proves the tenant transaction still bounds
// the surface: a block of another school is invisible, assignment or not.
func TestSchoolSupervisionsCrossTenant(t *testing.T) {
	t.Parallel()
	f := setupSupervisionFixture(t)

	otherTenantID, _ := testpkg.CreateTestTenant(t, f.db)
	otherRoom := testpkg.CreateTestRoomForTenant(t, f.db, otherTenantID, fmt.Sprintf("Fremdraum-%d", time.Now().UnixNano()))
	otherCtx := testpkg.TenantContext(otherTenantID)

	now := timezone.Now()
	foreign := &scheduleModel.ActivityInstance{
		Date:      timezone.TodayDate(),
		Title:     "Fremde Schule",
		StartTime: now.Add(-30 * time.Minute),
		EndTime:   now.Add(90 * time.Minute),
		RoomID:    otherRoom.ID,
		Status:    scheduleModel.InstanceStatusPlanned,
	}
	foreign.SetTenantID(otherTenantID)
	_, err := f.db.NewInsert().Model(foreign).ModelTableExpr("schedule.activity_instances").Exec(otherCtx)
	require.NoError(t, err)

	// Planting an assignment for this Lehrkraft at the OTHER school is not even
	// expressible: the composite FK on instance_staff binds (staff_id,
	// tenant_id), so a staff member of one school cannot be planned into
	// another school's block. The schema is the first of the three guards; the
	// token binding and the tenant transaction are the other two.
	foreignAssignment := &scheduleModel.InstanceStaff{InstanceID: foreign.ID, StaffID: f.staffID, IsPrimary: true}
	foreignAssignment.SetTenantID(otherTenantID)
	_, err = f.db.NewInsert().Model(foreignAssignment).ModelTableExpr("schedule.instance_staff").Exec(otherCtx)
	require.Error(t, err, "a staff member must not be assignable to another school's block")

	rec := f.request(t, http.MethodGet, "/supervisions/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, instanceIDsInDayList(t, rec.Body.Bytes()))

	rec = f.request(t, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", foreign.ID), nil)
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}

// TestSchoolSupervisionsRequirePermission keeps the permission gate honest:
// a school token without supervision:own reaches nothing, even when it is
// correctly scoped and assigned.
func TestSchoolSupervisionsRequirePermission(t *testing.T) {
	t.Parallel()
	f := setupSupervisionFixture(t)

	mine := f.todayInstance(t, "Lernzeit")
	testpkg.CreateTestInstanceStaff(t, f.db, mine.ID, f.staffID, testpkg.InstanceStaffOpts{IsPrimary: true})

	req := httptest.NewRequest(http.MethodGet, "/supervisions/", nil)
	rec := testutil.ExecuteWithAuthPermissions(t, f.chi(), req, f.claims, []string{"class_day:read"})
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}
