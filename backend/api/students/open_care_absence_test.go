package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Issue #2232: a school running operations.group_mode = open_care has no
// groups, so the supervisor-based write gate leaves every child admin-only.
// Staff holding users:absence must still be able to report and clear
// absences — and nothing else.

// setGroupMode overrides operations.group_mode for tenant 1 and resets it
// after the test.
func setGroupMode(t *testing.T, tc *testContext, mode string) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	require.NoError(t, tc.services.Settings.SetValue(ctx, configModel.KeyGroupMode, mode, nil, nil))
	t.Cleanup(func() {
		_ = tc.services.Settings.ResetValue(ctx, configModel.KeyGroupMode, nil, nil)
	})
}

func TestOpenCareAbsence_GrouplessStudent(t *testing.T) {
	tc := setupTestContext(t)
	setGroupMode(t, tc, configModel.GroupModeOpenCare)

	// A staff member (the Sekretariat case) who supervises nothing, and a child
	// with no group at all.
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Sekretariat", "Barnstorf")
	student := testpkg.CreateTestStudent(t, tc.db, "Gruppenlos", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID, student.ID)

	claims := testutil.TeacherTestClaims(int(account.ID))
	absencePerms := []string{"users:read", "users:absence"}
	today := timezone.TodayDate().String()

	var createdStatusDayID int64

	t.Run("can_report_planned_sick_day", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": "sick",
			"dates":  []string{today},
			"reason": "Fieber",
		})
		rr := authExec(t, tc, req, claims, absencePerms)
		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

		var env struct {
			Data []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		require.NotEmpty(t, env.Data)
		assert.Equal(t, "sick", env.Data[0].Status)
		createdStatusDayID = env.Data[0].ID
	})

	t.Run("can_read_the_planned_status_days", func(t *testing.T) {
		// The planning dialog refuses to save until it has loaded the existing
		// status days, so read access to them has to follow the write gate.
		rr := authExec(t, tc, testutil.NewRequest("GET",
			fmt.Sprintf("/%d/status-days?from=%s&to=%s", student.ID, today, today), nil), claims, absencePerms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		var env struct {
			Data []struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		require.NotEmpty(t, env.Data, "the day reported above must be visible to its author")
	})

	t.Run("can_clear_the_sick_day_again", func(t *testing.T) {
		require.NotZero(t, createdStatusDayID, "depends on the create sub-test")
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, createdStatusDayID), nil)
		rr := authExec(t, tc, req, claims, absencePerms)
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})

	t.Run("can_report_today_via_student_update", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"sick": true,
		})
		rr := authExec(t, tc, req, claims, absencePerms)
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})

	t.Run("can_clear_today_via_student_update", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"sick": false,
		})
		assert.Equal(t, http.StatusOK, authExec(t, tc, req, claims, absencePerms).Code)
	})

	t.Run("cannot_edit_stammdaten", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"school_class": "2b",
		})
		testutil.AssertForbidden(t, authExec(t, tc, req, claims, absencePerms))
	})

	t.Run("cannot_smuggle_stammdaten_alongside_the_absence", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{
			"sick":             true,
			"supervisor_notes": "smuggled",
		})
		testutil.AssertForbidden(t, authExec(t, tc, req, claims, absencePerms))
	})

	t.Run("without_the_absence_permission_it_stays_forbidden", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": "sick",
			"dates":  []string{today},
		})
		testutil.AssertForbidden(t, authExec(t, tc, req, claims, []string{"users:read", "users:update"}))
	})

	// users:absence is a write scope on top of what a caller may already see.
	// Without users:read it grants nothing — not the write, and not the read
	// the planning dialog needs — instead of half a feature for a child whose
	// list entry and detail page stay closed.
	t.Run("the_absence_permission_alone_grants_nothing", func(t *testing.T) {
		absenceOnly := []string{"users:absence"}

		write := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": "sick",
			"dates":  []string{today},
		})
		testutil.AssertForbidden(t, authExec(t, tc, write, claims, absenceOnly))

		read := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days", student.ID), nil)
		testutil.AssertForbidden(t, authExec(t, tc, read, claims, absenceOnly))
	})
}

