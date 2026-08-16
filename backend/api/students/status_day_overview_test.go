package students_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /students/status-days — the tenant-wide absence overview (#2288).

type statusDayOverviewTestBody struct {
	Data struct {
		From   string `json:"from"`
		To     string `json:"to"`
		Groups []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"groups"`
		Page     int  `json:"page"`
		PageSize int  `json:"page_size"`
		HasMore  bool `json:"has_more"`
		Entries  []struct {
			ID          string          `json:"id"`
			StudentID   string          `json:"student_id"`
			FirstName   string          `json:"first_name"`
			LastName    string          `json:"last_name"`
			SchoolClass string          `json:"school_class"`
			GroupID     string          `json:"group_id"`
			GroupName   string          `json:"group_name"`
			Date        string          `json:"date"`
			Status      string          `json:"status"`
			Label       string          `json:"label"`
			Source      string          `json:"source"`
			Note        json.RawMessage `json:"note"`
		} `json:"entries"`
	} `json:"data"`
}

type failingOverviewEducationService struct {
	educationService.Service
}

func (failingOverviewEducationService) ListGroups(context.Context, *modelBase.QueryOptions) ([]*educationModels.Group, error) {
	return nil, errors.New("group database unavailable")
}

func TestGetStudentStatusDaysOverview_AdminSeesEntries(t *testing.T) {
	tc := setupTestContext(t)

	groupA := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Gruppe A")
	groupB := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Gruppe B")
	emptyGroup := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Ohne Abwesenheit")
	sickChild := testpkg.CreateTestStudent(t, tc.db, "Selma", "Krank", "1a")
	tripChild := testpkg.CreateTestStudent(t, tc.db, "Theo", "Fahrt", "2b")
	endedChild := testpkg.CreateTestStudent(t, tc.db, "Ehemalig", "Beendet", "3c")
	inactiveLegacyChild := testpkg.CreateTestStudent(t, tc.db, "Ehemalig", "Legacy", "3c")
	defer testpkg.CleanupActivityFixtures(t, tc.db, sickChild.ID, tripChild.ID, endedChild.ID, inactiveLegacyChild.ID, groupA.ID, groupB.ID, emptyGroup.ID)
	testpkg.AssignStudentToGroup(t, tc.db, sickChild.ID, groupA.ID)
	testpkg.AssignStudentToGroup(t, tc.db, tripChild.ID, groupB.ID)
	testpkg.AssignStudentToGroup(t, tc.db, endedChild.ID, groupA.ID)
	testpkg.AssignStudentToGroup(t, tc.db, inactiveLegacyChild.ID, groupA.ID)

	today := timezone.TodayDate()
	sickDay := testpkg.CreateTestStudentStatusDay(t, tc.db, sickChild.ID, today.AddDays(2), active.StudentStatusDaySick)
	tripDay := testpkg.CreateTestStudentStatusDay(t, tc.db, tripChild.ID, today.AddDays(1), active.StudentStatusDayClassTrip)
	// Out of the default two-month window: must not be listed.
	farDay := testpkg.CreateTestStudentStatusDay(t, tc.db, sickChild.ID, timezone.NewDate(today.Year, today.Month+3, 1), active.StudentStatusDayExcused)
	clearedDay := testpkg.CreateTestStudentStatusDay(t, tc.db, tripChild.ID, today.AddDays(3), active.StudentStatusDaySick)
	endedDay := testpkg.CreateTestStudentStatusDay(t, tc.db, endedChild.ID, today.AddDays(1), active.StudentStatusDaySick)
	legacyDay := testpkg.CreateTestStudentStatusDay(t, tc.db, inactiveLegacyChild.ID, today.AddDays(1), active.StudentStatusDayExcused)
	defer testpkg.CleanupStudentStatusDays(t, tc.db, sickDay.ID, tripDay.ID, farDay.ID, clearedDay.ID, endedDay.ID, legacyDay.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := tc.db.NewUpdate().
		Model((*active.StudentStatusDay)(nil)).
		ModelTableExpr("active.student_status_days").
		Set("cleared_at = ?", time.Now()).
		Where("id = ?", clearedDay.ID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = tc.db.NewUpdate().
		Model((*usersModel.Student)(nil)).
		ModelTableExpr("users.students").
		Set("enrolled_until = ?", today).
		Where("id = ?", endedChild.ID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = tc.db.NewUpdate().
		Model((*usersModel.Student)(nil)).
		ModelTableExpr("users.students").
		Set("status = ?", usersModel.StudentStatusInactive).
		Set("enrolled_from = NULL").
		Set("enrolled_until = NULL").
		Where("id = ?", inactiveLegacyChild.ID).
		Exec(ctx)
	require.NoError(t, err)
	privateNote := "Vertraulicher Grund"
	_, err = tc.db.NewUpdate().
		Model((*active.StudentStatusDay)(nil)).
		ModelTableExpr("active.student_status_days").
		Set("note = ?", privateNote).
		Where("id = ?", sickDay.ID).
		Exec(ctx)
	require.NoError(t, err)

	req := testutil.NewRequest("GET", "/status-days", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var auditEntry auditModels.DataAccessLog
	require.NoError(t, tc.db.NewSelect().
		Model(&auditEntry).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where("resource_type = ?", auditModels.ResourceTypeStudentStatusDayOverview).
		OrderExpr("id DESC").
		Limit(1).
		Scan(context.Background()))
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().
			TableExpr("audit.data_access_log").
			Where("id = ?", auditEntry.ID).
			Exec(context.Background())
	})
	assert.True(t, today.BerlinMidnight().Equal(auditEntry.RangeStart))
	assert.True(t, timezone.NewDate(today.Year, today.Month+2, today.Day).EndOfDay().Equal(auditEntry.RangeEnd))
	groupIDs, ok := auditEntry.Metadata["group_ids"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, groupIDs, float64(groupA.ID))
	assert.Contains(t, groupIDs, float64(groupB.ID))

	var body statusDayOverviewTestBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, today.String(), body.Data.From)
	assert.Equal(t, 1, body.Data.Page)
	assert.Equal(t, 50, body.Data.PageSize)
	assert.Contains(t, body.Data.Groups, struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{ID: fmt.Sprintf("%d", emptyGroup.ID), Name: emptyGroup.Name})
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
	assert.Nil(t, second.Note, "free-text absence reasons must not be disclosed by the overview")
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

func TestGetStudentStatusDaysOverview_PageSizeIsCapped(t *testing.T) {
	tc := setupTestContext(t)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Page Cap")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

	req := testutil.NewRequest("GET", "/status-days?page_size=10000", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	var body statusDayOverviewTestBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, 100, body.Data.PageSize)
}

func TestGetStudentStatusDaysOverview_PaginatesEligibleEntriesByName(t *testing.T) {
	tc := setupTestContext(t)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Eligibility")
	endedChild := testpkg.CreateTestStudent(t, tc.db, "A", "Beendet", "1a")
	zChild := testpkg.CreateTestStudent(t, tc.db, "Zora", "Zulu", "1a")
	aChild := testpkg.CreateTestStudent(t, tc.db, "Anna", "Alpha", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, endedChild.ID, zChild.ID, aChild.ID, group.ID)
	testpkg.AssignStudentToGroup(t, tc.db, endedChild.ID, group.ID)
	testpkg.AssignStudentToGroup(t, tc.db, zChild.ID, group.ID)
	testpkg.AssignStudentToGroup(t, tc.db, aChild.ID, group.ID)

	today := timezone.TodayDate()
	endedDay := testpkg.CreateTestStudentStatusDay(t, tc.db, endedChild.ID, today, active.StudentStatusDaySick)
	zDay := testpkg.CreateTestStudentStatusDay(t, tc.db, zChild.ID, today, active.StudentStatusDaySick)
	aDay := testpkg.CreateTestStudentStatusDay(t, tc.db, aChild.ID, today, active.StudentStatusDayExcused)
	defer testpkg.CleanupStudentStatusDays(t, tc.db, endedDay.ID, zDay.ID, aDay.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := tc.db.NewUpdate().
		Model((*usersModel.Student)(nil)).
		ModelTableExpr("users.students").
		Set("enrolled_until = ?", today.AddDays(-1)).
		Where("id = ?", endedChild.ID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = tc.db.NewUpdate().
		Model((*usersModel.Student)(nil)).
		ModelTableExpr("users.students").
		Set("enrolled_from = ?", today.AddDays(7)).
		Set("status = ?", usersModel.StudentStatusActive).
		Where("id = ?", aChild.ID).
		Exec(ctx)
	require.NoError(t, err)

	req := testutil.NewRequest("GET", "/status-days?page_size=1", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var body statusDayOverviewTestBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Data.Entries, 1)
	assert.Equal(t, fmt.Sprintf("%d", aChild.ID), body.Data.Entries[0].StudentID)
	assert.True(t, body.Data.HasMore)

	req = testutil.NewRequest("GET", "/status-days?page=2&page_size=1", nil)
	rr = authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Data.Entries, 1)
	assert.Equal(t, fmt.Sprintf("%d", zChild.ID), body.Data.Entries[0].StudentID)
	assert.False(t, body.Data.HasMore)
}

func TestGetStudentStatusDaysOverview_AuditUnavailableFailsClosed(t *testing.T) {
	tc := setupTestContext(t)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Audit Failure")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)
	tc.resource.StudentHistoryService = nil

	req := testutil.NewRequest("GET", "/status-days", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "failed to record audit trail")
}

func TestGetStudentStatusDaysOverview_ServiceUnavailableFailsClosed(t *testing.T) {
	tc := setupTestContext(t)
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Overview Service Failure")
	defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)
	tc.resource.AbsenceOverview = nil

	before, err := tc.db.NewSelect().
		Model((*auditModels.DataAccessLog)(nil)).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where("resource_type = ?", auditModels.ResourceTypeStudentStatusDayOverview).
		Count(context.Background())
	require.NoError(t, err)

	req := testutil.NewRequest("GET", "/status-days", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "absence overview service is not configured")

	after, err := tc.db.NewSelect().
		Model((*auditModels.DataAccessLog)(nil)).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where("resource_type = ?", auditModels.ResourceTypeStudentStatusDayOverview).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestGetStudentStatusDaysOverview_GroupLookupFailureIsServerError(t *testing.T) {
	tc := setupTestContext(t)
	tc.resource.EducationService = failingOverviewEducationService{Service: tc.resource.EducationService}

	req := testutil.NewRequest("GET", "/status-days", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), "failed to resolve permitted groups")
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
