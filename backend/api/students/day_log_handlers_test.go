package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dayLogTestBody struct {
	Data struct {
		Date   string `json:"date"`
		Groups []struct {
			GroupID  string `json:"group_id"`
			Name     string `json:"name"`
			Students []struct {
				StudentID    string  `json:"student_id"`
				FirstName    string  `json:"first_name"`
				LastName     string  `json:"last_name"`
				Status       string  `json:"status"`
				Label        string  `json:"label"`
				CheckInTime  *string `json:"check_in_time"`
				CheckOutTime *string `json:"check_out_time"`
				Source       string  `json:"source"`
			} `json:"students"`
			Counters struct {
				Present int `json:"present"`
				Sick    int `json:"sick"`
				Excused int `json:"excused"`
				Absent  int `json:"absent"`
				Total   int `json:"total"`
			} `json:"counters"`
		} `json:"groups"`
		Counters struct {
			Present int `json:"present"`
			Sick    int `json:"sick"`
			Excused int `json:"excused"`
			Absent  int `json:"absent"`
			Total   int `json:"total"`
		} `json:"counters"`
	} `json:"data"`
}

func TestGetStudentsDayLog_FeatureDisabled(t *testing.T) {
	tc := setupTestContext(t)

	req := testutil.NewRequest("GET", "/day-log", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "feature_disabled")
}

func TestGetStudentsDayLog_AdminSeesStatuses(t *testing.T) {
	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "DayLog Gruppe A")
	present := testpkg.CreateTestStudent(t, tc.db, "Paula", "Praesent", "1a")
	sick := testpkg.CreateTestStudent(t, tc.db, "Sina", "Krank", "1a")
	excused := testpkg.CreateTestStudent(t, tc.db, "Emil", "Entschuldigt", "1b")
	missing := testpkg.CreateTestStudent(t, tc.db, "Momo", "Fehlt", "1b")
	staff := testpkg.CreateTestStaff(t, tc.db, "DayLog", "Staff")
	device := testpkg.CreateTestDevice(t, tc.db, "day-log-dev")
	// Students before the group: cleanup deletes in argument order, and the
	// group row only deletes cleanly once no student references it.
	defer testpkg.CleanupActivityFixtures(t, tc.db, present.ID, sick.ID, excused.ID, missing.ID, staff.ID, device.ID, group.ID)

	for _, s := range []int64{present.ID, sick.ID, excused.ID, missing.ID} {
		testpkg.AssignStudentToGroup(t, tc.db, s, group.ID)
	}

	checkIn := timezone.Today().Add(8 * time.Hour)
	checkOut := timezone.Today().Add(15 * time.Hour)
	att := testpkg.CreateTestAttendance(t, tc.db, present.ID, staff.ID, device.ID, checkIn, &checkOut)
	defer testpkg.CleanupTableRecords(t, tc.db, "active.attendance", att.ID)

	today := timezone.TodayDate()
	sickDay := testpkg.CreateTestStudentStatusDay(t, tc.db, sick.ID, today, active.StudentStatusDaySick)
	excusedDay := testpkg.CreateTestStudentStatusDay(t, tc.db, excused.ID, today, active.StudentStatusDayExcused)
	defer testpkg.CleanupStudentStatusDays(t, tc.db, sickDay.ID, excusedDay.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/day-log?group_id=%d", group.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body dayLogTestBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	assert.Equal(t, today.String(), body.Data.Date)
	require.Len(t, body.Data.Groups, 1, "group_id filter should return exactly one group")

	got := body.Data.Groups[0]
	assert.Equal(t, fmt.Sprintf("%d", group.ID), got.GroupID)
	require.Len(t, got.Students, 4)
	assert.Equal(t, 1, got.Counters.Present)
	assert.Equal(t, 1, got.Counters.Sick)
	assert.Equal(t, 1, got.Counters.Excused)
	assert.Equal(t, 1, got.Counters.Absent)
	assert.Equal(t, 4, got.Counters.Total)
	assert.Equal(t, got.Counters, body.Data.Counters)

	byID := map[string]int{}
	for i, s := range got.Students {
		byID[s.StudentID] = i
	}
	presentRow := got.Students[byID[fmt.Sprintf("%d", present.ID)]]
	assert.Equal(t, "present", presentRow.Status)
	assert.Equal(t, "Anwesend", presentRow.Label)
	require.NotNil(t, presentRow.CheckInTime)
	require.NotNil(t, presentRow.CheckOutTime)

	sickRow := got.Students[byID[fmt.Sprintf("%d", sick.ID)]]
	assert.Equal(t, "sick", sickRow.Status)
	assert.Equal(t, "Krank", sickRow.Label)
	assert.Equal(t, "manual", sickRow.Source)

	excusedRow := got.Students[byID[fmt.Sprintf("%d", excused.ID)]]
	assert.Equal(t, "excused", excusedRow.Status)
	assert.Equal(t, "Entschuldigt", excusedRow.Label)

	missingRow := got.Students[byID[fmt.Sprintf("%d", missing.ID)]]
	assert.Equal(t, "absent", missingRow.Status)
	assert.Equal(t, "Abwesend", missingRow.Label)
}

