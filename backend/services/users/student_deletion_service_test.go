package users_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type failingStudentDeletionAudit struct {
	err error
}

func (r failingStudentDeletionAudit) Create(context.Context, *auditModels.StudentDeletion) error {
	return r.err
}

func newStudentDeletionTestService(
	db *bun.DB,
	dataAuditRepo auditModels.DataDeletionRepository,
	auditRepo auditModels.StudentDeletionRepository,
) usersService.StudentDeletionService {
	repos := repositories.NewFactory(db)
	studentService := usersService.NewStudentService(
		repos.Student,
		repos.PrivacyConsent,
		repos.StudentCompanion,
		nil,
	)
	return usersService.NewStudentDeletionService(
		studentService,
		repos.Student,
		repos.Person,
		repos.StudentDeletion,
		repos.GradeTransition,
		dataAuditRepo,
		auditRepo,
		db,
	)
}

func cleanupStudentDeletionTest(
	t *testing.T,
	db *bun.DB,
	studentIDs, personIDs, assignmentIDs, instanceIDs, roomIDs, accountIDs, auditIDs []int64,
) {
	t.Helper()
	_, err := db.NewDelete().TableExpr(`audit.data_deletions`).Where(`student_id IN (?)`, bun.List(studentIDs)).Exec(context.Background())
	require.NoError(t, err)
	testpkg.CleanupTableRecords(t, db, "audit.student_deletions", auditIDs...)
	testpkg.CleanupTableRecords(t, db, "schedule.instance_students", assignmentIDs...)
	testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instanceIDs...)
	testpkg.CleanupTableRecords(t, db, "users.students", studentIDs...)
	testpkg.CleanupTableRecords(t, db, "users.persons", personIDs...)
	testpkg.CleanupTableRecords(t, db, "facilities.rooms", roomIDs...)
	testpkg.CleanupAuthFixtures(t, db, accountIDs...)
}

func tableRowCount(t *testing.T, db *bun.DB, table string, id int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewSelect().TableExpr(table).ColumnExpr("COUNT(*)").
		Where("id = ?", id).Scan(context.Background(), &count))
	return count
}

