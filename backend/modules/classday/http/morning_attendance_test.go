// Router-level tests for the morning attendance of moto schule (#4790): a
// Lehrkraft marks each child of an assigned class "da" or "fehlt" for today,
// gated by permission, the class assignment, the school's setting, today and
// the school day, and stored apart from the OGS attendance.
//
// "Today" is a fixed far-future Monday the module's clock is pinned to, so
// nothing here depends on the wall clock.
package classdayhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	classdayhttp "github.com/moto-nrw/project-phoenix/modules/classday/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const (
	morningAttendancePermission = "class_day:morning_attendance_write"
	morningAttendanceSettingKey = "operations.school_portal_morning_attendance"

	// The pinned today, the next day and a Saturday of the same week.
	morningMonday   = "2099-03-02"
	morningTuesday  = "2099-03-03"
	morningSaturday = "2099-03-07"
)

type morningAttendanceFixture struct {
	hub       *realtime.Hub
	db        *testpkg.DB
	module    rootAPI.ClassDayTestModule
	router    chi.Router
	claims    sessiontoken.AppClaims
	tenantID  int64
	class     string
	sick      int64
	present   int64
	listEntry int64
	other     int64
}

// setupMorningAttendanceFixture builds one Lehrkraft with an assigned class:
// a child reported sick today, a child without a report, a class-list-only
// entry, and a child of another class. The module's clock reads today.
func setupMorningAttendanceFixture(t *testing.T, today string) *morningAttendanceFixture {
	t.Helper()
	date, err := calendar.ParseDate(today)
	require.NoError(t, err)
	now := date.BerlinMidnight().Add(9 * time.Hour)
	db := testpkg.SetupTestDB(t)
	hub := realtime.NewHub(nil)
	module, err := rootAPI.NewClassDayTestModuleWithRealtime(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), hub, func() time.Time { return now })
	require.NoError(t, err)
	router := classdayhttp.NewResource(module.ClassDay, nil).SchoolRouter(rootAPI.ProtectedSchoolRoutes)
	tenantID := testpkg.Tenant(t)

	suffix := time.Now().UnixNano()
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Morgen", fmt.Sprintf("Lehrkraft-%d", suffix))
	className := fmt.Sprintf("ma%d", suffix%100000)
	_ = testpkg.CreateTestClassTeacher(t, db, staff.ID, className)
	sick := testpkg.CreateTestStudent(t, db, "Sina", "Krank", className)
	present := testpkg.CreateTestStudent(t, db, "Paul", "Da", className)
	entry := testpkg.CreateTestClassListEntry(t, db, "Lena", "Liste", className)
	other := testpkg.CreateTestStudent(t, db, "Otto", "Andere", className+"x")
	_ = testpkg.CreateTestStudentStatusDay(t, db, sick.ID, date, "sick")

	return &morningAttendanceFixture{
		db: db, module: module, hub: hub, router: router, tenantID: tenantID, class: className,
		sick: sick.ID, present: present.ID, listEntry: entry.ID, other: other.ID,
		claims: sessiontoken.AppClaims{
			ID: int(account.ID), Sub: account.Email,
			Roles: []string{"lehrkraft"}, TenantID: tenantID,
			Scope: tenant.ScopeSchool,
		},
	}
}

func (f *morningAttendanceFixture) do(t *testing.T, method, path, body string, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return testutil.ExecuteWithAuthPermissions(t, f.router, req, f.claims, perms)
}

func (f *morningAttendanceFixture) put(t *testing.T, date, body string, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(t, http.MethodPut, fmt.Sprintf("/morning-attendance/%s/%s", f.class, date), body, perms...)
}

func (f *morningAttendanceFixture) switchOff(t *testing.T) {
	t.Helper()
	_, err := f.db.ExecContext(testpkg.Ctx(t),
		`INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?, ?, 'false'::jsonb)
		 ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value`,
		f.tenantID, morningAttendanceSettingKey)
	require.NoError(t, err)
}

type morningReport struct {
	CanRecord bool `json:"can_record_morning_attendance"`
	Rows      []struct {
		StudentID         int64  `json:"student_id"`
		ListEntryID       string `json:"list_entry_id"`
		MorningAttendance *struct {
			Status string `json:"status"`
			Source string `json:"source"`
		} `json:"morning_attendance"`
	} `json:"rows"`
}

// report reads the day view and returns the morning status per child, keyed
// "s<student>" or "e<list entry>"; "" stands for none.
func (f *morningAttendanceFixture) report(t *testing.T, date string, perms ...string) (bool, map[string]string) {
	t.Helper()
	rec := f.do(t, http.MethodGet, "/?class="+f.class+"&date="+date, "", perms...)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data morningReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	statuses := map[string]string{}
	for _, row := range envelope.Data.Rows {
		key := fmt.Sprintf("s%d", row.StudentID)
		if row.ListEntryID != "" {
			key = "e" + row.ListEntryID
		}
		statuses[key] = ""
		if row.MorningAttendance != nil {
			statuses[key] = row.MorningAttendance.Status + "/" + row.MorningAttendance.Source
		}
	}
	return envelope.Data.CanRecord, statuses
}

