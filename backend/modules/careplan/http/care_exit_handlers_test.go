package careplanhttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCareExitHandlers_RequireDeletePermission(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRouteWithCareLifecycle(t, false)

	student := testpkg.CreateTestStudent(t, tc.db, "Api", "Gate", "1a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-exit-gate@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	body := map[string]any{
		"student_ids":   []string{fmt.Sprintf("%d", student.ID)},
		"last_care_day": calendar.TodayDate().String(),
		"reason":        careplan.CareExitReasonMovedAway,
	}

	// users:update is the permission the ordinary child edit needs. It is
	// deliberately NOT enough to end a care relationship.
	for _, path := range []string{"/care-end/preview", "/care-end", "/care-end/cancel", "/care-withdrawals/1/care-end/preview", "/care-withdrawals/1/care-end"} {
		request := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body)
		response := authExec(t, tc, request, claims, []string{"users:update"})
		assert.Equal(t, http.StatusForbidden, response.Code,
			"%s must be gated on users:delete. Body: %s", path, response.Body.String())
	}
	for method, path := range map[string]string{
		http.MethodGet:    "/care-withdrawals/1/deletion-impact",
		http.MethodDelete: "/care-withdrawals/1",
	} {
		request := testutil.NewAuthenticatedRequest(t, method, path, body)
		response := authExec(t, tc, request, claims, []string{"users:update"})
		assert.Equal(t, http.StatusForbidden, response.Code,
			"%s must be gated on users:delete. Body: %s", path, response.Body.String())
	}

	archiveRequest := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/ended-care", nil)
	archiveResponse := authExec(t, tc, archiveRequest, claims, []string{"users:update"})
	assert.Equal(t, http.StatusForbidden, archiveResponse.Code,
		"the archive carries the exit reason and is gated with it")
}

func TestCareWithdrawalHandlers_ListAndChildWarningRequireDeletePermission(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, false)
	student := testpkg.CreateTestStudent(t, tc.db, "Api", "Abmeldung", "2a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-withdrawal-list@example.com")
	testpkg.CreateTestCareWithdrawalCompletionConfirmedAt(t, tc.db, student.ID, actor.ID, calendar.TodayDate(), actor.CreatedAt)
	claims := testutil.AdminTestClaims(int(actor.ID))

	for _, path := range []string{"/care-withdrawals", fmt.Sprintf("/care-withdrawals?student_id=%d", student.ID)} {
		forbidden := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil), claims, []string{"users:update"})
		assert.Equal(t, http.StatusForbidden, forbidden.Code)
		allowed := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil), claims, []string{"users:delete"})
		require.Equal(t, http.StatusOK, allowed.Code, "Body: %s", allowed.Body.String())
		assert.Contains(t, allowed.Body.String(), "overdue")
	}
	invalid := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/care-withdrawals?student_id=invalid", nil), claims, []string{"users:delete"})
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
}

