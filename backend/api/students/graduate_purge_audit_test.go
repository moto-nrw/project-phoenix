package students_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurgeGraduatedStudent_CreatesDeletionAudits(t *testing.T) {
	tc := setupTestContext(t)
	repos := repositories.NewFactory(tc.db)
	tc.resource.StudentDeletionService = usersService.NewStudentDeletionService(
		tc.resource.StudentService,
		repos.Student,
		repos.Person,
		repos.StudentDeletion,
		repos.GradeTransition,
		repos.DataDeletion,
		repos.StudentDeletionAudit,
		tc.db,
	)
	tc.resource.GradeTransitionService = tc.services.GradeTransition

	student := testpkg.CreateTestStudent(t, tc.db, "Purge", "Audited", "4a")
	actor := testpkg.CreateTestAccount(t, tc.db, "graduate-purge-audit@example.com")
	room := testpkg.CreateTestRoom(t, tc.db, "graduate-purge-audit-room")
	instance := testpkg.CreateTestActivityInstance(t, tc.db, timezone.TodayDate().AddDays(-1), room.ID, testpkg.ActivityInstanceOpts{
		Title:         "Retained graduate roster assignment",
		IsSpontaneous: true,
	})
	assignment := testpkg.CreateTestInstanceStudent(t, tc.db, instance.ID, student.ID, "")
	graduateStudent(t, tc, student.ID)
	t.Cleanup(func() {
		_, _ = tc.db.NewDelete().TableExpr(`audit.student_deletions`).Where(`student_id = ?`, student.ID).Exec(context.Background())
		_, _ = tc.db.NewDelete().TableExpr(`audit.data_deletions`).Where(`student_id = ?`, student.ID).Exec(context.Background())
		testpkg.CleanupTableRecords(t, tc.db, "schedule.instance_students", assignment.ID)
		testpkg.CleanupTableRecords(t, tc.db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupTableRecords(t, tc.db, "facilities.rooms", room.ID)
		testpkg.CleanupActivityFixtures(t, tc.db, student.ID)
		testpkg.CleanupAuthFixtures(t, tc.db, actor.ID)
	})

	req := testutil.NewRequest(http.MethodDelete, fmt.Sprintf("/%d/purge", student.ID), nil)
	response := authExec(t, tc, req, testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())

	var deletionAudit auditModels.StudentDeletion
	require.NoError(t, tc.db.NewSelect().Model(&deletionAudit).
		Where(`tenant_id = ? AND student_id = ?`, student.TenantID, student.ID).
		Scan(testpkg.Ctx(t)))
	assert.Equal(t, actor.ID, deletionAudit.ActorAccountID)
	assert.Equal(t, usersService.StudentDeletionReasonGraduatePurge, deletionAudit.Reason)
	assert.Equal(t, 1, deletionAudit.Counts.TimetableAssignments)

	var dataAudit auditModels.DataDeletion
	require.NoError(t, tc.db.NewSelect().Model(&dataAudit).
		Where(`tenant_id = ? AND student_id = ? AND deletion_type = ?`, student.TenantID, student.ID, auditModels.DeletionTypeManual).
		Scan(testpkg.Ctx(t)))
	assert.Equal(t, 3, dataAudit.RecordsDeleted, "graduate purge deletes the student, person, and retained timetable assignment")
	assert.Equal(t, usersService.StudentDeletionReasonGraduatePurge, dataAudit.DeletionReason)
	assert.Equal(t, true, dataAudit.Metadata["student_deletion"])
	assert.Equal(t, usersModels.StudentDeletionCounts{TimetableAssignments: 1}, deletionAudit.Counts)

	var assignmentCount int
	require.NoError(t, tc.db.NewSelect().TableExpr(`schedule.instance_students`).ColumnExpr("COUNT(*)").
		Where("id = ?", assignment.ID).Scan(testpkg.Ctx(t), &assignmentCount))
	assert.Zero(t, assignmentCount, "the retained historical assignment must not block the student delete")
}
