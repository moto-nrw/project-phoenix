// The student deletion workflow (#2710) is the only coordinator of a
// permanent child deletion. These tests compose the real owners the way the
// production root does and prove the ticket's acceptance list: preview and
// execute agree, every owner performs its own mutation inside one unit of
// work, a failure after any command rolls everything back, a stale preview
// is refused before the cascade, retention/history rules hold, and cleanup
// intents survive the cascade as durable rows.
package users_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type deletionFixture struct {
	db       *bun.DB
	repos    *repositories.Factory
	people   peopledirectory.Capability
	deps     studentdeletion.Dependencies
	actorID  int64
	feedback *testpkg.FeedbackEntryCounterMock
}

// newDeletionFixture assembles the production composition over the real
// owners and replaces only the HTTP principal with a fixed actor.
func newDeletionFixture(t *testing.T, db *bun.DB) *deletionFixture {
	t.Helper()
	timetableDeps := repositories.NewUnobservedTimetableDependencies(db)
	repos := repositories.NewFactory(db, timetableDeps)
	people, err := repositories.NewPeopleDirectory(db)
	require.NoError(t, err)
	actor := testpkg.CreateTestAccount(t, db, "student-delete-actor@example.com")
	feedback := &testpkg.FeedbackEntryCounterMock{}
	deps, err := studentdeletioncompose.Assemble(studentdeletioncompose.Dependencies{
		DB: db, Directory: people, CarePlan: repos.CarePlan(), Timetable: timetableDeps.Capability, Feedback: feedback,
		IsVerifiedStaff:       func(context.Context) (bool, error) { return true, nil },
		LockCareBookingWrites: func(ctx context.Context) error { return scheduleSvc.LockTenantRecurrenceWrites(ctx, db) },
	})
	require.NoError(t, err)
	deps.Authorize = func(ctx context.Context) (studentdeletion.Actor, error) {
		return studentdeletion.Actor{TenantID: tenant.FromContext(ctx), AccountID: actor.ID}, nil
	}
	return &deletionFixture{db: db, repos: repos, people: people, deps: deps, actorID: actor.ID, feedback: feedback}
}

func (f *deletionFixture) workflow(t *testing.T) *studentdeletion.Workflow {
	t.Helper()
	workflow, err := studentdeletion.New(f.deps)
	require.NoError(t, err)
	return workflow
}

func confirm(preview studentdeletion.Preview, reason string) studentdeletion.Confirmation {
	return studentdeletion.Confirmation{
		ExpectedFingerprint: preview.Fingerprint, ConfirmationName: preview.ConfirmationName,
		Reason: reason, Acknowledged: true,
	}
}

func rowCount(t *testing.T, db *bun.DB, table string, id int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewSelect().TableExpr(table).ColumnExpr("COUNT(*)").
		Where("id = ?", id).Scan(context.Background(), &count))
	return count
}

func personSnapshot(t *testing.T, db *bun.DB, personID int64) (firstName string, deletedAt *time.Time) {
	t.Helper()
	var row struct {
		FirstName string
		DeletedAt *time.Time
	}
	require.NoError(t, db.NewRaw(`SELECT first_name, deleted_at FROM users.persons WHERE id = ?`, personID).
		Scan(context.Background(), &row))
	return row.FirstName, row.DeletedAt
}

func storeStudentDocument(t *testing.T, ctx context.Context, repo userModels.StudentDocumentRepository, studentID, accountID int64, category, stored string) *userModels.StudentDocument {
	t.Helper()
	doc := &userModels.StudentDocument{StudentID: studentID}
	doc.Category = category
	doc.FilenameDisplay = category + ".pdf"
	doc.FilenameStored = stored
	doc.SizeBytes = 512
	doc.ContentType = "application/pdf"
	doc.UploadedBy = accountID
	require.NoError(t, repo.Create(ctx, doc))
	return doc
}