func TestCareWithdrawalHandlers_PreviewThenConfirmOneTask(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, false)
	repos := newStudentTestPeopleRows(testpkg.NewDatabase(t, tc.db))
	student := testpkg.CreateTestStudent(t, tc.db, "Api", "Abschluss", "3a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-withdrawal-confirm@example.com")
	completion := testpkg.CreateTestCareWithdrawalCompletion(t, tc.db, student.ID, actor.ID, calendar.TodayDate().AddDays(1))
	claims := testutil.AdminTestClaims(int(actor.ID))
	body := map[string]any{
		"last_care_day": calendar.TodayDate().String(),
		"reason":        careplan.CareExitReasonNoCareNeed,
	}
	previewRequest := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/care-withdrawals/%d/care-end/preview", completion.ID), body,
	)
	previewResponse := authExec(t, tc, previewRequest, claims, []string{"users:delete"})
	require.Equal(t, http.StatusOK, previewResponse.Code, "Body: %s", previewResponse.Body.String())
	var preview struct {
		Data struct {
			Token  string `json:"token"`
			Reason string `json:"reason"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(previewResponse.Body.Bytes(), &preview))
	require.NotEmpty(t, preview.Data.Token)
	assert.Equal(t, careplan.CareExitReasonNoCareNeed, preview.Data.Reason)
	body["token"] = preview.Data.Token

	confirmRequest := testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/care-withdrawals/%d/care-end", completion.ID), body,
	)
	confirmResponse := authExec(t, tc, confirmRequest, claims, []string{"users:delete"})
	require.Equal(t, http.StatusOK, confirmResponse.Code, "Body: %s", confirmResponse.Body.String())
	pending, _, err := api.PendingStudentHTTPTestCareWithdrawals(testpkg.Ctx(t), repos.CareWithdrawal, student.ID)
	require.NoError(t, err)
	assert.Empty(t, pending)
	resolved, err := repos.CareWithdrawal.FindByID(testpkg.Ctx(t), completion.ID)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, careplan.WithdrawalStateResolved, resolved.State)
}

func TestCareExitHandlers_PreviewThenConfirm(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)

	student := testpkg.CreateTestStudent(t, tc.db, "Api", "Exit", "2a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-exit-happy@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	today := calendar.NewDate(2026, 8, 24)

	body := map[string]any{
		"student_ids":   []string{fmt.Sprintf("%d", student.ID)},
		"last_care_day": today.String(),
		"reason":        careplan.CareExitReasonOther,
		"reason_note":   "Wechsel in den Hort",
	}

	previewRequest := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/preview", body)
	previewResponse := authExec(t, tc, previewRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, previewResponse.Code, "Body: %s", previewResponse.Body.String())

	var preview struct {
		Data struct {
			Token    string `json:"token"`
			Blocked  bool   `json:"blocked"`
			Students []struct {
				StudentID string `json:"student_id"`
				FirstName string `json:"first_name"`
				Blocker   string `json:"blocker"`
			} `json:"students"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(previewResponse.Body.Bytes(), &preview))
	require.NotEmpty(t, preview.Data.Token)
	require.Len(t, preview.Data.Students, 1)
	assert.Equal(t, "Api", preview.Data.Students[0].FirstName,
		"the preview names every child, it does not just count them")
	assert.Empty(t, preview.Data.Students[0].Blocker)
	assert.False(t, preview.Data.Blocked)

	t.Run("a stale token is refused with a conflict", func(t *testing.T) {
		stale := map[string]any{}
		for key, value := range body {
			stale[key] = value
		}
		// Same length and hex alphabet, different digest. Prefixing "00"
		// is a no-op when the SHA-256 already starts with 00.
		stale["token"] = flipCareExitTokenHex(t, preview.Data.Token)
		request := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end", stale)
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		assert.Equal(t, http.StatusConflict, response.Code, "Body: %s", response.Body.String())
	})

	confirmBody := map[string]any{}
	for key, value := range body {
		confirmBody[key] = value
	}
	confirmBody["token"] = preview.Data.Token
	confirmRequest := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end", confirmBody)
	confirmResponse := authExec(t, tc, confirmRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, confirmResponse.Code, "Body: %s", confirmResponse.Body.String())

	stored, err := newStudentTestEnrollment(testpkg.NewDatabase(t, tc.db)).Student.FindByID(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.EnrolledUntil)
	assert.Equal(t, today, *stored.EnrolledUntil)

	t.Run("the planned end is cancellable while it is still ahead", func(t *testing.T) {
		request := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/cancel", map[string]any{
			"student_ids": []string{fmt.Sprintf("%d", student.ID)},
		})
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())

		after, err := newStudentTestEnrollment(testpkg.NewDatabase(t, tc.db)).Student.FindByID(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		assert.Nil(t, after.EnrolledUntil)
	})
}

func TestCareExitHandlers_ValidateCareExitDatesAndReasonNote(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRouteWithCareLifecycle(t, false)
	student := testpkg.CreateTestStudent(t, tc.db, "Api", "Validation", "2a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-exit-validation@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))

	missingDate := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/preview", map[string]any{
		"student_ids": []string{fmt.Sprintf("%d", student.ID)}, "reason": careplan.CareExitReasonMovedAway,
	})
	missingDateResponse := authExec(t, tc, missingDate, claims, []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, missingDateResponse.Code)
	assert.Contains(t, missingDateResponse.Body.String(), "bitte geben Sie den letzten Betreuungstag an")

	longNote := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/preview", map[string]any{
		"student_ids": []string{fmt.Sprintf("%d", student.ID)}, "last_care_day": calendar.TodayDate().String(),
		"reason": careplan.CareExitReasonOther, "reason_note": strings.Repeat("x", careplan.MaxCareExitNoteLen+1),
	})
	longNoteResponse := authExec(t, tc, longNote, claims, []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, longNoteResponse.Code)
	assert.Contains(t, longNoteResponse.Body.String(), "die Begründung ist zu lang.")
}

