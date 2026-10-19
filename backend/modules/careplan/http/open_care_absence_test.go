package careplanhttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestAbsenceWriterCannotEditStammdaten pins the separation the dedicated
// permission exists for: PUT /students/{id} carries the Krankmeldung AND every
// Stammdaten field, and its route gate admits users:absence. The per-child
// write check asks only for a staff record (#2329), so without the explicit
// users:update check in authorizeStudentUpdate every absence writer would
// inherit the full record write — address, class, notes — from being staff.
func TestAbsenceWriterCannotEditStammdaten(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Absence", "Supervisor")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AbsenceWriterGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Supervised", "Kind", "AW1")
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	claims := testutil.TeacherTestClaims(int(account.ID))
	absencePerms := []string{"users:read", "users:absence"}

	t.Run("stammdaten_stay_forbidden", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"school_class": "3c",
		})
		testutil.AssertForbidden(t, authExec(t, tc, req, claims, absencePerms))
	})

	t.Run("stammdaten_alongside_the_absence_stay_forbidden", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"sick":        true,
			"health_info": "smuggled",
		})
		testutil.AssertForbidden(t, authExec(t, tc, req, claims, absencePerms))
	})

	t.Run("the_absence_itself_is_allowed", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"sick": true,
		})
		rr := authExec(t, tc, req, claims, absencePerms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})

	t.Run("users_update_still_edits_stammdaten", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"school_class": "3c",
		})
		rr := authExec(t, tc, req, claims, []string{"users:read", "users:update"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})
}

// TestAbsenceWriter_DetailFlags pins the response contract the frontend gates
// on: an absence writer is offered the Krankmeldung action while the Stammdaten
// write stays refused.
//
// has_write_access is deliberately NOT asserted here: it is built from
// checkStudentFullAccess, which since #2329 answers "admin or staff" without
// re-checking users:update, so it no longer mirrors the outcome of the PUT
// below. Whether that flag should carry the permission is an open follow-up —
// this test asserts the enforced behavior, not the advisory flag.
func TestAbsenceWriter_DetailFlags(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Flag", "Staff")
	student := testpkg.CreateTestStudent(t, tc.db, "Flag", "Kind", "1a")

	claims := testutil.TeacherTestClaims(int(account.ID))
	absencePerms := []string{"users:read", "users:absence"}

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, claims, absencePerms)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var env struct {
		Data struct {
			HasAbsenceWriteAccess bool `json:"has_absence_write_access"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	assert.True(t, env.Data.HasAbsenceWriteAccess, "the Krankmeldung action must be offered")

	// The enforced counterpart: without users:update the Stammdaten write stays
	// refused for the very same caller.
	stammdaten := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
		"school_class": "2b",
	})
	testutil.AssertForbidden(t, authExec(t, tc, stammdaten, claims, absencePerms))
}

// TestOpenCareAbsence_ParentExcusedRequestDecidable covers the parent-side
// counterpart: a guardian's excused request is visible and decidable for an
// administrator holding the absence permission (#2232).
func TestAbsenceWriter_ParentExcusedRequestDecidable(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Queue", "Staff")

	day := calendar.TodayDate().AddDays(3)
	var pending *excusedrequests.Request
	require.NoError(t, testpkg.WithTenantTx(t, adminTenantCtx(chain.TenantID), tc.db, chain.TenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			var err error
			pending, err = tc.resource.ExcusedRequestService.CreateRequest(txCtx, chain.StudentID, chain.AccountID, []excusedrequests.Date{excusedrequests.Date(day)}, "Familienfeier")
			return err
		}))
	require.NotNil(t, pending)

	claims := testutil.AdminTestClaims(int(account.ID))
	absencePerms := []string{"users:read", "users:absence"}

	t.Run("request_is_visible_in_the_queue", func(t *testing.T) {
		rr := authExec(t, tc, testutil.NewRequest("GET", "/change-requests?types=excused", nil), claims, absencePerms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		var env struct {
			Data struct {
				Items []struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		ids := make([]string, 0, len(env.Data.Items))
		for _, item := range env.Data.Items {
			ids = append(ids, item.Data.ID)
		}
		assert.Contains(t, ids, strconv.FormatInt(pending.ID, 10))
	})

	t.Run("request_can_be_approved", func(t *testing.T) {
		// #2267: reason policy defaults to "both"
		req := testutil.NewAuthenticatedRequest(t, "POST",
			fmt.Sprintf("/excused-absence-requests/%d/decide", pending.ID),
			map[string]any{"approve": true, "reason": "Passt so"})
		rr := authExec(t, tc, req, claims, absencePerms)
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})
}

// adminTenantCtx builds the wildcard-admin, tenant-scoped context the fixture
// setup needs to file the guardian's request the way the parents portal does.
func adminTenantCtx(tenantID int64) context.Context {
	ctx := context.WithValue(testpkg.TenantContext(tenantID), sessiontoken.CtxPermissions, []string{"admin:*"})
	return ctx
}