func TestStudentDeletionWorkflow_DeletePreservesSharedInstanceAndAnonymizesPerson(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	target := testpkg.CreateTestStudent(t, db, "DeleteService", "Target", "1a")
	spared := testpkg.CreateTestStudent(t, db, "DeleteService", "Spared", "1a")
	room := testpkg.CreateTestRoom(t, db, "delete-service-room")
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Title: "Shared deletion service instance", IsSpontaneous: true,
	})
	targetAssignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, target.ID, "")
	sparedAssignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, spared.ID, "")
	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", f.actorID)
	history := &educationModels.GradeTransitionHistory{
		TransitionID: transition.ID, StudentID: target.ID, PersonName: "DeleteService Target",
		FromClass: "1a", Action: educationModels.ActionPromoted,
	}
	history.SetTenantID(target.TenantID)
	_, err := db.NewInsert().Model(history).ModelTableExpr(`education.grade_transition_history`).Exec(ctx)
	require.NoError(t, err)
	childAccount := testpkg.CreateTestAccount(t, db, "student-delete-child@example.com")
	messageGuardianAccount := testpkg.CreateTestAccount(t, db, "student-delete-message-guardian@example.com")
	legacyGuardianAccount := testpkg.CreateTestParentAccount(t, db, "student-delete-legacy-guardian@example.com")
	var messageThreadID, messageID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id)
		VALUES (?, ?, ?) RETURNING id`, target.TenantID, target.ID, messageGuardianAccount.ID).Scan(ctx, &messageThreadID))
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.parent_messages (tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body)
		VALUES (?, ?, ?, ?, 'guardian', 'Elternteil', 'Bitte um Rückruf') RETURNING id`,
		target.TenantID, messageThreadID, target.ID, messageGuardianAccount.ID).Scan(ctx, &messageID))
	_, err = db.NewRaw(`INSERT INTO users.parent_message_reads (tenant_id, thread_id, account_id) VALUES (?, ?, ?)`,
		target.TenantID, messageThreadID, f.actorID).Exec(ctx)
	require.NoError(t, err)
	card := testpkg.CreateTestRFIDCard(t, db, "STUDENTDELETE")
	_, err = db.NewRaw(`UPDATE users.persons SET account_id = ?, tag_id = ? WHERE id = ?`, childAccount.ID, card.ID, target.PersonID).Exec(ctx)
	require.NoError(t, err)
	photoPath := "students/delete-service-target.webp"
	_, err = db.NewUpdate().TableExpr(`users.students AS "student"`).Set(`photo_path = ?`, photoPath).
		Where(`"student".id = ?`, target.ID).Exec(ctx)
	require.NoError(t, err)
	var legacyGuardianLinkID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.persons_guardians (tenant_id, person_id, guardian_account_id, relationship_type)
		VALUES (?, ?, ?, 'parent') RETURNING id`, target.TenantID, target.PersonID, legacyGuardianAccount.ID).Scan(ctx, &legacyGuardianLinkID))
	var removedPhoto string
	f.deps.PhotoRemoved = func(_ context.Context, _ studentdeletion.Actor, path string) { removedPhoto = path }
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, target.ID)
	require.NoError(t, err)
	require.Equal(t, 1, preview.Counts.TimetableAssignments)
	require.Equal(t, 1, preview.Counts.GuardianLinks)
	require.Equal(t, 3, preview.Counts.Communications)
	require.Equal(t, 1, preview.Counts.OtherRecords)
	require.Equal(t, "DeleteService Target", preview.ConfirmationName)

	result, err := workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.NoError(t, err)
	assert.Equal(t, preview.Counts, result.Counts)
	assert.Equal(t, photoPath, result.PhotoPath)
	assert.Equal(t, photoPath, removedPhoto, "the photo unlink is handed to the owner after the commit")
	assert.Equal(t, 1, result.PrimaryRowsDeleted)
	assert.EqualValues(t, 1, result.HistoryAnonymized)

	assert.Zero(t, rowCount(t, db, "users.students", target.ID))
	assert.Zero(t, rowCount(t, db, "schedule.instance_students", targetAssignment.ID))
	assert.Zero(t, rowCount(t, db, "users.persons_guardians", legacyGuardianLinkID))
	assert.Zero(t, rowCount(t, db, "users.parent_message_threads", messageThreadID))
	assert.Zero(t, rowCount(t, db, "users.parent_messages", messageID))
	var readCursorCount int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM users.parent_message_reads WHERE thread_id = ?`, messageThreadID).Scan(ctx, &readCursorCount))
	assert.Zero(t, readCursorCount)
	assert.Equal(t, 1, rowCount(t, db, "users.students", spared.ID))
	assert.Equal(t, 1, rowCount(t, db, "schedule.instance_students", sparedAssignment.ID))
	assert.Equal(t, 1, rowCount(t, db, "schedule.activity_instances", instance.ID))
	var historyName string
	require.NoError(t, db.NewRaw(`SELECT person_name FROM education.grade_transition_history WHERE id = ?`, history.ID).Scan(ctx, &historyName))
	assert.Equal(t, "Gelöschtes Kind", historyName, "the ledger row is retained and anonymized")

	var anonymized struct {
		FirstName string
		LastName  string
		Birthday  *timezone.Date
		TagID     *string
		AccountID *int64
		DeletedAt *time.Time
	}
	require.NoError(t, db.NewRaw(`SELECT first_name, last_name, birthday, tag_id, account_id, deleted_at FROM users.persons WHERE id = ?`, target.PersonID).Scan(ctx, &anonymized))
	assert.Equal(t, "Gelöscht", anonymized.FirstName)
	assert.Equal(t, "Benutzer", anonymized.LastName)
	assert.Nil(t, anonymized.Birthday)
	assert.Nil(t, anonymized.TagID)
	assert.Nil(t, anonymized.AccountID)
	assert.NotNil(t, anonymized.DeletedAt)
	assert.Equal(t, 1, rowCount(t, db, "auth.accounts", childAccount.ID), "credentials are unlinked, never deleted")
	assert.Equal(t, 1, rowCount(t, db, "auth.accounts", messageGuardianAccount.ID))
	assert.Equal(t, 1, rowCount(t, db, "auth.accounts_parents", legacyGuardianAccount.ID))

	var audit struct {
		StudentID      int64
		ActorAccountID int64
		Reason         string
		Counts         auditModels.StudentDeletionCounts
	}
	require.NoError(t, db.NewRaw(`SELECT student_id, actor_account_id, reason, counts FROM audit.student_deletions
		WHERE tenant_id = ? AND actor_account_id = ? ORDER BY id DESC LIMIT 1`, target.TenantID, f.actorID).Scan(ctx, &audit))
	assert.Equal(t, target.ID, audit.StudentID)
	assert.Equal(t, f.actorID, audit.ActorAccountID)
	assert.Equal(t, studentdeletion.ReasonTestData, audit.Reason)
	assert.Equal(t, auditModels.StudentDeletionCounts(preview.Counts), audit.Counts)

	var dataAudit auditModels.DataDeletion
	require.NoError(t, db.NewSelect().Model(&dataAudit).
		Where(`tenant_id = ? AND student_id = ? AND deletion_type = ?`, target.TenantID, target.ID, auditModels.DeletionTypeManual).Scan(ctx))
	assert.Equal(t, preview.Counts.Total()+1, dataAudit.RecordsDeleted)
	assert.Equal(t, studentdeletion.ReasonTestData, dataAudit.DeletionReason)
	assert.Equal(t, "account:"+fmt.Sprint(f.actorID), dataAudit.DeletedBy)
	assert.Equal(t, true, dataAudit.Metadata["student_deletion"])
	assert.NotNil(t, dataAudit.Metadata["counts"])

	// Idempotent retry: the confirmed request cannot delete twice and does
	// not resurrect anything.
	_, err = workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.ErrorIs(t, err, studentdeletion.ErrStudentNotFound)
	firstName, deletedAt := personSnapshot(t, db, target.PersonID)
	assert.Equal(t, "Gelöscht", firstName)
	assert.NotNil(t, deletedAt)
}

func TestStudentDeletionWorkflow_PreviewIncludesFeedbackAndPassesLockedRecheck(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	f.feedback.Count = 2
	student := testpkg.CreateTestStudent(t, db, "Feedback", "Delete", "1a")
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, student.ID)
	require.NoError(t, err)
	require.Equal(t, 2, preview.Counts.OtherRecords)

	result, err := workflow.Execute(ctx, student.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.NoError(t, err)
	assert.Equal(t, preview.Counts, result.Counts)
}