// A child the school no longer knows and a child who left at the grade
// transition are expected refusals with their own code, not 500s (#4407).
func TestCareResumeHandler_RefusesUnknownAndGraduatedChildWithCodes(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)
	actor := testpkg.CreateTestAccount(t, tc.db, "care-resume-refusals@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	body := map[string]any{"new_start": studentsTestToday.String(), "checked": true}

	graduated := testpkg.CreateTestStudent(t, tc.db, "Api", "Abgang", "4a")
	_, err := tc.db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("status = ?", careplan.StudentStatusAlumnus).
		Where("student_profile_id = ? AND deleted_at IS NULL", graduated.ID).
		Exec(context.Background())
	require.NoError(t, err)

	cases := []struct {
		name      string
		studentID int64
		status    int
		code      string
	}{
		{"unknown child", graduated.ID + 1_000_000, http.StatusNotFound, common.CodeTimetableStudentNotFound},
		{"child left at grade transition", graduated.ID, http.StatusConflict, common.CodeStudentsCareResumeAlumnus},
	}
	for _, c := range cases {
		request := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/%d/care-end/resume", c.studentID), body)
		response := authExec(t, tc, request, claims, []string{"users:delete"})
		require.Equal(t, c.status, response.Code, "%s. Body: %s", c.name, response.Body.String())
		var problem struct {
			Code string `json:"code"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
		assert.Equal(t, c.code, problem.Code, c.name)
	}
}

func flipCareExitTokenHex(t *testing.T, token string) string {
	t.Helper()
	require.GreaterOrEqual(t, len(token), 2, "care-exit tokens are SHA-256 hex")
	flipped := []byte(token)
	if flipped[0] == '0' {
		flipped[0] = '1'
	} else {
		flipped[0] = '0'
	}
	return string(flipped)
}

// The transport reaches the workflow even without the domain lifecycle. The
// real root binding must retain membership, ledger and audit effects.
func TestCareResumeHandlerUsesIndependentWorkflowEntry(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)
	tc.resource.CareLifecycleService = nil
	today := studentsTestToday
	student := testpkg.CreateTestStudent(t, tc.db, "Returning", "Child", "2a")
	actor := testpkg.CreateTestAccount(t, tc.db, "resume-entry")
	ctx := testpkg.Ctx(t)
	ended := today.AddDays(-1)
	_, err := tc.db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", ended).Set("status = ?", careplan.StudentStatusInactive).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = tc.db.NewRaw(`INSERT INTO users.student_care_exits (tenant_id, student_id, reason) VALUES (?, ?, 'moved_away')`, testpkg.Tenant(t), student.ID).Exec(ctx)
	require.NoError(t, err)
	path := fmt.Sprintf("/%d/care-end/resume", student.ID)
	request := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, map[string]any{"new_start": today.String(), "checked": true})
	response := authExec(t, tc, request, testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())
	var body struct {
		Data struct {
			Resumed  bool   `json:"resumed"`
			NewStart string `json:"new_start"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.True(t, body.Data.Resumed)
	assert.Equal(t, today.String(), body.Data.NewStart)
	var membership struct {
		EnrolledFrom  *calendar.Date `bun:"enrolled_from"`
		EnrolledUntil *calendar.Date `bun:"enrolled_until"`
		Status        string         `bun:"status"`
	}
	require.NoError(t, tc.db.NewSelect().TableExpr("users.student_school_memberships").Column("enrolled_from", "enrolled_until", "status").Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).Scan(ctx, &membership))
	require.NotNil(t, membership.EnrolledFrom)
	assert.Equal(t, today, *membership.EnrolledFrom)
	assert.Nil(t, membership.EnrolledUntil)
	assert.Equal(t, careplan.StudentStatusActive, membership.Status)
	exits, err := tc.db.NewSelect().TableExpr("users.student_care_exits").Where("student_id = ?", student.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, exits)
	edits, err := tc.db.NewSelect().TableExpr("audit.student_field_edits").Where("student_id = ?", student.ID).Where("field_name = ?", "care_end").Where("edited_by = ?", actor.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), edits, "the workflow keeps the care-end audit")
}
