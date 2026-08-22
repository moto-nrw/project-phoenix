// The HTTP surface of "Betreuung beenden" (#2487): who may reach it, what a
// list shows on each side of the enrollment interval, and that the archive
// never hands the exit reason to somebody who may not read it.
package students_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func wireCareLifecycle(t *testing.T, tc *testContext) {
	t.Helper()
	repos := repositories.NewFactory(tc.db)
	tc.resource.CareLifecycleService = userService.NewCareLifecycleService(
		userService.CareLifecycleDependencies{
			StudentRepo:  repos.Student,
			PersonRepo:   repos.Person,
			CareExitRepo: repos.CareExit,
			CleanupRepo:  repos.CareExitCleanup,
			TagReleaser:  repos.GradeTransition,
			AuditService: userService.NewStudentAuditService(repos.StudentFieldEdit, slog.Default()),
			DB:           tc.db,
			Logger:       slog.Default(),
		})
}

func TestCareExitHandlers_RequireDeletePermission(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	wireCareLifecycle(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Api", "Gate", "1a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-exit-gate@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	body := map[string]any{
		"student_ids":   []string{fmt.Sprintf("%d", student.ID)},
		"last_care_day": timezone.TodayDate().String(),
		"reason":        userModels.CareExitReasonMovedAway,
	}

	// users:update is the permission the ordinary child edit needs. It is
	// deliberately NOT enough to end a care relationship.
	for _, path := range []string{"/care-end/preview", "/care-end", "/care-end/cancel"} {
		request := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body)
		response := authExec(t, tc, request, claims, []string{"users:update"})
		assert.Equal(t, http.StatusForbidden, response.Code,
			"%s must be gated on users:delete. Body: %s", path, response.Body.String())
	}

	archiveRequest := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/ended-care", nil)
	archiveResponse := authExec(t, tc, archiveRequest, claims, []string{"users:update"})
	assert.Equal(t, http.StatusForbidden, archiveResponse.Code,
		"the archive carries the exit reason and is gated with it")
}

func TestCareExitHandlers_PreviewThenConfirm(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	wireCareLifecycle(t, tc)

	student := testpkg.CreateTestStudent(t, tc.db, "Api", "Exit", "2a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-exit-happy@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	today := timezone.TodayDate()

	body := map[string]any{
		"student_ids":   []string{fmt.Sprintf("%d", student.ID)},
		"last_care_day": today.String(),
		"reason":        userModels.CareExitReasonOther,
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
		// Same shape, different content: a token from another state.
		stale["token"] = "00" + preview.Data.Token[2:]
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

	stored, err := repositories.NewFactory(tc.db).Student.FindByID(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.EnrolledUntil)
	assert.Equal(t, today, *stored.EnrolledUntil)

	t.Run("the planned end is cancellable while it is still ahead", func(t *testing.T) {
		request := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/cancel", map[string]any{
			"student_ids": []string{fmt.Sprintf("%d", student.ID)},
		})
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())

		after, err := repositories.NewFactory(tc.db).Student.FindByID(testpkg.Ctx(t), student.ID)
		require.NoError(t, err)
		assert.Nil(t, after.EnrolledUntil)
	})
}

func TestCareExitHandlers_ValidateCareExitDatesAndReasonNote(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	wireCareLifecycle(t, tc)
	student := testpkg.CreateTestStudent(t, tc.db, "Api", "Validation", "2a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-exit-validation@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))

	missingDate := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/preview", map[string]any{
		"student_ids": []string{fmt.Sprintf("%d", student.ID)}, "reason": userModels.CareExitReasonMovedAway,
	})
	missingDateResponse := authExec(t, tc, missingDate, claims, []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, missingDateResponse.Code)
	assert.Contains(t, missingDateResponse.Body.String(), "Bitte geben Sie den letzten Betreuungstag an.")

	longNote := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/preview", map[string]any{
		"student_ids": []string{fmt.Sprintf("%d", student.ID)}, "last_care_day": timezone.TodayDate().String(),
		"reason": userModels.CareExitReasonOther, "reason_note": strings.Repeat("x", userModels.MaxCareExitNoteLen+1),
	})
	longNoteResponse := authExec(t, tc, longNote, claims, []string{"admin:*"})
	assert.Equal(t, http.StatusBadRequest, longNoteResponse.Code)
	assert.Contains(t, longNoteResponse.Body.String(), "Die Begründung ist zu lang.")
}