func TestStudentDeletionService_DeletePreservesSharedInstanceAndAnonymizesPerson(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	repos := repositories.NewFactory(db)
	service := newStudentDeletionTestService(db, repos.DataDeletion, repos.StudentDeletionAudit)
	target := testpkg.CreateTestStudent(t, db, "DeleteService", "Target", "1a")
	spared := testpkg.CreateTestStudent(t, db, "DeleteService", "Spared", "1a")
	room := testpkg.CreateTestRoom(t, db, "delete-service-room")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Title:         "Shared deletion service instance",
		IsSpontaneous: true,
	})
	targetAssignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, target.ID, "")
	sparedAssignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, spared.ID, "")
	actor := testpkg.CreateTestAccount(t, db, "student-delete-actor@example.com")
	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", actor.ID)
	history := &educationModels.GradeTransitionHistory{
		TransitionID: transition.ID,
		StudentID:    target.ID,
		PersonName:   "DeleteService Target",
		FromClass:    "1a",
		Action:       educationModels.ActionPromoted,
	}
	history.SetTenantID(target.TenantID)
	_, err := db.NewInsert().Model(history).
		ModelTableExpr(`education.grade_transition_history`).
		Exec(ctx)
	require.NoError(t, err)
	childAccount := testpkg.CreateTestAccount(t, db, "student-delete-child@example.com")
	parentAccount := testpkg.CreateTestParentAccount(t, db, "student-delete-parent@example.com")
	var messageThreadID, messageID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id)
		VALUES (?, ?, ?)
		RETURNING id
	`, target.TenantID, target.ID, parentAccount.ID).Scan(ctx, &messageThreadID))
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.parent_messages
			(tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body)
		VALUES (?, ?, ?, ?, 'guardian', 'Elternteil', 'Bitte um Rückruf')
		RETURNING id
	`, target.TenantID, messageThreadID, target.ID, parentAccount.ID).Scan(ctx, &messageID))
	_, err = db.NewRaw(`
		INSERT INTO users.parent_message_reads (tenant_id, thread_id, account_id)
		VALUES (?, ?, ?)
	`, target.TenantID, messageThreadID, actor.ID).Exec(ctx)
	require.NoError(t, err)
	card := testpkg.CreateTestRFIDCard(t, db, "STUDENTDELETE")
	_, err = db.NewRaw(`
		UPDATE users.persons
		SET account_id = ?, tag_id = ?
		WHERE id = ?
	`, childAccount.ID, card.ID, target.PersonID).Exec(ctx)
	require.NoError(t, err)
	photoPath := "students/delete-service-target.webp"
	_, err = db.NewUpdate().
		TableExpr(`users.students AS "student"`).
		Set(`photo_path = ?`, photoPath).
		Where(`"student".id = ?`, target.ID).
		Exec(ctx)
	require.NoError(t, err)
	var legacyGuardianLinkID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.persons_guardians
			(tenant_id, person_id, guardian_account_id, relationship_type)
		VALUES (?, ?, ?, 'parent')
		RETURNING id
	`, target.TenantID, target.PersonID, parentAccount.ID).Scan(ctx, &legacyGuardianLinkID))
	var auditID int64
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "users.persons_guardians", legacyGuardianLinkID)
		testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)
		cleanupStudentDeletionTest(t, db,
			[]int64{target.ID, spared.ID}, []int64{target.PersonID, spared.PersonID},
			[]int64{targetAssignment.ID, sparedAssignment.ID}, []int64{instance.ID},
			[]int64{room.ID}, []int64{actor.ID, childAccount.ID}, []int64{auditID})
		testpkg.CleanupParentAccountFixtures(t, db, parentAccount.ID)
		testpkg.CleanupRFIDCards(t, db, card.ID)
	})

	preview, err := service.Preview(ctx, target.ID)
	require.NoError(t, err)
	require.Equal(t, 1, preview.Counts.TimetableAssignments)
	require.Equal(t, 1, preview.Counts.GuardianLinks)
	require.Equal(t, 3, preview.Counts.Communications)
	require.Equal(t, 1, preview.Counts.OtherRecords)

	result, err := service.Delete(ctx, usersService.StudentDeletionInput{
		StudentID:           target.ID,
		ActorAccountID:      actor.ID,
		ExpectedFingerprint: preview.Fingerprint,
		ConfirmationName:    preview.ConfirmationName,
		Reason:              usersService.StudentDeletionReasonTestData,
		Acknowledged:        true,
	})
	require.NoError(t, err)
	assert.Equal(t, preview.Counts, result.Counts)
	assert.Equal(t, photoPath, result.PhotoPath)

	assert.Zero(t, tableRowCount(t, db, "users.students", target.ID))
	assert.Zero(t, tableRowCount(t, db, "schedule.instance_students", targetAssignment.ID))
	assert.Zero(t, tableRowCount(t, db, "users.persons_guardians", legacyGuardianLinkID))
	assert.Zero(t, tableRowCount(t, db, "users.parent_message_threads", messageThreadID))
	assert.Zero(t, tableRowCount(t, db, "users.parent_messages", messageID))
	var readCursorCount int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM users.parent_message_reads WHERE thread_id = ?
	`, messageThreadID).Scan(ctx, &readCursorCount))
	assert.Zero(t, readCursorCount)
	assert.Equal(t, 1, tableRowCount(t, db, "users.students", spared.ID))
	assert.Equal(t, 1, tableRowCount(t, db, "schedule.instance_students", sparedAssignment.ID))
	assert.Equal(t, 1, tableRowCount(t, db, "schedule.activity_instances", instance.ID))
	var historyName string
	require.NoError(t, db.NewRaw(`
		SELECT person_name FROM education.grade_transition_history WHERE id = ?
	`, history.ID).Scan(ctx, &historyName))
	assert.Equal(t, "Gelöschtes Kind", historyName)

	var anonymized struct {
		FirstName string
		LastName  string
		Birthday  *timezone.Date
		TagID     *string
		AccountID *int64
		DeletedAt *time.Time
	}
	require.NoError(t, db.NewRaw(`
		SELECT first_name, last_name, birthday, tag_id, account_id, deleted_at
		FROM users.persons
		WHERE id = ?
	`, target.PersonID).Scan(ctx, &anonymized))
	assert.Equal(t, "Gelöscht", anonymized.FirstName)
	assert.Equal(t, "Benutzer", anonymized.LastName)
	assert.Nil(t, anonymized.Birthday)
	assert.Nil(t, anonymized.TagID)
	assert.Nil(t, anonymized.AccountID)
	assert.NotNil(t, anonymized.DeletedAt)
	assert.Equal(t, 1, tableRowCount(t, db, "auth.accounts", childAccount.ID))
	assert.Equal(t, 1, tableRowCount(t, db, "auth.accounts_parents", parentAccount.ID))

	var audit struct {
		ID             int64
		StudentID      int64
		ActorAccountID int64
		Reason         string
		Counts         userModels.StudentDeletionCounts
	}
	require.NoError(t, db.NewRaw(`
		SELECT id, student_id, actor_account_id, reason, counts
		FROM audit.student_deletions
		WHERE tenant_id = ? AND actor_account_id = ?
		ORDER BY id DESC
		LIMIT 1
	`, target.TenantID, actor.ID).Scan(ctx, &audit))
	auditID = audit.ID
	assert.Equal(t, target.ID, audit.StudentID)
	assert.Equal(t, actor.ID, audit.ActorAccountID)
	assert.Equal(t, usersService.StudentDeletionReasonTestData, audit.Reason)
	assert.Equal(t, preview.Counts, audit.Counts)

	var dataAudit auditModels.DataDeletion
	require.NoError(t, db.NewSelect().Model(&dataAudit).
		Where(`tenant_id = ? AND student_id = ? AND deletion_type = ?`, target.TenantID, target.ID, auditModels.DeletionTypeManual).
		Scan(ctx))
	assert.Equal(t, preview.Counts.Total()+1, dataAudit.RecordsDeleted)
	assert.Equal(t, usersService.StudentDeletionReasonTestData, dataAudit.DeletionReason)
	assert.Equal(t, "account:"+fmt.Sprint(actor.ID), dataAudit.DeletedBy)
	assert.Equal(t, true, dataAudit.Metadata["student_deletion"])
	assert.NotNil(t, dataAudit.Metadata["counts"])
}