func TestStudentDeletionWorkflow_DeleteCountsCrossTenantVisits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	hostingTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, hostingTenantID)
	f := newDeletionFixture(t, db)
	target := testpkg.CreateTestStudent(t, db, "CrossTenant", "Visitor", "1a")
	hostingGroup := testpkg.CreateTestActiveGroupForTenant(t, db, hostingTenantID)
	hostedVisit := testpkg.CreateTestVisitForTenant(t, db, hostingTenantID, target.ID, hostingGroup.ID, time.Now(), nil)
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, preview.Counts.AttendanceRecords, "holiday-care visits hosted elsewhere count for the child")

	_, err = workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.NoError(t, err)
	assert.Zero(t, rowCount(t, db, "active.visits", hostedVisit.ID))
}

func TestStudentDeletionWorkflow_PreviewExcludesPreservedDeletionAudits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	target := testpkg.CreateTestStudent(t, db, "Preserved", "Audit", "1a")
	priorAudit := auditModels.NewDataDeletion(target.ID, auditModels.DeletionTypeVisitRetention, 1, "system")
	require.NoError(t, f.repos.DataDeletion.Create(ctx, priorAudit))
	var accessLogID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO audit.data_access_log (tenant_id, actor_account_id, actor_role, resource_type, student_id, range_start, range_end)
		VALUES (?, ?, 'admin', 'attendance_history', ?, NOW(), NOW()) RETURNING id`, target.TenantID, f.actorID, target.ID).Scan(ctx, &accessLogID))
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, target.ID)
	require.NoError(t, err)
	assert.Zero(t, preview.Counts.OtherRecords)
	assert.Zero(t, preview.Counts.Total())

	_, err = workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.NoError(t, err)

	assert.Equal(t, 1, rowCount(t, db, "audit.data_deletions", priorAudit.ID), "history is retained per policy")
	var deletionAudit auditModels.DataDeletion
	require.NoError(t, db.NewSelect().Model(&deletionAudit).
		Where(`tenant_id = ? AND student_id = ? AND deletion_type = ?`, target.TenantID, target.ID, auditModels.DeletionTypeManual).Scan(ctx))
	assert.Equal(t, 1, deletionAudit.RecordsDeleted)
	var detachedStudentID *int64
	require.NoError(t, db.NewRaw(`SELECT student_id FROM audit.data_access_log WHERE id = ?`, accessLogID).Scan(ctx, &detachedStudentID))
	assert.Nil(t, detachedStudentID)
}

func TestStudentDeletionWorkflow_RejectsStalePreview(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	workflow := f.workflow(t)

	t.Run("an assignment added after the preview", func(t *testing.T) {
		target := testpkg.CreateTestStudent(t, db, "DeleteStale", "Target", "1a")
		room := testpkg.CreateTestRoom(t, db, "delete-stale-room")
		instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
			Title: "Stale deletion preview instance", IsSpontaneous: true,
		})
		preview, err := workflow.Preview(ctx, target.ID)
		require.NoError(t, err)
		assignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, target.ID, "")

		_, err = workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonTestData))
		require.ErrorIs(t, err, studentdeletion.ErrPreviewChanged)
		assert.Equal(t, 1, rowCount(t, db, "users.students", target.ID))
		assert.Equal(t, 1, rowCount(t, db, "schedule.instance_students", assignment.ID))
	})

	t.Run("a message read cursor added after the preview", func(t *testing.T) {
		target := testpkg.CreateTestStudent(t, db, "DeleteStale", "Read", "1a")
		parentAccount := testpkg.CreateTestAccount(t, db, "student-delete-stale-read-parent@example.com")
		var threadID int64
		require.NoError(t, db.NewRaw(`INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id)
			VALUES (?, ?, ?) RETURNING id`, target.TenantID, target.ID, parentAccount.ID).Scan(ctx, &threadID))
		preview, err := workflow.Preview(ctx, target.ID)
		require.NoError(t, err)
		require.Equal(t, 1, preview.Counts.Communications)
		_, err = db.NewRaw(`INSERT INTO users.parent_message_reads (tenant_id, thread_id, account_id) VALUES (?, ?, ?)`,
			target.TenantID, threadID, f.actorID).Exec(ctx)
		require.NoError(t, err)

		_, err = workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonTestData))
		require.ErrorIs(t, err, studentdeletion.ErrPreviewChanged)
		assert.Equal(t, 1, rowCount(t, db, "users.students", target.ID))
	})

	t.Run("a person edit after the confirmed name", func(t *testing.T) {
		target := testpkg.CreateTestStudent(t, db, "DeleteStale", "Renamed", "1a")
		preview, err := workflow.Preview(ctx, target.ID)
		require.NoError(t, err)
		_, err = db.NewRaw(`UPDATE users.persons SET last_name = 'Umbenannt', updated_at = NOW() + INTERVAL '1 second' WHERE id = ?`, target.PersonID).Exec(ctx)
		require.NoError(t, err)

		_, err = workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonTestData))
		require.ErrorIs(t, err, studentdeletion.ErrPreviewChanged)
		assert.Equal(t, 1, rowCount(t, db, "users.students", target.ID))
		firstName, deletedAt := personSnapshot(t, db, target.PersonID)
		assert.Equal(t, "DeleteStale", firstName)
		assert.Nil(t, deletedAt)
	})

	t.Run("a child deleted concurrently", func(t *testing.T) {
		target := testpkg.CreateTestStudent(t, db, "DeleteStale", "Gone", "1a")
		preview, err := workflow.Preview(ctx, target.ID)
		require.NoError(t, err)
		_, err = workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonTestData))
		require.NoError(t, err)

		_, err = workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonTestData))
		require.ErrorIs(t, err, studentdeletion.ErrStudentNotFound)
		_, err = workflow.Preview(ctx, target.ID)
		require.ErrorIs(t, err, studentdeletion.ErrStudentNotFound)
	})
}

func TestStudentDeletionWorkflow_RejectsIncompleteConfirmation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	student := testpkg.CreateTestStudent(t, db, "Incomplete", "Confirmation", "1a")
	workflow := f.workflow(t)
	preview, err := workflow.Preview(ctx, student.ID)
	require.NoError(t, err)

	_, err = workflow.Execute(ctx, student.ID, studentdeletion.Confirmation{})
	require.ErrorIs(t, err, studentdeletion.ErrPreviewChanged)
	_, err = workflow.Execute(ctx, student.ID, studentdeletion.Confirmation{ExpectedFingerprint: "aabb"})
	require.ErrorIs(t, err, studentdeletion.ErrNotAcknowledged)
	_, err = workflow.Execute(ctx, student.ID, studentdeletion.Confirmation{ExpectedFingerprint: "aabb", Acknowledged: true, Reason: "other"})
	require.ErrorIs(t, err, studentdeletion.ErrInvalidReason)
	_, err = workflow.Execute(ctx, student.ID, studentdeletion.Confirmation{ExpectedFingerprint: "aabb", Acknowledged: true, Reason: studentdeletion.ReasonGraduatePurge})
	require.ErrorIs(t, err, studentdeletion.ErrInvalidReason, "the purge reason is reserved for the graduate route")
	wrongName := confirm(preview, studentdeletion.ReasonTestData)
	wrongName.ConfirmationName = "Wrong Name"
	_, err = workflow.Execute(ctx, student.ID, wrongName)
	require.ErrorIs(t, err, studentdeletion.ErrConfirmationMismatch)
	assert.Equal(t, 1, rowCount(t, db, "users.students", student.ID))
}

func TestStudentDeletionWorkflow_GraduatesUsePurgeNotDelete(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	workflow := f.workflow(t)
	graduate := func(id int64) {
		_, err := db.NewUpdate().TableExpr(`users.students AS "student"`).Set(`status = ?`, userModels.StudentStatusAlumnus).
			Where(`"student".id = ?`, id).Exec(ctx)
		require.NoError(t, err)
	}

	alumnus := testpkg.CreateTestStudent(t, db, "DeleteAlumnus", "Target", "1a")
	graduate(alumnus.ID)
	_, err := workflow.Preview(ctx, alumnus.ID)
	require.ErrorIs(t, err, studentdeletion.ErrAlumnus)

	// Graduated between the preview and the locked re-read: the ordinary
	// delete would destroy what a transition revert restores.
	late := testpkg.CreateTestStudent(t, db, "DeleteAlumnus", "Late", "1a")
	preview, err := workflow.Preview(ctx, late.ID)
	require.NoError(t, err)
	graduate(late.ID)
	_, err = workflow.Execute(ctx, late.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.ErrorIs(t, err, studentdeletion.ErrGraduatedUnderLock)
	assert.Equal(t, 1, rowCount(t, db, "users.students", late.ID))

	// The purge is the mirror image: it needs the alumnus state under lock.
	active := testpkg.CreateTestStudent(t, db, "PurgeActive", "Target", "1a")
	_, err = workflow.PurgeGraduate(ctx, active.ID)
	require.ErrorIs(t, err, studentdeletion.ErrNotGraduated)
	assert.Equal(t, 1, rowCount(t, db, "users.students", active.ID))

	transition := testpkg.CreateTestGradeTransition(t, db, "2027-2028", f.actorID)
	history := &educationModels.GradeTransitionHistory{
		TransitionID: transition.ID, StudentID: alumnus.ID, PersonName: "DeleteAlumnus Target",
		FromClass: "4a", Action: educationModels.ActionGraduated,
	}
	history.SetTenantID(alumnus.TenantID)
	_, err = db.NewInsert().Model(history).ModelTableExpr(`education.grade_transition_history`).Exec(ctx)
	require.NoError(t, err)
	result, err := workflow.PurgeGraduate(ctx, alumnus.ID)
	require.NoError(t, err)
	assert.Equal(t, studentdeletion.ReasonGraduatePurge, result.Reason)
	assert.Equal(t, 2, result.PrimaryRowsDeleted)
	assert.Zero(t, rowCount(t, db, "users.students", alumnus.ID))
	var historyName string
	require.NoError(t, db.NewRaw(`SELECT person_name FROM education.grade_transition_history WHERE id = ?`, history.ID).Scan(ctx, &historyName))
	assert.Equal(t, "Gelöschtes Kind", historyName)
	firstName, deletedAt := personSnapshot(t, db, alumnus.PersonID)
	assert.Equal(t, "Gelöscht", firstName)
	assert.NotNil(t, deletedAt)
	var dataAudit auditModels.DataDeletion
	require.NoError(t, db.NewSelect().Model(&dataAudit).
		Where(`tenant_id = ? AND student_id = ? AND deletion_type = ?`, alumnus.TenantID, alumnus.ID, auditModels.DeletionTypeManual).Scan(ctx))
	assert.Equal(t, studentdeletion.ReasonGraduatePurge, dataAudit.DeletionReason)
	assert.Equal(t, result.Counts.Total()+2, dataAudit.RecordsDeleted, "student and person on top of the dependent rows")
}

// The owner ports below run the real command and then fail, so every row the
// command wrote must roll back with the rest of the unit of work.
type failingTimetable struct {
	studentdeletion.Timetable
	fail error
}

func (f failingTimetable) DeleteStudentAssignments(ctx context.Context, id int64) (int64, error) {
	rows, err := f.Timetable.DeleteStudentAssignments(ctx, id)
	if err != nil {
		return rows, err
	}
	return rows, f.fail
}

type failingDirectory struct {
	studentdeletion.Directory
	phase string
	fail  error
}

func (f failingDirectory) DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (int64, error) {
	rows, err := f.Directory.DeleteLegacyGuardianLinks(ctx, personID)
	if err == nil && f.phase == "guardian-links" {
		return rows, f.fail
	}
	return rows, err
}

func (f failingDirectory) DeleteStudent(ctx context.Context, id int64) (int64, error) {
	rows, err := f.Directory.DeleteStudent(ctx, id)
	if err == nil && f.phase == "student" {
		return rows, f.fail
	}
	return rows, err
}

func (f failingDirectory) AnonymizeDeletedStudentPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error) {
	ok, err := f.Directory.AnonymizeDeletedStudentPerson(ctx, personID, updatedAt)
	if err == nil && f.phase == "person" {
		return ok, f.fail
	}
	return ok, err
}

type failingCarePlan struct {
	studentdeletion.CarePlan
	phase string
	fail  error
}

func (f failingCarePlan) QueueCareDocumentCleanupForDeletedStudent(ctx context.Context, id int64, at time.Time) (int, error) {
	n, err := f.CarePlan.QueueCareDocumentCleanupForDeletedStudent(ctx, id, at)
	if err == nil && f.phase == "documents" {
		return n, f.fail
	}
	return n, err
}

func (f failingCarePlan) RedactWithdrawalsForDeletedStudent(ctx context.Context, id, actor int64, at time.Time) (int, error) {
	n, err := f.CarePlan.RedactWithdrawalsForDeletedStudent(ctx, id, actor, at)
	if err == nil && f.phase == "withdrawals" {
		return n, f.fail
	}
	return n, err
}

type failingStructure struct {
	studentdeletion.Structure
	fail error
}

func (f failingStructure) AnonymizeStudentTransitionHistory(ctx context.Context, id int64) (int64, error) {
	rows, err := f.Structure.AnonymizeStudentTransitionHistory(ctx, id)
	if err != nil {
		return rows, err
	}
	return rows, f.fail
}

func TestStudentDeletionWorkflow_RollsBackAfterEachOwnerCommand(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"timetable", "guardian-links", "documents", "withdrawals", "student", "history", "person", "audit"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.Ctx(t)
			f := newDeletionFixture(t, db)
			target := testpkg.CreateTestStudent(t, db, "Rollback", "Target", "1a")
			room := testpkg.CreateTestRoom(t, db, "rollback-room-"+phase)
			instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
				Title: "Rollback deletion instance", IsSpontaneous: true,
			})
			assignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, target.ID, "")
			document := storeStudentDocument(t, ctx, f.repos.StudentDocument, target.ID, f.actorID,
				userModels.StudentDocumentCategorySonstiges, fmt.Sprintf("rollback-%s-%d.pdf", phase, target.ID))
			completion := createWithdrawalCompletion(t, db, target.ID, f.actorID, timezone.TodayDate())
			transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", f.actorID)
			history := &educationModels.GradeTransitionHistory{
				TransitionID: transition.ID, StudentID: target.ID, PersonName: "Rollback Target",
				FromClass: "1a", Action: educationModels.ActionPromoted,
			}
			history.SetTenantID(target.TenantID)
			_, err := db.NewInsert().Model(history).ModelTableExpr(`education.grade_transition_history`).Exec(ctx)
			require.NoError(t, err)
			legacyGuardian := testpkg.CreateTestParentAccount(t, db, "rollback-legacy-guardian@example.com")
			var legacyLinkID int64
			require.NoError(t, db.NewRaw(`INSERT INTO users.persons_guardians (tenant_id, person_id, guardian_account_id, relationship_type)
				VALUES (?, ?, ?, 'parent') RETURNING id`, target.TenantID, target.PersonID, legacyGuardian.ID).Scan(ctx, &legacyLinkID))

			failure := errors.New("injected " + phase + " failure")
			switch phase {
			case "timetable":
				f.deps.Timetable = failingTimetable{Timetable: f.deps.Timetable, fail: failure}
			case "guardian-links", "student", "person":
				f.deps.Directory = failingDirectory{Directory: f.deps.Directory, phase: phase, fail: failure}
			case "documents", "withdrawals":
				f.deps.CarePlan = failingCarePlan{CarePlan: f.deps.CarePlan, phase: phase, fail: failure}
			case "history":
				f.deps.Structure = failingStructure{Structure: f.deps.Structure, fail: failure}
			case "audit":
				f.deps.AppendAudit = func(context.Context, studentdeletion.Actor, studentdeletion.Result) error { return failure }
			}
			photoRemoved := false
			f.deps.PhotoRemoved = func(context.Context, studentdeletion.Actor, string) { photoRemoved = true }
			workflow := f.workflow(t)

			preview, err := workflow.Preview(ctx, target.ID)
			require.NoError(t, err)
			require.Equal(t, 1, preview.Counts.TimetableAssignments)
			require.Equal(t, 1, preview.Counts.GuardianLinks)
			result, err := workflow.Execute(ctx, target.ID, confirm(preview, studentdeletion.ReasonIncorrectEntry))
			require.ErrorIs(t, err, failure)
			require.Equal(t, studentdeletion.Result{}, result)
			assert.False(t, photoRemoved)

			assert.Equal(t, 1, rowCount(t, db, "users.students", target.ID), "%s: the student row survives", phase)
			assert.Equal(t, 1, rowCount(t, db, "schedule.instance_students", assignment.ID), "%s: the assignment survives", phase)
			assert.Equal(t, 1, rowCount(t, db, "users.persons_guardians", legacyLinkID), "%s: the legacy guardian link survives", phase)
			assert.Equal(t, 1, rowCount(t, db, "users.student_documents", document.ID), "%s: the document row survives", phase)
			queued, err := f.repos.StudentDocument.ListQueuedFileCleanupByOwnerID(ctx, target.ID)
			require.NoError(t, err)
			assert.Empty(t, queued, "%s: no cleanup intent may outlive the rollback", phase)
			pending, err := f.repos.CareWithdrawal.FindByID(ctx, completion.ID)
			require.NoError(t, err)
			assert.Equal(t, userModels.CareWithdrawalStatePending, pending.State, "%s: the withdrawal task stays pending", phase)
			var historyName string
			require.NoError(t, db.NewRaw(`SELECT person_name FROM education.grade_transition_history WHERE id = ?`, history.ID).Scan(ctx, &historyName))
			assert.Equal(t, "Rollback Target", historyName, "%s: the ledger name survives", phase)
			firstName, deletedAt := personSnapshot(t, db, target.PersonID)
			assert.Equal(t, "Rollback", firstName, "%s: the person survives", phase)
			assert.Nil(t, deletedAt)
			var audits int
			require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM audit.student_deletions WHERE tenant_id = ? AND student_id = ?`, target.TenantID, target.ID).Scan(ctx, &audits))
			assert.Zero(t, audits, "%s: no tombstone without a deletion", phase)

			// The same request succeeds once the owner recovers.
			f.deps = newDeletionFixture(t, db).deps
			f.deps.Authorize = workflowActor(f.actorID)
			retry := f.workflow(t)
			fresh, err := retry.Preview(ctx, target.ID)
			require.NoError(t, err)
			_, err = retry.Execute(ctx, target.ID, confirm(fresh, studentdeletion.ReasonIncorrectEntry))
			require.NoError(t, err)
			assert.Zero(t, rowCount(t, db, "users.students", target.ID))
		})
	}
}