func TestGetStudentsDayLog_SupervisorLimitedToOwnGroup(t *testing.T) {
	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "DayLog", "Supervisor")
	ownGroup := testpkg.CreateTestEducationGroup(t, tc.db, "DayLog Eigene")
	foreignGroup := testpkg.CreateTestEducationGroup(t, tc.db, "DayLog Fremde")
	groupTeacher := testpkg.CreateTestGroupTeacher(t, tc.db, ownGroup.ID, teacher.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, account.ID, ownGroup.ID, foreignGroup.ID, groupTeacher.ID)

	req := testutil.NewRequest("GET", "/day-log", nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body dayLogTestBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Data.Groups, 1, "supervisor should only see the supervised group")
	assert.Equal(t, fmt.Sprintf("%d", ownGroup.ID), body.Data.Groups[0].GroupID)

	req = testutil.NewRequest("GET", fmt.Sprintf("/day-log?group_id=%d", foreignGroup.ID), nil)
	rr = authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, rr.Code, "foreign group must be refused. Body: %s", rr.Body.String())
}

func TestGetStudentsDayLog_UnlinkedStaffAccountForbidden(t *testing.T) {
	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	// Account without a person/staff row: supervisor-only mode must answer
	// with a 403 "no permitted groups", not a 500 dependency failure.
	account := testpkg.CreateTestAccount(t, tc.db, "daylog-unlinked@example.com")
	defer testpkg.CleanupActivityFixtures(t, tc.db, account.ID)

	req := testutil.NewRequest("GET", "/day-log", nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "no_permitted_groups")
}

func TestGetStudentsDayLog_FutureDateRejected(t *testing.T) {
	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	tomorrow := timezone.TodayDate().AddDays(1)
	req := testutil.NewRequest("GET", "/day-log?date="+tomorrow.String(), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "future")
}

func TestGetStudentsDayLog_HistoricalDateRejected(t *testing.T) {
	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	tooOld := timezone.TodayDate().AddDays(-40)
	req := testutil.NewRequest("GET", "/day-log?date="+tooOld.String(), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "dated group assignments")
}

func TestGetStudentsDayLog_RejectsHistoricalRosterWithoutGroupAssignments(t *testing.T) {
	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	date := timezone.TodayDate().AddDays(-1)
	req := testutil.NewRequest("GET", "/day-log?date="+date.String(), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "dated group assignments")
}

func TestGetStudentsDayLog_WritesAuditLog(t *testing.T) {
	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "DayLog Audit")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	// Earlier day-log tests in this package also write audit rows — clear the
	// slate so the count below sees exactly this request's entry.
	_, err := tc.db.NewDelete().
		TableExpr("audit.data_access_log").
		Where("resource_type = ?", "attendance_day_log").
		Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().
			TableExpr("audit.data_access_log").
			Where("resource_type = ?", "attendance_day_log").
			Exec(context.Background())
	})

	req := testutil.NewRequest("GET", fmt.Sprintf("/day-log?group_id=%d", group.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var count int
	err = tc.db.NewSelect().
		TableExpr("audit.data_access_log").
		Where("resource_type = ?", "attendance_day_log").
		Where("student_id IS NULL").
		ColumnExpr("count(*)").
		Scan(context.Background(), &count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should have written one group-scoped audit log entry")
}

func TestExportStudentsDayLog_RendersFile(t *testing.T) {
	tc := setupTestContext(t)
	enableAttendanceLog(t, tc)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "DayLog Export")
	student := testpkg.CreateTestStudent(t, tc.db, "Exportia", "Kind", "2a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID, group.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().
			TableExpr("audit.data_access_log").
			Where("resource_type = ?", "attendance_day_log").
			Exec(context.Background())
	})

	req := testutil.NewRequest("GET", fmt.Sprintf("/day-log/export?group_id=%d&format=xlsx", group.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Header().Get("Content-Type"), "spreadsheet")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "tagesauswertung-")
	assert.NotEmpty(t, rr.Body.Bytes())

	req = testutil.NewRequest("GET", "/day-log/export?format=csv", nil)
	rr = authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, "unsupported format must be rejected")
}
