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

// GET /students/status-days — the tenant-wide absence overview (#2288).

type statusDayOverviewTestBody struct {
	Data struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Entries []struct {
			ID          string  `json:"id"`
			StudentID   string  `json:"student_id"`
			FirstName   string  `json:"first_name"`
			LastName    string  `json:"last_name"`
			SchoolClass string  `json:"school_class"`
			GroupID     string  `json:"group_id"`
			GroupName   string  `json:"group_name"`
			Date        string  `json:"date"`
			Status      string  `json:"status"`
			Label       string  `json:"label"`
			Source      string  `json:"source"`
			Note        *string `json:"note"`
		} `json:"entries"`
	} `json:"data"`
}

func TestGetStudentStatusDaysOverview_AdminSeesEntries(t *testing.T) {
	tc := setupTestContext(t)

	groupA := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Gruppe A")
	groupB := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Gruppe B")
	sickChild := testpkg.CreateTestStudent(t, tc.db, "Selma", "Krank", "1a")
	tripChild := testpkg.CreateTestStudent(t, tc.db, "Theo", "Fahrt", "2b")
	defer testpkg.CleanupActivityFixtures(t, tc.db, sickChild.ID, tripChild.ID, groupA.ID, groupB.ID)
	testpkg.AssignStudentToGroup(t, tc.db, sickChild.ID, groupA.ID)
	testpkg.AssignStudentToGroup(t, tc.db, tripChild.ID, groupB.ID)

	today := timezone.TodayDate()
	sickDay := testpkg.CreateTestStudentStatusDay(t, tc.db, sickChild.ID, today.AddDays(2), active.StudentStatusDaySick)
	tripDay := testpkg.CreateTestStudentStatusDay(t, tc.db, tripChild.ID, today.AddDays(1), active.StudentStatusDayClassTrip)
	// Out of the default two-month window: must not be listed.
	farDay := testpkg.CreateTestStudentStatusDay(t, tc.db, sickChild.ID, timezone.NewDate(today.Year, today.Month+3, 1), active.StudentStatusDayExcused)
	clearedDay := testpkg.CreateTestStudentStatusDay(t, tc.db, tripChild.ID, today.AddDays(3), active.StudentStatusDaySick)
	defer testpkg.CleanupStudentStatusDays(t, tc.db, sickDay.ID, tripDay.ID, farDay.ID, clearedDay.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := tc.db.NewUpdate().
		Model((*active.StudentStatusDay)(nil)).
		ModelTableExpr("active.student_status_days").
		Set("cleared_at = ?", time.Now()).
		Where("id = ?", clearedDay.ID).
		Exec(ctx)
	require.NoError(t, err)

	req := testutil.NewRequest("GET", "/status-days", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body statusDayOverviewTestBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, today.String(), body.Data.From)
	require.Len(t, body.Data.Entries, 2, "cleared and out-of-range rows must be excluded")

	// Ordered by date ascending.
	first := body.Data.Entries[0]
	assert.Equal(t, fmt.Sprintf("%d", tripChild.ID), first.StudentID)
	assert.Equal(t, "Theo", first.FirstName)
	assert.Equal(t, "Fahrt", first.LastName)
	assert.Equal(t, "2b", first.SchoolClass)
	assert.Equal(t, fmt.Sprintf("%d", groupB.ID), first.GroupID)
	assert.Equal(t, groupB.Name, first.GroupName)
	assert.Equal(t, today.AddDays(1).String(), first.Date)
	assert.Equal(t, active.StudentStatusDayClassTrip, first.Status)
	assert.Equal(t, "Klassenfahrt", first.Label)

	second := body.Data.Entries[1]
	assert.Equal(t, fmt.Sprintf("%d", sickChild.ID), second.StudentID)
	assert.Equal(t, active.StudentStatusDaySick, second.Status)
	assert.Equal(t, "Krank", second.Label)
}

func TestGetStudentStatusDaysOverview_GroupFilter(t *testing.T) {
	tc := setupTestContext(t)

	groupA := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Filter A")
	groupB := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Filter B")
	inGroup := testpkg.CreateTestStudent(t, tc.db, "Ida", "Drin", "1a")
	outGroup := testpkg.CreateTestStudent(t, tc.db, "Omar", "Draussen", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, inGroup.ID, outGroup.ID, groupA.ID, groupB.ID)
	testpkg.AssignStudentToGroup(t, tc.db, inGroup.ID, groupA.ID)
	testpkg.AssignStudentToGroup(t, tc.db, outGroup.ID, groupB.ID)

	today := timezone.TodayDate()
	inDay := testpkg.CreateTestStudentStatusDay(t, tc.db, inGroup.ID, today.AddDays(1), active.StudentStatusDayExcused)
	outDay := testpkg.CreateTestStudentStatusDay(t, tc.db, outGroup.ID, today.AddDays(1), active.StudentStatusDaySick)
	defer testpkg.CleanupStudentStatusDays(t, tc.db, inDay.ID, outDay.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/status-days?group_id=%d", groupA.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body statusDayOverviewTestBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Data.Entries, 1)
	assert.Equal(t, fmt.Sprintf("%d", inGroup.ID), body.Data.Entries[0].StudentID)
}

func TestGetStudentStatusDaysOverview_PastFromRejected(t *testing.T) {
	tc := setupTestContext(t)

	yesterday := timezone.TodayDate().AddDays(-1)
	req := testutil.NewRequest("GET", "/status-days?from="+yesterday.String(), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "from must not be in the past")
}

func TestGetStudentStatusDaysOverview_RangeCapRejected(t *testing.T) {
	tc := setupTestContext(t)

	today := timezone.TodayDate()
	req := testutil.NewRequest("GET", fmt.Sprintf("/status-days?from=%s&to=%s", today.String(), today.AddDays(400).String()), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "date range cannot exceed")
}

func TestGetStudentStatusDaysOverview_UnlinkedStaffAccountForbidden(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "overview-unlinked@example.com")
	defer testpkg.CleanupActivityFixtures(t, tc.db, account.ID)

	req := testutil.NewRequest("GET", "/status-days", nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})
	assert.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "no_permitted_groups")
}

func TestGetStudentStatusDaysOverview_StaffSeesAllGroups(t *testing.T) {
	tc := setupTestContext(t)

	// #2329: tenant-wide for verified staff — no supervision narrowing.
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Overview", "Staff")
	foreignGroup := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Fremde")
	child := testpkg.CreateTestStudent(t, tc.db, "Frida", "Fremd", "3c")
	defer testpkg.CleanupActivityFixtures(t, tc.db, child.ID, teacher.ID, account.ID, foreignGroup.ID)
	testpkg.AssignStudentToGroup(t, tc.db, child.ID, foreignGroup.ID)

	day := testpkg.CreateTestStudentStatusDay(t, tc.db, child.ID, timezone.TodayDate().AddDays(1), active.StudentStatusDaySick)
	defer testpkg.CleanupStudentStatusDays(t, tc.db, day.ID)

	req := testutil.NewRequest("GET", "/status-days", nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body statusDayOverviewTestBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	found := false
	for _, entry := range body.Data.Entries {
		if entry.StudentID == fmt.Sprintf("%d", child.ID) {
			found = true
		}
	}
	assert.True(t, found, "unsupervised group's child must be listed for verified staff")
}