func workflowActor(accountID int64) func(context.Context) (studentdeletion.Actor, error) {
	return func(ctx context.Context) (studentdeletion.Actor, error) {
		return studentdeletion.Actor{TenantID: tenant.FromContext(ctx), AccountID: accountID}, nil
	}
}

// The document rows carry a foreign key to the child and cascade away with
// them. The cleanup intents deliberately do not, which is what lets them
// outlive the cascade, but only if they are written before it, inside the same
// transaction: written afterwards, a crash strands the bytes; written
// beforehand and outside, a failed deletion leaves the scheduler about to
// delete a living child's documents.
func TestStudentDeletionWorkflow_QueuesDocumentCleanupInsideTheTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	suffix := time.Now().UnixNano()
	student := testpkg.CreateTestStudent(t, db, "Dokumente", fmt.Sprintf("Loeschung-%d", suffix), "1a")
	live := storeStudentDocument(t, ctx, f.repos.StudentDocument, student.ID, f.actorID,
		userModels.StudentDocumentCategoryBetreuungsvertrag, fmt.Sprintf("vertrag-%d.pdf", suffix))
	// Already soft-deleted, bytes not yet unlinked: still pending cleanup.
	deleted := storeStudentDocument(t, ctx, f.repos.StudentDocument, student.ID, f.actorID,
		userModels.StudentDocumentCategoryAttest, fmt.Sprintf("attest-%d.pdf", suffix))
	require.NoError(t, f.repos.StudentDocument.SoftDelete(ctx, deleted, f.actorID))
	// Bytes already gone: nothing left to reclaim, so no intent must appear.
	settled := storeStudentDocument(t, ctx, f.repos.StudentDocument, student.ID, f.actorID,
		userModels.StudentDocumentCategorySonstiges, fmt.Sprintf("erledigt-%d.pdf", suffix))
	require.NoError(t, f.repos.StudentDocument.MarkFileDeleted(ctx, settled.ID))
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, student.ID)
	require.NoError(t, err)
	result, err := workflow.Execute(ctx, student.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.NoError(t, err)
	assert.Equal(t, 2, result.DocumentCleanups)

	assert.Zero(t, rowCount(t, db, "users.students", student.ID))
	assert.Zero(t, rowCount(t, db, "users.student_documents", live.ID))
	assert.Zero(t, rowCount(t, db, "users.student_documents", deleted.ID))

	queued, err := f.repos.StudentDocument.ListQueuedFileCleanupByOwnerID(ctx, student.ID)
	require.NoError(t, err)
	names := make([]string, 0, len(queued))
	for _, cleanup := range queued {
		names = append(names, cleanup.FilenameStored)
		assert.False(t, cleanup.RetryAfter.After(time.Now()), "intents are eligible immediately; no upload can still be in flight")
	}
	assert.ElementsMatch(t, []string{live.FilenameStored, deleted.FilenameStored}, names)

	// A durable intent is retried, not duplicated: queueing the same object
	// again reactivates the row instead of adding a second one.
	_, err = f.repos.CarePlan().QueueCareDocumentCleanupForDeletedStudent(ctx, student.ID, time.Now())
	require.NoError(t, err)
	queued, err = f.repos.StudentDocument.ListQueuedFileCleanupByOwnerID(ctx, student.ID)
	require.NoError(t, err)
	assert.Len(t, queued, 2)
}