func TestStudentDeletionService_DeleteCountsCrossTenantVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	const hostingTenantID int64 = 91
	testpkg.CleanupTenantTestData(t, db, hostingTenantID)
	t.Cleanup(func() { testpkg.CleanupTenantTestData(t, db, hostingTenantID) })
	testpkg.EnsureTestTenant(t, db, hostingTenantID)

	ctx := testpkg.TenantContext(1)
	repos := repositories.NewFactory(db)
	service := newStudentDeletionTestService(db, repos.DataDeletion, repos.StudentDeletionAudit)
	target := testpkg.CreateTestStudent(t, db, "CrossTenant", "Visitor", "1a")
	actor := testpkg.CreateTestAccount(t, db, "student-delete-cross-tenant@example.com")
	hostingGroup := testpkg.CreateTestActiveGroupForTenant(t, db, hostingTenantID)
	hostedVisit := testpkg.CreateTestVisitForTenant(t, db, hostingTenantID, target.ID, hostingGroup.ID, time.Now(), nil)
	t.Cleanup(func() {
		cleanupStudentDeletionTest(t, db,
			[]int64{target.ID}, []int64{target.PersonID}, nil, nil, nil, []int64{actor.ID}, nil)
		testpkg.CleanupTableRecords(t, db, "active.visits", hostedVisit.ID)
	})

	preview, err := service.Preview(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, preview.Counts.AttendanceRecords)

	_, err = service.Delete(ctx, usersService.StudentDeletionInput{
		StudentID:           target.ID,
		ActorAccountID:      actor.ID,
		ExpectedFingerprint: preview.Fingerprint,
		ConfirmationName:    preview.ConfirmationName,
		Reason:              usersService.StudentDeletionReasonTestData,
		Acknowledged:        true,
	})
	require.NoError(t, err)
	assert.Zero(t, tableRowCount(t, db, "active.visits", hostedVisit.ID))
}

