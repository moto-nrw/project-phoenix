package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestParentAbsenceReviewScopeKeepsReadsAndDecisionsConsistent(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Scope", "Reviewer")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "ReviewScope")
	testpkg.AssignStudentToGroup(t, tc.db, chain.StudentID, group.ID)
	claims := testutil.TeacherTestClaims(int(account.ID))
	perms := []string{"users:read", "users:absence"}
	const note = "Vertraulicher Termin"
	var requestID int64
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), tc.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		request, err := tc.resource.ExcusedRequestService.CreateRequest(ctx, chain.StudentID, chain.AccountID,
			[]excusedrequests.Date{excusedrequests.Date(timezone.TodayDate())}, note)
		if err == nil {
			requestID = request.ID
		}
		return err
	}))
	setScope := func(scope string) {
		t.Helper()
		require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), settings.KeyParentAbsenceReviewScope, scope, nil, nil))
	}
	checkReads := func(actor jwt.AppClaims, allowed bool) {
		t.Helper()
		list := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?types=excused&view=open", nil), actor, perms)
		require.Equal(t, http.StatusOK, list.Code, list.Body.String())
		var listed struct {
			Data struct {
				Items []json.RawMessage `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listed))
		want := 0
		if allowed {
			want = 1
		}
		require.Len(t, listed.Data.Items, want)
		count := authExec(t, tc, testutil.NewRequest("GET", "/change-requests/pending-count", nil), actor, perms)
		require.Equal(t, http.StatusOK, count.Code, count.Body.String())
		var counted struct {
			Data struct {
				PendingCount int `json:"pending_count"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(count.Body.Bytes(), &counted))
		require.Equal(t, want, counted.Data.PendingCount)
		detail := authExec(t, tc, testutil.NewRequest("GET", fmt.Sprintf("/%d", chain.StudentID), nil), actor, perms)
		require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
		var child struct {
			Data struct {
				PendingExcusedNote *string `json:"pending_excused_note"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(detail.Body.Bytes(), &child))
		if allowed {
			require.NotNil(t, child.Data.PendingExcusedNote)
			require.Equal(t, note, *child.Data.PendingExcusedNote)
		} else {
			require.Nil(t, child.Data.PendingExcusedNote)
			require.NotContains(t, list.Body.String(), note)
		}
	}
	decide := func(actor jwt.AppClaims, want int) {
		t.Helper()
		rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/excused-absence-requests/%d/decide", requestID),
			map[string]any{"approve": false, "reason": "Termin entfällt"}), actor, perms)
		require.Equal(t, want, rr.Code, rr.Body.String())
	}

	setScope(settings.ParentAbsenceReviewScopeAdmins)
	checkReads(claims, false)
	decide(claims, http.StatusForbidden)
	checkReads(testutil.AdminTestClaims(int(account.ID)), true)
	setScope(settings.ParentAbsenceReviewScopeGroupLeaders)
	checkReads(claims, false)
	decide(claims, http.StatusForbidden)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	checkReads(claims, true)
	setScope(settings.ParentAbsenceReviewScopeAdmins)
	checkReads(claims, false)
	decide(claims, http.StatusForbidden)
	setScope(settings.ParentAbsenceReviewScopeAllStaff)
	_, unassignedAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Unassigned", "Reviewer")
	unassigned := testutil.TeacherTestClaims(int(unassignedAccount.ID))
	checkReads(unassigned, true)
	masterData := &masterdatarequests.Request{
		StudentID: chain.StudentID, SubmittedBy: chain.AccountID,
		Target: "person", FieldKey: "first_name",
		NewValue: json.RawMessage(`"Neu"`), Status: "pending",
	}
	masterData.TenantID = testpkg.Tenant(t)
	testpkg.InsertTestMasterDataChangeRequest(t, tc.db, (*testpkg.MasterDataChangeRequestRow)(masterData))
	perms = []string{"users:read", "users:update", "users:absence"}
	checkReads(unassigned, true)
	otherQueue := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?types=master_data&view=open", nil), unassigned, perms)
	require.Equal(t, http.StatusOK, otherQueue.Code, otherQueue.Body.String())
	var otherList aggListEnvelope
	require.NoError(t, json.Unmarshal(otherQueue.Body.Bytes(), &otherList))
	require.Empty(t, otherList.Data.Items, "absence delegation does not expose other parent requests")
	otherDecision := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/master-data-change-requests/%d/decide", masterData.ID),
		map[string]any{"approve": false, "reason": "Nicht freigegeben"}), unassigned, perms)
	require.Equal(t, http.StatusForbidden, otherDecision.Code, otherDecision.Body.String())
	decide(unassigned, http.StatusOK)

	var stored testpkg.ExcusedAbsenceRequestRow
	require.NoError(t, tc.db.NewSelect().Model(&stored).Where("id = ?", requestID).Scan(testpkg.Ctx(t)))
	require.Equal(t, "rejected", stored.Status)
	require.NotNil(t, stored.ReviewedBy)
	require.Equal(t, unassignedAccount.ID, *stored.ReviewedBy)
	require.Equal(t, chain.AccountID, stored.SubmittedBy)

	// Direct team reports can remain admin-only while this independent
	// parent-request policy allows an unassigned reviewer to approve.
	require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), settings.KeyStudentAbsenceEditScope,
		settings.StudentAbsenceEditScopeAdmins, nil, nil))
	date := timezone.TodayDate().AddDays(1)
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), tc.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		request, err := tc.resource.ExcusedRequestService.CreateRequest(ctx, chain.StudentID, chain.AccountID,
			[]excusedrequests.Date{excusedrequests.Date(date)}, note)
		if err == nil {
			requestID = request.ID
		}
		return err
	}))
	// A stale mixed selection must not partially approve its allowed absence.
	adminList := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?view=open", nil),
		testutil.AdminTestClaims(int(account.ID)), perms)
	require.Equal(t, http.StatusOK, adminList.Code, adminList.Body.String())
	var pending struct {
		Data struct {
			Items []struct {
				RequestType     string          `json:"request_type"`
				ExpectedVersion string          `json:"expected_version"`
				Data            json.RawMessage `json:"data"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adminList.Body.Bytes(), &pending))
	require.Len(t, pending.Data.Items, 2)
	refs := make([]map[string]string, 0, 2)
	for _, item := range pending.Data.Items {
		var payload struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(item.Data, &payload))
		require.NotEmpty(t, item.ExpectedVersion)
		refs = append(refs, map[string]string{"kind": item.RequestType, "id": payload.ID, "expected_version": item.ExpectedVersion})
	}
	bulk := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "POST", "/change-requests/bulk-approve",
		map[string]any{"requests": refs, "reason": "Gemeinsam geprüft"}), unassigned, perms)
	require.Equal(t, http.StatusConflict, bulk.Code, bulk.Body.String())
	require.Contains(t, bulk.Body.String(), `"code":"bulk_approval_ineligible"`)
	var unchanged testpkg.ExcusedAbsenceRequestRow
	require.NoError(t, tc.db.NewSelect().Model(&unchanged).Where("id = ?", requestID).Scan(testpkg.Ctx(t)))
	require.Equal(t, "pending", unchanged.Status)
	statusCount, err := tc.db.NewSelect().Model((*testpkg.StudentStatusDayRow)(nil)).Where("student_id = ?", chain.StudentID).Count(testpkg.Ctx(t))
	require.NoError(t, err)
	require.Zero(t, statusCount)
	approved := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/excused-absence-requests/%d/decide", requestID),
		map[string]any{"approve": true, "reason": "Geprüft"}), unassigned, perms)
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var status testpkg.StudentStatusDayRow
	require.NoError(t, tc.db.NewSelect().Model(&status).Where("student_id = ?", chain.StudentID).
		Where("date = ?", date).Scan(testpkg.Ctx(t)))
	require.Equal(t, absencerecords.StudentStatusDayExcused, status.Status)
	require.Equal(t, absencerecords.StudentStatusSourceParent, status.Source)
	require.NotNil(t, status.GuardianAccountID)
	require.Equal(t, chain.AccountID, *status.GuardianAccountID)
	var decision testpkg.ExcusedAbsenceRequestRow
	require.NoError(t, tc.db.NewSelect().Model(&decision).Where("id = ?", requestID).Scan(testpkg.Ctx(t)))
	require.Equal(t, "approved", decision.Status)
	require.NotNil(t, decision.ReviewedBy)
	require.Equal(t, unassignedAccount.ID, *decision.ReviewedBy)
	for _, table := range []string{"active.attendance", "active.visits"} {
		count, err := tc.db.NewSelect().TableExpr(table).Where("student_id = ?", chain.StudentID).Count(testpkg.Ctx(t))
		require.NoError(t, err)
		require.Zero(t, count, "parent approval must not write actual presence")
	}
}