func TestStudentDeletionWorkflow_RedactsPendingWithdrawalOutsideCompletionFlow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	student := testpkg.CreateTestStudent(t, db, "Noah", "Direktloeschung", "3b")
	resolved := createWithdrawalCompletion(t, db, student.ID, f.actorID, timezone.TodayDate())
	resolvedAt := time.Now().Add(-2 * time.Hour)
	changed, err := f.repos.CareWithdrawal.MarkResolved(ctx, resolved.ID, f.actorID, resolvedAt)
	require.NoError(t, err)
	require.True(t, changed)
	obsolete := createWithdrawalCompletion(t, db, student.ID, f.actorID, timezone.TodayDate().AddDays(1))
	obsoleteAt := time.Now().Add(-time.Hour)
	changed, err = f.repos.CareWithdrawal.MarkObsoleteForRebooking(ctx, student.ID, timezone.TodayDate(), obsoleteAt)
	require.NoError(t, err)
	require.True(t, changed)
	pending := createWithdrawalCompletion(t, db, student.ID, f.actorID, timezone.TodayDate().AddDays(2))
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, student.ID)
	require.NoError(t, err)
	result, err := workflow.Execute(ctx, student.ID, confirm(preview, studentdeletion.ReasonPrivacyRequest))
	require.NoError(t, err)
	assert.Equal(t, 3, result.WithdrawalsRedacted)

	redacted, err := f.repos.CareWithdrawal.FindByID(ctx, pending.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.CareWithdrawalStateResolved, redacted.State)
	require.NotNil(t, redacted.Outcome)
	assert.Equal(t, userModels.CareWithdrawalOutcomeDeleted, *redacted.Outcome)
	assert.Nil(t, redacted.StudentID)
	assert.Nil(t, redacted.SourceRequestChildID)
	assert.Empty(t, redacted.SourceOfferings)

	redactedResolved, err := f.repos.CareWithdrawal.FindByID(ctx, resolved.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.CareWithdrawalStateResolved, redactedResolved.State)
	require.NotNil(t, redactedResolved.Outcome)
	assert.Equal(t, userModels.CareWithdrawalOutcomeCareEnded, *redactedResolved.Outcome)
	assert.Equal(t, resolvedAt.Unix(), redactedResolved.ResolvedAt.Unix())
	assert.Nil(t, redactedResolved.StudentID)

	redactedObsolete, err := f.repos.CareWithdrawal.FindByID(ctx, obsolete.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.CareWithdrawalStateObsolete, redactedObsolete.State)
	assert.Nil(t, redactedObsolete.Outcome)
	require.NotNil(t, redactedObsolete.ObsoleteReason)
	assert.Equal(t, userModels.CareWithdrawalObsoleteRebooked, *redactedObsolete.ObsoleteReason)
	assert.Equal(t, obsoleteAt.Unix(), redactedObsolete.ResolvedAt.Unix())
	assert.Nil(t, redactedObsolete.StudentID)
}