func TestStudentDeletionService_PreviewExcludesPreservedDeletionAudits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	repos := repositories.NewFactory(db)
	service := newStudentDeletionTestService(db, repos.DataDeletion, repos.StudentDeletionAudit)
	target := testpkg.CreateTestStudent(t, db, "Preserved", "Audit", "1a")
	actor := testpkg.CreateTestAccount(t, db, "preserved-deletion-audit@example.com")
	priorAudit := auditModels.NewDataDeletion(target.ID, auditModels.DeletionTypeVisitRetention, 1, "system")
	require.NoError(t, repos.DataDeletion.Create(ctx, priorAudit))
	var accessLogID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO audit.data_access_log
			(tenant_id, actor_account_id, actor_role, resource_type, student_id, range_start, range_end)
		VALUES (?, ?, 'admin', 'attendance_history', ?, NOW(), NOW())
		RETURNING id
	`, target.TenantID, actor.ID, target.ID).Scan(ctx, &accessLogID))
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr(`audit.data_access_log`).Where(`id = ?`, accessLogID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr(`audit.data_deletions`).Where(`student_id = ?`, target.ID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr(`audit.student_deletions`).Where(`student_id = ?`, target.ID).Exec(context.Background())
		testpkg.CleanupActivityFixtures(t, db, target.ID)
		testpkg.CleanupAuthFixtures(t, db, actor.ID)
	})

	preview, err := service.Preview(ctx, target.ID)
	require.NoError(t, err)
	assert.Zero(t, preview.Counts.OtherRecords)
	assert.Zero(t, preview.Counts.Total())

	_, err = service.Delete(ctx, usersService.StudentDeletionInput{
		StudentID:           target.ID,
		ActorAccountID:      actor.ID,
		ExpectedFingerprint: preview.Fingerprint,
		ConfirmationName:    preview.ConfirmationName,
		Reason:              usersService.StudentDeletionReasonTestData,
		Acknowledged:        true,
	})
	require.NoError(t, err)

	var retainedAuditCount int
	retainedAuditCount, err = db.NewSelect().TableExpr(`audit.data_deletions`).
		Where(`id = ?`, priorAudit.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, retainedAuditCount)
	var deletionAudit auditModels.DataDeletion
	require.NoError(t, db.NewSelect().Model(&deletionAudit).
		Where(`tenant_id = ? AND student_id = ? AND deletion_type = ?`, target.TenantID, target.ID, auditModels.DeletionTypeManual).
		Scan(ctx))
	assert.Equal(t, 1, deletionAudit.RecordsDeleted)
	var detachedStudentID *int64
	require.NoError(t, db.NewRaw(`
		SELECT student_id FROM audit.data_access_log WHERE id = ?
	`, accessLogID).Scan(ctx, &detachedStudentID))
	assert.Nil(t, detachedStudentID)
}

func TestStudentDeletionService_DeleteRejectsStalePreview(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	repos := repositories.NewFactory(db)
	service := newStudentDeletionTestService(db, repos.DataDeletion, repos.StudentDeletionAudit)
	target := testpkg.CreateTestStudent(t, db, "DeleteStale", "Target", "1a")
	room := testpkg.CreateTestRoom(t, db, "delete-stale-room")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Title:         "Stale deletion preview instance",
		IsSpontaneous: true,
	})
	actor := testpkg.CreateTestAccount(t, db, "student-delete-stale@example.com")

	preview, err := service.Preview(ctx, target.ID)
	require.NoError(t, err)
	assignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, target.ID, "")
	t.Cleanup(func() {
		cleanupStudentDeletionTest(t, db,
			[]int64{target.ID}, []int64{target.PersonID}, []int64{assignment.ID}, []int64{instance.ID},
			[]int64{room.ID}, []int64{actor.ID}, nil)
	})

	_, err = service.Delete(ctx, usersService.StudentDeletionInput{
		StudentID:           target.ID,
		ActorAccountID:      actor.ID,
		ExpectedFingerprint: preview.Fingerprint,
		ConfirmationName:    preview.ConfirmationName,
		Reason:              usersService.StudentDeletionReasonTestData,
		Acknowledged:        true,
	})
	require.ErrorIs(t, err, usersService.ErrStudentDeletionPreviewChanged)
	assert.Equal(t, 1, tableRowCount(t, db, "users.students", target.ID))
	assert.Equal(t, 1, tableRowCount(t, db, "schedule.instance_students", assignment.ID))
}

func TestStudentDeletionService_DeleteRejectsStalePreviewAfterMessageRead(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	repos := repositories.NewFactory(db)
	service := newStudentDeletionTestService(db, repos.DataDeletion, repos.StudentDeletionAudit)
	target := testpkg.CreateTestStudent(t, db, "DeleteStale", "Read", "1a")
	actor := testpkg.CreateTestAccount(t, db, "student-delete-stale-read@example.com")
	parentAccount := testpkg.CreateTestParentAccount(t, db, "student-delete-stale-read-parent@example.com")
	var messageThreadID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id)
		VALUES (?, ?, ?)
		RETURNING id
	`, target.TenantID, target.ID, parentAccount.ID).Scan(ctx, &messageThreadID))
	t.Cleanup(func() {
		cleanupStudentDeletionTest(t, db,
			[]int64{target.ID}, []int64{target.PersonID}, nil, nil, nil, []int64{actor.ID}, nil)
		testpkg.CleanupParentAccountFixtures(t, db, parentAccount.ID)
	})

	preview, err := service.Preview(ctx, target.ID)
	require.NoError(t, err)
	require.Equal(t, 1, preview.Counts.Communications)
	_, err = db.NewRaw(`
		INSERT INTO users.parent_message_reads (tenant_id, thread_id, account_id)
		VALUES (?, ?, ?)
	`, target.TenantID, messageThreadID, actor.ID).Exec(ctx)
	require.NoError(t, err)

	_, err = service.Delete(ctx, usersService.StudentDeletionInput{
		StudentID:           target.ID,
		ActorAccountID:      actor.ID,
		ExpectedFingerprint: preview.Fingerprint,
		ConfirmationName:    preview.ConfirmationName,
		Reason:              usersService.StudentDeletionReasonTestData,
		Acknowledged:        true,
	})
	require.ErrorIs(t, err, usersService.ErrStudentDeletionPreviewChanged)
	assert.Equal(t, 1, tableRowCount(t, db, "users.students", target.ID))
}