// TestAbsenceWriterCannotEditSupervisedStudent pins the separation the
// dedicated permission exists for: PUT /students/{id} carries the Krankmeldung
// AND every Stammdaten field, and its route gate admits users:absence. The
// per-child write check decides supervision only, so without an explicit
// users:update check a group supervisor holding users:absence would inherit the
// full record write — address, class, notes — from that supervision.
func TestAbsenceWriterCannotEditSupervisedStudent(t *testing.T) {
	tc := setupTestContext(t)
	setGroupMode(t, tc, configModel.GroupModeFixedGroups)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Absence", "Supervisor")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "AbsenceWriterGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Supervised", "Kind", "AW1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)
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
			"sick":             true,
			"supervisor_notes": "smuggled",
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

// TestFixedGroupsAbsence_GrouplessStudentStaysAdminOnly pins acceptance
// criterion 7: the relaxation is bound to open care. Under fixed groups the
// users:absence permission changes nothing.
func TestFixedGroupsAbsence_GrouplessStudentStaysAdminOnly(t *testing.T) {
	tc := setupTestContext(t)
	setGroupMode(t, tc, configModel.GroupModeFixedGroups)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Fixed", "Staff")
	student := testpkg.CreateTestStudent(t, tc.db, "Fixed", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID, student.ID)

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": "sick",
		"dates":  []string{timezone.TodayDate().String()},
	})
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read", "users:update", "users:absence"})
	testutil.AssertForbidden(t, rr)
}

// TestOpenCareAbsence_DetailFlagsDiverge pins the response contract the
// frontend gates on: an absence writer sees has_absence_write_access without
// has_write_access.
func TestOpenCareAbsence_DetailFlagsDiverge(t *testing.T) {
	tc := setupTestContext(t)
	setGroupMode(t, tc, configModel.GroupModeOpenCare)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Flag", "Staff")
	student := testpkg.CreateTestStudent(t, tc.db, "Flag", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID, student.ID)

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), []string{"users:read", "users:absence"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var env struct {
		Data struct {
			HasWriteAccess        bool `json:"has_write_access"`
			HasAbsenceWriteAccess bool `json:"has_absence_write_access"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	assert.False(t, env.Data.HasWriteAccess, "Stammdaten stay read-only")
	assert.True(t, env.Data.HasAbsenceWriteAccess, "the Krankmeldung action must be offered")
}

// TestOpenCareAbsence_ParentExcusedRequestDecidable covers the parent-side
// counterpart: a guardian's excused request is equally undecidable for every
// non-admin in a school without groups, so the queue and the decision follow
// the same absence gate (#2232).
func TestOpenCareAbsence_ParentExcusedRequestDecidable(t *testing.T) {
	tc := setupTestContext(t)
	setGroupMode(t, tc, configModel.GroupModeOpenCare)

	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	defer testpkg.CleanupParentGuardianChain(t, tc.db, chain)
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Queue", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

	day := timezone.TodayDate().AddDays(3)
	var pending *activeModels.ExcusedAbsenceRequest
	require.NoError(t, tenant.WithTenantTx(adminTenantCtx(chain.TenantID), tc.db, chain.TenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			var err error
			pending, err = tc.services.ExcusedRequests.CreateRequest(txCtx, chain.StudentID, chain.AccountID, []timezone.Date{day}, "Familienfeier")
			return err
		}))
	require.NotNil(t, pending)

	claims := testutil.TeacherTestClaims(int(account.ID))
	absencePerms := []string{"users:read", "users:absence"}

	t.Run("request_is_visible_in_the_queue", func(t *testing.T) {
		rr := authExec(t, tc, testutil.NewRequest("GET", "/excused-absence-requests", nil), claims, absencePerms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		var env struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		ids := make([]string, 0, len(env.Data))
		for _, item := range env.Data {
			ids = append(ids, item.ID)
		}
		assert.Contains(t, ids, strconv.FormatInt(pending.ID, 10))
	})

	t.Run("request_can_be_approved", func(t *testing.T) {
		req := testutil.NewAuthenticatedRequest(t, "POST",
			fmt.Sprintf("/excused-absence-requests/%d/decide", pending.ID), map[string]any{"approve": true})
		rr := authExec(t, tc, req, claims, absencePerms)
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})
}

// adminTenantCtx builds the wildcard-admin, tenant-scoped context the fixture
// setup needs to file the guardian's request the way the parents portal does.
func adminTenantCtx(tenantID int64) context.Context {
	ctx := context.WithValue(testpkg.TenantContext(tenantID), jwt.CtxPermissions, []string{"admin:*"})
	return ctx
}
