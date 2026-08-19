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
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Issue #2232 (reworked for #2329): users:absence is the write scope that lets
// a role report and clear absences WITHOUT the Stammdaten write. Since #2329
// the per-child question is only "admin or verified staff", so the group mode
// and the child's group no longer participate — what these tests still pin is
// the permission axis: what users:absence unlocks, what it must not unlock, and
// its users:read prerequisite.

func TestAbsenceWriter_GrouplessStudent(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

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

	t.Run("users_update_reports_the_absence_too", func(t *testing.T) {
		// users:update is the permission that gated these writes before #2232 and
		// still does: the route admits either, and the per-child gate asks only
		// for a staff record (#2329).
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": "sick",
			"dates":  []string{today},
		})
		rr := authExec(t, tc, req, claims, []string{"users:read", "users:update"})
		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

		var env struct {
			Data []struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
		require.NotEmpty(t, env.Data)

		// Clear it again so the shared child stays neutral for the sub-tests
		// that follow.
		del := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, env.Data[0].ID), nil)
		require.Equal(t, http.StatusOK, authExec(t, tc, del, claims, []string{"users:read", "users:update"}).Code)
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

// TestAbsenceWriterCannotEditStammdaten pins the separation the dedicated
// permission exists for: PUT /students/{id} carries the Krankmeldung AND every
// Stammdaten field, and its route gate admits users:absence. The per-child
// write check asks only for a staff record (#2329), so without the explicit
// users:update check in authorizeStudentUpdate every absence writer would
// inherit the full record write — address, class, notes — from being staff.
func TestAbsenceWriterCannotEditStammdaten(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

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

	tc := setupTestContext(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Flag", "Staff")
	student := testpkg.CreateTestStudent(t, tc.db, "Flag", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID, student.ID)

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
// counterpart: a guardian's excused request is equally undecidable for every
// non-admin in a school without groups, so the queue and the decision follow
// the same absence gate (#2232).
func TestAbsenceWriter_ParentExcusedRequestDecidable(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

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

// TestAbsenceWriter_PendingNoteReachesReviewer covers the read side of the same
// request: the parent's note is the queue's signal, so the person who decides
// the request must see it on the child's detail page. Since #2329 a staff
// reviewer reads the child fully, so the note travels alongside the rest of the
// record — a badge withheld from its decider would hide work they own.
func TestAbsenceWriter_PendingNoteReachesReviewer(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Note", "Reviewer")
	student := testpkg.CreateTestStudent(t, tc.db, "Note", "Kind", "1a")
	submitter, submitterAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Note", "Einreicher")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID, student.ID, submitter.ID)

	const note = "Kommt später, Termin beim Kinderarzt"
	require.NoError(t, tenant.WithTenantTx(context.Background(), tc.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := tc.services.ExcusedRequests.CreateRequest(
			txCtx, student.ID, submitterAccount.ID,
			[]timezone.Date{timezone.TodayDate()}, note,
		)
		return err
	}))

	rr := authExec(t, tc,
		testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil),
		testutil.TeacherTestClaims(int(account.ID)),
		[]string{"users:read", "users:absence"},
	)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var env struct {
		Data struct {
			HasFullAccess      bool    `json:"has_full_access"`
			PendingExcusedNote *string `json:"pending_excused_note"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	require.True(t, env.Data.HasFullAccess, "a verified staff reviewer reads the child (#2329)")
	require.NotNil(t, env.Data.PendingExcusedNote, "the decider of the request must see its note")
	assert.Equal(t, note, *env.Data.PendingExcusedNote)
}

// TestAbsenceWithoutReadPermissionRefused pins the read prerequisite for a
// caller holding users:absence alone. The route gate admits that permission on
// its own, so without this check the caller would inherit the absence write —
// and the guardian requests behind it — from merely being staff, in a school
// that never granted the pair.
func TestAbsenceWithoutReadPermissionRefused(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "NoRead", "Supervisor")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "NoReadGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "NoRead", "Kind", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, teacher.ID, group.ID, student.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	claims := testutil.TeacherTestClaims(int(account.ID))
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": "sick",
		"dates":  []string{timezone.TodayDate().String()},
	})
	testutil.AssertForbidden(t, authExec(t, tc, req, claims, []string{"users:absence"}))

	// The same caller WITH users:read keeps working — what was missing is the
	// permission, nothing else.
	ok := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": "sick",
		"dates":  []string{timezone.TodayDate().String()},
	})
	rr := authExec(t, tc, ok, claims, []string{"users:read", "users:absence"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
}

// adminTenantCtx builds the wildcard-admin, tenant-scoped context the fixture
// setup needs to file the guardian's request the way the parents portal does.
func adminTenantCtx(tenantID int64) context.Context {
	ctx := context.WithValue(testpkg.TenantContext(tenantID), jwt.CtxPermissions, []string{"admin:*"})
	return ctx
}