func TestStudentDeletionService_DeleteRejectsIncompleteConfirmation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	service := newStudentDeletionTestService(db, nil, nil)

	_, err := service.Delete(testpkg.TenantContext(1), usersService.StudentDeletionInput{})
	require.ErrorIs(t, err, usersService.ErrStudentDeletionPreviewChanged)

	_, err = service.Delete(testpkg.TenantContext(1), usersService.StudentDeletionInput{
		StudentID:           1,
		ActorAccountID:      1,
		ExpectedFingerprint: "aabb",
	})
	require.ErrorIs(t, err, usersService.ErrStudentDeletionNotAcknowledged)

	_, err = service.Delete(testpkg.TenantContext(1), usersService.StudentDeletionInput{
		StudentID:           1,
		ActorAccountID:      1,
		ExpectedFingerprint: "aabb",
		Acknowledged:        true,
		Reason:              "other",
	})
	require.ErrorIs(t, err, usersService.ErrStudentDeletionInvalidReason)
}

func TestStudentDeletionService_PreviewRejectsAlumnus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	student := testpkg.CreateTestStudent(t, db, "DeleteAlumnus", "Target", "1a")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "users.students", student.ID)
		testpkg.CleanupTableRecords(t, db, "users.persons", student.PersonID)
	})
	_, err := db.NewUpdate().
		TableExpr(`users.students AS "student"`).
		Set(`status = ?`, userModels.StudentStatusAlumnus).
		Where(`"student".id = ?`, student.ID).
		Exec(ctx)
	require.NoError(t, err)

	_, err = newStudentDeletionTestService(db, nil, nil).Preview(ctx, student.ID)
	require.ErrorIs(t, err, usersService.ErrStudentDeletionAlumnus)
}