func TestStudentDeletionWorkflow_WithdrawalDeletesStudentAndRedactsCompletionAtomically(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	student := testpkg.CreateTestStudent(t, db, "Lina", "Loeschung", "2a")
	completion := createWithdrawalCompletion(t, db, student.ID, f.actorID, timezone.TodayDate())
	workflow := f.workflow(t)

	preview, err := workflow.PreviewWithdrawal(ctx, completion.ID)
	require.NoError(t, err)
	assert.Equal(t, student.ID, preview.StudentID)
	stale := confirm(preview, studentdeletion.ReasonPrivacyRequest)
	stale.ExpectedFingerprint = "stale"
	_, err = workflow.ExecuteWithdrawal(ctx, completion.ID, stale)
	require.ErrorIs(t, err, studentdeletion.ErrPreviewChanged)
	stillPending, err := f.repos.CareWithdrawal.FindByID(ctx, completion.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.CareWithdrawalStatePending, stillPending.State, "a refused deletion rolls the task resolution back too")
	assert.Equal(t, student.ID, *stillPending.StudentID)

	result, err := workflow.ExecuteWithdrawal(ctx, completion.ID, confirm(preview, studentdeletion.ReasonPrivacyRequest))
	require.NoError(t, err)
	assert.Equal(t, student.ID, result.StudentID)
	redacted, err := f.repos.CareWithdrawal.FindByID(ctx, completion.ID)
	require.NoError(t, err)
	assert.Equal(t, userModels.CareWithdrawalStateResolved, redacted.State)
	require.NotNil(t, redacted.Outcome)
	assert.Equal(t, userModels.CareWithdrawalOutcomeDeleted, *redacted.Outcome)
	assert.Nil(t, redacted.StudentID)
	assert.Nil(t, redacted.SourceAdjustmentID)
	assert.Nil(t, redacted.SourceRequestChildID)
	assert.Empty(t, redacted.SourceOfferings)

	_, err = workflow.PreviewWithdrawal(ctx, completion.ID)
	require.ErrorIs(t, err, studentdeletion.ErrWithdrawalAlreadyResolved)
	_, err = workflow.ExecuteWithdrawal(ctx, completion.ID, confirm(preview, studentdeletion.ReasonPrivacyRequest))
	require.ErrorIs(t, err, studentdeletion.ErrWithdrawalAlreadyResolved, "a retry after the commit is refused, never re-run")
	_, err = workflow.PreviewWithdrawal(ctx, completion.ID+9_000_000)
	require.ErrorIs(t, err, studentdeletion.ErrWithdrawalNotFound)
}