func TestMorningAttendanceRoundTrip(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	sick, present, entry := fmt.Sprintf("s%d", f.sick), fmt.Sprintf("s%d", f.present), fmt.Sprintf("e%d", f.listEntry)

	// Before anything is marked: the sick child reads absent from the
	// report, everybody else is open, and the caller may record.
	canRecord, statuses := f.report(t, morningMonday, classDayReadPermission, morningAttendancePermission)
	assert.True(t, canRecord)
	assert.Equal(t, "absent/report", statuses[sick])
	assert.Empty(t, statuses[present])
	assert.Empty(t, statuses[entry])

	body := fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"present"},{"list_entry_id":"%d","status":"absent"},{"student_id":%d,"status":"present"}]}`,
		f.present, f.listEntry, f.sick)
	rec := f.put(t, morningMonday, body, morningAttendancePermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	_, statuses = f.report(t, morningMonday, classDayReadPermission, morningAttendancePermission)
	assert.Equal(t, "present/teacher", statuses[present])
	assert.Equal(t, "absent/teacher", statuses[entry], "a class-list-only child is marked like any other")
	assert.Equal(t, "present/teacher", statuses[sick], "the Lehrkraft's mark wins over the report")

	// A correction on the same day replaces the mark, attributed to the
	// Lehrkraft's staff row.
	rec = f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"absent"}]}`, f.present), morningAttendancePermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	_, statuses = f.report(t, morningMonday, classDayReadPermission)
	assert.Equal(t, "absent/teacher", statuses[present])
	var rows int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM active.morning_attendance WHERE tenant_id = ? AND student_id = ?`,
		f.tenantID, f.present).Scan(testpkg.Ctx(t), &rows))
	assert.Equal(t, 1, rows, "one row per child and day")
}

func TestMorningAttendanceLeavesOGSAttendanceUntouched(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	date, err := calendar.ParseDate(morningMonday)
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, f.db, fmt.Sprintf("morning-%d", time.Now().UnixNano()))
	staff := testpkg.CreateTestStaff(t, f.db, "OGS", "Kraft")
	checkIn := date.BerlinMidnight().Add(8 * time.Hour)
	attendance := testpkg.CreateTestAttendanceForDate(t, f.db, f.present, staff.ID, device.ID, date, checkIn, nil)
	snapshot := func() string {
		var row string
		require.NoError(t, f.db.NewRaw(`SELECT row_to_json(a)::text FROM active.attendance a WHERE id = ?`, attendance.ID).
			Scan(testpkg.Ctx(t), &row))
		return row
	}
	before := snapshot()

	rec := f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"absent"}]}`, f.present), morningAttendancePermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	assert.Equal(t, before, snapshot(), "a morning mark never changes the OGS check-in")
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM active.attendance WHERE student_id = ?`, f.present).Scan(testpkg.Ctx(t), &count))
	assert.Equal(t, 1, count, "a morning mark creates no OGS attendance")
}

func TestMorningAttendanceGates(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	mark := fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"present"}]}`, f.present)

	// The permission gate answers before anything else.
	rec := f.put(t, morningMonday, mark, classDayReadPermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	canRecord, _ := f.report(t, morningMonday, classDayReadPermission)
	assert.False(t, canRecord, "the verdict needs the permission")

	// Only today.
	rec = f.put(t, morningTuesday, mark, morningAttendancePermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.morning_attendance_not_today")
	canRecord, _ = f.report(t, morningTuesday, classDayReadPermission, morningAttendancePermission)
	assert.False(t, canRecord, "another day is read-only")

	// Only children on the class roster.
	rec = f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"present"}]}`, f.other), morningAttendancePermission)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.morning_attendance_unknown_child")

	// Only assigned classes.
	rec = f.do(t, http.MethodPut, "/morning-attendance/"+f.class+"x/"+morningMonday, mark, morningAttendancePermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// A malformed mark.
	rec = f.put(t, morningMonday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"late"}]}`, f.present), morningAttendancePermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// The school switched it off.
	f.switchOff(t)
	rec = f.put(t, morningMonday, mark, morningAttendancePermission)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.school_write_disabled")
	canRecord, _ = f.report(t, morningMonday, classDayReadPermission, morningAttendancePermission)
	assert.False(t, canRecord)

	var rows int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM active.morning_attendance WHERE tenant_id = ? AND student_id IN (?, ?)`,
		f.tenantID, f.present, f.other).Scan(testpkg.Ctx(t), &rows))
	assert.Zero(t, rows, "no refused write stored anything")
}

func TestMorningAttendanceRefusesADayWithoutSchool(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningSaturday)

	rec := f.put(t, morningSaturday, fmt.Sprintf(`{"marks":[{"student_id":%d,"status":"present"}]}`, f.present), morningAttendancePermission)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "classday.morning_attendance_no_school_day")
	canRecord, statuses := f.report(t, morningSaturday, classDayReadPermission, morningAttendancePermission)
	assert.False(t, canRecord)
	assert.Empty(t, statuses[fmt.Sprintf("s%d", f.sick)], "no school day, no morning status")
}