func TestStudentDeletionService_DeleteRollsBackWhenAuditRepositoryIsMissing(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := testpkg.TenantContext(1)
	student := testpkg.CreateTestStudent(t, db, "DeleteMissingAudit", "Target", "1a")
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "users.students", student.ID)
		testpkg.CleanupTableRecords(t, db, "users.persons", student.PersonID)
	})
	service := newStudentDeletionTestService(db, repositories.NewFactory(db).DataDeletion, nil)
	preview, err := service.Preview(ctx, student.ID)
	require.NoError(t, err)

	_, err = service.Delete(ctx, usersService.StudentDeletionInput{
		StudentID:           student.ID,
		ActorAccountID:      1,
		ExpectedFingerprint: preview.Fingerprint,
		ConfirmationName:    preview.ConfirmationName,
		Reason:              usersService.StudentDeletionReasonTestData,
		Acknowledged:        true,
	})
	require.ErrorContains(t, err, "audit repositories are not configured")
	assert.Equal(t, 1, tableRowCount(t, db, "users.students", student.ID))
}

func TestStudentDeletionService_DeleteRollsBackWhenAuditFails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	ctx := testpkg.TenantContext(1)
	auditErr := errors.New("audit unavailable")
	service := newStudentDeletionTestService(db, repositories.NewFactory(db).DataDeletion, failingStudentDeletionAudit{err: auditErr})
	target := testpkg.CreateTestStudent(t, db, "DeleteRollback", "Target", "1a")
	room := testpkg.CreateTestRoom(t, db, "delete-rollback-room")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Title:         "Rollback deletion instance",
		IsSpontaneous: true,
	})
	assignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, target.ID, "")
	actor := testpkg.CreateTestAccount(t, db, "student-delete-rollback@example.com")
	t.Cleanup(func() {
		cleanupStudentDeletionTest(t, db,
			[]int64{target.ID}, []int64{target.PersonID}, []int64{assignment.ID}, []int64{instance.ID},
			[]int64{room.ID}, []int64{actor.ID}, nil)
	})

	preview, err := service.Preview(ctx, target.ID)
	require.NoError(t, err)
	_, err = service.Delete(ctx, usersService.StudentDeletionInput{
		StudentID:           target.ID,
		ActorAccountID:      actor.ID,
		ExpectedFingerprint: preview.Fingerprint,
		ConfirmationName:    preview.ConfirmationName,
		Reason:              usersService.StudentDeletionReasonIncorrectEntry,
		Acknowledged:        true,
	})
	require.ErrorIs(t, err, auditErr)
	assert.Equal(t, 1, tableRowCount(t, db, "users.students", target.ID))
	assert.Equal(t, 1, tableRowCount(t, db, "schedule.instance_students", assignment.ID))

	var person struct {
		FirstName string
		LastName  string
		DeletedAt *time.Time
	}
	require.NoError(t, db.NewRaw(`
		SELECT first_name, last_name, deleted_at
		FROM users.persons
		WHERE id = ?
	`, target.PersonID).Scan(ctx, &person))
	assert.Equal(t, "DeleteRollback", person.FirstName)
	assert.Equal(t, "Target", person.LastName)
	assert.Nil(t, person.DeletedAt)
}