func TestStudentList_CareStatusDecidesWhichSideIsShown(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	running := testpkg.CreateTestStudent(t, tc.db, "Listed", "Running", "3a")
	planned := testpkg.CreateTestStudent(t, tc.db, "Listed", "Planned", "3a")
	ended := testpkg.CreateTestStudent(t, tc.db, "Listed", "Ended", "3a")

	today := timezone.TodayDate()
	setEnrolledUntil(t, tc, planned.ID, today.AddDays(14))
	setEnrolledUntil(t, tc, ended.ID, today.AddDays(-1))

	actor := testpkg.CreateTestAccount(t, tc.db, "care-list@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))

	listed := func(query string) map[int64]bool {
		t.Helper()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/"+query, nil)
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())
		var body struct {
			Data []struct {
				ID         int64  `json:"id"`
				CareEndsOn string `json:"care_ends_on"`
				CareEnded  bool   `json:"care_ended"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		out := map[int64]bool{}
		for _, row := range body.Data {
			out[row.ID] = row.CareEnded
		}
		return out
	}

	t.Run("the default list hides a child whose care has ended", func(t *testing.T) {
		rows := listed("?page_size=500")
		assert.Contains(t, rows, running.ID)
		assert.Contains(t, rows, planned.ID, "a PLANNED exit stays in the list")
		assert.NotContains(t, rows, ended.ID)
	})

	t.Run("care_status=ended shows exactly the other side", func(t *testing.T) {
		rows := listed("?page_size=500&care_status=ended")
		assert.Contains(t, rows, ended.ID)
		assert.True(t, rows[ended.ID], "the payload says the care has ended")
		assert.NotContains(t, rows, running.ID)
		assert.NotContains(t, rows, planned.ID)
	})

	t.Run("care_status=all shows both", func(t *testing.T) {
		rows := listed("?page_size=500&care_status=all")
		assert.Contains(t, rows, running.ID)
		assert.Contains(t, rows, planned.ID)
		assert.Contains(t, rows, ended.ID)
	})

	t.Run("an unknown care_status is rejected, not silently ignored", func(t *testing.T) {
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?care_status=vielleicht", nil)
		response := authExec(t, tc, request, claims, []string{"admin:*"})
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	// The archive view sits behind users:delete. Without this the same set of
	// children is one query parameter away for anybody who may read the list
	// at all (#2487).
	t.Run("plain users:read cannot reach the departed side", func(t *testing.T) {
		readOnly := []string{"users:read"}
		for _, query := range []string{"?care_status=ended", "?care_status=all"} {
			request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/"+query, nil)
			response := authExec(t, tc, request, claims, readOnly)
			assert.Equal(t, http.StatusForbidden, response.Code, "query %s", query)
		}

		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?page_size=500", nil)
		response := authExec(t, tc, request, claims, readOnly)
		require.Equal(t, http.StatusOK, response.Code, "the ordinary list stays open")
	})
}

// The list has to say WHY a child carries an end date: a recorded exit can be
// changed and cancelled here, the mere end of an enrolment phase cannot. Told
// apart by the exit row, never by how far the date lies ahead — a school that
// plans a departure for the end of the school year must still be able to take
// it back (#2487).
func TestStudentList_MarksRecordedExitsOnly(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	wireCareLifecycle(t, tc)

	recorded := testpkg.CreateTestStudent(t, tc.db, "Flagged", "Recorded", "4a")
	phaseEnd := testpkg.CreateTestStudent(t, tc.db, "Flagged", "PhaseEnd", "4a")
	actor := testpkg.CreateTestAccount(t, tc.db, "care-flag@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	today := timezone.TodayDate()

	// Both carry the same end date, far ahead. Only one of them was entered
	// through "Betreuung beenden".
	farAhead := today.AddDays(200)
	setEnrolledUntil(t, tc, phaseEnd.ID, farAhead)
	confirmCareExitVia(t, tc, claims, recorded.ID, farAhead)

	request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/?page_size=500", nil)
	response := authExec(t, tc, request, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())

	var body struct {
		Data []struct {
			ID               int64  `json:"id"`
			CareEndsOn       string `json:"care_ends_on"`
			CareExitRecorded bool   `json:"care_exit_recorded"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))

	flags := map[int64]bool{}
	dates := map[int64]string{}
	for _, row := range body.Data {
		flags[row.ID] = row.CareExitRecorded
		dates[row.ID] = row.CareEndsOn
	}
	require.Contains(t, flags, recorded.ID, "a planned exit stays in the ordinary list")
	require.Contains(t, flags, phaseEnd.ID)
	assert.Equal(t, farAhead.String(), dates[recorded.ID])
	assert.Equal(t, farAhead.String(), dates[phaseEnd.ID])
	assert.True(t, flags[recorded.ID], "an entered exit can be changed and cancelled")
	assert.False(t, flags[phaseEnd.ID], "the end of an enrolment phase is not an exit")
}

// confirmCareExitVia ends a child's care through the real HTTP surface, so the
// exit row is written exactly the way the product writes it.
func confirmCareExitVia(t *testing.T, tc *testContext, claims jwt.AppClaims, studentID int64, lastCareDay timezone.Date) {
	t.Helper()
	body := map[string]any{
		"student_ids":   []string{fmt.Sprintf("%d", studentID)},
		"last_care_day": lastCareDay.String(),
		"reason":        userModels.CareExitReasonNoCareNeed,
	}
	previewRequest := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/preview", body)
	previewResponse := authExec(t, tc, previewRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, previewResponse.Code, "Body: %s", previewResponse.Body.String())

	var preview struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(previewResponse.Body.Bytes(), &preview))
	body["token"] = preview.Data.Token

	confirmRequest := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end", body)
	confirmResponse := authExec(t, tc, confirmRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, confirmResponse.Code, "Body: %s", confirmResponse.Body.String())
}

func setEnrolledUntil(t *testing.T, tc *testContext, studentID int64, day timezone.Date) {
	t.Helper()
	_, err := tc.db.NewUpdate().
		TableExpr("users.students").
		Set("enrolled_until = ?", day).
		Where("id = ?", studentID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}
