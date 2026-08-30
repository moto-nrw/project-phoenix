package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func studentDeletionRowCount(t *testing.T, tc *testContext, table string, id int64) int {
	t.Helper()
	var count int
	require.NoError(t, tc.db.NewSelect().TableExpr(table).ColumnExpr("COUNT(*)").
		Where("id = ?", id).Scan(context.Background(), &count))
	return count
}

func TestStudentDeletionHandlers_RequirePreviewAndExplicitConfirmation(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	repos := repositories.NewFactory(tc.db)
	studentService := userService.NewStudentService(
		repos.Student,
		repos.PrivacyConsent,
		repos.StudentCompanion,
		nil,
	)
	tc.resource.StudentDeletionService = userService.NewStudentDeletionService(
		studentService,
		repos.Student,
		repos.Person,
		repos.StudentDeletion,
		repos.GradeTransition,
		repos.DataDeletion,
		repos.StudentDeletionAudit,
		tc.db,
	)

	target := testpkg.CreateTestStudent(t, tc.db, "DeleteApi", "Target", "1a")
	spared := testpkg.CreateTestStudent(t, tc.db, "DeleteApi", "Spared", "1a")
	room := testpkg.CreateTestRoom(t, tc.db, "delete-api-room")
	instance := testpkg.CreateTestActivityInstance(t, tc.db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Title:         "Shared API deletion instance",
		IsSpontaneous: true,
	})
	targetAssignment := testpkg.CreateTestInstanceStudent(t, tc.db, instance.ID, target.ID, "")
	sparedAssignment := testpkg.CreateTestInstanceStudent(t, tc.db, instance.ID, spared.ID, "")
	actor := testpkg.CreateTestAccount(t, tc.db, "student-delete-api@example.com")
	var auditID int64
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().TableExpr(`audit.data_deletions`).Where(`student_id = ?`, target.ID).Exec(context.Background())
	})
	claims := testutil.AdminTestClaims(int(actor.ID))

	previewRequest := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/%d/delete-impact", target.ID), nil)
	previewResponse := authExec(t, tc, previewRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, previewResponse.Code, "Body: %s", previewResponse.Body.String())
	var previewBody struct {
		Data struct {
			ConfirmationName string                           `json:"confirmation_name"`
			Fingerprint      string                           `json:"fingerprint"`
			Total            int                              `json:"total"`
			Counts           userModels.StudentDeletionCounts `json:"counts"`
			Preserved        struct {
				OtherStudents   bool `json:"other_students"`
				SharedInstances bool `json:"shared_instances"`
			} `json:"preserved"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(previewResponse.Body.Bytes(), &previewBody))
	assert.Equal(t, "DeleteApi Target", previewBody.Data.ConfirmationName)
	assert.NotEmpty(t, previewBody.Data.Fingerprint)
	assert.Equal(t, 1, previewBody.Data.Counts.TimetableAssignments)
	assert.Equal(t, previewBody.Data.Counts.Total(), previewBody.Data.Total)
	assert.True(t, previewBody.Data.Preserved.OtherStudents)
	assert.True(t, previewBody.Data.Preserved.SharedInstances)

	legacyRequest := testutil.NewAuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/%d", target.ID), nil)
	legacyResponse := authExec(t, tc, legacyRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusConflict, legacyResponse.Code, "Body: %s", legacyResponse.Body.String())
	assert.Equal(t, 1, studentDeletionRowCount(t, tc, "users.students", target.ID))

	wrongNameRequest := testutil.NewAuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/%d", target.ID), map[string]any{
		"expected_fingerprint": previewBody.Data.Fingerprint,
		"confirmation_name":    "Wrong Name",
		"reason":               userService.StudentDeletionReasonTestData,
		"acknowledged":         true,
	})
	wrongNameResponse := authExec(t, tc, wrongNameRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusBadRequest, wrongNameResponse.Code, "Body: %s", wrongNameResponse.Body.String())
	assert.Equal(t, 1, studentDeletionRowCount(t, tc, "users.students", target.ID))

	deleteRequest := testutil.NewAuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/%d", target.ID), map[string]any{
		"expected_fingerprint": previewBody.Data.Fingerprint,
		"confirmation_name":    previewBody.Data.ConfirmationName,
		"reason":               userService.StudentDeletionReasonTestData,
		"acknowledged":         true,
	})
	deleteResponse := authExec(t, tc, deleteRequest, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteResponse.Code, "Body: %s", deleteResponse.Body.String())

	assert.Zero(t, studentDeletionRowCount(t, tc, "users.students", target.ID))
	assert.Zero(t, studentDeletionRowCount(t, tc, "schedule.instance_students", targetAssignment.ID))
	assert.Equal(t, 1, studentDeletionRowCount(t, tc, "users.students", spared.ID))
	assert.Equal(t, 1, studentDeletionRowCount(t, tc, "schedule.instance_students", sparedAssignment.ID))
	assert.Equal(t, 1, studentDeletionRowCount(t, tc, "schedule.activity_instances", instance.ID))

	require.NoError(t, tc.db.NewRaw(`
		SELECT id
		FROM audit.student_deletions
		WHERE tenant_id = ? AND actor_account_id = ?
		ORDER BY id DESC
		LIMIT 1
	`, testpkg.Tenant(t), actor.ID).Scan(testpkg.Ctx(t), &auditID))
	assert.Positive(t, auditID)
}