// The retention reason describes a record whose keeping period has run out.
// For a child still in care nothing is running out yet, so choosing it would
// put a false statement into the deletion audit (#2487).
func TestStudentDeletionWorkflow_RetentionReasonOnlyForEndedCare(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	student := testpkg.CreateTestStudent(t, db, "Nora", "Winter", "4a")
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, student.ID)
	require.NoError(t, err)
	_, err = workflow.Execute(ctx, student.ID, confirm(preview, studentdeletion.ReasonRetentionExpired))
	require.ErrorIs(t, err, studentdeletion.ErrRetentionNotEnded)
	assert.Equal(t, 1, rowCount(t, db, "users.students", student.ID), "the refused deletion left the child untouched")

	_, err = db.NewUpdate().TableExpr("users.students").Set("enrolled_until = ?", timezone.TodayDate().AddDays(-1)).
		Where("id = ?", student.ID).Exec(ctx)
	require.NoError(t, err)
	fresh, err := workflow.Preview(ctx, student.ID)
	require.NoError(t, err)
	assert.True(t, fresh.CareEnded)
	_, err = workflow.Execute(ctx, student.ID, confirm(fresh, studentdeletion.ReasonRetentionExpired))
	require.NoError(t, err)
}

func TestStudentDeletionWorkflow_AuthorizationAndTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	student := testpkg.CreateTestStudent(t, db, "Isolation", "Target", "1a")

	t.Run("an unauthorized principal never reaches an owner read", func(t *testing.T) {
		deps := f.deps
		deps.Authorize = func(context.Context) (studentdeletion.Actor, error) { return studentdeletion.Actor{}, nil }
		deps.Directory = refusingDirectory{t: t, Directory: deps.Directory}
		workflow, err := studentdeletion.New(deps)
		require.NoError(t, err)
		_, err = workflow.Preview(ctx, student.ID)
		require.ErrorIs(t, err, studentdeletion.ErrUnauthorized)
		_, err = workflow.Execute(ctx, student.ID, studentdeletion.Confirmation{ExpectedFingerprint: "aa", Acknowledged: true, Reason: studentdeletion.ReasonTestData})
		require.ErrorIs(t, err, studentdeletion.ErrUnauthorized)
		assert.Equal(t, 1, rowCount(t, db, "users.students", student.ID))
	})

	t.Run("the composed gate requires the tenant principal with users:delete", func(t *testing.T) {
		_, err := studentdeletioncompose.Authorize(ctx, func(context.Context) (bool, error) { return true, nil })
		require.ErrorIs(t, err, studentdeletion.ErrUnauthorized, "no principal in context")
	})

	t.Run("a child of another tenant is invisible", func(t *testing.T) {
		foreignTenantID := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, foreignTenantID)
		foreignCtx := tenant.WithTenantID(context.Background(), foreignTenantID)
		foreign := testpkg.CreateTestStudentForTenant(t, db, foreignTenantID, "Foreign", "Child", "2b")
		workflow := f.workflow(t)
		_, err := workflow.Preview(ctx, foreign.ID)
		require.ErrorIs(t, err, studentdeletion.ErrStudentNotFound)
		_, err = workflow.Execute(ctx, foreign.ID, studentdeletion.Confirmation{ExpectedFingerprint: "aa", Acknowledged: true, Reason: studentdeletion.ReasonTestData})
		require.ErrorIs(t, err, studentdeletion.ErrStudentNotFound)
		var count int
		require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM users.students WHERE id = ? AND tenant_id = ?`, foreign.ID, foreignTenantID).Scan(foreignCtx, &count))
		assert.Equal(t, 1, count)
	})
}

type refusingDirectory struct {
	t *testing.T
	studentdeletion.Directory
}

func (d refusingDirectory) ReadEnrollmentStudent(context.Context, int64, string) (peopledirectory.EnrollmentRecord, error) {
	d.t.Fatal("unauthorized request reached an owner query")
	return peopledirectory.EnrollmentRecord{}, nil
}

// Deleting a child drops every "läuft mit" edge, which edits the other
// child's record. The workflow therefore takes the companion graph locks in
// ascending order and refuses to strand a linked child before the cascade.
func TestStudentDeletionWorkflow_CompanionGraphLockAndStrandingCheck(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	service := newCompanionTestService(db)
	subject := testpkg.CreateTestStudent(t, db, "DeleteSubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "DeleteCompanion", "Companion", "1a")
	setAccompaniedDays(t, db, ctx, subject.ID, "mon")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon")
	conflicts, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
	})
	require.NoError(t, err)
	require.Empty(t, conflicts)
	clearCompanionNote(t, db, companion.ID)
	broadcasts := 0
	f.deps.CompanionsChanged = func(context.Context, studentdeletion.Actor, int64) { broadcasts++ }
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, subject.ID)
	require.NoError(t, err)
	require.Equal(t, 1, preview.Counts.CompanionLinks)

	// Without a note the companion's Monday plan is answered only by this
	// link; deleting the subject would strand them.
	_, err = workflow.Execute(ctx, subject.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.ErrorIs(t, err, studentdeletion.ErrCompanionWouldLoseDeparture)
	assert.Equal(t, 1, rowCount(t, db, "users.students", subject.ID))
	assert.Zero(t, broadcasts)

	// A concurrent holder of the far end's row blocks the ascending lock pass
	// instead of being skipped: the deletion waits, then times out, and the
	// rows stay untouched.
	setAccompaniedDays(t, db, ctx, companion.ID, "mon")
	holdStudentRowLock(t, db, companion.ID)
	lockCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	preview, err = workflow.Preview(ctx, subject.ID)
	require.NoError(t, err)
	_, err = workflow.Execute(lockCtx, subject.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.Error(t, err)
	assert.NotErrorIs(t, err, studentdeletion.ErrCompanionLockBusy, "the first pass waits in ascending order; NOWAIT is only for late lower ids")
	assert.Equal(t, 1, rowCount(t, db, "users.students", subject.ID))
}

func TestStudentDeletionWorkflow_RemovesCompanionEdgesAndNotifies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	service := newCompanionTestService(db)
	subject := testpkg.CreateTestStudent(t, db, "DeleteSubject", "Notified", "1a")
	companion := testpkg.CreateTestStudent(t, db, "DeleteCompanion", "Notified", "1a")
	setAccompaniedDays(t, db, ctx, subject.ID, "mon")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon")
	conflicts, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
	})
	require.NoError(t, err)
	require.Empty(t, conflicts)
	var notified []int64
	f.deps.CompanionsChanged = func(_ context.Context, _ studentdeletion.Actor, studentID int64) {
		notified = append(notified, studentID)
	}
	workflow := f.workflow(t)

	preview, err := workflow.Preview(ctx, subject.ID)
	require.NoError(t, err)
	result, err := workflow.Execute(ctx, subject.ID, confirm(preview, studentdeletion.ReasonTestData))
	require.NoError(t, err)
	assert.Equal(t, []int64{companion.ID}, result.CompanionIDs)
	assert.Equal(t, []int64{subject.ID}, notified)
	assert.Equal(t, 1, rowCount(t, db, "users.students", companion.ID))
	remaining, err := service.ListCompanions(ctx, companion.ID)
	require.NoError(t, err)
	assert.Empty(t, remaining, "the cascade removed the edge from the surviving child's card")
}

// Local cutover evidence for #2710, not a production observation or a new
// checkpoint: preview and execute latency, statement counts, unit-of-work
// outcomes and the deadlock counter over the real composition.
func TestStudentDeletionRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := newDeletionFixture(t, db)
	workflow := f.workflow(t)
	counter := testpkg.CaptureQueriesForContext(t, db)
	testpkg.AttachLockWaitEvidence(db)
	ctx, events := testpkg.CaptureUnitOfWorkEvidence(counter.Context(testpkg.Ctx(t)))
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &version))
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
		return count
	}
	beforeDeadlocks := deadlocks()
	samples := map[string][]testpkg.RuntimeCheckpointSample{}
	measure := func(operation string, iteration int, fn func() error) {
		counter.Reset()
		before := db.Stats()
		started := time.Now()
		err := fn()
		elapsed := time.Since(started)
		after := db.Stats()
		require.NoError(t, err)
		if iteration < 5 {
			return
		}
		writes := counter.WriteRows()
		rows, statements := counter.Rows()
		samples[operation] = append(samples[operation], testpkg.RuntimeCheckpointSample{
			DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(),
			WriteRowsAffected: &writes, RowsAffected: rows, StatementsWithRows: statements,
			PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
		})
	}
	room := testpkg.CreateTestRoom(t, db, "Deletion runtime")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "deletion-runtime-guardian@example.com")
	for iteration := range 35 {
		student := testpkg.CreateTestStudent(t, db, "Runtime", "Subject", "3a")
		instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate().AddDays(1), room.ID, testpkg.ActivityInstanceOpts{})
		testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, "")
		testpkg.CreateTestStudentGuardianLink(t, db, student.ID, guardian.ID, "parent")
		var preview studentdeletion.Preview
		measure("preview", iteration, func() error { var err error; preview, err = workflow.Preview(ctx, student.ID); return err })
		measure("execute", iteration, func() error {
			_, err := workflow.Execute(ctx, student.ID, confirm(preview, studentdeletion.ReasonTestData))
			return err
		})
	}
	raw, err := json.Marshal(map[string]any{"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1,
		"samples": samples, "unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks})
	require.NoError(t, err)
	t.Logf("student-deletion-runtime %s", raw)
}
