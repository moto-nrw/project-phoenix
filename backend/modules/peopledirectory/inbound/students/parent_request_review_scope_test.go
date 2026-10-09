package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// #3804: one school setting decides who reviews the write queues (master
// data, care schedule with pickup time and pickup mode, offerings). The list,
// the pending count and every decision follow the same scope.
func TestParentRequestReviewScopeCoversEveryWriteQueue(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	leader, leaderAccount := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Scope", "Leitung")
	_, memberAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Scope", "Tablet")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "TeamFreigabe")
	otherGroup := testpkg.CreateTestEducationGroup(t, tc.db, "TeamFreigabeFremd")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, leader.ID)
	submitter := testpkg.CreateTestParentGuardianChain(t, tc.db).AccountID

	student := func(firstName string, groupID int64) int64 {
		t.Helper()
		row := testpkg.CreateTestStudent(t, tc.db, firstName, "Zteamfreigabe", "TF1")
		testpkg.AssignStudentToGroup(t, tc.db, row.ID, groupID)
		return row.ID
	}
	masterStudent := student("Stammdaten", group.ID)
	pickupStudent := student("Abholzeit", group.ID)
	modeStudent := student("Abholart", group.ID)
	offeringStudent := student("Angebot", group.ID)
	foreignStudent := student("Fremd", otherGroup.ID)

	masterData := func(studentID int64) int64 {
		t.Helper()
		row := &masterdatarequests.Request{StudentID: studentID, SubmittedBy: submitter,
			Target: "person", FieldKey: "first_name",
			NewValue: json.RawMessage(`"Neu"`), Status: "pending"}
		row.TenantID = testpkg.Tenant(t)
		testpkg.InsertTestMasterDataChangeRequest(t, tc.db, (*testpkg.MasterDataChangeRequestRow)(row))
		return row.ID
	}
	careSchedule := func(studentID int64, payload string) int64 {
		t.Helper()
		row := &testpkg.CareScheduleChangeRequestRow{TenantID: testpkg.Tenant(t), StudentID: studentID,
			SubmittedBy: submitter, RequestKind: "weekly_schedule", Payload: json.RawMessage(payload), Status: "pending"}
		testpkg.InsertTestCareScheduleChangeRequest(t, tc.db, row)
		return row.ID
	}
	fixture := setupCorrectionFixture(t, tc, offeringStudent, testpkg.Tenant(t), "Zteamfreigabe")
	decisions := []struct{ kind, path string }{
		{"master_data", fmt.Sprintf("/master-data-change-requests/%d/decide", masterData(masterStudent))},
		{"pickup_time", fmt.Sprintf("/care-schedule-change-requests/%d/decide", careSchedule(pickupStudent, `{"weekdays":[{"weekday":1,"pickup":"16:00"}]}`))},
		{"pickup_mode", fmt.Sprintf("/care-schedule-change-requests/%d/decide", careSchedule(modeStudent, `{"weekdays":[{"weekday":2,"mode":"alone"}]}`))},
		{"offering", fmt.Sprintf("/offering-change-requests/%d/decide", insertPendingOfferingChangeRequest(t, tc, fixture, offeringStudent, submitter).ID)},
	}
	foreignPath := fmt.Sprintf("/master-data-change-requests/%d/decide", masterData(foreignStudent))
	const ownGroupRequests, allRequests = 4, 5

	perms := []string{"users:read", "users:update"}
	leaderClaims := testutil.TeacherTestClaims(int(leaderAccount.ID))
	memberClaims := testutil.TeacherTestClaims(int(memberAccount.ID))
	openRequests := func(actor jwt.AppClaims) int {
		t.Helper()
		list := authExec(t, tc, testutil.NewRequest("GET",
			"/change-requests?view=open&search=Zteamfreigabe&types=master_data,care_schedule,offering", nil), actor, perms)
		require.Equal(t, http.StatusOK, list.Code, list.Body.String())
		var listed aggListEnvelope
		require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listed))
		count := authExec(t, tc, testutil.NewRequest("GET", "/change-requests/pending-count", nil), actor, perms)
		require.Equal(t, http.StatusOK, count.Code, count.Body.String())
		var counted struct {
			Data struct {
				PendingCount int `json:"pending_count"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(count.Body.Bytes(), &counted))
		require.Equal(t, len(listed.Data.Items), counted.Data.PendingCount, "badge and list share one scope")
		return len(listed.Data.Items)
	}
	reviewAccess := func(actor jwt.AppClaims) string {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests/access", nil), actor, perms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data struct {
				ReviewAccess string `json:"review_access"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		return body.Data.ReviewAccess
	}
	reject := func(actor jwt.AppClaims, path string) int {
		t.Helper()
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "POST", path,
			map[string]any{"approve": false, "reason": "Bitte im Büro klären"}), actor, perms)
		return rr.Code
	}
	setScope := func(scope string) {
		t.Helper()
		require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), settings.KeyParentRequestReviewScope, scope, nil, nil))
	}

	// Without an override the switch keeps today's behavior: admins only.
	require.Zero(t, openRequests(leaderClaims))
	require.Zero(t, openRequests(memberClaims))
	require.Equal(t, "none", reviewAccess(memberClaims))

	// The switch still drives the inherited choice.
	require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), settings.KeyParentRequestGroupLeaderReviewEnabled, true, nil, nil))
	require.Equal(t, ownGroupRequests, openRequests(leaderClaims))
	require.Zero(t, openRequests(memberClaims))

	// An explicit choice replaces the switch.
	setScope(settings.ParentRequestReviewScopeAdmins)
	require.Zero(t, openRequests(leaderClaims))
	for _, decision := range decisions {
		require.Equal(t, http.StatusForbidden, reject(memberClaims, decision.path), decision.kind)
	}

	setScope(settings.ParentRequestReviewScopeGroupLeaders)
	require.Equal(t, ownGroupRequests, openRequests(leaderClaims))
	require.Equal(t, "group_leader", reviewAccess(leaderClaims))
	require.Zero(t, openRequests(memberClaims))
	require.Equal(t, http.StatusForbidden, reject(leaderClaims, foreignPath), "a leader stays in their own groups")

	// The team reviews every child of the school, without any group.
	setScope(settings.ParentRequestReviewScopeAllStaff)
	require.Equal(t, allRequests, openRequests(memberClaims))
	require.Equal(t, allRequests, openRequests(leaderClaims))
	require.Equal(t, "team", reviewAccess(memberClaims))
	require.Equal(t, "admin", reviewAccess(testutil.AdminTestClaims(int(memberAccount.ID))))

	// Staff of another school holds no staff record here: no team access.
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, tc.db, otherTenant)
	_, outsiderAccount := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, otherTenant, "Scope", "Fremdschule")
	outsider := testutil.TeacherTestClaims(int(outsiderAccount.ID))
	outsiderList := authExec(t, tc, testutil.NewRequest("GET",
		"/change-requests?view=open&search=Zteamfreigabe", nil), outsider, perms)
	require.NotContains(t, outsiderList.Body.String(), "Zteamfreigabe", "no row leaks across schools")
	require.NotEqual(t, http.StatusOK, reject(outsider, foreignPath))

	for _, decision := range decisions {
		require.Equal(t, http.StatusOK, reject(memberClaims, decision.path), decision.kind)
	}
	require.Equal(t, http.StatusOK, reject(memberClaims, foreignPath))
	require.Zero(t, openRequests(memberClaims))

	var reviewer []int64
	require.NoError(t, tc.db.NewSelect().TableExpr("users.student_data_change_requests").Column("reviewed_by").
		Where("student_id IN (?, ?)", masterStudent, foreignStudent).Scan(testpkg.Ctx(t), &reviewer))
	require.Equal(t, []int64{memberAccount.ID, memberAccount.ID}, reviewer, "the team member decided as themselves")
}
